package baconsigner

import (
	"fmt"
	"strings"
	"sync"

	"github.com/pkg/errors"

	ledger "github.com/bakingbacon/goledger/ledger-apps/tezos"
	log "github.com/sirupsen/logrus"

	"bakinbacon/storage"
)

const (
	DEFAULT_BIP_PATH = "/44'/1729'/0'/1'"
)

type LedgerInfo struct {
	Version  string `json:"version"`
	PrevAuth bool   `json:"prevAuth"`
	Pkh      string `json:"pkh"`
	BipPath  string `json:"bipPath"`
}

type LedgerSigner struct {
	Info   *LedgerInfo

	// Actual object of the ledger
	ledger *ledger.TezosLedger
	lock sync.Mutex
}

var L *LedgerSigner

func InitLedgerSigner() error {

	L = &LedgerSigner{}
	L.Info = &LedgerInfo{}

	// Get device
	dev, err := ledger.Get()
	if err != nil {
		return errors.Wrap(err, "Cannot get ledger device")
	}

	L.ledger = dev

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
	if _, err := L.IsBakingApp(); err != nil {
		return err
	}

	// Get the bipPath that is authorized to bake
	authBipPath, err := L.GetAuthorizedKeyPath()
	if err != nil {
		return errors.Wrap(err, "Cannot get auth BIP path from ledger")
	}

	// Compare to DB config for sanity
	if dbBipPath != authBipPath {
		return errors.New(fmt.Sprintf("Authorized BipPath, %s, does not match DB Config, %s", authBipPath, dbBipPath))
	}

	// Set dbBipPath from DB config
	if err := L.SetBipPath(dbBipPath); err != nil {
		return errors.Wrap(err, "Cannot set BIP path on ledger device")
	}

	// Get the pkh from dbBipPath from DB config
	compPkh, err := L.GetPublicKey()
	if err != nil {
		return errors.Wrap(err, "Cannot fetch pkh from ledger")
	}

	if pkh != compPkh {
		return errors.New(fmt.Sprintf("Authorized PKH, %s, does not match DB Config, %s", compPkh, pkh))
	}

	L.Info.Pkh = pkh
	L.Info.BipPath = authBipPath

	log.WithFields(log.Fields{"KeyPath": authBipPath, "PKH": pkh}).Debug("Ledger Baking Config")

	return nil
}

func (s *LedgerSigner) Close() {

	s.lock.Lock()
	defer s.lock.Unlock()

	s.ledger.Close()
}

// Gets the public key from ledger device
func (s *LedgerSigner) GetPublicKey() (string, error) {

	s.lock.Lock()
	defer s.lock.Unlock()

	// ledger.GetPublicKey returns (pk, pkh, error)
	_, pkh, err := s.ledger.GetPublicKey()

	return pkh, err
}

func (s *LedgerSigner) SignBytes(opBytes []byte) (string, error) {

	s.lock.Lock()
	defer s.lock.Unlock()

	return s.ledger.SignBytes(opBytes) // Returns b58 encoded signature
}

func (s *LedgerSigner) IsBakingApp() (string, error) {

	s.lock.Lock()
	defer s.lock.Unlock()

	version, err := s.ledger.GetVersion()
	if err != nil {
		log.WithError(err).Error("Unable to GetVersion")
		return "", errors.Wrap(err, "Unable to get app version")
	}

	// Check if baking or wallet app is open
	if strings.HasPrefix(version, "Wallet") {
		return "", errors.New("The Tezos Wallet app is currently open. Please close it and open the Tezos Baking app.")
	}

	return version, nil
}

func (s *LedgerSigner) GetAuthorizedKeyPath() (string, error) {

	s.lock.Lock()
	defer s.lock.Unlock()

	return s.ledger.GetAuthorizedKeyPath()
}

func (s *LedgerSigner) SetBipPath(p string) error {

	s.lock.Lock()
	defer s.lock.Unlock()

	return s.ledger.SetBipPath(p)
}

//
// This function will open the ledger, get the version string of the running app, and
// fetch either the currently auth'd baking key, or fetch the default BIP path key
func (s *LedgerSigner) TestLedger() (interface{}, error) {

	version, err := s.IsBakingApp()
	if err != nil {
		return s.Info, err
	}

	s.Info.Version = version
	log.WithField("Version", s.Info.Version).Info("Ledger Version")

	// Check if ledger is already configured for baking
	s.Info.BipPath = DEFAULT_BIP_PATH

	bipPath, err := s.GetAuthorizedKeyPath()
	if err != nil {
		log.WithError(err).Error("Unable to GetAuthorizedKeyPath")
		return s.Info, errors.Wrap(err, "Unable to query auth path")
	}

	// Check returned path from device
	if bipPath != "" {
		// Ledger is already setup for baking
		s.Info.PrevAuth = true
		s.Info.BipPath = bipPath
	}

	// Get the key from the path
	if err := s.SetBipPath(s.Info.BipPath); err != nil {
		log.WithError(err).Error("Unable to SetBipPath")
		return s.Info, errors.Wrap(err, "Unable to set bip path")
	}

	pkh, err := s.GetPublicKey()
	if err != nil {
		log.WithError(err).Error("Unable to GetPublicKey")
		return s.Info, err
	}

	s.Info.Pkh = pkh

	return s.Info, nil
}

//
// Ask ledger to display request for public key. User must press button to confirm.
func (s *LedgerSigner) ConfirmBakingPkh(pkh, bipPath string) error {

	s.lock.Lock()
	defer s.lock.Unlock()

	// Get the key from the path
	if err := s.SetBipPath(bipPath); err != nil {
		log.WithError(err).Error("Unable to SetBipPath")
		return errors.Wrap(err, "Unable to set bip path")
	}

	// Ask user to confirm PKH and authorize for baking
	_, authPkh, err := s.ledger.AuthorizeBaking()
	if err != nil {
		log.WithError(err).Error("Unable to AuthorizeBaking")
		return errors.Wrap(err, "Unable to authorize baking on device")
	}

	// Minor sanity check
	if pkh != authPkh {
		log.WithFields(log.Fields{
			"AuthPKH": authPkh, "PKH": pkh,
		}).Error("PKH and authorized PKH do not match.")
		return errors.New("PKH and authorized PKH do not match.")
	}

	// Save config to DB
	if err := storage.DB.SaveLedgerToDB(authPkh, bipPath, SIGNER_LEDGER); err != nil {
		log.WithError(err).Error("Cannot save key/wallet to db")
		return err
	}

	s.Info.Pkh = authPkh
	s.Info.BipPath = bipPath

	log.WithFields(log.Fields{
		"BakerPKH": authPkh, "BipPath": bipPath,
	}).Info("Saved authorized baking on ledger")

	// No errors; User confirmed key on device. All is good.
	return nil
}

// Saves Sk/Pkh to DB
func (s *LedgerSigner) SaveSigner() error {

	if err := storage.DB.SaveLedgerToDB(s.Info.Pkh, s.Info.BipPath, SIGNER_LEDGER); err != nil {
		log.WithError(err).Error("Cannot save key/wallet to db")
		return err
	}

	return nil
}
