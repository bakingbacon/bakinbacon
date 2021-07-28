package webserver

import (
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"

	"bakinbacon/storage"
)

//
// Adding, Listing, Deleting endpoints
func addEndpoint(w http.ResponseWriter, r *http.Request) {

	log.Trace("API - addEndpoint")

	var k map[string]string

	err := json.NewDecoder(r.Body).Decode(&k)
	if err != nil {
		apiError(errors.Wrap(err, "Cannot decode body for rpc add"), w)
		return
	}

	// Save new RPC to db to get id
	id, err := storage.DB.AddRPCEndpoint(k["rpc"])
	if err != nil {
		log.WithError(err).WithField("Endpoint", k).Error("API AddEndpoint")
		apiError(errors.Wrap(err, "Cannot add endpoint to DB"), w)

		return
	}

	// Init new bacon watcher for this RPC
	baconClient.AddRpc(id, k["rpc"])

	log.WithField("Endpoint", k["rpc"]).Debug("API Added Endpoint")

	apiReturnOk(w)
}

func listEndpoints(w http.ResponseWriter, r *http.Request) {

	log.Trace("API - listEndpoints")

	endpoints, err := storage.DB.GetRPCEndpoints()
	if err != nil {
		apiError(errors.Wrap(err, "Cannot get endpoints"), w)
		return
	}

	log.WithField("Endpoints", endpoints).Debug("API List Endpoints")

	if err := json.NewEncoder(w).Encode(map[string]map[int]string{
		"endpoints": endpoints,
	}); err != nil {
		log.WithError(err).Error("UI Return Encode Failure")
	}
}

func deleteEndpoint(w http.ResponseWriter, r *http.Request) {

	log.Trace("API - deleteEndpoint")

	var k map[string]int

	err := json.NewDecoder(r.Body).Decode(&k)
	if err != nil {
		apiError(errors.Wrap(err, "Cannot decode body for rpc delete"), w)
		return
	}

	// Need to shutdown the RPC client first
	if e := baconClient.ShutdownRpc(k["rpc"]); e != nil {
		log.WithError(e).WithField("Endpoint", k).Error("API DeleteEndpoint")
		apiError(errors.Wrap(err, "Cannot shutdown RPC client for deletion"), w)

		return
	}

	// Then delete from storage
	if e := storage.DB.DeleteRPCEndpoint(k["rpc"]); e != nil {
		log.WithError(e).WithField("Endpoint", k).Error("API DeleteEndpoint")
		apiError(errors.Wrap(err, "Cannot delete endpoint from DB"), w)

		return
	}

	log.WithField("Endpoint", k["rpc"]).Debug("API Deleted Endpoint")

	apiReturnOk(w)
}
