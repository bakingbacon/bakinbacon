package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/bakingbacon/go-tezos/v4/forge"
	"github.com/bakingbacon/go-tezos/v4/rpc"

	log "github.com/sirupsen/logrus"

	"bakinbacon/storage"
)

func handleEndorsement(ctx context.Context, wg *sync.WaitGroup, block rpc.Block) {

	defer wg.Done()

	endorsingLevel := block.Header.Level

	// Check watermark to ensure we have not endorsed at this level before
	watermark := storage.DB.GetEndorsingWatermark()
	if watermark >= endorsingLevel {
		log.WithFields(log.Fields{
			"EndorsingLevel": endorsingLevel, "Watermark": watermark,
		}).Error("Watermark level higher than endorsing level; Canceling to prevent double endorsing")
		return
	}

	// look for endorsing rights at this level
	hashBlockID := rpc.BlockIDHash(block.Hash)
	endorsingRightsFilter := rpc.EndorsingRightsInput{
		BlockID:  &hashBlockID,
		Level:    endorsingLevel,
		Delegate: bc.Signer.BakerPkh,
	}

	resp, endorsingRights, err := bc.Current.EndorsingRights(endorsingRightsFilter)

	log.WithFields(log.Fields{
		"Request": resp.Request.URL, "Response": string(resp.Body()),
	}).Debug("Fetching endorsing rights")

	if err != nil {
		log.WithError(err).Error("Unable to fetch endorsing rights")
		return
	}

	if len(endorsingRights) == 0 {
		log.WithField("Level", endorsingLevel).Info("No endorsing rights for this level")
		return
	}

	// continue since we have at least 1 endorsing right
	for _, e := range endorsingRights {
		log.WithField("Slots",
			strings.Trim(strings.Join(strings.Fields(fmt.Sprint(e.Slots)), ","), "[]")).Info("Endorsing rights found")
	}

	endoContent := rpc.Content{
		Kind:  rpc.ENDORSEMENT,
		Level: endorsingLevel,
	}

	endorsementBytes, err := forge.Encode(block.Hash, endoContent)
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
	// TODO Attempt this more than once
	// ERRO[2021-02-02T22:56:53Z] Signer endorsement failure                    error="failed signer: failed to execute request: Post \"http://127.0.0.1:18734/keys/tz1RMmSzPSWPSSaKU193Voh4PosWSZx1C7Hs\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"
	signedEndorsement, err := bc.Signer.SignEndorsement(endorsementBytes, block.ChainID)
	if err != nil {
		log.WithError(err).Error("Signer endorsement failure")
		return
	}
	log.WithField("Signature", signedEndorsement.EDSig).Debug("Signer Signature")

	// Really low-level debugging
	//log.WithField("SignedOp", signedEndorsement.SignedOperation).Debug("SIGNED OP")
	//log.WithField("Signature", signedEndorsement.EDSig).Debug("Wallet Signature")
	//log.WithField("DecodedSig", signedEndorsement.Signature).Debug("DECODED SIG")

	// Prepare to pre-apply the operation
	preapplyEndoOp := rpc.PreapplyOperationsInput{
		BlockID:    &hashBlockID,
		Operations: []rpc.Operations{
			{
				Branch: block.Hash,
				Contents: rpc.Contents{
					endoContent,
				},
				Protocol:  block.Protocol,
				Signature: signedEndorsement.EDSig,
			},
		},
	}

	// Validate the operation against the node for any errors
	_, preApplyResp, err := bc.Current.PreapplyOperations(preapplyEndoOp)
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
	_, opHash, err := bc.Current.InjectionOperation(injectionInput)
	if err != nil {
		log.WithError(err).Error("Endorsement Failure")
		return
	}
	log.WithField("Operation", opHash).Info("Endorsement Injected")

	// Save endorsement to DB for watermarking
	storage.DB.RecordEndorsement(endorsingLevel, opHash)
	
	// Update status for UI
	bc.Status.SetRecentEndorsement(endorsingLevel, block.Metadata.Level.Cycle, opHash)
}
