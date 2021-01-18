package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/goat-systems/go-tezos/v3/rpc"
	log "github.com/sirupsen/logrus"

	"goendorse/nonce"
	"goendorse/storage"
	"goendorse/util"
)

const (
	PRIORITY_LENGTH   int = 2
	POW_HEADER_LENGTH int = 4
	POW_LENGTH        int = 4
)

var (
	// Chain constants backup
	MinimumBlockTimes = map[string]int{
		"NetXdQprcVkpaWU": 60, // Mainnet
		"NetXjD3HPJJjmcd": 30, // Carthagenet
		"NetXm8tYqnMWky1": 30, // Delphinet
	}
)

func handleBake(ctx context.Context, wg *sync.WaitGroup, block rpc.Block, maxBakePriority int) {

	defer wg.Done()

	// Reference
	// https://gitlab.com/tezos/tezos/-/blob/mainnet-staging/src/proto_006_PsCARTHA/lib_delegate/client_baking_forge.ml

	// TODO
	// error="failed to preapply new block: response returned code 400 with body Failed to
	// parse the request body: No case matched:\n  At /kind, unexpected string instead of
	// endorsement\n  Missing object field nonce

	// TODO
	// Check that we have not already baked this block
	// ie: implement internal watermark

	// TODO
	//  error="failed to preapply new block: response returned code 500 with body
	//  "kind":"permanent", "id":"proto.006-PsCARTHA.baking.timestamp_too_early",
	//  "minimum":"2020-09-20T22:17:28Z","provided":"2020-09-20T22:16:58Z"

	//
	// Steps to making a block
	//
	// 1. Fetch operations from mempool (transactions, endorsements, reveals, etc)
	// 2. Parse and sort mempool operations into correct operation slots
	// 3. Construct the protocol data with dummy values, plus the operations and preapply
	// 4. Re-sort 'applied' operations from preapply
	// 5. Forge the block header using the shell, and sorted operations returned from preapply
	// 6. Execute proof-of-work
	// 7. Sign block header + POW
	// 8. Inject
	// 9. If needed, reveal nonce

	// look for baking rights for next level because that's what we will inject
	nextLevelToBake := block.Header.Level + 1

	// Check watermark to ensure we have not baked at this level before
	watermark := storage.DB.GetBakingWatermark()
	if watermark >= nextLevelToBake {
		log.WithFields(log.Fields{
			"BakingLevel": nextLevelToBake, "Watermark": watermark,
		}).Error("Watermark level higher than baking level; Cancel bake to prevent double baking")
		return
	}

	// Look for baking rights
	bakingRightsFilter := rpc.BakingRightsInput{
		BlockHash:   block.Hash,
		Level:       nextLevelToBake,
		Delegate:    (*bakerPkh),
		MaxPriority: maxBakePriority,
	}

	bakingRights, err := baconClient.Current.BakingRights(bakingRightsFilter)
	if err != nil {
		log.WithError(err).Error("Unable to fetch baking rights")
		return
	}

	// Got any rights?
	if len(*bakingRights) == 0 {
		log.WithFields(log.Fields{
			"Level": nextLevelToBake, "MaxPriority": maxBakePriority,
		}).Info("No baking rights for level")
		return
	}

	// Have rights. More than one?
	bakingRight := (*bakingRights)[0]
	if len(*bakingRights) > 1 {

		log.WithField("Rights", bakingRights).Warn("Found more than 1 baking right; Picking best priority.")

		// Sort baking rights based on lowest priority; You only get one opportunity
		for _, r := range *bakingRights {
			if r.Priority > bakingRight.Priority {
				bakingRight = r
			}
		}
	}

	priority := bakingRight.Priority
	timeBetweenBlocks := baconClient.Current.NetworkConstants.TimeBetweenBlocks[0]
	blocksPerCommitment := baconClient.Current.NetworkConstants.BlocksPerCommitment

	log.WithFields(log.Fields{
		"Priority":  priority,
		"Level":     nextLevelToBake,
		"CurrentTS": time.Now().UTC().Format(time.RFC3339),
	}).Info("Baking slot found")

	// Ignore baking rights of priority higher than what we care about
	if bakingRight.Priority > maxBakePriority {
		log.Infof("Priority higher than %d; Ignoring", maxBakePriority)
		return
	}

	// If the priority is > 0, we need to wait at least priority * minBlockTime.
	// This gives the bakers with priorities lower than us a chance to bake their right.
	//
	// While we wait, if they bake their slot, we will notice a change in /head and
	// will abort our processing.

	if priority > 0 {
		priorityDiffSeconds := time.Duration(timeBetweenBlocks * priority)
		log.Infof("Priority greater than 0; Sleeping for %ds", priorityDiffSeconds)

		select {
		case <-ctx.Done():
			log.Info("New block arrived; Canceling current bake")
			return
		case <-time.After(priorityDiffSeconds * time.Second):
			break
		}
	}

	// Determine if we need to calculate a nonce
	// It is our responsibility to create a nonce on specific levels (usually level % 32),
	// then reveal the seed used to create the nonce in the next cycle.
	var n nonce.Nonce
	if nextLevelToBake % blocksPerCommitment == 0 {

		n, err = generateNonce()
		if err != nil {
			log.WithError(err)
		}

		log.WithFields(log.Fields{
			"Nonce": n.NonceHash, "Seed": n.Seed,
		}).Info("Nonce required at this level")

		n.Level = nextLevelToBake
	}

	// Retrieve mempool operations
	// There's a minimum required number of endorsements at priority 0 which is 24,
	// so we will keep fetching from the mempool until we get at least 24, or
	// 1/2 block time elapses whichever comes first
	endMempool := time.Now().UTC().Add(time.Duration(timeBetweenBlocks / 2) * time.Second)
	endorsingPower := 0
	var operations [][]rpc.Operations

	mempoolInput := rpc.MempoolInput{
		Applied:       true,
		BranchDelayed: true,
	}

	for time.Now().UTC().Before(endMempool) && endorsingPower < 24 {

		// Sleep 5s to let mempool accumulate
		log.Infof("Sleeping 5s for more endorsements and ops")

		// Sleep, but also check if new block arrived
		select {
		case <-ctx.Done():
			log.Info("New block arrived; Canceling current bake")
			return
		case <-time.After(5 * time.Second):
			break
		}

		// Get mempool contents
		mempoolOps, err := baconClient.Current.Mempool(mempoolInput)
		if err != nil {
			log.WithError(err).Error("Failed to fetch mempool ops")
			return
		}

		// Parse/filter mempool operations into correct
		// operation slots for adding to the block
		operations, err = parseMempoolOperations(mempoolOps, block.Hash, block.Header.Level, block.Protocol)
		if err != nil {
			log.WithError(err).Error("Failed to sort mempool ops")
			return
		}

		log.Infof("Found %d endorsement operations in mempool", len(operations[0]))

		// compute_endorsing_power with current endorsements
		// Send all operations in the first slot, which are endorsements
		endorsingPower, err = computeEndorsingPower(block.ChainID, operations[0])
		if err != nil {
			log.WithError(err).Error("Unable to compute endorsing power; Using 0 power")
			endorsingPower = 0
		}
		log.WithField("EndorsingPower", endorsingPower).Debug("Computed Endorsing Power")
	}

	// Check if a new block has been posted to /head and we should abort
	select {
	case <-ctx.Done():
		log.Info("New block arrived; Canceling current bake")
		return
	default:
		break
	}

	// With endorsing power and priority, compute earliest timestamp to inject block
	nowTimestamp := time.Now().UTC().Round(time.Second)
	minimalInjectionTime, err := baconClient.Current.MinimalValidTime(endorsingPower, priority, block.ChainID)
	if err != nil {
		log.WithError(err).Error("Unable to get minimal valid timestamp")
		return
	}

	minimalInjectionTime = minimalInjectionTime.Add(1 * time.Second).Round(time.Second) // Just a 1s buffer
	log.WithFields(log.Fields{
		"MinimalTS": minimalInjectionTime.Format(time.RFC3339Nano), "CurrentTS": nowTimestamp.Format(time.RFC3339Nano),
	}).Debug("Minimal Injection Timestamp")

	// Need to sleep until minimal injection timestamp
	// TODO If we need to sleep, there could be some more endorsements in mempool to grab
	if nowTimestamp.Before(minimalInjectionTime) {
		sleepDuration := time.Duration(minimalInjectionTime.Sub(nowTimestamp).Seconds())
		log.Infof("Sleeping for %ds, based on endorsing power", sleepDuration)
		select {
		case <-ctx.Done():
			log.Info("New block arrived; Canceling current bake")
			return
		case <-time.After(sleepDuration * time.Second):
			break
		}
	}

	dummyProtocolData := rpc.ProtocolData{
		block.Protocol,
		priority,
		"0000000000000000",
		"edsigtXomBKi5CTRf5cjATJWSyaRvhfYNHqSUGrn4SdbYRcGwQrUGjzEfQDTuqHhuA8b2d8NarZjz8TRf65WkpQmo423BtomS8Q",
		n.NonceHash,
	}

	preapplyBlockheader := rpc.PreapplyBlockOperationInput{
		dummyProtocolData,    // ProtocolData
		operations,           // Operations
		true,                 // Sort
		minimalInjectionTime, // Timestamp
	}

	// Attempt to preapply the block header we created using the protocol data,
	// and operations pulled from mempool.
	//
	// If the initial preapply fails, attempt again using an empty list of operations
	//
	preapplyResp, err := baconClient.Current.PreapplyBlockOperation(preapplyBlockheader)
	if err != nil {
		log.WithError(err).WithField("Resp", preapplyResp).Error("Unable to preapply block")
		return
	}
	log.WithField("Resp", preapplyResp).Trace("Preapply Response")

	// Re-filter the applied operations that came back from pre-apply
	appliedOperations := parsePreapplyOperations(preapplyResp.Operations)
	log.WithField("Operations", appliedOperations).Trace("Preapply Operations")

	// Start constructing the actual block with info that comes back from the preapply
	shellHeader := preapplyResp.Shellheader

	// Actual protocol data will be added during the POW loop
	protocolData := createProtocolData(priority, "", "", "")
	log.WithField("ProtocolData", protocolData).Trace("Preapply Shell")

	shellHeader.ProtocolData = protocolData
	log.WithField("S", shellHeader).Trace("SHELL HEADER")

	// Forge the block header
	forgedBlockHeaderRes, err := baconClient.Current.ForgeBlockHeader(shellHeader)
	if err != nil {
		log.WithError(err).Error("Unable to forge block header")
		return
	}
	log.WithField("Forged", forgedBlockHeaderRes).Trace("Forged Header")

	// Get just the forged block header
	forgedBlock := forgedBlockHeaderRes.BlockHeader
	forgedBlock = forgedBlock[:len(forgedBlock)-22]

	// Perform a lame proof-of-work computation
	blockBytes, attempts, err := powLoop(forgedBlock, priority, n.SeedHashHex)
	if err != nil {
		log.WithError(err).Error("Unable to POW!")
		return
	}

	// POW done
	log.WithField("Attempts", attempts).Debug("Proof-of-Work Complete")
	log.WithField("Bytes", blockBytes).Trace("Proof-of-Work")

	// Take blockbytes and sign it with signer
	// TODO Attempt this more than once
	signedBlock, err := signerWallet.SignBlock(blockBytes, block.ChainID)
	if err != nil {
		log.WithError(err).Error("Signer block failure")
		return
	}
	log.WithField("Signature", signedBlock.EDSig).Debug("Block Signer Signature")

	// The data of the block
	ibi := rpc.InjectionBlockInput{
		SignedBytes: signedBlock.SignedOperation,
		Operations:  appliedOperations,
	}

	// Check if a new block has been posted to /head and we should abort
	select {
	case <-ctx.Done():
		log.Info("New block arrived; Canceling current bake")
		return
	default:
		break
	}

	// Dry-run check
	if *dryRunBake {
		log.Warn("Not Injecting Block; Dry-Run Mode")
		return
	}

	// Inject block
	resp, err := baconClient.Current.InjectionBlock(ibi)
	if err != nil {
		log.WithError(err).WithField("Extra", string(resp)).Error("Failed Block Injection")
		return
	}

	blockHash := util.StripQuote(string(resp))

	log.WithFields(log.Fields{
		"BlockHash": blockHash, "CurrentTS": time.Now().UTC().Format(time.RFC3339Nano),
	}).Info("Successfully injected block")

	// Save watermark to DB
	if err := storage.DB.RecordBakedBlock(nextLevelToBake, blockHash); err != nil {
		log.WithError(err).Error("Unable to save block; Watermark compromised")
	} else {
		log.Info("Saved injected block watermark")
	}

	// Save nonce to DB for reveal in next cycle
	if n.SeedHashHex != "" {
		storage.DB.SaveNonce(block.Metadata.Level.Cycle, n)
	}
}

