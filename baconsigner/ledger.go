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
	DEFAULT_BIP_PATH = "/44'/1729'/0'/0'"
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
	ledger  *ledger.TezosLedger
	storage *storage.Storage
	lock    sync.Mutex
}

var L *LedgerSigner

func InitLedgerSigner(db *storage.Storage) error {

	L = &LedgerSigner{
		Info: &LedgerInfo{},
		storage: db,
	}

	// Get device
	dev, err := ledger.Get()
	if err != nil {
		return errors.Wrap(err, "Cannot get ledger device")
	}

	L.ledger = dev

	// Get bipPath and PKH from DB
	pkh, dbBipPath, err := L.storage.GetLedgerConfig()
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
	_, compPkh, err := L.GetPublicKey()
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

// GetPublicKey Gets the public key from ledger device
func (s *LedgerSigner) GetPublicKey() (string, string, error) {

	s.lock.Lock()
	defer s.lock.Unlock()

	// ledger.GetPublicKey returns (pk, pkh, error)
	pk, pkh, err := s.ledger.GetPublicKey()

	return pk, pkh, err
}

func (s *LedgerSigner) SignBytes(opBytes []byte) (string, error) {

	s.lock.Lock()
	defer s.lock.Unlock()

	// Returns b58 encoded signature
	return s.ledger.SignBytes(opBytes)
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

// TestLedger This function is only called from web UI during initial setup.
// It will open the ledger, get the version string of the running app, and
// fetch either the currently auth'd baking key, or fetch the default BIP path key
func TestLedger(db *storage.Storage) (*LedgerInfo, error) {

	L = &LedgerSigner{
		Info: &LedgerInfo{},
		storage: db,
	}

	// Get device
	dev, err := ledger.Get()
	if err != nil {
		return L.Info, errors.Wrap(err, "Cannot get ledger device")
	}
	L.ledger = dev

	version, err := L.IsBakingApp()
	if err != nil {
		return L.Info, err
	}

	L.Info.Version = version
	log.WithField("Version", L.Info.Version).Info("Ledger Version")

	// Check if ledger is already configured for baking
	L.Info.BipPath = DEFAULT_BIP_PATH

	bipPath, err := L.GetAuthorizedKeyPath()
	if err != nil {
		log.WithError(err).Error("Unable to GetAuthorizedKeyPath")
		return L.Info, errors.Wrap(err, "Unable to query auth path")
	}

	// Check returned path from device
	if bipPath != "" {
		// Ledger is already setup for baking
		log.WithField("Path", bipPath).Info("Ledger previously configured for baking")
		L.Info.PrevAuth = true
		L.Info.BipPath = bipPath
	}

	// Get the key from the path
	if err := L.SetBipPath(L.Info.BipPath); err != nil {
		log.WithError(err).Error("Unable to SetBipPath")
		return L.Info, errors.Wrap(err, "Unable to set bip path")
	}

	_, pkh, err := L.GetPublicKey()
	if err != nil {
		log.WithError(err).Error("Unable to GetPublicKey")
		return L.Info, err
	}

	L.Info.Pkh = pkh

	return L.Info, nil
}

//
// ConfirmBakingPkh Ask ledger to display request for public key. User must press button to confirm.
func (s *LedgerSigner) ConfirmBakingPkh(pkh, bipPath string) error {

	log.WithFields(log.Fields{
		"PKH": pkh, "Path": bipPath,
	}).Debug("Confirming Baking PKH")

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
	if err := s.storage.SaveLedgerToDB(authPkh, bipPath, SIGNER_LEDGER); err != nil {
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

// SaveSigner Saves Pkh and BipPath to DB
func (s *LedgerSigner) SaveSigner() error {

	if err := s.storage.SaveLedgerToDB(s.Info.Pkh, s.Info.BipPath, SIGNER_LEDGER); err != nil {
		log.WithError(err).Error("Cannot save key/wallet to db")
		return err
	}

	return nil
}
