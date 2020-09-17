package main

import (
	"fmt"
	"time"
	"strings"
	"strconv"
	"encoding/hex"
	"crypto/rand"

	"golang.org/x/crypto/blake2b"
	"github.com/pkg/errors"
	
	log "github.com/sirupsen/logrus"
	
	gotezos "github.com/goat-systems/go-tezos"
)

const (
	BAKER string = "tz1MTZEJE7YH3wzo8YYiAGd8sgiCTxNRHczR"
	PRIORITY_LENGTH int = 2
	POW_HEADER_LENGTH int = 4
	POW_LENGTH =  4
)

var (
	Prefix_nonce []byte = []byte{69, 220, 169}
	
	gt *gotezos.GoTezos
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
	log.WithField("Wallet", wallet.Address).Info("Loaded Wallet")
	
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

		case <- ticker.C:
			log.Debug("tick...")
		}
	}
}


func handleBake(blk gotezos.Block) {

	// look for baking rights at this level
	bakingRights := gotezos.BakingRightsInput{
		Level: blk.Header.Level,
		Delegate: BAKER,
		BlockHash: blk.Hash,
	}

	rights, err := gt.BakingRights(bakingRights)
	if err != nil {
		log.Println(err)
	}
	
	if len(*rights) == 0 {
		log.Info("No Baking Rights")
		//return
	}

	if len(*rights) > 1 {
		log.WithField("Rights", rights).Error("Found more than 1 baking right")
	}

	// Determine if we need to calculate a nonce
	var nonceHash, seedHashHex string
	if blk.Header.Level + 1 % 32 == 0 {
		log.Info("Nonce required");
		nonceHash, seedHashHex, err = generateNonce("")
		if err != nil {
			log.WithError(err)
		}

		log.WithFields(log.Fields{
			"SeedHash": seedHashHex, "Nonce": nonceHash,
		}).Info("Generated Nonce")
	}

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

	// temp
	priority := 1

	pd := gotezos.ProtocolData{
            blk.Protocol,
            //(*rights)[0].Priority,
            priority,
            "0000000000000000",
			"edsigtXomBKi5CTRf5cjATJWSyaRvhfYNHqSUGrn4SdbYRcGwQrUGjzEfQDTuqHhuA8b2d8NarZjz8TRf65WkpQmo423BtomS8Q",
			nonceHash,
	}

	bh := gotezos.PreapplyBlockOperationInput{
		pd,
		operations,
		true,
		//(*rights)[0].EstimatedTime,
		time.Now().UTC().Add(2 * time.Minute),
	}

	//log.Debug(bh)
	
	preapplyResp, err := gt.PreapplyBlockOperation(bh)
	if err != nil {
		log.WithError(err).Error("Unable to preapply block")
		return
	}

	// With the returned preapply block result, execute a simple proof-of-work
	log.WithField("Resp", preapplyResp).Debug("PREAPPLY RESPONSE")

	// Start constructing the actual block with info that comes back
	// from the preapply
	shellHeader := preapplyResp.Shellheader
	protocolData, err := createProtocolData(priority, "", "", "")
	if err != nil {
		log.WithError(err).Error("Bad protocol data")
		return
	}
	log.WithField("ProtocolData", protocolData).Debug("PreApply-Shell")
	shellHeader.ProtocolData = protocolData
	
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
	forgedBlock = forgedBlock[:len(forgedBlock) - 22]

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
}


func stampcheck(buf []byte) int {
	v := 0
	for i := 0; i < 8; i++ {
		v = (v * 256) + int(buf[i])
	}
	return v
}


func powLoop(forgedBlock string, priority int, seed string) (string, int, error) {

	newProtocolData, _ := createProtocolData(priority, "00bc0303", "00000000", seed)
	
	log.WithField("newProtocolData", newProtocolData).Info("POW New Protocol Data")

	blockBytes := forgedBlock + newProtocolData + "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
	hashBuffer, _ := hex.DecodeString(blockBytes)

	protocolOffset := (len(forgedBlock) / 2) + PRIORITY_LENGTH + POW_HEADER_LENGTH

	attempts := 0
	powThreshold, _ := strconv.Atoi(gt.NetworkConstants.ProofOfWorkThreshold)

	for {
		attempts++

		for i := POW_LENGTH - 1; i >= 0; i-- {
			if hashBuffer[protocolOffset + i] == 255 {
				hashBuffer[protocolOffset + i] = 0
			} else {
				hashBuffer[protocolOffset + i]++
				break  // break out of for-loop
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
			mhex = mhex[:len(mhex) - 128]

			return mhex, attempts, nil
		}
		
		// Safety
		if attempts > 1e4 {
			return "", attempts, errors.Wrap(err, "POW exceeded safety limits")
		}
	}	

	return "", 0, nil
}


func createProtocolData(priority int, powHeader, pow, seed string) (string, error) {

// 	   if (typeof seed == "undefined") seed = "";
//     if (typeof pow == "undefined") pow = "";
//     if (typeof powHeader == "undefined") powHeader = "";
//     return priority.toString(16).padStart(4,"0") + 
//     powHeader.padEnd(8, "0") + 
//     pow.padEnd(8, "0") + 
//     (seed ? "ff" + seed.padEnd(64, "0") : "00") +
//     '';

	padEnd := func(s string, llen int) string {
		return s + strings.Repeat("0", llen - len(s))
	}

	newSeed := "00"
	if seed != "" {
		newSeed = "ff" + padEnd(seed, 64)
	}

	return fmt.Sprintf("%04x%s%s%s",
		priority,
		padEnd(powHeader, 8),
		padEnd(pow, 8),
		newSeed), nil
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
		operations[opSlot] = append(operations[opSlot], struct{
				Protocol	string `json:"protocol"`
				Branch		string `json:"branch"`
				Contents	[]gotezos.Contents `json:"contents"`
				Signature	string `json:"signature"`
			}{
				headProtocol,
				op.Branch,
				op.Contents,
				op.Signature,
			},
		)

	}

	return operations, nil
}


