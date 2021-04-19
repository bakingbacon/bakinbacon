package webserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"
	
	log "github.com/sirupsen/logrus"
	
	"bakinbacon/storage"
	"bakinbacon/baconclient"
)

//
// Generate new key
// Save generated key to database, and set signer type to wallet
// ------------------------------------------------------------------------------------
func generateNewKey(w http.ResponseWriter, r *http.Request) {
	
	log.Debug("API - generateNewKey")
	
	// Generate new key; Saves to DB
	newEdsk, newPkh, err := baconClient.Signer.GenerateNewKey()
	if err != nil {
    	e, _ := json.Marshal(ApiError{ err.Error() })
        http.Error(w, string(e), http.StatusBadRequest)
    	return
	}
	
	if err := baconClient.Signer.SetSignerTypeWallet(); err != nil {
	    e, _ := json.Marshal(ApiError{ err.Error() })
        http.Error(w, string(e), http.StatusBadRequest)
    	return
	}
	
	log.WithFields(log.Fields{
		"EDSK": newEdsk, "PKH": newPkh,
	}).Info("Generated new key-pair")
	
	// Return back to UI
	json.NewEncoder(w).Encode(map[string]string{
		"edsk": newEdsk,
		"pkh": newPkh,
	})
	
	return
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
    	e, _ := json.Marshal(ApiError{ err.Error() })
        http.Error(w, string(e), http.StatusBadRequest)
        return
    }
    
    // Imports and saves to DB
    edsk, pkh, err := baconClient.Signer.ImportSecretKey(k["edsk"])
    if err != nil {
    	e, _ := json.Marshal(ApiError{ err.Error() })
        http.Error(w, string(e), http.StatusBadRequest)
        return
    }
    
    if err := baconClient.Signer.SetSignerTypeWallet(); err != nil {
    	e, _ := json.Marshal(ApiError{ err.Error() })
    	http.Error(w, string(e), http.StatusBadRequest)
    	return
	}
    
    log.WithFields(log.Fields{
		"EDSK": edsk, "PKH": pkh,
	}).Info("Imported secret key-pair")
	
	// Return back to UI
	json.NewEncoder(w).Encode(map[string]string{
		"edsk": edsk,
		"pkh": pkh,
	})
	
	return
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
		e, _ := json.Marshal(ApiError{ err.Error() })
    	http.Error(w, string(e), http.StatusBadRequest)
    	return
	}
	
	log.WithFields(log.Fields{
		"OpHash": opHash,
	}).Info("Injected registration operation")
	
	// Return to UI
	json.NewEncoder(w).Encode(map[string]string{
		"ophash": opHash,
	})
	
	return
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

	j := json.NewEncoder(w)
	j.Encode(s)
}

//
// Get the address of the delegate
// ------------------------------------------------------------------------------------
func getDelegate(w http.ResponseWriter, r *http.Request) {

	_, pkh := storage.DB.GetDelegate()
	log.WithField("Delegate", pkh).Debug("API - GetDelegate")
	json.NewEncoder(w).Encode(map[string]string{"pkh": pkh})
}

//
// Set delegate (from UI config)
// ------------------------------------------------------------------------------------
func setDelegate(w http.ResponseWriter, r *http.Request) {
	j := json.NewEncoder(w)
	body, err := ioutil.ReadAll(r.Body)
    if err != nil {
    	j.Encode(map[string]string{"error": err.Error()})
    	return
    }
    
    pkh := string(body)
	storage.DB.SetDelegate("", pkh)  // No esdk if using ledger
	log.WithField("PKH", pkh).Debug("API - SetDelegate")
	j.Encode(map[string]bool{"ok": true})
}

//
// Adding, Listing, Deleting endpoints
// ------------------------------------------------------------------------------------

func addEndpoint(w http.ResponseWriter, r *http.Request) {

	j := json.NewEncoder(w)
	
	body, err := ioutil.ReadAll(r.Body)
    if err != nil {
    	j.Encode(map[string]string{"error": err.Error()})
    	return
    }

	if e := storage.DB.AddRPCEndpoint(string(body)); e != nil {
		log.WithError(e).WithField("Endpoint", string(body)).Error("API AddEndpoint")
		j.Encode(map[string]string{"error": err.Error()})
		return
	}
	
	log.WithField("Endpoint", string(body)).Debug("API AddEndpoint")
	j.Encode(map[string]bool{"ok": true})
}

func listEndpoints(w http.ResponseWriter, r *http.Request) {

	endpoints, err := storage.DB.GetRPCEndpoints()
	if err != nil {
		log.WithError(err).Error("Unable to get endpoints")
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	
	log.WithField("Endpoints", endpoints).Debug("API ListEndpoints")
	json.NewEncoder(w).Encode(map[string][]string{"endpoints": endpoints})
}

// TODO: Delete
