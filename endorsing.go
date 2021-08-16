package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"github.com/bakingbacon/go-tezos/v4/forge"
	"github.com/bakingbacon/go-tezos/v4/rpc"

	log "github.com/sirupsen/logrus"

	"bakinbacon/storage"
)

/*
$ ~/.opam/for_tezos/bin/tezos-codec decode 009-PsFLoren.operation from 5b3c0553c157d641f205f97c6fa480c98b156a75ca2db43e2a202c2460b689f90a000000655b3c0553c157d641f205f97c6fa480c98b156a75ca2db43e2a202c2460b689f9000002392a9b99b4c1f735fb26bc376703ef3ab6b3bf69e07aab1dd09596ac7f196c9a365dbf384d88147aef2c697577596176c6991f46dcd9eb43752ce9632774e2c26008000900000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000
{ "branch": "BLQToCX7mDU2bVuDQXcNAD4FixVNxBA8CBEpcmSQBiPRfgAk6Mc",
  "contents":
    [ { "kind": "endorsement_with_slot",
        "endorsement":
          { "branch": "BLQToCX7mDU2bVuDQXcNAD4FixVNxBA8CBEpcmSQBiPRfgAk6Mc",
            "operations": { "kind": "endorsement", "level": 145706 },
            "signature":
              "sigiLztJohJDzskahhQ2YAfcjRrZjRE8GuxTZK3B3bLdEtfQtHyeQ7VWbBYwiquYg4yU5CDPyGgW4Fecd54q9NiVVWHurdtR" },
        "slot": 9 } ],
  "signature":
    "edsigtXomBKi5CTRf5cjATJWSyaRvhfYNHqSUGrn4SdbYRcGwQrUGjzEfQDTuqHhuA8b2d8NarZjz8TRf65WkpQmo423BtomS8Q" }
*/

