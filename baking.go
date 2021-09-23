package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/bakingbacon/go-tezos/v4/forge"
	"github.com/bakingbacon/go-tezos/v4/rpc"
	log "github.com/sirupsen/logrus"

	"bakinbacon/baconsigner"
	"bakinbacon/nonce"
	"bakinbacon/notifications"
	"bakinbacon/util"
)

const (
	ProtocolBb10    string = "42423130"
	MaxBakePriority int    = 4

	PriorityLength  int = 2
	PowHeaderLength int = 4
	PowLength       int = 4
)

// TODO refactor into a few smaller methods
func (s *BakinBaconServer) handleBake(ctx context.Context, wg *sync.WaitGroup, block rpc.Block) {

	// Decrement waitGroup on exit
	defer wg.Done()

	// Handle panic gracefully
	defer func() {
		if r := recover(); r != nil {
			log.WithField("Message", r).Error("Panic recovered in handleBake")
		}
	}()

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
	//  error="failed to pre-apply new block: response returned code 500 with body
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
	watermark, err := s.GetBakingWatermark()
	if err != nil {
		// watermark = 0 on DB error
		log.WithError(err).Error("Unable to get baking watermark from DB")
	}

	if watermark >= nextLevelToBake {
		log.WithFields(log.Fields{
			"BakingLevel": nextLevelToBake, "Watermark": watermark,
		}).Error("Watermark level higher than baking level; Cancel bake to prevent double baking")

		return
	}

	// Look for baking rights
	hashBlockID := rpc.BlockIDHash(block.Hash)
	bakingRightsFilter := rpc.BakingRightsInput{
		BlockID:     &hashBlockID,
		Level:       nextLevelToBake,
		Delegate:    s.Signer.BakerPkh,
		MaxPriority: MaxBakePriority,
	}

	resp, bakingRights, err := s.Current.BakingRights(bakingRightsFilter)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"Request": resp.Request.URL, "Response": resp.Body(),
		}).Error("Unable to fetch baking rights")

		return
	}

	// Got any rights?
	if len(bakingRights) == 0 {
		log.WithFields(log.Fields{
			"Level": nextLevelToBake, "MaxPriority": MaxBakePriority,
		}).Info("No baking rights for level")

		return
	}

	// Have rights. More than one?
	bakingRight := bakingRights[0]
	if len(bakingRights) > 1 {

		log.WithField("Rights", bakingRights).Warn("Found more than 1 baking right; Picking best priority.")

		// Sort baking rights based on lowest priority; You only get one opportunity
		for _, r := range bakingRights {
			if r.Priority < bakingRight.Priority {
				bakingRight = r
			}
		}
	}

	priority := bakingRight.Priority
	timeBetweenBlocks := util.NetworkConstants[s.networkName].TimeBetweenBlocks
	blocksPerCommitment := util.NetworkConstants[s.networkName].BlocksPerCommitment

	log.WithFields(log.Fields{
		"Priority":  priority,
		"Level":     nextLevelToBake,
		"CurrentTS": time.Now().UTC().Format(time.RFC3339),
	}).Info("Baking slot found")

	// Ignore baking rights of priority higher than what we care about
	if bakingRight.Priority > MaxBakePriority {
		log.Infof("Priority higher than %d; Ignoring", MaxBakePriority)
		return
	}

	// Check if we have enough bond to cover the bake
	requiredBond := util.NetworkConstants[s.networkName].BlockSecurityDeposit

	if spendableBalance, err := s.GetSpendableBalance(); err != nil {
		log.WithError(err).Error("Unable to get spendable balance")

		// Even if error here, we can still proceed.
		// Might have enough to post bond, might not.

	} else {

		// If not enough bond, exit early
		if requiredBond > spendableBalance {
			msg := "Bond balance too low for baking"
			log.WithFields(log.Fields{
				"Spendable": spendableBalance, "ReqBond": requiredBond,
			}).Error(msg)

			s.Status.SetError(errors.New(msg))
			s.Send(msg, notifications.BALANCE)

			return
		}
	}

	// If the priority is > 0, we need to wait at least priority * minBlockTime.
	// This gives the bakers with priorities lower than us a chance to bake their right.
	//
	// While we wait, if they bake their slot, we will notice a change in /head and
	// will abort our processing.

	if priority > 0 {
		priorityDiff := time.Duration(timeBetweenBlocks * priority)
		log.Infof("Priority greater than 0; Sleeping for %ds", priorityDiff)

		select {
		case <-ctx.Done():
			log.Info("New block arrived; Canceling current bake")
			return
		case <-time.After(priorityDiff * time.Second):
			break
		}
	}

	// Determine if we need to calculate a nonce
	// It is our responsibility to create a nonce on specific levels (usually level % 32),
	// then reveal the seed used to create the nonce in the next cycle.
	var n nonce.Nonce
	if nextLevelToBake % blocksPerCommitment == 0 {

		n, err = s.generateNonce()
		if err != nil {
			log.WithError(err)
		}

		log.WithFields(log.Fields{
			"NONCE": n.EncodedNonce, "Seed": n.Seed,
		}).Info("NONCE required at this level")

		n.Level = nextLevelToBake
	}

	// Retrieve mempool operations
	// There's a minimum required number of endorsements at priority 0 which is 192,
	// so we will keep fetching from the mempool until we get at least 192, or
	// 1/2 block time elapses whichever comes first
	endMempool := time.Now().UTC().Add(time.Duration(timeBetweenBlocks / 2) * time.Second)
	minEndorsingPower := util.NetworkConstants[s.networkName].InitialEndorsers
	endorsingPower := 0

	var operations [][]rpc.Operations

	mempoolInput := rpc.MempoolInput{
		Applied:       true,
		BranchDelayed: true,
	}

	for time.Now().UTC().Before(endMempool) && endorsingPower < minEndorsingPower {

		// Sleep 10s to let mempool accumulate
		log.Infof("Sleeping 10s for more endorsements and ops")

		// Sleep, but also check if new block arrived
		select {
		case <-ctx.Done():
			log.Info("New block arrived; Canceling current bake")
			return
		case <-time.After(10 * time.Second):
			break
		}

		// Get mempool contents
		_, mempoolOps, err := s.Current.Mempool(mempoolInput)
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
		endorsingPower, err = s.computeEndorsingPower(&hashBlockID, block.Header.Level, operations[0])
		if err != nil {
			log.WithError(err).Error("Unable to compute endorsing power; Using 90% of minimum power")

			endorsingPower = int(float32(minEndorsingPower) * 0.90)
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

	// TODO Remove minimal_valid_time by calculating ourselves
	// Ref: https://gitlab.com/tezos/tezos/-/blob/master/src/proto_010_PtGRANAD/lib_protocol/baking.ml#L451
	// As long as we have at least 192 endorsing power, we can submit bake at 30s
	// Timestamp of previous block + 30s = minimal timestamp

	// With endorsing power and priority, compute earliest timestamp to inject block
	resp, minimalInjectionTime, err := s.Current.MinimalValidTime(rpc.MinimalValidTimeInput{
		BlockID:        &hashBlockID,
		Priority:       priority,
		EndorsingPower: endorsingPower,
	})
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"Request": resp.Request.URL, "Response": string(resp.Body()),
		}).Error("Unable to get minimal valid timestamp")
		return
	}

	now := time.Now().UTC().Round(time.Second)
	minimalInjectionTime = minimalInjectionTime.Add(1 * time.Second).Round(time.Second) // Just a 1s buffer

	log.WithFields(log.Fields{
		"MinimalTS": minimalInjectionTime.Format(time.RFC3339Nano), "CurrentTS": now.Format(time.RFC3339Nano),
	}).Debug("Minimal Injection Timestamp")

	// Need to sleep until minimal injection timestamp
	// TODO If we need to sleep, there could be some more endorsements in mempool to grab
	if now.Before(minimalInjectionTime) {
		sleepDuration := time.Duration(minimalInjectionTime.Sub(now).Seconds())
		log.Infof("Sleeping for %ds, based on endorsing power", sleepDuration)
		select {
		case <-ctx.Done():
			log.Info("New block arrived; Canceling current bake")
			return
		case <-time.After(sleepDuration * time.Second):
			break
		}
	}

	dummyProtocolData := rpc.PreapplyBlockProtocolData{
		Protocol:            block.Protocol,
		Priority:            priority,
		ProofOfWorkNonce:    "0000000000000000",
		SeedNonceHash:       n.EncodedNonce,
		LiquidityEscapeVote: false,
		Signature:           "edsigtXomBKi5CTRf5cjATJWSyaRvhfYNHqSUGrn4SdbYRcGwQrUGjzEfQDTuqHhuA8b2d8NarZjz8TRf65WkpQmo423BtomS8Q",
	}

	preapplyBlockheader := rpc.PreapplyBlockInput{
		BlockID: &hashBlockID, // BlockID
		Block: rpc.PreapplyBlockBody{
			ProtocolData: dummyProtocolData, // ProtocolData
			Operations:   operations,        // Operations
		},
		Sort:      true,                  // Sort
		Timestamp: &minimalInjectionTime, // Timestamp
	}

	// Attempt to preapply the block header we created using the protocol data,
	// and operations pulled from mempool.
	//
	// If the initial preapply fails, attempt again using an empty list of operations
	//
	resp, preapplyBlockResp, err := s.Current.PreapplyBlock(preapplyBlockheader)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"Request": resp.Request.URL, "Response": string(resp.Body()),
		}).Error("Unable to pre-apply block")

		return
	}

	log.WithField("Resp", preapplyBlockResp).Trace("Pre-apply Response")

	// Re-filter the applied operations that came back from pre-apply
	appliedOperations := parsePreapplyOperations(preapplyBlockResp.Operations)
	log.WithField("Operations", appliedOperations).Trace("Pre-apply Operations")

	// Start constructing the actual block with info that comes back from the pre-apply
	shellHeader := preapplyBlockResp.ShellHeader

	// Protocol data (commit hash, proof-of-work nonce, seed, liquidity vote)
	protocolData := createProtocolData(priority, n.NoPrefixNonce)
	log.WithField("ProtocolData", protocolData).Debug("Generated Protocol Data")

	// Forge the block header using RPC
