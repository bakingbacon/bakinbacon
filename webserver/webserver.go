package webserver

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"bakinbacon/baconclient"
)

var (
	// Global vars for the webserver package
	httpSvr     *http.Server
	baconClient *baconclient.BaconClient
)

// Embed all UI objects
//go:embed build
var staticUi embed.FS

type ApiError struct {
	Error string `json:"error"`
}

func apiError(err error, w http.ResponseWriter) {
	e, _ := json.Marshal(ApiError{err.Error()})
	http.Error(w, string(e), http.StatusBadRequest)
}

func apiReturnOk(w http.ResponseWriter) {
	if err := json.NewEncoder(w).Encode(map[string]string{"ok": "ok"}); err != nil {
		log.WithError(err).Error("UI Return Encode Failure")
	}
}

func Start(_baconClient *baconclient.BaconClient, bindAddr string, bindPort int, shutdownChannel <-chan interface{}, wg *sync.WaitGroup) {

	// Set the package global
	baconClient = _baconClient

	// Repoint web ui down one directory
	contentStatic, _ := fs.Sub(staticUi, "build")

	// index.html
	indexTemplate, err := template.ParseFS(contentStatic, "index.html")
	if err != nil {
		log.Fatal(err)
	}

	// Set things up
	var router = mux.NewRouter()

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err := indexTemplate.Execute(w, nil); err != nil {
			log.WithError(err).Error("Unable to render index")
		}
	})

	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.HandleFunc("/status", getStatus).Methods("GET")
	apiRouter.HandleFunc("/delegate", setDelegate).Methods("POST")
	apiRouter.HandleFunc("/health", getHealth).Methods("GET")

	// Settings tab
	settingsRouter := apiRouter.PathPrefix("/settings").Subrouter()
	settingsRouter.HandleFunc("/", getSettings).Methods("GET")
	settingsRouter.HandleFunc("/savetelegram", saveTelegram).Methods("POST")
	settingsRouter.HandleFunc("/saveemail", saveEmail).Methods("POST")
	settingsRouter.HandleFunc("/addendpoint", addEndpoint).Methods("POST")
	settingsRouter.HandleFunc("/listendpoints", listEndpoints).Methods("GET")
	settingsRouter.HandleFunc("/deleteendpoint", deleteEndpoint).Methods("POST")

	// Voting tab
	votingRouter := apiRouter.PathPrefix("/voting").Subrouter()
	votingRouter.HandleFunc("/upvote", handleUpvote).Methods("POST", "OPTIONS")

	// Setup wizards
	wizardRouter := apiRouter.PathPrefix("/wizard").Subrouter()
	wizardRouter.HandleFunc("/testLedger", testLedger)
	wizardRouter.HandleFunc("/confirmBakingPkh", confirmBakingPkh)
	wizardRouter.HandleFunc("/generateNewKey", generateNewKey)
	wizardRouter.HandleFunc("/importKey", importSecretKey).Methods("POST", "OPTIONS")
	wizardRouter.HandleFunc("/registerBaker", registerBaker).Methods("POST", "OPTIONS")
	wizardRouter.HandleFunc("/finishwallet", finishWalletWizard)

	// For static content (js, images)
	router.PathPrefix("/static/").Handler(http.FileServer(http.FS(contentStatic)))

	// Make the http server
	httpAddr := fmt.Sprintf("%s:%d", bindAddr, bindPort)
	httpSvr = &http.Server{
		Handler: handlers.CORS(
			handlers.AllowedHeaders([]string{"Content-Type"}),
			handlers.AllowedOrigins([]string{"*"}),
			handlers.AllowedMethods([]string{"GET", "POST", "OPTIONS"}),
		)(router),
		Addr:         httpAddr,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.WithField("Addr", httpAddr).Info("Bakin'Bacon WebUI Listening")

	// Launch webserver in background
	go func() {
		// TODO: SSL for localhost?
		//var err error
		//if wantSSL {
		//	err = httpSvr.ListenAndServeTLS("ssl/cert.pem", "ssl/key.pem")
		//} else {
		//	err = httpSvr.ListenAndServe()
		//}
		if err := httpSvr.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Errorf("Httpserver: ListenAndServe()")
		}

		log.Info("Httpserver: Shutdown")
	}()

	// Wait for shutdown signal on channel
	go func() {
		defer wg.Done()
		<-shutdownChannel

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := httpSvr.Shutdown(ctx); err != nil {
			log.WithError(err).Errorf("Httpserver: Shutdown()")
		}
	}()
}
