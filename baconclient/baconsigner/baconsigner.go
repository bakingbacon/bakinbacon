package baconsigner

import (
	"encoding/hex"
	"fmt"

	"github.com/Messer4/base58check"
	"github.com/pkg/errors"

	gtks "github.com/bakingbacon/go-tezos/v4/keys"
	ledger "github.com/bakingbacon/goledger/ledger-apps/tezos"
	log "github.com/sirupsen/logrus"

	"bakinbacon/storage"
)

const (
	SIGNER_WALLET    = 1
	SIGNER_LEDGER    = 2
)

type BaconSigner struct {
	BakerPkh      string
	bakerSk       string // temp for UI wizard
	LedgerBipPath string
	SignerType    int

	gt_wallet *gtks.Key
	lg_wallet *ledger.TezosLedger
}

// SignOperationOutput contains an operation with the signature appended, and the signature
type SignOperationOutput struct {
	SignedOperation string
	Signature       string
	EDSig           string
}

// New
func New() *BaconSigner {

	bs := BaconSigner{}

	if err := bs.SignerStatus(false); err != nil {
		log.WithError(err).Error("Signer Status")
	}

	return &bs
}

// Returns error if baking is not configured. Delegate secret key must be configured in DB,
// and signer type must also be set and wallet must be loadable
func (s *BaconSigner) SignerStatus(silent bool) error {

	// Try to load the bakers SK
	if err := s.LoadDelegate(silent); err != nil {
		return errors.Wrap(err, "Loading Delegate")
	}

	// Try to load signer type
	if err := s.LoadSigner(silent); err != nil {
		return errors.Wrap(err, "Loading Signer")
	}

	return nil
}