// 	resp, forgedBlockHeader, err := bc.Current.ForgeBlockHeader(rpc.ForgeBlockHeaderInput{
// 		BlockID: &hashBlockID,
// 		BlockHeader: rpc.ForgeBlockHeaderBody{
// 			Level:          shellHeader.Level,
// 			Proto:          shellHeader.Proto,
// 			Predecessor:    shellHeader.Predecessor,
// 			Timestamp:      shellHeader.Timestamp,
// 			ValidationPass: shellHeader.ValidationPass,
// 			OperationsHash: shellHeader.OperationsHash,
// 			Fitness:        shellHeader.Fitness,
// 			Context:        shellHeader.Context,
// 			ProtocolData:   protocolData,
// 		},
// 	})
// 	if err != nil {
// 		log.WithError(err).WithFields(log.Fields{
// 			"Request": resp.Request.URL, "Response": string(resp.Body()),
// 		}).Error("Unable to forge block header")
// 		return
// 	}
//
// 	log.WithField("Forged", forgedBlockHeader).Trace("Forged Header (RPC)")

	//
	// Forge block header locally
	locallyForgedBlock, err := forge.ForgeBlockShell(rpc.ForgeBlockHeaderBody{
		Level:          shellHeader.Level,
		Proto:          shellHeader.Proto,
		Predecessor:    shellHeader.Predecessor,
		Timestamp:      shellHeader.Timestamp,
		ValidationPass: shellHeader.ValidationPass,
		OperationsHash: shellHeader.OperationsHash,
		Fitness:        shellHeader.Fitness,
		Context:        shellHeader.Context,
		ProtocolData:   protocolData,
	})
	if err != nil {
		log.WithError(err).Error("Unable to locally forge block header")
	}

	localForgedBlockHex := hex.EncodeToString(locallyForgedBlock)
	log.WithField("Local", localForgedBlockHex).Info("Locally Forged Block")

	// forged block header includes protocol_data and proof-of-work placeholder bytes
	// protocol_data can sometimes contain seed_nonce_hash, so send the offset to powLoop
	//forgedBlock := forgedBlockHeader.Block
	protocolDataLength := len(protocolData)

	// Perform a lame proof-of-work computation
	blockBytes, attempts, err := s.powLoop(localForgedBlockHex, protocolDataLength)
	if err != nil {
		log.WithError(err).Error("Unable to POW!")
		return
	}

	// POW done
	log.WithFields(log.Fields{
		"Bytes": blockBytes, "Attempts": attempts,
	}).Trace("Proof-of-Work Complete")

	// Attempt to sign twice, short sleep in-between
	var signedBlock baconsigner.SignOperationOutput
	var signedErr error

	for i := 1; i < 3; i++ {
		signedBlock, signedErr = s.Signer.SignBlock(blockBytes, block.ChainID)
		if err != nil {
			log.WithField("Attempt", i).WithError(err).Error("Failed to sign block")
			time.Sleep(1 * time.Second)
			continue
		}
		break // Break loop; No error; Success sign
	}

	if signedErr != nil {
		msg := "Unable to sign block bytes; Cannot inject block"
		log.Error(msg)
		s.Send(msg, notifications.BAKING_FAIL)
		return
	}

	log.WithField("Signature", signedBlock.EDSig).Debug("Signed New Block")

	// The data of the block
	ibi := rpc.InjectionBlockInput{
		SignedBlock: signedBlock.SignedOperation,
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
	if s.dryRunBake {
		log.Warn("Not Injecting Block; Dry-Run Mode")
		return
	}

	// Inject block
	resp, blockHash, err := s.Current.InjectionBlock(ibi)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"Request": resp.Request.URL, "Response": string(resp.Body()),
		}).Error("Block Injection Failure")
		return
	}

	log.WithFields(log.Fields{
		"BlockHash": blockHash, "CurrentTS": time.Now().UTC().Format(time.RFC3339Nano),
	}).Info("Block Injected")

	// Save watermark to DB
	if err := s.RecordBakedBlock(nextLevelToBake, blockHash); err != nil {
		log.WithError(err).Error("Unable to save block; Watermark compromised")
	}

	// Save nonce to DB for reveal in next cycle
	withNonce := ""
	if n.EncodedNonce != "" {
		if err := s.SaveNonce(block.Metadata.Level.Cycle, n); err != nil {
			log.WithError(err).Error("Unable to save nonce for reveal")
		}
		withNonce = ", with nonce"
	}

	// Update status for UI
	s.Status.SetRecentBake(nextLevelToBake, block.Metadata.Level.Cycle, blockHash)

	// Send notification
	s.Send(fmt.Sprintf("Bakin'Bacon baked block %d%s!", nextLevelToBake, withNonce), notifications.BAKING_OK)
}

