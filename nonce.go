package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
	"sync"

	"github.com/bakingbacon/go-tezos/v4/crypto"
	"github.com/bakingbacon/go-tezos/v4/forge"
	"github.com/bakingbacon/go-tezos/v4/rpc"

	log "github.com/sirupsen/logrus"

	"bakinbacon/nonce"
	"bakinbacon/storage"
	"bakinbacon/util"
)

var previouslyInjectedErr = regexp.MustCompile(`while applying operation (o[a-zA-Z0-9]{50}).*previously revealed`)

func generateNonce() (nonce.Nonce, error) {
	// Generate a 64 char hexadecimal seed from random 32 bytes
	randBytes := make([]byte, 32)
	if _, err := rand.Read(randBytes); err != nil {
		log.WithError(err).Error("Unable to read random bytes")
		return nonce.Nonce{}, err
	}

	nonceHash, err := util.CryptoGenericHash(randBytes, []byte{})
	if err != nil {
		log.WithError(err).Error("Unable to hash rand bytes for nonce")
		return nonce.Nonce{}, err
	}

	// B58 encode seed hash with nonce prefix
	encodedNonce := crypto.B58cencode(nonceHash, nonce.PrefixNonce)

	n := nonce.Nonce{
		Seed:          hex.EncodeToString(randBytes),
		Nonce:         nonceHash,
		EncodedNonce:  encodedNonce,
		NoPrefixNonce: hex.EncodeToString(nonceHash),
	}

	return n, nil
}

func revealNonces(ctx context.Context, wg *sync.WaitGroup, block rpc.Block) {

	// Decrement waitGroup on exit
	defer wg.Done()

	// Handle panic gracefully
	defer func() {
		if r := recover(); r != nil {
			log.WithField("Message", r).Error("Panic recovered in revealNonces")
		}
	}()

	// Only reveal in levels 1-16 of cycle
	cyclePosition := block.Metadata.Level.CyclePosition
	if cyclePosition == 0 || cyclePosition > 256 {
		return
	}

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
		log.WithField("Cycle", previousCycle).Info("No nonces to reveal")
		return
	}

	log.WithField("Cycle", previousCycle).Infof("Found %d unrevealed nonces", len(unrevealedNonces))

	hashBlockID := rpc.BlockIDHash(block.Hash)

	// loop over unrevealed nonces and inject
	for _, nonce := range unrevealedNonces {

		log.WithFields(log.Fields{
			"Level": nonce.Level, "Nonce": nonce.EncodedNonce, "Seed": nonce.Seed,
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
		nonceRevelationBytes += strings.Repeat("0", 128) // Nonce requires a null signature

		log.WithField("Bytes", nonceRevelationBytes).Trace("Forged Nonce Reveal")

		// Build preapply using null signature
		preapplyNonceRevealOp := rpc.PreapplyOperationsInput{
			BlockID: &hashBlockID,
			Operations: []rpc.Operations{
				{
					Protocol: block.Protocol,
					Branch:   block.Hash,
					Contents: rpc.Contents{
						nonceRevelation,
					},
					Signature: "edsigtXomBKi5CTRf5cjATJWSyaRvhfYNHqSUGrn4SdbYRcGwQrUGjzEfQDTuqHhuA8b2d8NarZjz8TRf65WkpQmo423BtomS8Q",
				},
			},
		}

		// Validate the operation against the node for any errors
		resp, preApplyResp, err := bc.Current.PreapplyOperations(preapplyNonceRevealOp)
		if err != nil {

			// If somehow the nonce reveal was already injected, but we have no record of the opHash,
			// we can inject it again without worry to discover the opHash and save it
			if strings.Contains(resp.String(), "nonce.previously_revealed") {

				log.Warn("Nonce previously injected, unknown opHash.")

			} else {

				// Any other error we display and move to next nonce
				log.WithError(err).WithFields(log.Fields{
					"Request": resp.Request.URL, "Response": string(resp.Body()),
				}).Error("Could not preapply nonce reveal operation")

				continue
			}

		} else {

			// Preapply success
			log.WithField("Response", preApplyResp).Trace("Nonce Preapply")
			log.Info("Nonce Preapply Successful")
		}

		// Check if new block came in
		select {
		case <-ctx.Done():
			log.Warn("New block arrived; Canceling nonce reveal")
			return
		default:
			// No need to wait
			break
		}

		// Inject nonce reveal op
		injectionInput := rpc.InjectionOperationInput{
			Operation: nonceRevelationBytes,
		}

		resp, revealOpHash, err := bc.Current.InjectionOperation(injectionInput)
		if err != nil {

			// Check error message for possible previous injection. If notice not present
			// then we have a real error on our hands. If notice present, let func finish
			// and save operational hash to DB
			parts := previouslyInjectedErr.FindStringSubmatch(resp.String())
			if len(parts) > 0 {
				revealOpHash = parts[1]
			} else {

				log.WithError(err).WithFields(log.Fields{
					"Response": resp.String(),
				}).Error("Error Injecting Nonce Reveal")

				continue
			}
		}

		log.WithField("OperationHash", revealOpHash).Info("Nonce Reveal Injected")

		// Update DB with hash of reveal operation
		nonce.RevealOp = revealOpHash
		if err := storage.DB.SaveNonce(previousCycle, nonce); err != nil {
			log.WithError(err).Error("Unable to save nonce reveal to DB")
		}
	}
}
