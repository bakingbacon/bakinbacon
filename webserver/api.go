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

// Dummy health check
func (ws *WebServer) getHealth(w http.ResponseWriter, r *http.Request) {

	log.Debug("API - GetHealth")

	if err := json.NewEncoder(w).Encode(map[string]bool{
		"ok": true,
	}); err != nil {
		log.WithError(err).Error("Heath Check Failure")
	}
}

// Get current status
func (ws *WebServer) getStatus(w http.ResponseWriter, r *http.Request) {

	log.Debug("API - GetStatus")

	_, pkh, err := storage.DB.GetDelegate()
	if err != nil {
		apiError(errors.Wrap(err, "Cannot get delegate"), w)
		return
	}

	s := struct {
		*baconclient.BaconStatus
		Delegate  string `json:"pkh"`
		Timestamp int64  `json:"ts"`
	}{
		ws.baconClient.Status,
		pkh,
		time.Now().Unix(),
	}

	if err := json.NewEncoder(w).Encode(s); err != nil {
		log.WithError(err).Error("UI Return Encode Failure")
	}
}

//
// Set delegate (from UI config)
func (ws *WebServer) setDelegate(w http.ResponseWriter, r *http.Request) {

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

	apiReturnOk(w)
}
