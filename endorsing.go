package main

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	gotezos "github.com/goat-systems/go-tezos"
)

func handleEndorsement(ctx context.Context, blk gotezos.Block) {

	log.WithField("BlockHash", blk.Hash).Debug("Received Endorsement Hash")

	// look for endorsing rights at this level
	endorsingLevel := blk.Header.Level
	endoRightsFilter := gotezos.EndorsingRightsInput{
		BlockHash: blk.Hash,
		Level:     endorsingLevel,
		Delegate:  BAKER,
	}

	rights, err := gt.EndorsingRights(endoRightsFilter)
	if err != nil {
		log.Println(err)
	}

	if len(*rights) == 0 {
		log.WithField("Level", endorsingLevel).Info("No endorsing rights for level")
		return
	}

	// continue since we have at least 1 endorsing right
	for _, e := range *rights {
		log.WithField("Slots",
			strings.Trim(strings.Join(strings.Fields(fmt.Sprint(e.Slots)), ","), "[]")).Info("Endorsing rights found")
	}

	// v2
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

	// v2
	endorsementBytes, err := gt.ForgeOperationWithRPC(endorsementOperation)
	if err != nil {
		log.WithError(err).Error("Error Forging Endorsement")
		return
	}
	//log.WithField("Bytes", endorsementBytes).Debug("FORGED ENDORSEMENT")

	// Check if a new block has been posted to /head and we should abort
	select {
	case <-ctx.Done():
		log.Info("New block arrived; Canceling endorsement")
		return
	default:
		break
	}

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

	// Check if a new block has been posted to /head and we should abort
	select {
	case <-ctx.Done():
		log.Info("New block arrived; Canceling endorsement")
		return
	default:
		break
	}

	opHash, err := gt.InjectionOperation(injectionInput)
	if err != nil {
		log.WithError(err).Error("Endorsement Failure")
	} else {
		log.WithField("Operation", opHash).Info("Endorsement Injected")
	}
}
