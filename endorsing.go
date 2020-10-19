package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/goat-systems/go-tezos/v3/forge"
	"github.com/goat-systems/go-tezos/v3/rpc"

	log "github.com/sirupsen/logrus"

	"goendorse/storage"
)

func handleEndorsement(ctx context.Context, wg *sync.WaitGroup, blk rpc.Block) {

	defer wg.Done()

	endorsingLevel := blk.Header.Level

	// Check watermark to ensure we have not endorsed at this level before
	watermark := storage.DB.GetEndorsingWatermark()
	if watermark >= endorsingLevel {
		log.WithFields(log.Fields{
			"EndorsingLevel": endorsingLevel, "Watermark": watermark,
		}).Error("Watermark level higher than endorsing level; Canceling to prevent double endorsing")
		return
	}

	// look for endorsing rights at this level
	endorsingRightsFilter := rpc.EndorsingRightsInput{
		BlockHash: blk.Hash,
		Level:     endorsingLevel,
		Delegate:  (*bakerPkh),
	}

	endorsingRights, err := gt.EndorsingRights(endorsingRightsFilter)
	if err != nil {
		log.WithError(err).Error("Unable to fetch endorsing rights")
	}

	if len(*endorsingRights) == 0 {
		log.WithField("Level", endorsingLevel).Info("No endorsing rights for this level")
		return
	}

	// continue since we have at least 1 endorsing right
	for _, e := range *endorsingRights {
		log.WithField("Slots",
			strings.Trim(strings.Join(strings.Fields(fmt.Sprint(e.Slots)), ","), "[]")).Info("Endorsing rights found")
	}

	endoContent := rpc.Content{
		Kind:  rpc.ENDORSEMENT,
		Level: endorsingLevel,
	}

	endorsementBytes, err := forge.Encode(blk.Hash, endoContent)
	if err != nil {
		log.WithError(err).Error("Error Forging Endorsement")
		return
	}
	log.WithField("Bytes", endorsementBytes).Debug("Forged Endorsement")

	// Check if a new block has been posted to /head and we should abort
	select {
	case <-ctx.Done():
		log.Warn("New block arrived; Canceling endorsement")
		return
	default:
		break
	}

	// Sign with tezos-signer
	signedEndorsement, err := signerWallet.SignEndorsement(endorsementBytes, blk.ChainID)
	if err != nil {
		log.WithError(err).Error("tezos-signer failure")
	}
	log.WithField("Signature", signedEndorsement.EDSig).Debug("Signer Signature")

	// Really low-level debugging
	//log.WithField("SignedOp", signedEndorsement.SignedOperation).Debug("SIGNED OP")
	//log.WithField("Signature", signedEndorsement.EDSig).Debug("Wallet Signature")
	//log.WithField("DecodedSig", signedEndorsement.Signature).Debug("DECODED SIG")

	// Prepare to pre-apply the operation
	preapplyEndoOp := rpc.PreapplyOperationsInput{
		Blockhash: blk.Hash,
		Operations: []rpc.Operations{
			{
				Branch: blk.Hash,
				Contents: rpc.Contents{
					endoContent,
				},
				Protocol:  blk.Protocol,
				Signature: signedEndorsement.EDSig,
			},
		},
	}

	// Validate the operation against the node for any errors
	preApplyResp, err := gt.PreapplyOperations(preapplyEndoOp)
	if err != nil {
		log.WithError(err).Error("Could not preapply operations")
		return
	}
	log.WithField("Resp", preApplyResp).Trace("Preapply Response")

	// Create injection
	injectionInput := rpc.InjectionOperationInput{
		Operation: signedEndorsement.SignedOperation,
	}

	// Check if a new block has been posted to /head and we should abort
	select {
	case <-ctx.Done():
		log.Warn("New block arrived; Canceling endorsement")
		return
	default:
		break
	}

	// Dry-run check
	if *dryRunEndorsement {
		log.Warn("Not Injecting Endorsement; Dry-Run Mode")
		return
	}

	// Inject endorsement
	opHash, err := gt.InjectionOperation(injectionInput)
	if err != nil {
		log.WithError(err).Error("Endorsement Failure")
		return
	}
	log.WithField("Operation", opHash).Info("Endorsement Injected")

	// Save endorsement to DB for watermarking
	storage.DB.RecordEndorsement(endorsingLevel, opHash)
}
