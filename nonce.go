package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"

	"github.com/goat-systems/go-tezos/v3/crypto"
	"github.com/goat-systems/go-tezos/v3/forge"
	"github.com/goat-systems/go-tezos/v3/rpc"

	log "github.com/sirupsen/logrus"

	"goendorse/nonce"
	"goendorse/storage"
	"goendorse/util"
)

func generateNonce() (nonce.Nonce, error) {

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
		return nonce.Nonce{}, err
	}
	seed := hex.EncodeToString(randBytes)[:64]

	seedHash, err := util.CryptoGenericHash(seed, []byte{})
	if err != nil {
		log.WithError(err).Error("Unable to hash seed for nonce")
		return nonce.Nonce{}, err
	}

	// B58 encode seed hash with nonce prefix
	nonceHash := crypto.B58cencode(seedHash, nonce.Prefix_nonce)
	seedHashHex := hex.EncodeToString(seedHash)

	n := nonce.Nonce{
		Seed:        seed,
		NonceHash:   nonceHash,
		SeedHashHex: seedHashHex,
	}

	return n, nil
}

func revealNonces(ctx context.Context, wg *sync.WaitGroup, block rpc.Block) {

	defer wg.Done()

	// Only reveal in levels 1-16 of cycle
	cyclePosition := block.Metadata.Level.CyclePosition
	if cyclePosition == 0 || cyclePosition > 16 {
		return
	}

	// Debug nonce
	// n := nonce.Nonce{
	// 	Level: 817184,
	// 	Seed: "27beb3170dceeff2b95e561f069cac55fa3d208a4b77711e58c5c1b807b01b43",
	// 	NonceHash: "nceUeGTSCZsR2Hm3So9MEyVC89pikEoB3Bi85QC1qVo1L95cr7qEt",
	// 	SeedHashHex: "37376a745e04d66d01a6552602fb6b7f87a51657f50edc8507a7490a72aee46d",
	// }
	// storage.DB.SaveNonce(399, n)

	// Get nonces for previous cycle from DB
	previousCycle := block.Metadata.Level.Cycle - 1
	nonces, err := storage.DB.GetNoncesForCycle(previousCycle)
	if err != nil {
		log.WithError(err).WithField("Cycle", previousCycle).Warn("Unable to get nonces from DB")
		return
	}

	// Filter out nonces which have been revealed
	var unrevealedNonces []nonce.Nonce
	for _, n := range nonces {
		if n.RevealOp != "" {
			log.WithFields(log.Fields{
				"Level": n.Level, "RevealedOp": n.RevealOp,
			}).Debug("Nonce already revealed")
			continue
		}
		unrevealedNonces = append(unrevealedNonces, n)
	}

	// Any unrevealed nonces?
	if len(unrevealedNonces) == 0 {
		log.Trace("No nonces to reveal")
		return
	}
	log.WithField("Cycle", previousCycle).Infof("Found %d unrevealed nonces", len(unrevealedNonces))

	// loop over unrevealed nonces and inject
	for _, nonce := range unrevealedNonces {

		log.WithFields(log.Fields{
			"Level": nonce.Level, "Hash": nonce.NonceHash, "Seed": nonce.Seed,
		}).Info("Revealing nonce")

		nonceRevelation := rpc.Content{
			Kind:  rpc.SEEDNONCEREVELATION,
			Level: nonce.Level,
			Nonce: nonce.Seed,
		}

		nonceRevelationBytes, err := forge.Encode(block.Hash, nonceRevelation)
		if err != nil {
			log.WithError(err).Error("Error Forging Nonce Reveal")
			return
		}
		log.WithField("Bytes", nonceRevelationBytes).Trace("Forged Nonce Reveal")

		// Sign using http(s) signer
		signedNonceReveal, err := signerWallet.SignNonce(nonceRevelationBytes, block.ChainID)
		if err != nil {
			log.WithError(err).Error("Signer nonce failure")
			continue
		}
		log.WithField("Signature", signedNonceReveal.EDSig).Debug("Signed Nonce Reveal")

		// Go-Tezos Wallet
		// signedNonceReveal, err := wallet.SignEndorsementOperation(forgedNonceRevealBytes, block.ChainID)
		// if err != nil {
		// 	log.WithError(err).Error("Could not sign nonce reveal bytes")
		// 	continue
		// }

		preapplyNonceRevealOp := rpc.PreapplyOperationsInput{
			Blockhash: block.Hash,
			Operations: []rpc.Operations{
				{
					Branch: block.Hash,
					Contents: rpc.Contents{
						nonceRevelation,
					},
					Protocol:  block.Protocol,
					Signature: signedNonceReveal.EDSig,
				},
			},
		}

		// Validate the operation against the node for any errors
		preApplyResp, err := gt.PreapplyOperations(preapplyNonceRevealOp)
		if err != nil {
			log.WithError(err).Error("Could not preapply nonce reveal operation")
			continue
		}
		log.Info("Nonce Preapply Successful")
		log.WithField("Response", preApplyResp).Trace("Nonce Preapply")

		// Check if new block came in
		select {
		case <-ctx.Done():
			log.Warn("New block arrived; Canceling nonce reveal")
			return
		default:
			break
		}

		// Inject nonce reveal op
		injectionInput := rpc.InjectionOperationInput{
			Operation: signedNonceReveal.SignedOperation,
		}

		revealOpHash, err := gt.InjectionOperation(injectionInput)
		if err != nil {
			log.WithError(err).Error("Error Injecting Nonce Reveal")
			continue
		}

		log.WithField("OperationHash", revealOpHash).Info("Nonce Reveal Injected")

		// Update DB with hash of reveal operation
		nonce.RevealOp = revealOpHash
		storage.DB.SaveNonce(previousCycle, nonce)
	}
}