func parsePreapplyOperations(ops []rpc.ShellOperations) [][]interface{} {

	operations := make([][]interface{}, 4)
	for i := range operations {
		operations[i] = make([]interface{}, 0)
	}

	for i, o := range ops {
		for _, oo := range o.Applied {
			operations[i] = append(operations[i], struct {
				Branch string `json:"branch"`
				Data   string `json:"data"`
			}{
				Branch: oo.Branch,
				Data:   oo.Data,
			})
		}
	}

	return operations
}

func stampcheck(buf []byte) uint64 {
	var value uint64 = 0
	for i := 0; i < 8; i++ {
		value = (value * 256) + uint64(buf[i])
	}
	return value
}

func powLoop(forgedBlock string, priority int, seed string) (string, int, error) {

	// powHeader = 00                 bc0303
	//             Protocol Revision  Git Hash
	// 42 42 31 30 = BB10 (BakinBacon v1.0)
	newProtocolData := createProtocolData(priority, "42423130", "00000000", seed)

	blockBytes := forgedBlock + newProtocolData
	hashBuffer, _ := hex.DecodeString(blockBytes + "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")

	protocolOffset := (len(forgedBlock) / 2) + PRIORITY_LENGTH + POW_HEADER_LENGTH

	// log.WithField("PDD", newProtocolData).Debug("PDD")
	// log.WithField("FB", forgedBlock).Debug("FORGED")
	// log.WithField("SH", seed).Debug("SEEDHEX")
	// log.WithField("BB", blockBytes).Debug("BLOCKBYTES")
	// log.WithField("HB", hashBuffer).Debug("HASHBUFFER")
	// log.WithField("PO", protocolOffset).Debug("OFFSET")

	attempts := 0
	powThreshold := baconClient.Current.NetworkConstants.ProofOfWorkThreshold

	for {
		attempts++

		for i := POW_LENGTH - 1; i >= 0; i-- {
			if hashBuffer[protocolOffset+i] == 255 {
				hashBuffer[protocolOffset+i] = 0
			} else {
				hashBuffer[protocolOffset+i]++
				break // break out of for-loop
			}
		}

		// check the hash after manipulating
		rr, err := util.CryptoGenericHash(hex.EncodeToString(hashBuffer), []byte{})
		if err != nil {
			return "", 0, errors.Wrap(err, "POW Unable to check hash")
		}

		// Did we reach our mark?
		if stampcheck(rr) <= powThreshold {
			mhex := hex.EncodeToString(hashBuffer)
			mhex = mhex[:len(mhex)-128]

			return mhex, attempts, nil
		}

		// Safety
		if attempts > 1e7 {
			return "", attempts, errors.New("POW exceeded safety limits")
		}
	}

	return "", 0, nil
}

