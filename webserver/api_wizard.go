package webserver

import (
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"
)

//
// Test existence of ledger device and get app version (Step 1)
func testLedger(w http.ResponseWriter, r *http.Request) {

	log.Debug("API - testLedger")

	ledgerInfo, err := baconClient.Signer.TestLedger()
	if err != nil {
		apiError(errors.Wrap(err, "Unable to access ledger"), w)
		return
	}

	// Return back to UI
	if err := json.NewEncoder(w).Encode(ledgerInfo); err != nil {
		log.WithError(err).Error("UI Return Encode Failure")
	}
}

//
// Confirm the current bipPath and associated key
func confirmBakingPkh(w http.ResponseWriter, r *http.Request) {

	log.Debug("API - confirmBakingPkh")

	var k map[string]string

	err := json.NewDecoder(r.Body).Decode(&k)
	if err != nil {
		apiError(errors.Wrap(err, "Cannot decode body for bipPath"), w)
		return
	}

	// Confirming will prompt user on device to push button,
	// also saves config to DB on success
	err = baconClient.Signer.ConfirmBakingPkh(k["pkh"], k["bp"])
	if err != nil {
		apiError(err, w)
		return
	}

	// Update bacon status so when user refreshes page it is updated
	// non-silent checks (silent = false)
	_ = baconClient.CanBake(false)

	// Return to UI
	apiReturnOk(w)
}

//
// Generate new key
// Save generated key to database, and set signer type to wallet
func generateNewKey(w http.ResponseWriter, r *http.Request) {

	log.Debug("API - generateNewKey")

	// Generate new key temporarily
	newEdsk, newPkh, err := baconClient.Signer.GenerateNewKey()
	if err != nil {
		apiError(errors.Wrap(err, "Cannot generate new key"), w)
		return
	}

	log.WithField("PKH", newPkh).Info("Generated new key-pair")

	// Return back to UI
	if err := json.NewEncoder(w).Encode(map[string]string{
		"edsk": newEdsk,
		"pkh":  newPkh,
	}); err != nil {
		log.WithError(err).Error("UI Return Encode Failure")
	}
}

//
// Import a secret key
// Save imported key to database, and set signer type to wallet
func importSecretKey(w http.ResponseWriter, r *http.Request) {

	log.Debug("API - importSecretKey")

	// CORS crap; Handle OPTION preflight check
	if r.Method == http.MethodOptions {
		return
	}

	var k map[string]string

	err := json.NewDecoder(r.Body).Decode(&k)
	if err != nil {
		apiError(errors.Wrap(err, "Cannot decode body for secret key import"), w)
		return
	}

	// Imports key temporarily
	edsk, pkh, err := baconClient.Signer.ImportSecretKey(k["edsk"])
	if err != nil {
		apiError(errors.Wrap(err, "Cannot import secret key"), w)
		return
	}

	log.WithField("PKH", pkh).Info("Imported secret key-pair")

	// Return back to UI
	if err := json.NewEncoder(w).Encode(map[string]string{
		"edsk": edsk,
		"pkh":  pkh,
	}); err != nil {
		log.WithError(err).Error("UI Return Encode Failure")
	}
}

//
// Call baconClient.RegisterBaker() to construct and inject registration operation.
// This will also check if reveal is needed.
func registerBaker(w http.ResponseWriter, r *http.Request) {

	log.Debug("API - registerbaker")

	// CORS crap; Handle OPTION preflight check
	if r.Method == http.MethodOptions {
		return
	}

	opHash, err := baconClient.RegisterBaker()
	if err != nil {
		apiError(errors.Wrap(err, "Cannot register baker"), w)
		return
	}

	log.WithFields(log.Fields{
		"OpHash": opHash,
	}).Info("Injected registration operation")

	// Return to UI
	if err := json.NewEncoder(w).Encode(map[string]string{
		"ophash": opHash,
	}); err != nil {
		log.WithError(err).Error("UI Return Encode Failure")
	}
}

//
// Finish wallet wizard
// This API saves the generated, or imported, secret key to the DB and saves the signer method
func finishWalletWizard(w http.ResponseWriter, r *http.Request) {

	log.Debug("API - FinishWalletWizard")

	if err := baconClient.Signer.SaveKeyWalletTypeToDB(); err != nil {
		apiError(errors.Wrap(err, "Cannot save key/wallet to db"), w)
		return
	}

	// Return to UI
	apiReturnOk(w)
}
