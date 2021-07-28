package baconsigner

import (

	"github.com/pkg/errors"

	gtks "github.com/bakingbacon/go-tezos/v4/keys"
	log "github.com/sirupsen/logrus"

	"bakinbacon/storage"
)

// Generates a new ED25519 keypair
func (s *BaconSigner) GenerateNewKey() (string, string, error) {

	newKey, err := gtks.Generate(gtks.Ed25519)
	if err != nil {
		log.WithError(err).Error("Failed to generate new key")
		return "", "", errors.Wrap(err, "failed to generate new key")
	}

	s.bakerSk = newKey.GetSecretKey()
	s.BakerPkh = newKey.PubKey.GetAddress()
	s.SignerType = SIGNER_WALLET

	return s.bakerSk, s.BakerPkh, nil
}

// Imports a secret key, saves to DB, and sets signer type to wallet
func (s *BaconSigner) ImportSecretKey(iEdsk string) (string, string, error) {

	key, err := gtks.FromBase58(iEdsk, gtks.Ed25519)
	if err != nil {
		log.WithError(err).Error("Failed to import key")
		return "", "", err
	}

	s.bakerSk = iEdsk
	s.BakerPkh = key.PubKey.GetAddress()
	s.SignerType = SIGNER_WALLET

	return s.bakerSk, s.BakerPkh, nil
}

// Saves Sk/Pkh to DB
func (s *BaconSigner) SaveKeyWalletTypeToDB() error {

	if err := storage.DB.SetDelegate(s.bakerSk, s.BakerPkh); err != nil {
		return errors.Wrap(err, "Unable to save key/wallet")
	}

	if err := storage.DB.SetSignerType(SIGNER_WALLET); err != nil {
		return errors.Wrap(err, "Unable to save key/wallet")
	}

	return nil
}
