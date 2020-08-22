package main

import (
	"fmt"
	"time"
	"strings"
	"strconv"
	"encoding/json"
	"encoding/hex"
	"crypto/rand"

	"golang.org/x/crypto/blake2b"
	
	log "github.com/sirupsen/logrus"
	gotezos "github.com/DefinitelyNotAGoat/go-tezos/gt"
	gtb "github.com/DefinitelyNotAGoat/go-tezos/block"
	gtw "github.com/DefinitelyNotAGoat/go-tezos/account"
	gto "github.com/DefinitelyNotAGoat/go-tezos/operations"
	gtc "github.com/DefinitelyNotAGoat/go-tezos/crypto"
	
	goat "github.com/goat-systems/go-tezos"
)

const (
	BAKER string = "tz1MTZEJE7YH3wzo8YYiAGd8sgiCTxNRHczR"
)

var (
	gt *gotezos.GoTezos
	wallet gtw.Wallet
	
	Prefix_nonce []byte = []byte{69, 220, 169}
	
	gtv2 *goat.GoTezos
	walletv2 *goat.Wallet
)

type Block struct {
	Level int `json:"level"`
	Proto int `json:"proto"`
	Predecessor string `json:"predecessor"`
	Timestamp time.Time `json:"timestamp"`
	ValidationPass int `json:"validation_pass"`
	OperationsHash string `json:"operations_hash"`
	Fitness []string `json:"fitness"`
	Context string `json:"context"`
	Priority int `json:"priority"`
	PoWNonce string `json:"proof_of_work_nonce"`
}

func main() {

	log.SetLevel(log.DebugLevel)

	var err error
	gt, err = gotezos.NewGoTezos("127.0.0.1:18732")
	if err != nil {
		log.WithError(err).Fatal("could not connect to network")
	}
	
	// v2
	gtv2, _ = goat.New("127.0.0.1:18732")
	
// 	thirtyfour, err := gotezos.NewGoTezos("http://34.65.191.139:8732")
// 	if err != nil {
// 		log.Printf("could not connect to network: %v", err)
// 	}
	
	// tz1MTZEJE7YH3wzo8YYiAGd8sgiCTxNRHczR
	pk := "edpkvEbxZAv15SAZAacMAwZxjXToBka4E49b3J1VNrM1qqy5iQfLUx"
	sk := "edsk3yXukqCQXjCnS4KRKEiotS7wRZPoKuimSJmWnfH2m3a2krJVdf"
	
	wallet, err = gt.Account.ImportWallet(BAKER, pk, sk)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.WithField("Wallet", wallet.Address).Info("Loaded Wallet")
	
	// v2
	walletv2, _ = goat.ImportWallet(BAKER, pk, sk)
	
	//log.Printf("Constants: PreservedCycles: %d, BlocksPerCycle: %d, BlocksPerRollSnapshot: %d",
	//	gt.Constants.PreservedCycles, gt.Constants.BlocksPerCycle, gt.Constants.BlocksPerRollSnapshot)
	
	newHeadNotifier := make(chan gtb.Block, 1)

	// this go func should loop forever, checking every 20s if a new block has appeared
	go func(cH chan<- gtb.Block) {

		var curHead gtb.Block
		
		ticker := time.NewTicker(20 * time.Second)
		
		for {
		
			// watch for new head block
			block, err := gt.Block.GetHead()
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
		
			handleEndorsement(block)
			//handleBake(block)

		case <- ticker.C:
			log.Debug("tick...")
		}
	}
}

func handleBake(blk gtb.Block) {
// 
// 	// look for baking rights at this level
// 	rights, err := gt.Delegate.GetBakingRightsForDelegateAtHashLevel(blk.Hash, blk.Header.Level, BAKER)
// 	if err != nil {
// 		log.Println(err)
// 	}
// 	
// 	if len(rights) == 0 {
// 		log.Info("No Baking Rights")
// 		return
// 	}
// 	
// 	if len(rights) > 1 {
// 		log.WithField("Rights", rights).Error("Found more than 1 baking right")
// 	}
// 	
// 	// Determine if we need to calculate a nonce
// 	if blk.Header.Level + 1 % 32 == 0 {
// 		log.Info("Nonce required");
// 		nonceHash, seedHashHex, err := generateNonce()
// 		if err != nil {
// 			log.WithError(err)
// 		}
// 
// 		log.WithFields(log.Fields{
// 			"SeedHash": seedHashHex, "Nonce": nonceHash,
// 		}).Info("Generated Nonce")
// 	}
// 
// 	// Retrieve mempool operations
// 	
// 
// 	operations, err := parseMempoolOperations(mempoolOps)
// 	if err != nil {
// 		log.WithError(err).Error("Failed to parse mempool ops")
// 	}
// 	
// 	return
// 
// 	// current head contains information we need to construct our block
// 	myLevel := blk.Header.Level + 1
// 	myTs := blk.Header.Timestamp.Add(time.Second * 50)
// 	myFitness := improveFitness(blk.Header.Fitness)
// 	myPoWNonce := improvePoWNonce(blk.Header.ProofOfWorkNonce)
// 	//myOperations := make([][]gtb.Operations, 4)
// 	
// 	// create block
// 	block := Block{
// 		Level: myLevel,
// 		Proto: blk.Header.Proto,
// 		Predecessor: blk.Hash,
// 		Timestamp: myTs,
// 		ValidationPass: 4,
// 		Fitness: myFitness,
// 		OperationsHash: "LLoanPsQQELqiHWt9dTFBprDqKrgoS5XdxoRApuui1LpCTK3hFp8w",
// 		Context: blk.Header.Context,
// 		Priority: 0,
// 		PoWNonce: myPoWNonce,
// 	}
// 	
// 	b, e := json.Marshal(block);
// 	if e != nil {
// 		log.Error("Error marshaling block:", e)
// 	}
// 	log.WithField("JSON", string(b)).Info("Block")
// 
// 	// convert JSON operation into Tezos bytes
// 	blockBytes, err := gt.Block.ForgeBlockHeader(string(b))
// 	if err != nil {
// 		log.WithError(err).Error("Error Forging Block")
// 		return
// 	}
// 	log.WithField("Bytes", string(blockBytes)).Debug("FORGED BLOCK")
}


