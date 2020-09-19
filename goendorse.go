package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	log "github.com/sirupsen/logrus"

	gotezos "github.com/goat-systems/go-tezos"
)

const (
	BAKER             string = "tz1MTZEJE7YH3wzo8YYiAGd8sgiCTxNRHczR"
	PRIORITY_LENGTH   int    = 2
	POW_HEADER_LENGTH int    = 4
	POW_LENGTH               = 4
)

var (
	Prefix_nonce []byte = []byte{69, 220, 169}

	gt     *gotezos.GoTezos
	wallet *gotezos.Wallet
)

func main() {

	log.SetLevel(log.DebugLevel)

	var err error
	gt, err = gotezos.New("127.0.0.1:18732")
	if err != nil {
		log.WithError(err).Fatal("could not connect to network")
	}

	// tz1MTZEJE7YH3wzo8YYiAGd8sgiCTxNRHczR
	pk := "edpkvEbxZAv15SAZAacMAwZxjXToBka4E49b3J1VNrM1qqy5iQfLUx"
	sk := "edsk3yXukqCQXjCnS4KRKEiotS7wRZPoKuimSJmWnfH2m3a2krJVdf"

	wallet, err = gotezos.ImportWallet(BAKER, pk, sk)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.WithField("Wallet", wallet.Sk).Info("Loaded Wallet")

	//log.Printf("Constants: PreservedCycles: %d, BlocksPerCycle: %d, BlocksPerRollSnapshot: %d",
	//	gt.Constants.PreservedCycles, gt.Constants.BlocksPerCycle, gt.Constants.BlocksPerRollSnapshot)

	newHeadNotifier := make(chan *gotezos.Block, 1)

	// this go func should loop forever, checking every 20s if a new block has appeared
	go func(cH chan<- *gotezos.Block) {

		curHead := &gotezos.Block{}

		ticker := time.NewTicker(20 * time.Second)

		for {

			// watch for new head block
			block, err := gt.Head()
			if err != nil {
				log.Println(err)
			}

			if block.Hash != curHead.Hash {

				// notify new block
				cH <- block

				curHead = block

				chainid := curHead.ChainID
				hash := curHead.Hash
				level := curHead.Metadata.Level.Level
				cycle := curHead.Metadata.Level.Cycle

				log.WithFields(log.Fields{
					"ChainID": chainid, "Hash": hash, "Level": level, "Cycle": cycle},
				).Info("New Block")
			}

			// wait here for timer
			select {
			case <-ticker.C:
				break
			}
		}

	}(newHeadNotifier)

	// loop forever, waiting for new blocks
	// when new block, check for endorsing rights
	for {

		ticker := time.NewTicker(10 * time.Second)

		select {
		case block := <-newHeadNotifier:

			handleEndorsement(*block)
			handleBake(*block)

		case <-ticker.C:
			log.Debug("tick...")
		}
	}
}

func handleEndorsement(blk gotezos.Block) {

	// look for endorsing rights at this level
	endoRightsFilter := gotezos.EndorsingRightsInput{
		BlockHash: blk.Hash,
		Level:     blk.Header.Level,
		Delegate:  BAKER,
	}

	rights, err := gt.EndorsingRights(endoRightsFilter)
	if err != nil {
		log.Println(err)
	}

	if len(*rights) == 0 {
		log.Info("No Endorsing Rights")
		return
	}

	// continue since we have at least 1 endorsing right
	for _, e := range *rights {
		log.WithField("Slots", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(e.Slots)), ","), "[]")).Info("Endorsing Rights")
	}

	// v2
	endorsementOperation := gotezos.ForgeOperationWithRPCInput{
		Blockhash: blk.Hash,
		Branch:    blk.Hash,
		Contents: []gotezos.Contents{
			gotezos.Contents{
				Kind:  "endorsement",
				Level: blk.Header.Level,
			},
		},
	}

	// v2
	endorsementBytes, err := gt.ForgeOperationWithRPC(endorsementOperation)
	if err != nil {
		log.WithError(err).Error("Error Forging Endorsement")
		return
	}
	//log.WithField("Bytes", endorsementBytes).Debug("FORGED ENDORSEMENT")

	// v2
	signedEndorsement, err := wallet.SignEndorsementOperation(endorsementBytes, blk.ChainID)
	if err != nil {
		log.WithError(err).Error("Could not sign endorsement bytes")
		return
	}

	//log.WithField("SignedOp", signedEndorsement.SignedOperation).Debug("SIGNED OP")
	//log.WithField("Signature", signedEndorsement.EDSig).Debug("SIGNED SIGNATURE")
	//log.WithField("DecodedSig", signedEndorsement.Signature).Debug("DECODED SIG")

	// V2
	preapplyEndoOp := gotezos.PreapplyOperationsInput{
		Blockhash: blk.Hash,
		Protocol:  blk.Protocol,
		Signature: signedEndorsement.EDSig,
		Contents:  endorsementOperation.Contents,
	}

	// Validate the operation against the node for any errors
	if _, err := gt.PreapplyOperations(preapplyEndoOp); err != nil {
		log.WithError(err).Error("Could not preapply operations")
		return
	}

	// v2
	injectionInput := gotezos.InjectionOperationInput{
		Operation: signedEndorsement.SignedOperation,
	}

	opHash, err := gt.InjectionOperation(injectionInput)
	if err != nil {
		log.WithError(err).Error("ERROR INJECTING")
	} else {
		log.WithField("Operation", opHash).Info("Injected Endorsement")
	}
}

