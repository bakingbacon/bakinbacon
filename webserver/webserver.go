package webserver

import (
	"bakinbacon/notifications"
	"bakinbacon/storage"
	"bakinbacon/util"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
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
	// Embed all UI objects
	//go:embed build
	staticUI embed.FS
)

type errorWrapper struct {
	Error string `json:"error"`
}

type TemplateVars struct {
	Network        string
	BlocksPerCycle int
	MinBlockTime   int
	UIBaseURL      string
}

type WebServer struct {
	// Global vars for the webserver package
	httpSvr             *http.Server
	baconClient         *baconclient.BaconClient
	network             string
	notificationHandler *notifications.NotificationHandler
	storage             *storage.Storage
}

type WebServerArgs struct {
	Network             string
	Client              *baconclient.BaconClient
	NotificationHandler *notifications.NotificationHandler
	Storage             *storage.Storage

	BindAddr        string
	BindPort        int
	TemplateVars    TemplateVars
	ShutdownChannel <-chan interface{}
	WG              *sync.WaitGroup
}

func (a *WebServerArgs) Validate() error {
	if !util.IsValidNetwork(a.Network) {
		return errors.Errorf("Network not recognized: %s", a.Network)
	}
	if a.Client == nil {
		return errors.New("bacon client not instantiated")
	}
	if a.Storage == nil {
		return errors.New("storage is not instantiated")
	}
	if a.BindAddr == "" {
		return errors.New("bind addr empty")
	}
	if a.ShutdownChannel == nil {
		return errors.New("shutdown channel not instantiated")
	}
	if a.WG == nil {
		return errors.New("wait group not instantiated")
	}
	if a.NotificationHandler == nil {
		log.Warn("no notification handler set")
	}
	return nil
}

func Start(args WebServerArgs) (*WebServer, error) {
	if err := args.Validate(); err != nil {
		return nil, errors.Wrap(err, "could not start web server")
	}
	ws := &WebServer{
		baconClient:         args.Client,
		notificationHandler: args.NotificationHandler,
		storage:             args.Storage,
	}

	// Repoint web ui down one directory
	staticContent, err := fs.Sub(staticUI, "build")
	if err != nil {
		log.WithError(err).Fatal("could not find UI build directory")
	}

	// index.html
	indexTemplate, err := template.ParseFS(staticContent, "index.html")
	if err != nil {
		log.Fatal(err)
	}

	// Set things up
	router := mux.NewRouter()

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err := indexTemplate.Execute(w, args.TemplateVars); err != nil {
			log.WithError(err).Error("Unable to render index")
		}
	})

	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.HandleFunc("/status", ws.getStatus).Methods("GET")
	apiRouter.HandleFunc("/delegate", ws.setDelegate).Methods("POST")
	apiRouter.HandleFunc("/health", ws.getHealth).Methods("GET")

	// Settings tab
	settingsRouter := apiRouter.PathPrefix("/settings").Subrouter()
	settingsRouter.HandleFunc("/", ws.getSettings).Methods("GET")
	settingsRouter.HandleFunc("/saveTelegram", ws.saveTelegram).Methods("POST")
	settingsRouter.HandleFunc("/saveEmail", ws.saveEmail).Methods("POST")
	settingsRouter.HandleFunc("/addEndpoint", ws.addEndpoint).Methods("POST")
	settingsRouter.HandleFunc("/listEndpoints", ws.listEndpoints).Methods("GET")
	settingsRouter.HandleFunc("/deleteEndpoint", ws.deleteEndpoint).Methods("POST")

	// Voting tab
	votingRouter := apiRouter.PathPrefix("/voting").Subrouter()
	votingRouter.HandleFunc("/upvote", ws.handleUpvote).Methods("POST", "OPTIONS")

	// Setup wizards
	wizardRouter := apiRouter.PathPrefix("/wizard").Subrouter()
	wizardRouter.HandleFunc("/testLedger", ws.testLedger)
	wizardRouter.HandleFunc("/confirmBakingPkh", ws.confirmBakingPkh)
	wizardRouter.HandleFunc("/generateNewKey", ws.generateNewKey)
	wizardRouter.HandleFunc("/importKey", ws.importSecretKey).Methods("POST", "OPTIONS")
	wizardRouter.HandleFunc("/registerBaker", ws.registerBaker).Methods("POST", "OPTIONS")
	wizardRouter.HandleFunc("/finishWallet", ws.finishWalletWizard)

	// For static content (js, images)
	router.PathPrefix("/static/").Handler(http.FileServer(http.FS(staticContent)))

	// Make the http server
	httpAddr := fmt.Sprintf("%s:%d", args.BindAddr, args.BindPort)
	ws.httpSvr = &http.Server{
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
		if err := ws.httpSvr.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Errorf("Httpserver: ListenAndServe()")
		}

		log.Info("Httpserver: Shutdown")
	}()

	// Wait for shutdown signal on channel
	go func() {
		defer args.WG.Done()
		<-args.ShutdownChannel

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := ws.httpSvr.Shutdown(ctx); err != nil {
			log.WithError(err).Errorf("Httpserver: Shutdown()")
		}
	}()
	return ws, nil
}

func apiError(err error, w http.ResponseWriter) {
	e, _ := json.Marshal(errorWrapper{err.Error()})
	http.Error(w, string(e), http.StatusBadRequest)
}

func apiReturnOk(w http.ResponseWriter) {
	if err := json.NewEncoder(w).Encode(map[string]string{"ok": "ok"}); err != nil {
		log.WithError(err).Error("UI Return Encode Failure")
	}
}