func (s *BaconSigner) LoadDelegate(silent bool) error {

	var err error

	_, s.BakerPkh, err = storage.DB.GetDelegate()
	if err != nil {
		log.WithError(err).Error("Unable to load delegate from DB")
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

func (s *BaconSigner) LoadSigner(silent bool) error {

	var err error

	// Get which signing method (wallet or ledger), so we can perform sanity checks
	s.SignerType, err = storage.DB.GetSignerType()
	if err != nil {
		return errors.Wrap(err, "Unable to get signer type from DB")
	}

	switch s.SignerType {
	case 0:
		return errors.New("No signer type defined. Cannot bake.")

	case SIGNER_WALLET:

		walletSk, err := storage.DB.GetSignerSk()
		if err != nil {
			return errors.Wrap(err, "Unable to get signer sk from DB")
		}

		if walletSk == "" {
			return errors.New("No wallet secret key found. Cannot bake.")
		} else {

			s.gt_wallet, err = gtks.FromBase58(walletSk, gtks.Ed25519)
			if err != nil {
				return errors.Wrap(err, "Failed to load wallet from secret key")
			}

			if !silent {
				log.WithFields(log.Fields{
					"Baker": s.gt_wallet.PubKey.GetAddress(), "PublicKey": s.gt_wallet.PubKey.GetPublicKey(),
				}).Info("Loaded software wallet")
			}
		}

	case SIGNER_LEDGER:

		// Get device
		s.lg_wallet, err = ledger.Get()
		if err != nil {
			return errors.Wrap(err, "Cannot get ledger device")
		}
		defer s.lg_wallet.Close()

		// Get bipPath and PKH from DB
		pkh, dbBipPath, err := storage.DB.GetLedgerConfig()
		if err != nil {
			return errors.Wrap(err, "Cannot load ledger config from DB")
		}

		// Sanity
		if dbBipPath == "" {
			return errors.New("No BIP path found in DB. Cannot configure ledger.")
		}

		// Sanity check if wallet app is open instead of baking app
		if _, err := s.IsBakingApp(s.lg_wallet); err != nil {
			return err
		}

		// Get the bipPath that is authorized to bake
		authBipPath, err := s.lg_wallet.GetAuthorizedKeyPath()
		if err != nil {
			return errors.Wrap(err, "Cannot get auth BIP path from ledger")
		}

		// Compare to DB config for sanity
		if dbBipPath != authBipPath {
			return errors.New(fmt.Sprintf("Authorized BipPath, %s, does not match DB Config, %s", authBipPath, dbBipPath))
		}

		// Set dbBipPath from DB config
		if err := s.lg_wallet.SetBipPath(dbBipPath); err != nil {
			return errors.Wrap(err, "Cannot set BIP path on ledger device")
		}

		// Get the pkh from dbBipPath from DB config
		_, compPkh, err := s.lg_wallet.GetPublicKey()
		if err != nil {
			return errors.Wrap(err, "Cannot fetch pkh from ledger")
		}

		if pkh != compPkh {
			return errors.New(fmt.Sprintf("Authorized PKH, %s, does not match DB Config, %s", pkh, compPkh))
		}

		s.BakerPkh = pkh
		s.LedgerBipPath = authBipPath

		if !silent {
			log.WithFields(log.Fields{ "KeyPath": authBipPath, "PKH": pkh }).Debug("Ledger Baking Config")
		}
	}

	// All good
	return nil
}

// Gets the public key, depending on signer type
func (s *BaconSigner) GetPublicKey() (string, error) {

	switch s.SignerType {
	case SIGNER_WALLET:
		return s.gt_wallet.PubKey.GetPublicKey(), nil

	case SIGNER_LEDGER:

		// Get device
		ledgerDev, err := ledger.Get()
		if err != nil {
			return "", errors.Wrap(err, "Cannot get ledger device")
		}
		defer ledgerDev.Close()

		if err := ledgerDev.SetBipPath(s.LedgerBipPath); err != nil {
			return "", errors.Wrap(err, "Cannot set BIP path on ledger device")
		}

		pk, _, err := ledgerDev.GetPublicKey()
		if err != nil {
			return "", err
		}

		return pk, nil

	default:
		return "", errors.New("No signer type defined.")
	}
}

//
// Signing Functions
//
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
	var opBytes = opPrefix

	if chainID != "" {
		// Strip off the network watermark (prefix), and then base58 decode the chain id string (ie: NetXUdfLh6Gm88t)
		chainIdBytes := b58cdecode(chainID, networkprefix)
//		fmt.Println("ChainID:    ", chainID)
//		fmt.Println("ChainIDByt: ", chainIdBytes)
//		fmt.Println("ChainIDHex: ", hex.EncodeToString(chainIdBytes))

		opBytes = append(opBytes, chainIdBytes...)
	}

	// Decode the incoming operational hex to bytes
	incOpBytes, err := hex.DecodeString(incOpHex)
	if err != nil {
		return SignOperationOutput{}, errors.Wrap(err, "failed to sign operation")
	}
	//fmt.Println("IncOpHex:   ", incOpHex)
	//fmt.Println("IncOpBytes: ", incOpBytes)

	// Append incoming op bytes to either prefix, or prefix + chainId
	opBytes = append(opBytes, incOpBytes...)

	// Convert op bytes back to hex; anyone need this?
	finalOpHex := hex.EncodeToString(opBytes)
//	fmt.Println("ToSignBytes: ", opBytes)
	fmt.Println("ToSignByHex: ", finalOpHex)

	edSig := ""

	switch s.SignerType {
	case SIGNER_WALLET:
		sig, err := s.gt_wallet.SignRawBytes(opBytes) // Returns 'Signature' object
		if err != nil {
			return SignOperationOutput{}, errors.Wrap(err, "Failed wallet signer")
		}

		edSig = sig.ToBase58()

	case SIGNER_LEDGER:

		// Get device
		ledgerDev, err := ledger.Get()
		if err != nil {
			return SignOperationOutput{}, errors.Wrap(err, "Cannot get ledger device")
		}
		defer ledgerDev.Close()

		if err := ledgerDev.SetBipPath(s.LedgerBipPath); err != nil {
			return SignOperationOutput{}, errors.Wrap(err, "Cannot set BIP path on ledger device")
		}

		edSig, err = ledgerDev.SignBytes(opBytes) // Returns b58 encoded signature
		if err != nil {
			return SignOperationOutput{}, errors.Wrap(err, "Failed ledger signer")
		}
	}

	// Decode out the signature from the operation
	decodedSig, err := decodeSignature(edSig)
	if err != nil {
		return SignOperationOutput{}, errors.Wrap(err, "failed to decode signed block")
	}
	//fmt.Println("DecodedSign: ", decodedSig)

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
