package baconsigner

import (
	"github.com/pkg/errors"

	gtks "github.com/bakingbacon/go-tezos/v4/keys"
	log "github.com/sirupsen/logrus"

	"bakinbacon/storage"
)

type WalletSigner struct {
	sk     string
	Pkh    string
	wallet *gtks.Key
}

var W *WalletSigner

func InitWalletSigner() error {

	W = &WalletSigner{}

	walletSk, err := storage.DB.GetSignerSk()
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
	W.Pkh = wallet.PubKey.GetAddress()

	log.WithFields(log.Fields{
		"Baker": W.Pkh, "PublicKey": W.wallet.PubKey.GetPublicKey(),
	}).Info("Loaded software wallet")

	return nil
}

// Generates a new ED25519 keypair; Only used on first setup through
// UI wizard so init the signer here
func GenerateNewKey() (string, string, error) {

	W = &WalletSigner{}

	newKey, err := gtks.Generate(gtks.Ed25519)
	if err != nil {
		log.WithError(err).Error("Failed to generate new key")
		return "", "", errors.Wrap(err, "failed to generate new key")
	}

	W.sk = newKey.GetSecretKey()
	W.Pkh = newKey.PubKey.GetAddress()

	return W.sk, W.Pkh, nil
}

// Imports a secret key, saves to DB, and sets signer type to wallet
func ImportSecretKey(iEdsk string) (string, string, error) {

	W = &WalletSigner{}

	key, err := gtks.FromBase58(iEdsk, gtks.Ed25519)
	if err != nil {
		log.WithError(err).Error("Failed to import key")
		return "", "", err
	}

	W.sk = iEdsk
	W.Pkh = key.PubKey.GetAddress()

	return W.sk, W.Pkh, nil
}

// Saves Sk/Pkh to DB
func (s *WalletSigner) SaveSigner() error {

	if err := storage.DB.SetDelegate(s.sk, s.Pkh); err != nil {
		return errors.Wrap(err, "Unable to save key/wallet")
	}

	if err := storage.DB.SetSignerType(SIGNER_WALLET); err != nil {
		return errors.Wrap(err, "Unable to save key/wallet")
	}

	return nil
}

func (s *WalletSigner) SignBytes(opBytes []byte) (string, error) {

	sig, err := s.wallet.SignRawBytes(opBytes) // Returns 'Signature' object
	if err != nil {
		return "", errors.Wrap(err, "Failed wallet signer")
	}

	return sig.ToBase58(), nil
}

func (s *WalletSigner) GetPublicKey() (string, error) {
	return s.Pkh, nil
}
