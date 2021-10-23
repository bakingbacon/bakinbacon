package webserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"

	"bakinbacon/storage"
)

func (ws *WebServer) getPayouts(w http.ResponseWriter, r *http.Request) {

	log.Trace("API - getPayouts")

	// Get all rewards metadata from DB
	// Since this is just going straight to browser, DB.GetPayoutsMetadata() returns a bytestring
	// that we just send back to API call
	payoutsMetadata, err := storage.DB.GetPayoutsMetadata()
	if err != nil {
		log.WithError(err).Error("API - getPayouts")
		apiError(errors.Wrap(err, "Unable to get metadata from DB"), w)

		return
	}

	// return raw JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payoutsMetadata); err != nil {
		log.WithError(err).Error("UI Return getPayouts Failure")
	}
}

func (ws *WebServer) getCyclePayouts(w http.ResponseWriter, r *http.Request) {

	log.Trace("API - getCyclePayouts")

	// Get query parameter
	keys := r.URL.Query()
	payoutsCycle, err := strconv.Atoi(keys.Get("c"))
	if err != nil {
		log.WithError(err).Error("Unable to parse cycle")
		apiError(errors.Wrap(err, "Unable to parse cycle"), w)

		return
	}

	// Fetch cycle payout data from DB and return map
	payoutsData, err := storage.DB.GetCyclePayouts(payoutsCycle)
	if err != nil {
		log.WithError(err).Error("API - getCyclePayouts")
		apiError(errors.Wrap(err, "Unable to get cycle payout from DB"), w)

		return
	}

	// return raw JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payoutsData); err != nil {
		log.WithError(err).Error("UI Return getCyclePayouts Failure")
	}
}
