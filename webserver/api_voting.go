package webserver

import (
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"
)

//
// Set delegate (from UI config)
func handleUpvote(w http.ResponseWriter, r *http.Request) {

	log.Debug("API - handleUpvote")

	// CORS crap; Handle OPTION preflight check
	if r.Method == http.MethodOptions {
		return
	}

	var k map[string]interface{}

	err := json.NewDecoder(r.Body).Decode(&k)
	if err != nil {
		apiError(errors.Wrap(err, "Cannot decode body for voting parameters"), w)
		return
	}

	proposal := k["p"].(string)
	period := int(k["i"].(float64))

	opHash, err := baconClient.UpvoteProposal(proposal, period)
	if err != nil {
		apiError(errors.Wrap(err, "Cannot cast upvote"), w)
		return
	}

	log.WithFields(log.Fields{
		"OpHash": opHash,
	}).Info("Injected voting operation")

	// Return to UI
	if err := json.NewEncoder(w).Encode(map[string]string{
		"ophash": opHash,
	}); err != nil {
		log.WithError(err).Error("UI Return Encode Failure")
	}

	apiReturnOk(w)
}