func handleBake(blk gotezos.Block) {

	// TODO
	// error="failed to preapply new block: response returned code 400 with body Failed to
	// parse the request body: No case matched:\n  At /kind, unexpected string instead of
	// endorsement\n  Missing object field nonce

	// TODO
	// Check that we have not already baked this block
	// ie: implement internal watermark

	// look for baking rights at this level
	bakingRights := gotezos.BakingRightsInput{
		Level:     blk.Header.Level + 1,
		Delegate:  BAKER,
		BlockHash: blk.Hash,
	}

	rights, err := gt.BakingRights(bakingRights)
	if err != nil {
		log.Println(err)
	}

	if len(*rights) == 0 {
		log.Info("No Baking Rights")
		return
	}
	log.WithField("Rights", rights).Info("Rights")

	if len(*rights) > 1 {
		log.WithField("Rights", rights).Warn("Found more than 1 baking right!")
	}

	if (*rights)[0].Priority > 4 {
		log.Info("Priority > 4; Skipping...")
		return
	}

	// Determine if we need to calculate a nonce
	var nonceHash, seedHashHex string
	if blk.Header.Level+1%32 == 0 {
		log.Info("Nonce required")
		nonceHash, seedHashHex, err = generateNonce()
		if err != nil {
			log.WithError(err)
		}

		log.WithFields(log.Fields{
			"SeedHash": seedHashHex, "Nonce": nonceHash,
		}).Info("Generated Nonce")
	}

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

	// Retrieve mempool operations
	mempoolOps, err := gt.Mempool()
	if err != nil {
		log.WithError(err).Error("Failed to fetch mempool ops")
		return
	}

	// Parse/filter mempool operations into correct
	// operation slots for adding to the block
	operations, err := parseMempoolOperations(mempoolOps, blk.Hash, blk.Protocol)
	if err != nil {
		log.WithError(err).Error("Failed to sort mempool ops")
		return
	}

	priority := (*rights)[0].Priority

	pd := gotezos.ProtocolData{
		blk.Protocol,
		priority,
		"0000000000000000",
		"edsigtXomBKi5CTRf5cjATJWSyaRvhfYNHqSUGrn4SdbYRcGwQrUGjzEfQDTuqHhuA8b2d8NarZjz8TRf65WkpQmo423BtomS8Q",
		nonceHash,
	}

	preapplyBlockheader := gotezos.PreapplyBlockOperationInput{
		pd,                         // ProtocolData
		operations,                 // Operations
		true,                       // Sort
		(*rights)[0].EstimatedTime, // Timestamp
	}

	// Attempt to preapply the block header we created using the protocol data,
	// and operations pulled from mempool.
	//
	// If the initial preapply fails, attempt again using an empty list of operations
	//
	preapplyResp, err := func(bh gotezos.PreapplyBlockOperationInput) (gotezos.PreapplyResult, error) {

		var par gotezos.PreapplyResult
		var err error

		// Attempt with mempool operations
		par, err = gt.PreapplyBlockOperation(bh)
		if err == nil {
			return par, nil
		}

		// else
		log.WithError(err).Error("Preapply block with mempool operations failure. Attempting 0-op bake.")

		// Reset blockHeader Operations to 4 empty slices
		tempOps := make([][]interface{}, 4)
		for i, _ := range tempOps {
			tempOps[i] = make([]interface{}, 0)
		}
		bh.Operations = tempOps

		// Try again
		par, err = gt.PreapplyBlockOperation(bh)
		if err == nil {
			return par, nil
		}

		return par, errors.Wrap(err, "Preapply block with empty operations; fatal failure.")

	}(preapplyBlockheader)

	if err != nil {
		log.WithError(err).Error("Unable to preapply block")
		return
	}

	log.WithField("Resp", preapplyResp).Debug("PREAPPLY RESPONSE")

	// Re-filter the applied operations that came back from pre-apply
	operations = parsePreapplyOperations(preapplyResp.Operations)
	log.WithField("OPS", operations).Debug("PREAPPLY OPS")

	// Start constructing the actual block with info that comes back from the preapply
	shellHeader := preapplyResp.Shellheader

	// Actual protocol data will be added during the POW loop
	protocolData := createProtocolData(priority, "", "", "")
	log.WithField("ProtocolData", protocolData).Debug("PreApply-Shell")

	shellHeader.ProtocolData = protocolData
	log.WithField("S", shellHeader).Debug("SHELL HEADER")

	// Forge the block header
	forgedBlockHeaderRes, err := gt.ForgeBlockHeader(shellHeader)
	if err != nil {
		log.WithError(err).Error("Unable to forge block header")
		return
	}
	log.WithField("Forged", forgedBlockHeaderRes).Debug("FORGED HEADER")

	// Get just the forged block
	forgedBlock := forgedBlockHeaderRes.BlockHeader

	// The last 22 characters of forged block header we don't need
	forgedBlock = forgedBlock[:len(forgedBlock)-22]

	log.WithField("F", forgedBlock).Debug("FORGED SUB")

	// Perform a lame proof-of-work computation
	blockbytes, attempts, err := powLoop(forgedBlock, priority, seedHashHex)
	if err != nil {
		log.WithError(err).Error("Unable to POW!")
		return
	}

	// POW done
	log.WithFields(log.Fields{
		"Attempts": attempts, "Bytes": blockbytes,
	}).Info("Proof-of-Work Completed")

	// Take blockbytes and sign it
	signedBlock, err := wallet.SignBlock(blockbytes, blk.ChainID)
	if err != nil {
		log.WithError(err).Error("Could not sign block bytes")
		return
	}
	log.WithField("SB", signedBlock).Debug("SIGNED BYTES")

	// The data of the block
	ibi := gotezos.InjectionBlockInput{
		SignedBytes: signedBlock.SignedOperation,
		Operations:  operations,
	}

	resp, err := gt.InjectionBlock(ibi)
	if err != nil {
		log.WithError(err).Error("FAILED BLOCK INJECTION")
		return
	}

	log.WithField("BLOCK", stripQuote(string(resp))).Info("Injected Block")

	// Inject nonce, if required
	if seedHashHex != "" {
		revealNonce(blk, nonceHash, seedHashHex)
	}

}