func handleEndorsement(ctx context.Context, wg *sync.WaitGroup, block rpc.Block) {

	// Decrement waitGroup on exit
	defer wg.Done()

	// Handle panic gracefully
	defer func() {
		if r := recover(); r != nil {
			log.WithField("Message", r).Error("Panic recovered in handleEndorsement")
		}
	}()

	endorsingLevel := block.Header.Level

	// Check watermark to ensure we have not endorsed at this level before
	watermark, err := storage.DB.GetEndorsingWatermark()
	if err != nil {
		// watermark = 0 on DB error
		log.WithError(err).Error("Unable to get endorsing watermark from DB")
	}

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
		"Level": endorsingLevel, "Request": resp.Request.URL, "Response": string(resp.Body()),
	}).Trace("Fetching endorsing rights")

	if err != nil {
		log.WithError(err).Error("Unable to fetch endorsing rights")
		return
	}

	if len(endorsingRights) == 0 {
		log.WithField("Level", endorsingLevel).Info("No endorsing rights for this level")
		return
	}

	// Check for new block
	select {
	case <-ctx.Done():
		log.Warn("New block arrived; Canceling endorsement")
		return
	default:
		break
	}

	// Continue since we have at least 1 endorsing right
	requiredBond := networkConstants[network].EndorsementSecurityDeposit

	if spendableBalance, err := bc.GetSpendableBalance(); err != nil {
		log.WithError(err).Error("Unable to get spendable balance")

		// Even if error here, we can still proceed.
		// Might have enough to post bond, might not.

	} else {

		// If not enough bond, exit early
		if requiredBond > spendableBalance {

			msg := "Spendable balance too low to cover bond"
			log.WithFields(log.Fields{
				"Spendable": spendableBalance, "ReqBond": requiredBond,
			}).Error(msg)

			bc.Status.SetError(errors.New(msg))

			return
		}
	}

	// Join up all endorsing slots for sorting
	var allSlots []int
	for _, e := range endorsingRights {
		allSlots = append(allSlots, e.Slots...)
	}

	// 009 requires the lowest slot be submitted
	sort.Ints(allSlots)

	slotString := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(allSlots)), ","), "[]")
	log.WithFields(log.Fields{
		"Level": endorsingLevel, "Slots": slotString,
	}).Info("Endorsing rights found")

	// Inner endorsement; forge and sign
	endoContent := rpc.Content{
		Kind:  rpc.ENDORSEMENT,
		Level: endorsingLevel,
	}

	// Inner endorsement bytes
	endorsementBytes, err := forge.Encode(block.Hash, endoContent)
	if err != nil {
		log.WithError(err).Error("Error Forging Inner Endorsement")
		return
	}

	log.WithField("Bytes", endorsementBytes).Debug("Forged Inlined Endorsement")

	// sign inner endorsement
	signedInnerEndorsement, err := bc.Signer.SignEndorsement(endorsementBytes, block.ChainID)
	if err != nil {
		log.WithError(err).Error("Signer endorsement failure")
		return
	}

	// Outer endorsement
	endoWithSlot := rpc.Content{
		Kind: rpc.ENDORSEMENT_WITH_SLOT,
		Endorsement: &rpc.InlinedEndorsement{
			Branch: block.Hash,
			Operations: &rpc.InlinedEndorsementOperations{
				Kind:  rpc.ENDORSEMENT,
				Level: endorsingLevel,
			},
			Signature: signedInnerEndorsement.EDSig,
		},
		Slot: allSlots[0],
	}

	// Outer bytes
	endoWithSlotBytes, err := forge.Encode(block.Hash, endoWithSlot)
	if err != nil {
		log.WithError(err).Error("Error Forging Outer Endorsement")
		return
	}

	// Really low-level debugging
	//log.WithField("SignedOp", signedInnerEndorsement.SignedOperation).Debug("SIGNED OP")
	//log.WithField("DecodedSig", signedInnerEndorsement.Signature).Debug("DECODED SIG")
	//log.WithField("Signature", signedInnerEndorsement.EDSig).Debug("EDSIG")

	// Check if a new block has been posted to /head and we should abort
	select {
	case <-ctx.Done():
		log.Warn("New block arrived; Canceling endorsement")
		return
	default:
		break
	}

	// TODO Fix Preapply

// 	// Prepare to pre-apply the operation
// 	preapplyEndoOp := rpc.PreapplyOperationsInput{
// 		BlockID: &hashBlockID,
// 		Operations: []rpc.Operations{
// 			{
// 				Branch: block.Hash,
// 				Contents: rpc.Contents{
// 					endoWithSlot,
// 				},
// 				Protocol:  block.Protocol,
// 				Signature: signedInnerEndorsement.EDSig,
// 			},
// 		},
// 	}
//
// 	// Validate the operation against the node for any errors
// 	resp, preApplyResp, err := bc.Current.PreapplyOperations(preapplyEndoOp)
// 	if err != nil {
// 		log.WithError(err).WithFields(log.Fields{
// 			"Request": resp.Request.URL, "Response": string(resp.Body()),
// 		}).Error("Could not preapply operations")
// 		//return
// 	}
//
// 	log.WithField("Resp", preApplyResp).Trace("Preapply Response")

	// Create injection
	injectionInput := rpc.InjectionOperationInput{
		Operation: endoWithSlotBytes,
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
	resp, opHash, err := bc.Current.InjectionOperation(injectionInput)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"Request": resp.Request.URL, "Response": string(resp.Body()),
		}).Error("Endorsement Injection Failure")

		// TODO
		// Check error message against known issues
		return
	}

	log.WithField("Operation", opHash).Info("Endorsement Injected")

	// Save endorsement to DB for watermarking
	if err := storage.DB.RecordEndorsement(endorsingLevel, opHash); err != nil {
		log.WithError(err).Error("Unable to save endorsement; Watermark compromised")
	}

	// Update status for UI
	bc.Status.SetRecentEndorsement(endorsingLevel, block.Metadata.Level.Cycle, opHash)
}
