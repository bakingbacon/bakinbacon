package webserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"

	"bakinbacon/baconclient"
	"bakinbacon/storage"
)

//
// Generate new key
// Save generated key to database, and set signer type to wallet
// ------------------------------------------------------------------------------------
func generateNewKey(w http.ResponseWriter, r *http.Request) {

	log.Debug("API - generateNewKey")

	// Generate new key temporarily
	newEdsk, newPkh, err := baconClient.Signer.GenerateNewKey()
	if err != nil {
		apiError(errors.Wrap(err, "Cannot generate new key"), w)
		return
	}

	log.WithFields(log.Fields{
		"EDSK": newEdsk, "PKH": newPkh,
	}).Info("Generated new key-pair")

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
// ------------------------------------------------------------------------------------

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

	log.WithFields(log.Fields{
		"EDSK": edsk, "PKH": pkh,
	}).Info("Imported secret key-pair")

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
// ------------------------------------------------------------------------------------
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
// Finish wizard
// This API saves the generated, or imported, secret key to the DB and saves the signer method
func finishWizard(w http.ResponseWriter, r *http.Request) {

	log.Debug("API - FinishWizard")

	if err := baconClient.Signer.SaveKeyWalletTypeToDB(); err != nil {
		apiError(errors.Wrap(err, "Cannot save key/wallet to db"), w)
		return
	}

	// Return to UI
	if err := json.NewEncoder(w).Encode(map[string]string{
		"ok": "ok",
	}); err != nil {
		log.WithError(err).Error("UI Return Encode Failure")
	}
}

//
// Get current status
// ------------------------------------------------------------------------------------
func getStatus(w http.ResponseWriter, r *http.Request) {

	log.Debug("API - GetStatus")

	s := struct {
		*baconclient.BaconStatus
		Ts int64 `json:"ts"`
	}{
		baconClient.Status,
		time.Now().Unix(),
	}

	if err := json.NewEncoder(w).Encode(s); err != nil {
		log.WithError(err).Error("UI Return Encode Failure")
	}
}

//
// Get the address of the delegate
// ------------------------------------------------------------------------------------
func getDelegate(w http.ResponseWriter, r *http.Request) {

	_, pkh, err := storage.DB.GetDelegate()
	if err != nil {
		apiError(errors.Wrap(err, "Cannot get delegate"), w)
		return
	}

	log.WithField("Delegate", pkh).Debug("API - GetDelegate")

	if err := json.NewEncoder(w).Encode(map[string]string{
		"pkh": pkh,
	}); err != nil {
		log.WithError(err).Error("UI Return Encode Failure")
	}
}

//
// Set delegate (from UI config)
// ------------------------------------------------------------------------------------
func setDelegate(w http.ResponseWriter, r *http.Request) {

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		apiError(errors.Wrap(err, "Cannot read set delegate"), w)
		return
	}

	pkh := string(body)

	// No esdk if using ledger
	if err := storage.DB.SetDelegate("", pkh); err != nil {
		apiError(errors.Wrap(err, "Cannot set delegate"), w)
		return
	}

	log.WithField("PKH", pkh).Debug("API - SetDelegate")

	if err := json.NewEncoder(w).Encode(map[string]bool{
		"ok": true,
	}); err != nil {
		log.WithError(err).Error("UI Return Encode Failure")
	}
}

//
// Adding, Listing, Deleting endpoints
// ------------------------------------------------------------------------------------

func addEndpoint(w http.ResponseWriter, r *http.Request) {

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		apiError(errors.Wrap(err, "Cannot read add endpoint"), w)
		return
	}

	if e := storage.DB.AddRPCEndpoint(string(body)); e != nil {
		log.WithError(e).WithField("Endpoint", string(body)).Error("API AddEndpoint")
		apiError(errors.Wrap(err, "Cannot add endpoint to DB"), w)

		return
	}

	log.WithField("Endpoint", string(body)).Debug("API AddEndpoint")

	if err := json.NewEncoder(w).Encode(map[string]bool{
		"ok": true,
	}); err != nil {
		log.WithError(err).Error("UI Return Encode Failure")
	}
}

func listEndpoints(w http.ResponseWriter, r *http.Request) {

	endpoints, err := storage.DB.GetRPCEndpoints()
	if err != nil {
		apiError(errors.Wrap(err, "Cannot get endpoints"), w)
		return
	}

	log.WithField("Endpoints", endpoints).Debug("API ListEndpoints")

	if err := json.NewEncoder(w).Encode(map[string][]string{
		"endpoints": endpoints,
	}); err != nil {
		log.WithError(err).Error("UI Return Encode Failure")
	}
}

// TODO: Delete
