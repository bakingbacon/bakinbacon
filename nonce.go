package main

import (
	"context"
	"sync"

	gotezos "github.com/utdrmac/go-tezos/v2"
	log "github.com/sirupsen/logrus"
	"goendorse/storage"
)

func registerNonce(cycle, level int, seedHashHex string) error {

	// Nonces are revealed within the first few blocks of
	// the next cycle. So we need to save it for now, and
	// then reveal it after the start of the next cycle.

	return storage.DB.SaveNonce(cycle, level, seedHashHex, "")
}

func revealNonces(ctx context.Context, wg *sync.WaitGroup, block gotezos.Block) {

	defer wg.Done()

	// Only reveal in levels 1, 2, 3, 4 of cycle
	cyclePosition := block.Metadata.Level.CyclePosition
	if cyclePosition == 0 || cyclePosition > 4 {
		return
	}

	// Get nonces for previous cycle from DB
	previousCycle := block.Metadata.Level.Cycle - 1
	nonces, err := storage.DB.GetNoncesForCycle(previousCycle)
	if err != nil {
		log.WithError(err).Error("Unable to get nonces from DB")
		return
	}

	if len(nonces) == 0 {
		log.Info("No nonces to reveal")
		return
	}

	log.WithField("Cycle", previousCycle).Infof("Found %d nonces", len(nonces))

	// loop over nonces and inject
	for _, nonce := range nonces {

		// If a nonce has a revealed operation, don't need to reveal it again
		if nonce.RevealOp != "" {
			log.WithFields(log.Fields{
				"Level": nonce.Level, "RevealedOp": nonce.RevealOp,
			}).Info("Nonce already revealed")
			continue
		}

		log.WithFields(log.Fields{
			"Level": nonce.Level, "Hash": nonce.Hash,
		}).Info("Revealing nonce")

		nonceRevealOperation := gotezos.ForgeOperationWithRPCInput{
			Blockhash: block.Hash,
			Branch:    block.Hash,
			Contents: []gotezos.Contents{
				gotezos.Contents{
					Kind:  "seed_nonce_revelation",
					Level: nonce.Level,
					Nonce: nonce.Hash,
				},
			},
		}

		// Forge nonce reveal operation
		forgedNonceRevealBytes, err := gt.ForgeOperationWithRPC(nonceRevealOperation) // operations.go
		if err != nil {
			log.WithError(err).Error("Error Forging Nonce Reveal")
			continue
		}
		log.WithField("Bytes", forgedNonceRevealBytes).Trace("Forged Reveal Nonce")

		// Nonce reveals have the same watermark as endorsements
		signedNonceReveal, err := wallet.SignEndorsementOperation(forgedNonceRevealBytes, block.ChainID)
		if err != nil {
			log.WithError(err).Error("Could not sign nonce reveal bytes")
			continue
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
			continue
		}

		// Check if new block came in
		select {
		case <-ctx.Done():
			log.Warn("New block arrived; Canceling nonce reveal")
			return
		default:
			break
		}

		// Inject nonce reveal op
		injectionInput := gotezos.InjectionOperationInput{
			Operation: signedNonceReveal.SignedOperation,
		}
		return
		revealOpHash, err := gt.InjectionOperation(injectionInput)
		if err != nil {
			log.WithError(err).Error("Error Injecting Nonce Reveal")
			continue
		}

		log.WithField("OpHash", revealOpHash).Info("Injected Nonce Reveal")

		// Update DB with hash of reveal operation
		storage.DB.SaveNonce(previousCycle, nonce.Level, nonce.Hash, revealOpHash)
	}
}
