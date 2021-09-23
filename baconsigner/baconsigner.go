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
	SIGNER_WALLET int = 1
	SIGNER_LEDGER int = 2
)

var (
	NoSignerType = errors.New("No signer type defined")
)

type BaconSigner struct {
	BakerPkh     string
	signerType   int
	signerAccess SignerAccess
	storage      *storage.Storage
}

// SignOperationOutput contains an operation with the signature appended, and the signature
type SignOperationOutput struct {
	SignedOperation string
	Signature       string
	EDSig           string
}

func New(db *storage.Storage) (*BaconSigner, error) {

	bs := &BaconSigner{
		storage:      db,
		signerAccess: NewSignerAccess(db),
	}

	// Get which signing method (wallet or ledger), so we can perform sanity checks
	signerType, err := bs.storage.GetSignerType()
	if err != nil {
		return bs, errors.Wrap(err, "Unable to get signer type from DB")
	}
	bs.signerType = signerType

	switch bs.signerType {
	case SIGNER_WALLET:
		_, err = bs.signerAccess.GetWalletSigner()
		if err != nil {
			return nil, err
		}
	case SIGNER_LEDGER:
		_, err = bs.signerAccess.GetLedgerSigner()
		if err != nil {
			return nil, err
		}
	default:
		log.WithField("Type", signerType).Warn("No signer type defined. New setup?")
	}

	return bs, nil
}

// SignerStatus Returns error if baking is not configured. Delegate secret key must be configured in DB,
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

// ConfirmBakingPkh Confirm action on ledger; Not applicable to signer
func (s *BaconSigner) ConfirmBakingPkh(pkh, bip string) error {

	if err := s.signerAccess.ConfirmBakingPkh(pkh, bip); err != nil {
		return err
	}
	// set BaconSigner if all is good
	s.signerType = SIGNER_LEDGER
	return nil
}

// GetPublicKey Gets the public key, and public key hash, depending on signer type
func (s *BaconSigner) GetPublicKey() (string, string, error) {

	switch s.signerType {
	case SIGNER_WALLET:
		signer, err := s.signerAccess.GetWalletSigner()
		if err != nil {
			return "", "", errors.Wrap(err, "could not get wallet signer")
		}
		return signer.GetPublicKey()
	case SIGNER_LEDGER:
		signer, err := s.signerAccess.GetLedgerSigner()
		if err != nil {
			return "", "", errors.Wrap(err, "could not get wallet signer")
		}
		return signer.GetPublicKey()
	}

	return "", "", NoSignerType
}

// GenerateNewKey Generates new key; Not applicable to Ledger
func (s *BaconSigner) GenerateNewKey() (string, string, error) {

	signer := s.signerAccess.CreateWalletSigner()

	sk, pkh, err := signer.GenerateNewKey()

	// Need to set if all is good
	if err == nil {
		s.signerType = SIGNER_WALLET
	}

	return sk, pkh, err
}

// ImportSecretKey Imports a secret key; Not applicable to ledger
func (s *BaconSigner) ImportSecretKey(k string) (string, string, error) {

	signer := s.signerAccess.CreateWalletSigner()

	sk, pkh, err := signer.ImportSecretKey(k)

	// Need to set if all is good
	if err == nil {
		s.signerType = SIGNER_WALLET
	}

	return sk, pkh, err
}

// TestLedger Not applicable to wallet
func (s *BaconSigner) TestLedger() (*LedgerInfo, error) {

	signer, err := s.signerAccess.GetLedgerSigner()
	if err != nil {
		return nil, errors.Wrap(err, "could not get wallet signer")
	}

	return signer.TestLedger()
}

// SaveSigner For ledger - save to the DB, for wallet - check we have the key (it's already saved)
func (s *BaconSigner) SaveSigner() error {

	switch s.signerType {
	case SIGNER_WALLET:
		signer, err := s.signerAccess.GetWalletSigner()
		if err != nil {
			return errors.Wrap(err, "wallet signer was not saved")
		}
		if _, _, err = signer.GetPublicKey(); err != nil {
			return errors.Wrap(err, "wallet signer was not saved")
		}
		return nil
	case SIGNER_LEDGER:
		signer, err := s.signerAccess.GetLedgerSigner()
		if err != nil {
			return errors.Wrap(err, "could not save ledger signer")
		}
		return signer.SaveSigner()
	}

	return NoSignerType
}

// Close ledger or wallet
func (s *BaconSigner) Close() {

	switch s.signerType {
	case SIGNER_LEDGER:
		signer, err := s.signerAccess.GetLedgerSigner()
		if err != nil {
			log.Error("could not get ledger signer to close")
		}
		signer.Close()
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
		chainIDBytes := b58cdecode(chainID, networkprefix)
		// fmt.Println("ChainID:    ", chainID)
		// fmt.Println("ChainIDByt: ", chainIDBytes)
		// fmt.Println("ChainIDHex: ", hex.EncodeToString(chainIDBytes))

		opBytes = append(opBytes, chainIDBytes...)
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
			signer, err := s.signerAccess.GetWalletSigner()
			if err != nil {
				return "", errors.Wrap(err, "could not get wallet signer to sign")
			}
			return signer.SignBytes(b)
		case SIGNER_LEDGER:
			signer, err := s.signerAccess.GetLedgerSigner()
			if err != nil {
				return "", errors.Wrap(err, "could not get ledger signer to sign")
			}
			return signer.SignBytes(b)
		}
		return "", NoSignerType
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
