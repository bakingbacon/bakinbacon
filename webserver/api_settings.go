package webserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"

	"bakinbacon/notifications"
	"bakinbacon/storage"
)

func (ws *WebServer) saveTelegram(w http.ResponseWriter, r *http.Request) {

	log.Trace("API - SaveTelegram")

	// Read the POST body as a string
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).Error("API SaveTelegram")
		apiError(errors.Wrap(err, "Failed to parse body"), w)

		return
	}

	// Send string to configure for JSON unmarshaling; make sure to save config to db
	if err := notifications.N.Configure("telegram", body, true); err != nil {
		log.WithError(err).Error("API SaveTelegram")
		apiError(errors.Wrap(err, "Failed to configure telegram"), w)

		return
	}

	if err := notifications.N.TestSend("telegram", "Test message from BakinBacon"); err != nil {
		log.WithError(err).Error("API SaveTelegram")
		apiError(errors.Wrap(err, "Failed to execute telegram test"), w)

		return
	}

	apiReturnOk(w)
}

func (ws *WebServer) saveEmail(w http.ResponseWriter, r *http.Request) {
	apiReturnOk(w)
}

func (ws *WebServer) getSettings(w http.ResponseWriter, r *http.Request) {

	log.Trace("API - GetSettings")

	// Get RPC endpoints
	endpoints, err := storage.DB.GetRPCEndpoints()
	if err != nil {
		apiError(errors.Wrap(err, "Cannot get endpoints"), w)

		return
	}
	log.WithField("Endpoints", endpoints).Debug("API Settings Endpoints")

	// Get Notification settings
	notifications, err := notifications.N.GetConfig() // Returns json.RawMessage
	if err != nil {
		apiError(errors.Wrap(err, "Cannot get notification settings"), w)

		return
	}
	log.WithField("Notifications", string(notifications)).Debug("API Settings Notifications")

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"endpoints":     endpoints,
		"notifications": notifications,
	}); err != nil {
		log.WithError(err).Error("UI Return Encode Failure")
	}
}

//
// Adding, Listing, Deleting endpoints
func (ws *WebServer) addEndpoint(w http.ResponseWriter, r *http.Request) {

	log.Trace("API - AddEndpoint")

	k := make(map[string]string)

	if err := json.NewDecoder(r.Body).Decode(&k); err != nil {
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
	ws.baconClient.AddRpc(id, k["rpc"])

	log.WithField("Endpoint", k["rpc"]).Debug("API Added Endpoint")

	apiReturnOk(w)
}

func (ws *WebServer) listEndpoints(w http.ResponseWriter, r *http.Request) {

	log.Trace("API - ListEndpoints")

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

func (ws *WebServer) deleteEndpoint(w http.ResponseWriter, r *http.Request) {

	log.Trace("API - DeleteEndpoint")

	k := make(map[string]int)

	if err := json.NewDecoder(r.Body).Decode(&k); err != nil {
		apiError(errors.Wrap(err, "Cannot decode body for rpc delete"), w)

		return
	}

	// Need to shutdown the RPC client first
	if err := ws.baconClient.ShutdownRpc(k["rpc"]); err != nil {
		log.WithError(err).WithField("Endpoint", k).Error("API DeleteEndpoint")
		apiError(errors.Wrap(err, "Cannot shutdown RPC client for deletion"), w)

		return
	}

	// Then delete from storage
	if err := storage.DB.DeleteRPCEndpoint(k["rpc"]); err != nil {
		log.WithError(err).WithField("Endpoint", k).Error("API DeleteEndpoint")
		apiError(errors.Wrap(err, "Cannot delete endpoint from DB"), w)

		return
	}

	log.WithField("Endpoint", k["rpc"]).Debug("API Deleted Endpoint")

	apiReturnOk(w)
}
