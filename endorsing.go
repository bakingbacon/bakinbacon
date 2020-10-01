package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	//"goendorse/signerclient"

	gotezos "github.com/utdrmac/go-tezos/v2"

	log "github.com/sirupsen/logrus"
)

func handleEndorsement(ctx context.Context, wg *sync.WaitGroup, blk gotezos.Block) {

	defer wg.Done()

	log.WithField("BlockHash", blk.Hash).Trace("Received Endorsement Hash")

	// look for endorsing rights at this level
	endorsingLevel := blk.Header.Level
	endoRightsFilter := gotezos.EndorsingRightsInput{
		BlockHash: blk.Hash,
		Level:     endorsingLevel,
		Delegate:  (*bakerPkh),
	}

	rights, err := gt.EndorsingRights(endoRightsFilter)
	if err != nil {
		log.WithError(err).Error("Unable to fetch endorsing rights")
	}

	if len(*rights) == 0 {
		log.WithField("Level", endorsingLevel).Info("No endorsing rights for this level")
		return
	}

	// continue since we have at least 1 endorsing right
	for _, e := range *rights {
		log.WithField("Slots",
			strings.Trim(strings.Join(strings.Fields(fmt.Sprint(e.Slots)), ","), "[]")).Info("Endorsing rights found")
	}

	// Forge operation input
	endorsementOperation := gotezos.ForgeOperationWithRPCInput{
		Blockhash: blk.Hash,
		Branch:    blk.Hash,
		Contents: []gotezos.Contents{
			gotezos.Contents{
				Kind:  "endorsement",
				Level: endorsingLevel,
			},
		},
	}

	// Forge the operation using RPC and get the bytes back
	endorsementBytes, err := gt.ForgeOperationWithRPC(endorsementOperation)
	if err != nil {
		log.WithError(err).Error("Error Forging Endorsement")
		return
	}
	log.WithField("Bytes", string(endorsementBytes)).Debug("Forged Endorsement")

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

	// Create injection
	injectionInput := gotezos.InjectionOperationInput{
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

	// Inject endorsement
	opHash, err := gt.InjectionOperation(injectionInput)
	if err != nil {
		log.WithError(err).Error("Endorsement Failure")
	} else {
		log.WithField("Operation", opHash).Info("Endorsement Injected")
	}
}
