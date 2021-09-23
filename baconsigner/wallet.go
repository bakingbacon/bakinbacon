package baconsigner

import (
	"github.com/pkg/errors"

	gtks "github.com/bakingbacon/go-tezos/v4/keys"
	log "github.com/sirupsen/logrus"

	"bakinbacon/storage"
)

type WalletSigner struct {
	storage *storage.Storage
	sk     string
	pkh    string
	wallet *gtks.Key
}

func (s *Access) HydrateWalletSigner() (*WalletSigner, error) {

	w := &WalletSigner{
		storage: s.storage,
	}

	walletSk, err := s.storage.GetSignerSk()
	if err != nil {
		log.WithError(err).Error()
		return nil, errors.Wrap(err, "Unable to get signer sk from DB")
	}

	if walletSk == "" {
		log.WithError(err).Error()
		return nil, errors.New("No wallet secret key found. Cannot bake.")
	}

	// Import key
	wallet, err := gtks.FromBase58(walletSk, gtks.Ed25519)
	if err != nil {
		log.WithError(err).Error()
		return nil, errors.Wrap(err, "Failed to load wallet from secret key")
	}

	w.wallet = wallet
	w.pkh = wallet.PubKey.GetAddress()

	log.WithFields(log.Fields{
		"Baker": w.pkh, "PublicKey": w.wallet.PubKey.GetPublicKey(),
	}).Info("Loaded software wallet")

	return w, nil
}

// GenerateNewKey Generates a new ED25519 keypair; Only used on first setup through
// UI wizard so init the signer here
func (s *WalletSigner) GenerateNewKey() (string, string, error) {

	newKey, err := gtks.Generate(gtks.Ed25519)
	if err != nil {
		log.WithError(err).Error("Failed to generate new key")
		return "", "", errors.Wrap(err, "failed to generate new key")
	}

	s.wallet = newKey
	if err := s.SaveSigner(newKey.GetSecretKey(), newKey.PubKey.GetAddress()); err != nil {
		return "", "", errors.Wrap(err, "could not save new wallet signer")
	}

	return s.sk, s.pkh, nil
}

// ImportSecretKey Imports a secret key, saves to DB, and sets signer type to wallet
func (s *WalletSigner) ImportSecretKey(b58Ed25519Key string) (string, string, error) {

	importKey, err := gtks.FromBase58(b58Ed25519Key, gtks.Ed25519)
	if err != nil {
		log.WithError(err).Error("Failed to import key")
		return "", "", err
	}

	s.wallet = importKey
	if err := s.SaveSigner(b58Ed25519Key, importKey.PubKey.GetAddress()); err != nil {
		return "", "", errors.Wrap(err, "could not save new wallet signer")
	}

	return s.sk, s.pkh, nil
}

// SaveSigner Saves Sk/pkh to DB
func (s *WalletSigner) SaveSigner(sk, pkh string) error {

	if sk == "" {
		log.Warn("sk is empty")
	}
	if pkh == "" {
		log.Warn("pkh is empty")
	}
	s.sk = sk
	s.pkh = pkh

	if err := s.storage.SetDelegate(sk, pkh); err != nil {
		return errors.Wrap(err, "Unable to save key/wallet")
	}

	if err := s.storage.SetSignerType(SIGNER_WALLET); err != nil {
		return errors.Wrap(err, "Unable to save key/wallet")
	}

	if err := s.storage.SetSignerSk(sk); err != nil {
		return errors.Wrap(err, "Unable to save key/wallet")
	}

	log.Info("Saved wallet signer to DB")

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
