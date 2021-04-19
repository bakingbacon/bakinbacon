package baconsigner

import (
	"encoding/hex"
	"fmt"

	"github.com/pkg/errors"
	"github.com/Messer4/base58check"
	
	log "github.com/sirupsen/logrus"
	gtks "github.com/bakingbacon/go-tezos/v4/keys"
	ledger "github.com/bakingbacon/goledger/ledger-apps/tezos"

	"bakinbacon/storage"
)

const (
	SIGNER_WALLET = 1
	SIGNER_LEDGER = 2
)

type BaconSigner struct {
	BakerPkh		string
	SignerOk		bool
	SignerType		int
	
	gt_wallet		*gtks.Key
	lg_wallet		*ledger.TezosLedger
}

// SignOperationOutput contains an operation with the signature appended, and the signature
type SignOperationOutput struct {
    SignedOperation		string
    Signature			string
    EDSig				string
}

// New
func New() (*BaconSigner) {

	bs := BaconSigner{}

	if err := bs.SignerStatus(); err != nil {
		log.WithError(err).Error("Signer Status")
	}

	return &bs
}

// Returns error if baking is not configured. Delegate secret key must be configured in DB,
// and signer type must also be set and wallet must be loadable
func (s *BaconSigner) SignerStatus() (error) {

	// If checks have passed before, no need to keep checking
	if s.SignerOk {
		return nil
	}

	// Try to load the bakers SK
	if err := s.LoadDelegate(); err != nil {
		s.SignerOk = false
		return errors.Wrap(err, "Loading Delegate")
	}
	
	// Try to load signer type
	if err := s.LoadSigner(); err != nil {
		s.SignerOk = false
		return errors.Wrap(err, "Loading Signer")
	}
	
	s.SignerOk = true

	return nil
}

func (s *BaconSigner) LoadDelegate() (error) {

	_, s.BakerPkh = storage.DB.GetDelegate()
	if s.BakerPkh == "" {
		// Missing delegate; cannot bake/endorse/nonce; User needs to configure via UI
		return errors.New("No delegate key defined")
	}

	log.WithField("Delegate", s.BakerPkh).Info("Loaded delegate public key hash from DB")
	
	return nil
}

func (s *BaconSigner) LoadSigner() (error) {

	var err error

	// Get which signing method (wallet or ledger), so we can perform sanity checks	
	s.SignerType = storage.DB.GetSignerType()

	switch s.SignerType {
	case 0:
		return errors.New("No signer type defined. Cannot bake.")

	case SIGNER_WALLET:
		
		walletSk := storage.DB.GetSignerSk()
		if walletSk == "" {
			return errors.New("No wallet secret key found. Cannot bake.")

		} else {

			log.Info("Wallet secret key found. Loading wallet...")

			s.gt_wallet, err = gtks.FromBase58(walletSk, gtks.Ed25519)
			if err != nil {
				return errors.Wrap(err, "Failed to load wallet")
			}

			log.WithFields(log.Fields{
				"Baker": s.gt_wallet.PubKey.GetAddress(), "PublicKey": s.gt_wallet.PubKey.GetPublicKey(),
			}).Info("Loaded software wallet")
		}
	
	case SIGNER_LEDGER:

		// Get device
	    s.lg_wallet, err = ledger.Get()
    	if err != nil {
			return errors.Wrap(err, "Cannot get ledger device")
		}
		defer s.lg_wallet.Close()
		
		// Get first key
	    if err := s.lg_wallet.SetBipPath("/44'/1729'/0'/0'"); err != nil {
        	return errors.Wrap(err, "Cannot set BIP path on ledger device")
    	}
    	
    	authKeyPath, err := s.lg_wallet.GetAuthorizedKeyPath()
		if err != nil {
			return errors.Wrap(err, "No key-path authorized for baking")
		}
		log.Debug("GetAuthKeyPath:", authKeyPath)
	}
	
	// All good
	return nil
}

// Generates a new ED25519 keypair, saves to DB, and sets signer type to wallet
func (s *BaconSigner) GenerateNewKey() (string, string, error) {
	
	newKey, err := gtks.Generate(gtks.Ed25519)
	if err != nil {
		log.WithError(err).Error("Failed to generate new key")
		return "", "", errors.Wrap(err, "failed to generate new key")
	}
	
	edsk := newKey.GetSecretKey()
	pkh := newKey.PubKey.GetAddress()
	
	// Save generated key to storage
	storage.DB.SetDelegate(edsk, pkh)
	
	return edsk, pkh, nil
}

// Imports a secret key, saves to DB, and sets signer type to wallet
func (s *BaconSigner) ImportSecretKey(iEdsk string) (string, string, error) {

	key, err := gtks.FromBase58(iEdsk, gtks.Ed25519)
	if err != nil {
		log.WithError(err).Error("Failed to import key")
		return "", "", errors.Wrap(err, "failed to import key")
	}
	
	pkh := key.PubKey.GetAddress()
	
	// Save generated key to storage
	storage.DB.SetDelegate(iEdsk, pkh)
	
	return iEdsk, pkh, nil
}

// Sets signing type to wallet in DB
func (s *BaconSigner) SetSignerTypeWallet() (error) {
	return storage.DB.SetSignerType(SIGNER_WALLET)
}

// Sets signing type to ledger in DB
func (s *BaconSigner) SetSignerTypeLedger() (error) {
	return storage.DB.SetSignerType(SIGNER_LEDGER)
}

// Gets the public key, depending on signer type
func (s *BaconSigner) GetPublicKey() (string, error) {

	switch s.SignerType {
	case SIGNER_WALLET:
		return s.gt_wallet.PubKey.GetPublicKey(), nil
	case SIGNER_LEDGER:
		pk, _, err := s.lg_wallet.GetPublicKey()
		if err != nil {
			return "", err
		}
		return pk, nil
	default:
		return "", errors.New("No signer type defined. Loading failed?")
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

// Nonce reveals have the same watermark as endorsements
func (s *BaconSigner) SignNonce(nonceBytes string, chainID string) (SignOperationOutput, error) {
    return s.signGeneric(genericopprefix, nonceBytes, chainID)
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

// Generic raw signing function
// Takes the incoming operation hex-bytes and signs using whichever wallet type is in use
func (s *BaconSigner) signGeneric(opPrefix prefix, incOpHex, chainID string) (SignOperationOutput, error) {

    // Base bytes of operation; all ops begin with prefix
    var opBytes = opPrefix

    if chainID != "" {

        // Strip off the network watermark (prefix), and then base58 decode the chain id string (ie: NetXUdfLh6Gm88t)
        chainIdBytes := b58cdecode(chainID, networkprefix)
        //fmt.Println("ChainIDByt: ", chainIdBytes)
        //fmt.Println("ChainIDHex: ", hex.EncodeToString(chainIdBytes))

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
    // finalOpHex := hex.EncodeToString(opBytes))
    //fmt.Println("ToSignBytes: ", opBytes)
    //fmt.Println("ToSignByHex: ", finalOpHex)

	edSig := ""
	switch s.SignerType {
	case SIGNER_WALLET:
		sig, err := s.gt_wallet.SignRawBytes(opBytes)  // Returns 'Signature' object
		if err != nil {
			return SignOperationOutput{}, errors.Wrap(err, "Failed wallet signer")
		}
		edSig = sig.ToBase58()
		

	case SIGNER_LEDGER:
		edSig, err = s.lg_wallet.SignBytes(opBytes)  // Returns b58 encoded signature
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
        Signature: decodedSig,
        EDSig: edSig,
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
