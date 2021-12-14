package webserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"
)

func (ws *WebServer) getPayouts(w http.ResponseWriter, r *http.Request) {

	log.Trace("API - getPayouts")

	payoutsData := make(map[string]interface{}, 2)
	payoutsData["status"] = "ok"

	// Check if payouts are disabled
	if ws.payoutsHandler.Disabled {
		payoutsData["status"] = "disabled"
	}

	// Get all rewards metadata from DB
	payoutsMetadata, err := ws.payoutsHandler.GetPayoutsMetadataAll()
	if err != nil {
		log.WithError(err).Error("API - getPayouts")
		apiError(errors.Wrap(err, "Unable to get metadata from DB"), w)

		return
	}

	payoutsData["metadata"] = payoutsMetadata

	// return raw JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payoutsData); err != nil {
		log.WithError(err).Error("UI Return getPayouts Failure")
	}
}

// getCyclePayouts will return a map[string] containing the cycle's metadata,
// and the individual rewards payouts data
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

	// Fetch cycle metadata from DB
	cycleMetadata, err := ws.payoutsHandler.GetRewardMetadataForCycle(payoutsCycle)
	if err != nil {
		log.WithError(err).Error("API - getCyclePayouts")
		apiError(errors.Wrap(err, "Unable to get cycle metadata from DB"), w)

		return
	}

	// Fetch cycle payout data from DB
	payoutsData, err := ws.payoutsHandler.GetDelegatorRewardAllForCycle(payoutsCycle)
	if err != nil {
		log.WithError(err).Error("API - getCyclePayouts")
		apiError(errors.Wrap(err, "Unable to get cycle payout from DB"), w)

		return
	}

	cyclePayoutsData := make(map[string]interface{}, 2)
	cyclePayoutsData["metadata"] = cycleMetadata
	cyclePayoutsData["payouts"] = payoutsData

	// return raw JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cyclePayoutsData); err != nil {
		log.WithError(err).Error("UI Return getCyclePayouts Failure")
	}
}

func (ws *WebServer) sendCyclePayouts(w http.ResponseWriter, r *http.Request) {

	log.Trace("API - sendCyclePayouts")

	// Get query parameter
	k := make(map[string]int)
	if err := json.NewDecoder(r.Body).Decode(&k); err != nil {
		apiError(errors.Wrap(err, "Cannot decode body for sendCyclePayouts"), w)

		return
	}

	payoutsCycle, ok := k["cycle"]
	if !ok {
		apiError(errors.New("missing cycle parameter for sending payouts"), w)

		return
	}

	// Execute the payouts process
	if err := ws.payoutsHandler.SendCyclePayouts(payoutsCycle); err != nil {
		log.WithError(err).Error("Unable send cycle payouts")
		apiError(errors.Wrap(err, "Unable send cycle payouts"), w)

		return
	}

	apiReturnOk(w)
}