func parseMempoolOperations(ops []interface{}, headHash, headProtocol string) ([][]interface{}, error) {

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
	operations := make([][]interface{}, 4)

// 	for _, op := range ops {
// 
// 		// TODO: Check to make sure we have not already added an op
// 
// 		// Operation must match our head/branch
// 		if op.branch != headHash {
// 			continue
// 		}
// 		
// 		// Determine the type of op to find out into which slot it goes
// 		opSlot := func(opKind string) int {
// 			switch opKind {
// 			case "endorsement":
// 				return 0
// 			case "proposals":
// 			case "ballot":
// 				return 1
// 			case "seed_nonce_revelation":
// 			case "double_endorsement_evidence":
// 			case "double_baking_evidence":
// 			case "activate_account":
// 				return 2
// 			default:
// 				return 3
// 		}(op.contents[0].kind)
// 
// 		operations[opSlot] = append(operations[opSlot], []struct{
// 				Protocol	string `json:"protocol"`
// 				Branch		string `json:"branch"`
// 				Contents	[]gtb.Contents `json:"contents"`
// 				Signature	string `json:"signature"`
// 			}{
// 				{
// 					headProtocol,
// 					op.branch,
// 					op.contents,
// 					op.signature,
// 				},
// 			}
// 		)
// 
// 	}

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

	// Convert random hex seed to bytes
	seedBytes, err := hex.DecodeString(seed)
	if err != nil {
		log.WithError(err).Error("Unable to hex decode seed bytes")
		return "", "", err
	}

	// Generic hash of 32 bytes
	seedHashGen, err := blake2b.New(32, []byte{})
	if err != nil {
		log.WithError(err).Error("Unable create blake2b hash object")
		return "", "", err
	}
	
	// Write seed bytes to hash
	_, err = seedHashGen.Write(seedBytes)
	if err != nil {
		log.WithError(err).Error("Unable write nonce seedBytes to hash")
		return "", "", err
	}

	// Generate checksum of seed
	seedHash := seedHashGen.Sum([]byte{})

	// B58 encode seed hash with nonce prefix
	nonceHash := gtc.B58cencode(seedHash, Prefix_nonce)
	seedHashHex := hex.EncodeToString(seedHash)

	return nonceHash, seedHashHex, nil
}


func improvePoWNonce(oldNonce string) string {
	nonce, _ := strconv.ParseInt(oldNonce, 16, 64)
	nonce++
	return fmt.Sprintf("%016x", nonce)
}

func improveFitness(oldFitness []string) []string {

	newFitness := make([]string, 2)

	// copy first element
	// TODO: what does first element mean?
	newFitness[0] = oldFitness[0]
	
	// Convert and increment second element
	fitness, _ := strconv.ParseInt(oldFitness[1], 16, 0)
	
	fitness++
	
	newFitness[1] = fmt.Sprintf("%016x", fitness)
	
	return newFitness
}