func revealNonce(block gotezos.Block, nonceHash, seedHashHex string) {

	nonceRevealOperation := gotezos.ForgeOperationWithRPCInput{
		Blockhash: block.Hash,
		Branch:    block.Hash,
		Contents: []gotezos.Contents{
			gotezos.Contents{
				Kind:  "seed_nonce_revelation",
				Level: block.Header.Level,
				Nonce: seedHashHex,
			},
		},
	}

	// Forge nonce reveal operation
	forgedNonceRevealBytes, err := gt.ForgeOperationWithRPC(nonceRevealOperation)
	if err != nil {
		log.WithError(err).Error("Error Forging Nonce Reveal")
		return
	}
	log.WithField("Bytes", forgedNonceRevealBytes).Debug("Forged Reveal Nonce")

	// Nonce reveals have the same watermark as endorsements
	signedNonceReveal, err := wallet.SignEndorsementOperation(forgedNonceRevealBytes, block.ChainID)
	if err != nil {
		log.WithError(err).Error("Could not sign nonce reveal bytes")
		return
	}

	preapplyNonceRevealOp := gotezos.PreapplyOperationsInput{
		Blockhash: block.Hash,
		Protocol:  block.Protocol,
		Signature: signedNonceReveal.EDSig,
		Contents:  nonceRevealOperation.Contents,
	}

	// Validate the operation against the node for any errors
	if _, err := gt.PreapplyOperations(preapplyNonceRevealOp); err != nil {
		log.WithError(err).Error("Could not preapply nonce reveal operation")
		return
	}

	// Inject nonce reveal op
	injectionInput := gotezos.InjectionOperationInput{
		Operation: signedNonceReveal.SignedOperation,
	}

	opHash, err := gt.InjectionOperation(injectionInput)
	if err != nil {
		log.WithError(err).Error("ERROR INJECTING NONCE REVEAL")
	} else {
		log.WithField("OpHash", opHash).Info("Injected Nonce Reveal")
	}
}

