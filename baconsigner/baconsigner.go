package baconsigner

import (
	"encoding/hex"
	"fmt"

	"github.com/Messer4/base58check"
	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"

	"bakinbacon/storage"
)

const (
	SIGNER_WALLET = 1
	SIGNER_LEDGER = 2
)

var (
	NO_SIGNER_TYPE = errors.New("No signer type defined")
)

type BaconSigner struct {
	BakerPkh   string
	signerType int
	storage    *storage.Storage
}

// SignOperationOutput contains an operation with the signature appended, and the signature
type SignOperationOutput struct {
	SignedOperation string
	Signature       string
	EDSig           string
}

// New
func New(db *storage.Storage) (*BaconSigner, error) {

	bs := &BaconSigner{
		storage: db,
	}

	// Get which signing method (wallet or ledger), so we can perform sanity checks
	signerType, err := bs.storage.GetSignerType()
	if err != nil {
		return bs, errors.Wrap(err, "Unable to get signer type from DB")
	}
	bs.signerType = signerType

	switch bs.signerType {
	case SIGNER_WALLET:
		if err := InitWalletSigner(db); err != nil {
			return bs, errors.Wrap(err, "Cannot init wallet signer")
		}
	case SIGNER_LEDGER:
		if err := InitLedgerSigner(db); err != nil {
			return bs, errors.Wrap(err, "Cannot init ledger signer")
		}
	default:
		log.WithField("Type", signerType).Warn("No signer type defined. New setup?")
	}

	return bs, nil
}

// SignerStatus returns error if baking is not configured. Delegate secret key must be configured in DB,
// and signer type must also be set and wallet must be loadable
func (s *BaconSigner) SignerStatus(silent bool) error {

	// Try to load the bakers SK
	if err := s.LoadDelegate(silent); err != nil {
		return errors.Wrap(err, "Loading Delegate")
	}

	return nil
}

func (s *BaconSigner) LoadDelegate(silent bool) error {

	var err error

	_, s.BakerPkh, err = s.storage.GetDelegate()
	if err != nil {
		log.WithError(err).Error("Unable to load delegate from DB")
		return err
	}

	if s.BakerPkh == "" {
		// Missing delegate; cannot bake/endorse/nonce; User needs to configure via UI
		return errors.New("No delegate key defined")
	}

	if !silent {
		log.WithField("Delegate", s.BakerPkh).Info("Loaded delegate public key hash from DB")
	}

	return nil
}

// ConfirmBakingPkh Confirms action on ledger; Not applicable to signer
func (s *BaconSigner) ConfirmBakingPkh(pkh, bip string) error {

	if err := L.ConfirmBakingPkh(pkh, bip); err != nil {
		return errors.Wrap(err, "Cannot confirm baking address")
	}

	// Set BaconSigner if all is good
	s.signerType = SIGNER_LEDGER

	return nil
}

// GetPublicKey Gets the public key, and public key hash, depending on signer type
func (s *BaconSigner) GetPublicKey() (string, string, error) {

	switch s.signerType {
	case SIGNER_WALLET:
		return W.GetPublicKey()
	case SIGNER_LEDGER:
		return L.GetPublicKey()
	}

	return "", "", NO_SIGNER_TYPE
}

// GenerateNewKey Generates new key; Not applicable to Ledger
func (s *BaconSigner) GenerateNewKey() (string, string, error) {

	sk, pkh, err := GenerateNewKey()
	if err != nil {
		return "", "", errors.Wrap(err, "Cannot generate new key")
	}

	// Set if all is good
	s.signerType = SIGNER_WALLET

	return sk, pkh, nil
}

// ImportSecretKey Imports a secret key; Not applicable to ledger
func (s *BaconSigner) ImportSecretKey(k string) (string, string, error) {

	sk, pkh, err := ImportSecretKey(k)
	if err != nil {
		return "", "", errors.Wrap(err, "Cannot import secret key")
	}

	// Set if all is good
	s.signerType = SIGNER_WALLET

	return sk, pkh, nil
}

// TestLedger Will check if Ledger is plugged in and app is open; Not applicable to wallet
func (s *BaconSigner) TestLedger() (*LedgerInfo, error) {
	return TestLedger()
}

// SaveSigner Saves signer config to DB
func (s *BaconSigner) SaveSigner() error {

	switch s.signerType {
	case SIGNER_WALLET:
		return W.SaveSigner()
	case SIGNER_LEDGER:
		return L.SaveSigner()
	}

	return NO_SIGNER_TYPE
}

