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
	"github.com/pkg/errors"

	"bakinbacon/baconclient"
	"bakinbacon/notifications"
	"bakinbacon/payouts"
	"bakinbacon/storage"
)

var (
	// Embed all UI objects
	//go:embed build
	staticUi embed.FS
)

type ApiError struct {
	Error string `json:"error"`
}

type TemplateVars struct {
	Network        string
	BlocksPerCycle int
	MinBlockTime   int
	UiBaseUrl      string
}

type WebServer struct {
	// Global vars for the webserver package
	httpSvr             *http.Server
	baconClient         *baconclient.BaconClient
	notificationHandler *notifications.NotificationHandler
	payoutsHandler      *payouts.PayoutsHandler
	storage             *storage.Storage
}

type WebServerArgs struct {
	Client              *baconclient.BaconClient
	NotificationHandler *notifications.NotificationHandler
	PayoutsHandler      *payouts.PayoutsHandler
	Storage             *storage.Storage

	BindAddr     string
	BindPort     int
	TemplateVars TemplateVars

	ShutdownChannel <-chan interface{}
	WG              *sync.WaitGroup
}


func Start(args WebServerArgs) error {

	if err := args.Validate(); err != nil {
		return errors.Wrap(err, "Could not start web server")
	}

	ws := &WebServer{
		baconClient:         args.Client,
		notificationHandler: args.NotificationHandler,
		payoutsHandler:      args.PayoutsHandler,
		storage:             args.Storage,
	}

	// Repoint web ui down one directory
	staticContent, err := fs.Sub(staticUi, "build")
	if err != nil {
		return errors.Wrap(err, "Could not find UI build directory")

	}

	// index.html
	indexTemplate, err := template.ParseFS(staticContent, "index.html")
	if err != nil {
		return errors.Wrap(err, "Could not parse UI template")
	}

	// Set things up
	router := mux.NewRouter()

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err := indexTemplate.Execute(w, args.TemplateVars); err != nil {
			log.WithError(err).Error("Unable to render index")
		}
	})

	// Root APIs
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.HandleFunc("/status", ws.getStatus).Methods("GET")
	apiRouter.HandleFunc("/delegate", ws.setDelegate).Methods("POST")
	apiRouter.HandleFunc("/health", ws.getHealth).Methods("GET")

	// Settings tab
	settingsRouter := apiRouter.PathPrefix("/settings").Subrouter()
	settingsRouter.HandleFunc("/", ws.getSettings).Methods("GET")
	settingsRouter.HandleFunc("/savetelegram", ws.saveTelegram).Methods("POST")
	settingsRouter.HandleFunc("/saveemail", ws.saveEmail).Methods("POST")
	settingsRouter.HandleFunc("/addendpoint", ws.addEndpoint).Methods("POST")
	settingsRouter.HandleFunc("/listendpoints", ws.listEndpoints).Methods("GET")
	settingsRouter.HandleFunc("/deleteendpoint", ws.deleteEndpoint).Methods("POST")
	settingsRouter.HandleFunc("/bakersettings", ws.saveBakerSettings).Methods("POST")

	// Payouts tab
	payoutsRouter := apiRouter.PathPrefix("/payouts").Subrouter()
	payoutsRouter.HandleFunc("/list", ws.getPayouts).Methods("GET")
	payoutsRouter.HandleFunc("/cycledetail", ws.getCyclePayouts).Methods("GET")
	payoutsRouter.HandleFunc("/sendpayouts", ws.sendCyclePayouts).Methods("POST")

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
	args.WG.Add(1)
	go func() {
		// TODO: SSL for localhost?
		// var err error
		// if wantSSL {
		//   err = httpSvr.ListenAndServeTLS("ssl/cert.pem", "ssl/key.pem")
		// } else {
		//   err = httpSvr.ListenAndServe()
		// }
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

	return nil
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

func (a *WebServerArgs) Validate() error {

	if a.Client == nil {
		return errors.New("BaconClient is not instantiated")
	}

	if a.BindAddr == "" {
		return errors.New("Bind address empty")
	}

	if a.ShutdownChannel == nil {
		return errors.New("Shutdown channel is not instantiated")
	}

	if a.WG == nil {
		return errors.New("WaitGroup is not instantiated")
	}

	return nil
}