func handleEndorsement(blk gtb.Block) {

	// look for endorsing rights at this level
	rights, err := gt.Delegate.GetEndorsingRightsForDelegateAtHashLevel(blk.Hash, blk.Header.Level, BAKER)
	if err != nil {
		log.Println(err)
	}
	
	if len(rights) == 0 {
		log.Info("No Endorsing Rights")
		//return
	}
	
	// continue since we have at least 1 endorsing right
	for _, e := range rights {
		log.WithField("Slots", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(e.Slots)), ","), "[]")).Info("Endorsing Rights")
	}

	// create endorsement operation
	endo := gto.Conts{
		Branch: blk.Hash,
		Contents: []gtb.Contents{
			gtb.Contents {
				Kind: "endorsement",
				Level: blk.Header.Level,
			},
		},
	}
	
	s, e := json.Marshal(endo);
	if e != nil {
		log.Error("Error marshaling:", e)
	}
	//log.WithField("S", string(s)).Debug("v1")

	// v2
	endov2 := goat.ForgeOperationWithRPCInput{
		Blockhash: blk.Hash,
		Branch: blk.Hash,
		Contents: []goat.Contents{
			goat.Contents {
				Kind: "endorsement",
				Level: blk.Header.Level,
			},
		},
	}
	//log.WithField("V2", endov2).Debug("v2")


	// v1 convert JSON operation into Tezos bytes
	endorsementBytes, err := gt.Operation.ForgeOperationBytes(string(s))
	if err != nil {
		log.Error("Error Forging Endorsement:", err)
	}
	log.WithField("Bytes", string(endorsementBytes)).Debug("FORGED ENDORSEMENT")
	
	
	// v2
	ebv2, err := gtv2.ForgeOperationWithRPC(endov2)
	if err != nil {
		log.Error("Error Forging Endorsement:", err)
	}
	log.WithField("Bytes", ebv2).Debug("FORGED ENDORSEMENT v2")

	// v1 Sign forged endorsement bytes with the secret key and chain id; return that signature
	edsig, err := gt.Operation.SignEndorsementBytes(endorsementBytes, blk.ChainID, wallet)
	if err != nil {
		log.WithField("Message", err.Error()).Error("Could not sign endorsement bytes")
	}
	log.WithField("Signature", edsig).Debug("SIGNED SIGNATURE")

	// v2
	signedEndorsement, err := walletv2.SignEndorsementOperation(ebv2, blk.ChainID)
	if err != nil {
		log.WithField("Message", err.Error()).Error("Could not sign endorsement bytes")
	}
	log.WithField("Signature", signedEndorsement.EDSig).Debug("SIGNED SIGNATURE v2")

	// Extract and decode the bytes of the signature
	decodedSignature, _, err := gt.Operation.DecodeSignature(edsig)
	if err != nil {
		log.WithField("Message", err.Error()).Fatal("Could not decode signature")
	}
	
	// Strip off first 10 chars of the signature which are a watermark
	decodedSignature = decodedSignature[10:(len(decodedSignature))]
	log.WithField("DecodedSig", decodedSignature).Debug("DECODED SIG")
	log.WithField("DecodedSig", signedEndorsement.Signature).Debug("DECODED SIG V2")

	// preapply operation
	op := []struct{
		Protocol	string `json:"protocol"`
		Branch		string `json:"branch"`
		Contents	[]gtb.Contents `json:"contents"`
		Signature	string `json:"signature"`
	}{
		{
			blk.Protocol,
			blk.Hash,
			endo.Contents,
			edsig,
		},
	}
	
	opJson, _ := json.Marshal(op)
	log.WithField("OP", string(opJson)).Debug("PREAPPLY")

	// We can validate gt batch against the node for any errors
	err = gt.Operation.PreApplyOperations(string(opJson))
	if err != nil {
		log.WithField("Message", err.Error()).Error("Could not preapply operations")
	}


	// V2
	preapplyEndoOp := goat.PreapplyOperationsInput{
		Blockhash: blk.Hash,
		Protocol: blk.Protocol,
		Signature: signedEndorsement.EDSig,
		Contents: endov2.Contents,
	}

	// We can validate the operation against the node for any errors
	_, err = gtv2.PreapplyOperations(preapplyEndoOp)
	if err != nil {
		log.WithField("Message", err.Error()).Error("Could not preapply operations")
	}
	//log.WithField("FinalOp", finalOperations).Debug("PREAPPLY v2")



	// The signed bytes of the entire endorsement operation
	// endorsement operation + signature(endorsment watermark (0x02) + chain id + endorsement operation)
	// chain id = strip off the chainId prefix, then base58 decode
	fullOperation := endorsementBytes + decodedSignature
	
	log.WithField("Operation", fullOperation).Debug("FULL OPERATION")
	log.WithField("Operation", signedEndorsement.SignedOperation).Debug("FULL OPERATION V2")

// 	operation, err := gt.Operation.InjectOperation(fullOperation)
// 	if err != nil {
// 		log.WithField("Message", err.Error()).Error("ERROR INJECTING")
// 	} else {
// 		log.WithField("Operation", stripQuote(string(operation))).Info("Injected Endorsement")
// 	}

	// v2
	injectionInput := goat.InjectionOperationInput{
		Operation: signedEndorsement.SignedOperation,
	}

	opHash, err := gtv2.InjectionOperation(injectionInput)
	if err != nil {
		log.WithField("Message", err.Error()).Error("ERROR INJECTING")
	} else {
		log.WithField("Operation", opHash).Info("Injected Endorsement")
	}

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