// Close ledger or wallet
func (s *BaconSigner) Close() {

	switch s.signerType {
	case SIGNER_LEDGER:
		L.Close()
	}
}

// Signing Functions

func (s *BaconSigner) SignEndorsement(endorsementBytes, chainID string) (SignOperationOutput, error) {
	return s.signGeneric(endorsementprefix, endorsementBytes, chainID)
}

func (s *BaconSigner) SignBlock(blockBytes, chainID string) (SignOperationOutput, error) {
	return s.signGeneric(blockprefix, blockBytes, chainID)
}

func (s *BaconSigner) SignNonce(nonceBytes string, chainID string) (SignOperationOutput, error) {
	// Nonce reveals have the same watermark as endorsements
	return s.signGeneric(endorsementprefix, nonceBytes, chainID)
}

func (s *BaconSigner) SignReveal(revealBytes string) (SignOperationOutput, error) {
	return s.signGeneric(genericopprefix, revealBytes, "")
}

func (s *BaconSigner) SignTransaction(trxBytes string) (SignOperationOutput, error) {
	return s.signGeneric(genericopprefix, trxBytes, "")
}

func (s *BaconSigner) SignSetDelegate(delegateBytes string) (SignOperationOutput, error) {
	return s.signGeneric(genericopprefix, delegateBytes, "")
}

func (s *BaconSigner) SignProposalVote(proposalBytes string) (SignOperationOutput, error) {
	return s.signGeneric(genericopprefix, proposalBytes, "")
}

// Generic raw signing function
// Takes the incoming operation hex-bytes and signs using whichever wallet type is in use
func (s *BaconSigner) signGeneric(opPrefix prefix, incOpHex, chainID string) (SignOperationOutput, error) {

	// Base bytes of operation; all ops begin with prefix
	opBytes := opPrefix

	if chainID != "" {
		// Strip off the network watermark (prefix), and then base58 decode the chain id string (ie: NetXUdfLh6Gm88t)
		chainIdBytes := b58cdecode(chainID, networkprefix)
		// fmt.Println("ChainID:    ", chainID)
		// fmt.Println("ChainIDByt: ", chainIdBytes)
		// fmt.Println("ChainIDHex: ", hex.EncodeToString(chainIdBytes))

		opBytes = append(opBytes, chainIdBytes...)
	}

	// Decode the incoming operational hex to bytes
	incOpBytes, err := hex.DecodeString(incOpHex)
	if err != nil {
		return SignOperationOutput{}, errors.Wrap(err, "Failed to sign operation")
	}
	//fmt.Println("IncOpHex:   ", incOpHex)
	//fmt.Println("IncOpBytes: ", incOpBytes)

	// Append incoming op bytes to either prefix, or prefix + chainId
	opBytes = append(opBytes, incOpBytes...)

	// Convert op bytes back to hex; anyone need this?
	//finalOpHex := hex.EncodeToString(opBytes)
	//fmt.Println("ToSignBytes: ", opBytes)
	//fmt.Println("ToSignByHex: ", finalOpHex)

	edSig, err := func(b []byte) (string, error) {
		switch s.signerType {
		case SIGNER_WALLET:
			return W.SignBytes(b)
		case SIGNER_LEDGER:
			return L.SignBytes(b)
		}
		return "", NO_SIGNER_TYPE
	}(opBytes)

	if err != nil {
		return SignOperationOutput{}, errors.Wrap(err, "Failed sign bytes")
	}

	// Decode out the signature from the operation
	decodedSig, err := decodeSignature(edSig)
	if err != nil {
		return SignOperationOutput{}, errors.Wrap(err, "Failed to decode signed block")
	}

	return SignOperationOutput{
		SignedOperation: fmt.Sprintf("%s%s", incOpHex, decodedSig),
		Signature:       decodedSig,
		EDSig:           edSig,
	}, nil
}

// Helper function to return the decoded signature
func decodeSignature(signature string) (string, error) {

	decBytes, err := base58check.Decode(signature)
	if err != nil {
		return "", errors.Wrap(err, "failed to decode signature")
	}

	decodedSigHex := hex.EncodeToString(decBytes)

	// sanity
	if len(decodedSigHex) > 10 {
		decodedSigHex = decodedSigHex[10:]
	} else {
		return "", errors.Wrap(err, "decoded signature is invalid length")
	}

	return decodedSigHex, nil
}