func parsePreapplyOperations(ops []rpc.PreappliedBlockOperations) [][]interface{} {

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

func stampCheck(buf []byte) uint64 {
	var value uint64 = 0
	for i := 0; i < 8; i++ {
		value = (value * 256) + uint64(buf[i])
	}

	return value
}

func (s *BakinBaconServer) powLoop(forgedBlock string, protocolDataLength int) (string, int, error) {
	// The hash buffer is the byte-decoded forged block, including shell and protocol data.
	// Protocol data should include a 64 byte signature but at this point, we have not
	// signed anything because we need to sign the proof-of-work result which is generated below.
	//
	// Since we can't sign something that we have not created, we append a dummy signature of
	// all 0's so that the checksum of the entire block with PoW can be correctly compared
	// against the networkName constant's proof of work threshold

	hashBuffer, _ := hex.DecodeString(forgedBlock + strings.Repeat("0", 128))
	protocolOffset := ((len(forgedBlock) - protocolDataLength) / 2) + PriorityLength + PowHeaderLength
	powThreshold := util.NetworkConstants[s.networkName].ProofOfWorkThreshold

	// log.WithField("FB", forgedBlock).Debug("FORGED")
	// log.WithField("HB", hashBuffer).Debug("HASHBUFFER")
	// log.WithField("PO", protocolOffset).Debug("OFFSET")

	attempts := 0
	for ; attempts < 1e7; attempts++ {

		for i := PowLength - 1; i >= 0; i-- {
			if hashBuffer[protocolOffset+i] == 255 {
				hashBuffer[protocolOffset+i] = 0
			} else {
				hashBuffer[protocolOffset+i]++
				break // break out of for-loop
			}
		}

		// check the hash after manipulating
		rr, err := util.CryptoGenericHash(hashBuffer, []byte{})
		if err != nil {
			return "", 0, errors.Wrap(err, "POW Unable to check hash")
		}

		// Did we reach our mark?
		if stampCheck(rr) <= powThreshold {
			mhex := hex.EncodeToString(hashBuffer)
			mhex = mhex[:len(mhex)-128]

			return mhex, attempts, nil
		}
	}

	// Exited loop due to safety limit of 1e7 attempts
	return "", attempts, errors.New("POW exceeded safety limits")
}

// Create the `protocol_data` component of the block header (shell)
// https://tezos.gitlab.io/shell/p2p_api.html#block-header-alpha-specific
func createProtocolData(priority int, nonceHex string) string {

	// nonceHex is the hex-encoded, prefix-stripped representation of the
	// crypto-hashed random seed bytes

	// Helper function for padding 0s
	padEnd := func(s string, llen int) string {
		return s + strings.Repeat("0", llen-len(s))
	}

	// If no seed_nonce_hash, set 00 (false)
	// Otherwise, set ff (true) and append nonce hash
	newNonce := "00"
	if len(nonceHex) > 0 {
		newNonce = "ff" + padEnd(nonceHex, 64)
	}

	return fmt.Sprintf("%04x%s%s%s%s",
		priority,                // 2-byte priority
		padEnd(ProtocolBb10, 8), // 4-byte commit hash
		padEnd("0", 8),          // 4-byte proof of work
		newNonce,                // nonce presence flag + nonce
		"00")                     // 1-byte LB escape vote
}

func parseMempoolOperations(ops *rpc.Mempool, curBranch string, curLevel int, headProtocol string) ([][]rpc.Operations, error) {

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

	// Determine the type of each applied operation to find out into which slot it goes
	for _, op := range ops.Applied {

		// Default slot
		var opSlot int = 3

		// If there's more than one, probably a transfer which we don't handle yet
		if len(op.Contents) == 1 {

			opSlot = func(branch string, opContent rpc.Content) int {

				switch opContent.Kind {
				case rpc.ENDORSEMENT_WITH_SLOT:

					endorsement := op.Contents[0].Endorsement

					// Endorsements must match the current head block level and block hash
					if endorsement.Operations.Level != curLevel {
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
			}(op.Branch, op.Contents[0])
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

func (s *BakinBaconServer) computeEndorsingPower(blockId rpc.BlockID, bakingLevel int, operations []rpc.Operations) (int, error) {

	// Endorsing power is just the total number of endorsing slots for a delegate.
	// We fetch endorsing rights for this level, validate entry, increment power.

	var endorsingPower int

	// Get endorsing rights for this level
	endorsingRightsInput := rpc.EndorsingRightsInput{
		BlockID: blockId,
		Level:   bakingLevel,
	}
	resp, endorsingRights, err := s.Current.EndorsingRights(endorsingRightsInput)
	if err != nil {
		return endorsingPower, err
	}

	log.WithFields(log.Fields{
		"Level": bakingLevel, "Request": resp.Request.URL, "Response": string(resp.Body()),
	}).Trace("Fetched block endorsing rights")

	// Convert endorsing rights to map for faster searching
	rightsMap := make(map[int]int, 256)

	for _, r := range endorsingRights {
		k := r.Slots[0]
		v := len(r.Slots)
		rightsMap[k] = v
	}

	// For each mempool endorsement operation, search for
	// endorsing rights to calculate total number of slots
	for _, o := range operations {

		for _, c := range o.Contents {
			slot := c.Slot // lowest slot for this delegate

			// Find slot in endorsing rights of delegate.
			// Add length of total number of slots array to endorsing power.
			endorsingPower += rightsMap[slot]
		}
	}

	return endorsingPower, nil
}
