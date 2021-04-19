package webserver

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
	
	log "github.com/sirupsen/logrus"
	
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	
	//"bakinbacon/storage"
	"bakinbacon/baconclient"
)

const (
	BIND_ADDR = "10.10.10.203"
	BIND_PORT = "8082"
)

var (
	// Global vars for the webserver package
	httpSvr *http.Server
	baconClient *baconclient.BaconClient
)

type ApiError struct {
	Error string `json:"err"`
}


func Start(_baconClient *baconclient.BaconClient, shutdownChannel <-chan interface{}, wg *sync.WaitGroup) {

	// Set the package global
	baconClient = _baconClient
	
	// Set things up
	var router = mux.NewRouter()
	
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "webserver/build/index.html")
	})
	
	apiRouter := router.PathPrefix("/api").Subrouter()
	
	apiRouter.HandleFunc("/endpoints/add", addEndpoint)
	apiRouter.HandleFunc("/endpoints/list", listEndpoints)
	
	apiRouter.HandleFunc("/status", getStatus)
	apiRouter.HandleFunc("/delegate", getDelegate).Methods("GET")
	apiRouter.HandleFunc("/delegate", setDelegate).Methods("POST")
	
	apiRouter.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	
	// APIs dealing with the setup wizards
	wizardRouter := apiRouter.PathPrefix("/wizard").Subrouter()
	wizardRouter.HandleFunc("/generateNewKey", generateNewKey)
	wizardRouter.HandleFunc("/importKey", importSecretKey).Methods("POST", "OPTIONS")
	wizardRouter.HandleFunc("/registerbaker", registerBaker).Methods("POST", "OPTIONS")
	
	//router.HandleFunc("/login", WS.loginHandler).Methods("POST")
	//router.HandleFunc("/logout", WS.logoutHandler)
	
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("webserver/build/static"))))

	httpAddr := BIND_ADDR + ":" + BIND_PORT
	httpSvr = &http.Server{
		Handler: handlers.CORS(
			handlers.AllowedHeaders([]string{"Content-Type"}),
			handlers.AllowedOrigins([]string{"*"}),
			handlers.AllowedMethods([]string{"GET", "POST", "OPTIONS"}),
		)(router),
		Addr: httpAddr,
		WriteTimeout: 15 * time.Second,
		ReadTimeout: 15 * time.Second,
	}
	
	log.WithField("Addr", httpAddr).Info("Bakin'Bacon WebUI Listening")
	
	// Launch webserver in background
	go func() {
//		var err error
//		if wantSSL {
//			err = httpSvr.ListenAndServeTLS("ssl/cert.pem", "ssl/key.pem")
//		} else {
//			err = httpSvr.ListenAndServe()
//		}
		if err := httpSvr.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Errorf("Httpserver: ListenAndServe()")
		}
		log.Info("Httpserver: Shutdown")
	}()

	// Wait for shutdown signal on channel
	go func() {
		<-shutdownChannel
		ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
		if err := httpSvr.Shutdown(ctx); err != nil {
			log.WithError(err).Errorf("Httpserver: Shutdown()")
    	}
    	wg.Done()
	}()
}