func createProtocolData(priority int, powHeader, pow, seed string) string {

	// if (typeof seed == "undefined") seed = "";
	// if (typeof pow == "undefined") pow = "";
	// if (typeof powHeader == "undefined") powHeader = "";
	// return priority.toString(16).padStart(4,"0") +
	// powHeader.padEnd(8, "0") +
	// pow.padEnd(8, "0") +
	// (seed ? "ff" + seed.padEnd(64, "0") : "00") +
	// '';

	padEnd := func(s string, llen int) string {
		return s + strings.Repeat("0", llen-len(s))
	}

	newSeed := "00"
	if seed != "" {
		newSeed = "ff" + padEnd(seed, 64)
	}

	return fmt.Sprintf("%04x%s%s%s",
		priority,
		padEnd(powHeader, 8),
		padEnd(pow, 8),
		newSeed)
}

func parseMempoolOperations(ops rpc.Mempool, curBranch string, curLevel int, headProtocol string) ([][]rpc.Operations, error) {

	// 	for(var i = 0; i < r.applied.length; i++){
	// 		if (addedOps.indexOf(r.applied[i].hash) < 0) {
	// 			if (r.applied[i].branch != head.hash) continue;
	// 			if (badOps.indexOf(r.applied[i].hash) >= 0) continue;
	// 			if (operationPass(r.applied[i]) == 3) continue; //todo fee filter
	//
	// 			addedOps.push(r.applied[i].hash);
	//
	// 			operations[operationPass(r.applied[i])].push({
	// 				"protocol" : head.protocol,
	// 				"branch" : r.applied[i].branch,
	// 				"contents" : r.applied[i].contents,
	// 				"signature" : r.applied[i].signature,
	// 			});
	// 		}
	// 	}

	// 4 slots for operations to be sorted into
	// Init each slot to size 0 so that marshaling returns "[]" instead of null
	operations := make([][]rpc.Operations, 4)
	for i := range operations {
		operations[i] = make([]rpc.Operations, 0)
	}

	for _, op := range ops.Applied {

		// Determine the type of op to find out into which slot it goes
		var opSlot int = 3
		if len(op.Contents) == 1 {
			opSlot = func(branch string, opKind rpc.Kind, level int) int {
				switch opKind {
				case rpc.ENDORSEMENT:
					// Endorsements must match the current head block level and block hash
					if level != curLevel {
						return -1
					}
					if branch != curBranch {
						return -1
					}
					return 0
				case rpc.PROPOSALS, rpc.BALLOT:
					return 1
				case rpc.SEEDNONCEREVELATION, rpc.DOUBLEENDORSEMENTEVIDENCE,
					rpc.DOUBLEBAKINGEVIDENCE, rpc.ACTIVATEACCOUNT:
					return 2
				}
				return 3
			}(op.Branch, op.Contents[0].Kind, op.Contents[0].Level)
		}

		// For now, skip transactions and other unknown operations
		if opSlot == 3 {
			log.WithField("OP", op).Debug("Mempool Operation")
			continue
		}

		// Make sure any endorsements are for the current level
		if opSlot == -1 {
			continue
		}

		// Add operation to slot
		// operations[opSlot] = append(operations[opSlot], struct {
		// 	Protocol  string         `json:"protocol"`
		// 	Branch    string         `json:"branch"`
		// 	Contents  []rpc.Contents `json:"contents"`
		// 	Signature string         `json:"signature"`
		// }{
		// 	headProtocol,
		// 	op.Branch,
		// 	op.Contents,
		// 	op.Signature,
		// })

		operations[opSlot] = append(operations[opSlot], rpc.Operations{
			Protocol:  headProtocol,
			Branch:    op.Branch,
			Contents:  op.Contents,
			Signature: op.Signature,
		})

	}

	return operations, nil
}

func computeEndorsingPower(chainID string, operations []rpc.Operations) (int, error) {

	// https://blog.nomadic-labs.com/emmy-an-improved-consensus-algorithm.html
	// >>>>149: http://172.17.0.5:8732/chains/NetXjD3HPJJjmcd/blocks/BLfXz42dc2.../endorsing_power
	// { "endorsement_operation":
	//       { "branch": "BLfXz42dc2JyHUkHpp7gErVP5djwrtcwsxXy8hvt7dPHy5jBqrM",
	//         "contents": [ { "kind": "endorsement", "level": 43256 } ],
	//         "signature": "sigQPBM4f4aCZWHXeJz7g2gEDP...."
	//        },
	//   "chain_id": "NetXjD3HPJJjmcd"
	//  }
	// <<<<149: 200 OK
	// 2

	var endorsingPower int

	for _, o := range operations {

		endorsementOperation := rpc.EndorsingPowerInput{ // block.go
			o,
			chainID,
		}

		ep, err := baconClient.Current.GetEndorsingPower(endorsementOperation) // block.go
		if err != nil {
			log.WithError(err).WithField("Op", o).Error("Unable to compute endorsing power")
			ep = 0
		}
		endorsingPower += ep
	}

	return endorsingPower, nil
}
