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
	DefaultBIPPath = "/44'/1729'/0'/1'"
)

type LedgerInfo struct {
	Version  string `json:"version"`
	PrevAuth bool   `json:"prevAuth"`
	Pkh      string `json:"pkh"`
	BIPPath  string `json:"bipPath"`
}

type LedgerSigner struct {
	Info *LedgerInfo

	// Actual object of the ledger
	ledger *ledger.TezosLedger
	lock   sync.Mutex
}

var L *LedgerSigner

func InitLedgerSigner() error {

	L = new(LedgerSigner)
	L.Info = new(LedgerInfo)

	// Get device
	dev, err := ledger.Get()
	if err != nil {
		return errors.Wrap(err, "Cannot get ledger device")
	}

	L.ledger = dev

	// Get bipPath and PKH from DB
	pkh, dbBIPPath, err := storage.DB.GetLedgerConfig()
	if err != nil {
		return errors.Wrap(err, "Cannot load ledger config from DB")
	}

	// Sanity
	if dbBIPPath == "" {
		return errors.New("No BIP path found in DB. Cannot configure ledger.")
	}

	// Sanity check if wallet app is open instead of baking app
	if _, err := L.IsBakingApp(); err != nil {
		return err
	}

	// Get the bipPath that is authorized to bake
	authBIPPath, err := L.GetAuthorizedKeyPath()
	if err != nil {
		return errors.Wrap(err, "Cannot get auth BIP path from ledger")
	}

	// Compare to DB config for sanity
	if dbBIPPath != authBIPPath {
		return errors.New(fmt.Sprintf("Authorized BIPPath, %s, does not match DB Config, %s", authBIPPath, dbBIPPath))
	}

	// Set dbBIPPath from DB config
	if err := L.SetBipPath(dbBIPPath); err != nil {
		return errors.Wrap(err, "Cannot set BIP path on ledger device")
	}

	// Get the pkh from dbBIPPath from DB config
	_, compPkh, err := L.GetPublicKey()
	if err != nil {
		return errors.Wrap(err, "Cannot fetch pkh from ledger")
	}

	if pkh != compPkh {
		return errors.New(fmt.Sprintf("Authorized PKH, %s, does not match DB Config, %s", compPkh, pkh))
	}

	L.Info.Pkh = pkh
	L.Info.BIPPath = authBIPPath

	log.WithFields(log.Fields{"KeyPath": authBIPPath, "PKH": pkh}).Debug("Ledger Baking Config")

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

// TestLedger
// This function is only called from web UI during initial setup.
// It will open the ledger, get the version string of the running app, and
// fetch either the currently auth'd baking key, or fetch the default BIP path key
func TestLedger() (*LedgerInfo, error) {

	L = new(LedgerSigner)
	L.Info = new(LedgerInfo)

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
	L.Info.BIPPath = DefaultBIPPath

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
		L.Info.BIPPath = bipPath
	}

	// Get the key from the path
	if err := L.SetBipPath(L.Info.BIPPath); err != nil {
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
	if err := storage.DB.SaveLedgerToDB(authPkh, bipPath, SignerLedger); err != nil {
		log.WithError(err).Error("Cannot save key/wallet to db")
		return err
	}

	s.Info.Pkh = authPkh
	s.Info.BIPPath = bipPath

	log.WithFields(log.Fields{
		"BakerPKH": authPkh, "BIPPath": bipPath,
	}).Info("Saved authorized baking on ledger")

	// No errors; User confirmed key on device. All is good.
	return nil
}

// SaveSigner Saves Sk/Pkh to DB
func (s *LedgerSigner) SaveSigner() error {
	if err := storage.DB.SaveLedgerToDB(s.Info.Pkh, s.Info.BIPPath, SignerLedger); err != nil {
		log.WithError(err).Error("Cannot save key/wallet to db")
		return err
	}

	return nil
}
