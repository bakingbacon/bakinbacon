package baconsigner

import (
	"strings"

	"github.com/pkg/errors"

	tezosLedger "github.com/bakingbacon/goledger/ledger-apps/tezos"
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

func (s *BaconSigner) IsBakingApp(ledger *tezosLedger.TezosLedger) (string, error) {

	version, err := ledger.GetVersion()
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

//
// This function will open the ledger, get the version string of the running app, and
// fetch either the currently auth'd baking key, or fetch the default BIP path key
func (s *BaconSigner) TestLedger() (*LedgerInfo, error) {

	ledgerInfo := &LedgerInfo{}

	ledger, err := tezosLedger.Get()
	if err != nil {
		log.WithError(err).Error("Unable to open ledger")
		return ledgerInfo, errors.Wrap(err, "Unable to open ledger")
	}
	defer ledger.Close()  // Cleanup USB comms

	version, err := s.IsBakingApp(ledger)
	if err != nil {
		return ledgerInfo, err
	}

	ledgerInfo.Version = version
	log.WithField("Version", ledgerInfo.Version).Info("Ledger Version")

	// Check if ledger is already configured for baking
	ledgerInfo.BipPath = DEFAULT_BIP_PATH;

	bipPath, err := ledger.GetAuthorizedKeyPath()
	if err != nil {
		log.WithError(err).Error("Unable to GetAuthorizedKeyPath")
		return ledgerInfo, errors.Wrap(err, "Unable to query auth path")
	}

	// Check returned path from device
	if bipPath != "" {
		// Ledger is already setup for baking
		ledgerInfo.PrevAuth = true;
		ledgerInfo.BipPath = bipPath
	}

	// Get the key from the path
	if err := ledger.SetBipPath(ledgerInfo.BipPath); err != nil {
		log.WithError(err).Error("Unable to SetBipPath")
		return ledgerInfo, errors.Wrap(err, "Unable to set bip path")
	}

	_, pkh, err := ledger.GetPublicKey()
	if err != nil {
		log.WithError(err).Error("Unable to GetPublicKey")
		return ledgerInfo, err
	}

	ledgerInfo.Pkh = pkh

	return ledgerInfo, nil
}

//
// Ask ledger to display request for public key. User must press button to confirm.
func (s *BaconSigner) ConfirmBakingPkh(pkh, bipPath string) (error) {

	ledger, err := tezosLedger.Get()
	if err != nil {
		log.WithError(err).Error("Unable to open ledger")
		return errors.Wrap(err, "Unable to open ledger")
	}
	defer ledger.Close()

	// Get the key from the path
	if err := ledger.SetBipPath(bipPath); err != nil {
		log.WithError(err).Error("Unable to SetBipPath")
		return errors.Wrap(err, "Unable to set bip path")
	}

	// Ask user to confirm PKH and authorize for baking
	_, authPkh, err := ledger.AuthorizeBaking()
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

	s.BakerPkh = authPkh
	s.LedgerBipPath = bipPath
	s.SignerType = SIGNER_LEDGER

	log.WithFields(log.Fields{
		"BakerPKH": authPkh, "BipPath": bipPath,
	}).Info("Saved authorized baking on ledger")

	// No errors; User confirmed key on device. All is good.
	return nil
}
