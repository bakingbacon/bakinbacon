package baconsigner

import (
	"github.com/pkg/errors"

	gtks "github.com/bakingbacon/go-tezos/v4/keys"
	log "github.com/sirupsen/logrus"

	"bakinbacon/storage"
)

type WalletSigner struct {
	sk      string
	pkh     string
	wallet  *gtks.Key
	storage *storage.Storage
}

var W *WalletSigner

func InitWalletSigner(db *storage.Storage) error {

	W = &WalletSigner{
		storage: db,
	}

	walletSk, err := W.storage.GetSignerSk()
	if err != nil {
		return errors.Wrap(err, "Unable to get signer sk from DB")
	}

	if walletSk == "" {
		return errors.New("No wallet secret key found. Cannot bake.")
	}

	// Import key
	wallet, err := gtks.FromBase58(walletSk, gtks.Ed25519)
	if err != nil {
		return errors.Wrap(err, "Failed to load wallet from secret key")
	}

	W.wallet = wallet
	W.pkh = wallet.PubKey.GetAddress()

	log.WithFields(log.Fields{
		"Baker": W.pkh, "PublicKey": W.wallet.PubKey.GetPublicKey(),
	}).Info("Loaded software wallet")

	return nil
}

// GenerateNewKey Generates a new ED25519 keypair; Only used on first setup through UI wizard so init the signer here
func GenerateNewKey() (string, string, error) {

	W = &WalletSigner{}

	newKey, err := gtks.Generate(gtks.Ed25519)
	if err != nil {
		log.WithError(err).Error("Failed to generate new key")
		return "", "", errors.Wrap(err, "failed to generate new key")
	}

	W.wallet = newKey
	W.sk = newKey.GetSecretKey()
	W.pkh = newKey.PubKey.GetAddress()

	if err := W.SaveSigner(); err != nil {
		return "", "", errors.Wrap(err, "Could not save generated key")
	}

	return W.sk, W.pkh, nil
}

// ImportSecretKey Imports a secret key, saves to DB, and sets signer type to wallet
func ImportSecretKey(iEdsk string) (string, string, error) {

	W = &WalletSigner{}

	importKey, err := gtks.FromBase58(iEdsk, gtks.Ed25519)
	if err != nil {
		log.WithError(err).Error("Failed to import key")
		return "", "", err
	}

	W.wallet = importKey
	W.sk = iEdsk
	W.pkh = importKey.PubKey.GetAddress()

	if err := W.SaveSigner(); err != nil {
		return "", "", errors.Wrap(err, "Could not save imported key")
	}

	return W.sk, W.pkh, nil
}

// Saves Sk/Pkh to DB
func (s *WalletSigner) SaveSigner() error {

	if err := s.storage.SetDelegate(s.sk, s.pkh); err != nil {
		return errors.Wrap(err, "Unable to save key/wallet")
	}

	if err := s.storage.SetSignerType(SIGNER_WALLET); err != nil {
		return errors.Wrap(err, "Unable to save key/wallet")
	}

	return nil
}

func (s *WalletSigner) SignBytes(opBytes []byte) (string, error) {

	// Returns 'Signature' object
	sig, err := s.wallet.SignRawBytes(opBytes)
	if err != nil {
		return "", errors.Wrap(err, "Failed wallet signer")
	}

	return sig.ToBase58(), nil
}

func (s *WalletSigner) GetPublicKey() (string, string, error) {
	return s.wallet.PubKey.GetPublicKey(), s.pkh, nil
}