func generateNonce(seed string) (string, string, error) {

	//  Testing:
	// 	  Seed:       e6d84e1e98a65b2f4551be3cf320f2cb2da38ab7925edb2452e90dd5d2eeeead
	// 	  Seed Buf:   230,216,78,30,152,166,91,47,69,81,190,60,243,32,242,203,45,163,138,183,146,94,219,36,82,233,13,213,210,238,238,173
	// 	  Seed Hash:  160,103,236,225,73,68,157,114,194,194,162,215,255,44,50,118,157,176,236,62,104,114,219,193,140,196,133,63,179,229,139,204
	// 	  Nonce Hash: nceVSbP3hcecWHY1dYoNUMfyB7gH9S7KbC4hEz3XZK5QCrc5DfFGm
	// 	  Seed Hex:   a067ece149449d72c2c2a2d7ff2c32769db0ec3e6872dbc18cc4853fb3e58bcc

	if seed != "" {
		// Generate a hexadecimal seed from random bytes
		randBytes := make([]byte, 64)
		if _, err := rand.Read(randBytes); err != nil {
			log.WithError(err).Error("Unable to read random bytes")
			return "", "", err
		}
		seed = hex.EncodeToString(randBytes)[:64]
	}

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


func handleEndorsement(blk gotezos.Block) {

	// look for endorsing rights at this level
	endoRightsFilter := gotezos.EndorsingRightsInput{
		BlockHash: blk.Hash,
		Level: blk.Header.Level,
		Delegate: BAKER,
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
	endov2 := gotezos.ForgeOperationWithRPCInput{
		Blockhash: blk.Hash,
		Branch: blk.Hash,
		Contents: []gotezos.Contents{
			gotezos.Contents {
				Kind: "endorsement",
				Level: blk.Header.Level,
			},
		},
	}	
	
	// v2
	ebv2, err := gt.ForgeOperationWithRPC(endov2)
	if err != nil {
		log.Error("Error Forging Endorsement:", err)
	}
	log.WithField("Bytes", ebv2).Debug("FORGED ENDORSEMENT v2")

	// v2
	signedEndorsement, err := wallet.SignEndorsementOperation(ebv2, blk.ChainID)
	if err != nil {
		log.WithField("Message", err.Error()).Error("Could not sign endorsement bytes")
	}

	log.WithField("SignedOp", signedEndorsement.SignedOperation).Debug("SIGNED OP V2")
	log.WithField("Signature", signedEndorsement.EDSig).Debug("SIGNED SIGNATURE V2")
	log.WithField("DecodedSig", signedEndorsement.Signature).Debug("DECODED SIG V2")

	// V2
	preapplyEndoOp := gotezos.PreapplyOperationsInput{
		Blockhash: blk.Hash,
		Protocol: blk.Protocol,
		Signature: signedEndorsement.EDSig,
		Contents: endov2.Contents,
	}

	// Validate the operation against the node for any errors
	if _, err := gt.PreapplyOperations(preapplyEndoOp); err != nil {
		log.WithField("Message", err.Error()).Error("Could not preapply operations")
	}

	// v2
	injectionInput := gotezos.InjectionOperationInput{
		Operation: signedEndorsement.SignedOperation,
	}

	opHash, err := gt.InjectionOperation(injectionInput)
	if err != nil {
		log.WithField("Message", err.Error()).Error("ERROR INJECTING")
	} else {
		log.WithField("Operation", opHash).Info("Injected Endorsement")
	}
}


func cryptoGenericHash(buffer string) ([]byte, error) {

	// Convert hex buffer to bytes
	bufferBytes, err := hex.DecodeString(buffer)
	if err != nil {
		return []byte{0}, errors.Wrap(err, "Unable to hex decode buffer bytes")
	}
	fmt.Println(bufferBytes)
	log.WithField("BufferBytes", bufferBytes).Debug("Crypto Generic Hash")

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