func parsePreapplyOperations(ops []gotezos.ShellOperations) [][]interface{} {

	operations := make([][]interface{}, 4)
	for i, _ := range operations {
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

	newProtocolData := createProtocolData(priority, "00bc0303", "00000000", seed)

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
	powThreshold := gt.NetworkConstants.ProofOfWorkThreshold

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
		rr, err := cryptoGenericHash(hex.EncodeToString(hashBuffer))
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

func parseMempoolOperations(ops gotezos.Mempool, headHash, headProtocol string) ([][]interface{}, error) {

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
	// Init each slot to size 0 so that marshaling returns "[]" instead of nulll
	operations := make([][]interface{}, 4)
	for i, _ := range operations {
		operations[i] = make([]interface{}, 0)
	}

	for _, op := range ops.Applied {

		// TODO: Check to make sure we have not already added an op

		// Operation must match our head/branch
		if op.Branch != headHash {
			continue
		}

		// Determine the type of op to find out into which slot it goes
		var opSlot int = 3
		if len(op.Contents) == 1 {
			opSlot = func(opKind string) int {
				switch opKind {
				case "endorsement":
					return 0
				case "proposals", "ballot":
					return 1
				case "seed_nonce_revelation", "double_endorsement_evidence",
					"double_baking_evidence", "activate_account":
					return 2
				}
				return 3
			}(op.Contents[0].Kind)
		}

		// For now, skip transactions and other unknown operations
		if opSlot == 3 {
			continue
		}

		// Add operation to slot
		operations[opSlot] = append(operations[opSlot], struct {
			Protocol  string             `json:"protocol"`
			Branch    string             `json:"branch"`
			Contents  []gotezos.Contents `json:"contents"`
			Signature string             `json:"signature"`
		}{
			headProtocol,
			op.Branch,
			op.Contents,
			op.Signature,
		})

	}

	return operations, nil
}

func generateNonce() (string, string, error) {

	//  Testing:
	// 	  Seed:       e6d84e1e98a65b2f4551be3cf320f2cb2da38ab7925edb2452e90dd5d2eeeead
	// 	  Seed Buf:   230,216,78,30,152,166,91,47,69,81,190,60,243,32,242,203,45,163,138,183,146,94,219,36,82,233,13,213,210,238,238,173
	// 	  Seed Hash:  160,103,236,225,73,68,157,114,194,194,162,215,255,44,50,118,157,176,236,62,104,114,219,193,140,196,133,63,179,229,139,204
	// 	  Nonce Hash: nceVSbP3hcecWHY1dYoNUMfyB7gH9S7KbC4hEz3XZK5QCrc5DfFGm
	// 	  Seed Hex:   a067ece149449d72c2c2a2d7ff2c32769db0ec3e6872dbc18cc4853fb3e58bcc

	// Generate a hexadecimal seed from random bytes
	randBytes := make([]byte, 64)
	if _, err := rand.Read(randBytes); err != nil {
		log.WithError(err).Error("Unable to read random bytes")
		return "", "", err
	}
	seed := hex.EncodeToString(randBytes)[:64]

	seedHash, err := cryptoGenericHash(seed)
	if err != nil {
		log.WithError(err).Error("Unable to hash seed for nonce")
		return "", "", err
	}

	// B58 encode seed hash with nonce prefix
	nonceHash := gotezos.B58cencode(seedHash, Prefix_nonce)
	seedHashHex := hex.EncodeToString(seedHash)

	return nonceHash, seedHashHex, nil
}

func cryptoGenericHash(buffer string) ([]byte, error) {

	// Convert hex buffer to bytes
	bufferBytes, err := hex.DecodeString(buffer)
	if err != nil {
		return []byte{0}, errors.Wrap(err, "Unable to hex decode buffer bytes")
	}

	// Generic hash of 32 bytes
	bufferBytesHashGen, err := blake2b.New(32, []byte{})
	if err != nil {
		return []byte{0}, errors.Wrap(err, "Unable create blake2b hash object")
	}

	// Write buffer bytes to hash
	_, err = bufferBytesHashGen.Write(bufferBytes)
	if err != nil {
		return []byte{0}, errors.Wrap(err, "Unable write buffer bytes to hash function")
	}

	// Generate checksum of buffer bytes
	bufferHash := bufferBytesHashGen.Sum([]byte{})

	return bufferHash, nil
}

func stripQuote(s string) string {
	m := strings.TrimSpace(s)
	if len(m) > 0 && m[0] == '"' {
		m = m[1:]
	}
	if len(m) > 0 && m[len(m)-1] == '"' {
		m = m[:len(m)-1]
	}
	return m
}
