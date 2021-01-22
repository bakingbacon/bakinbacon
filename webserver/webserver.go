package webserver

import (
	"context"
	"encoding/json"
	_"html/template"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
	
	log "github.com/sirupsen/logrus"
	
	"github.com/gorilla/mux"
	
	"goendorse/storage"
)

const (
	BIND_ADDR = "10.10.10.203"
	BIND_PORT = "8082"
)

var httpSvr *http.Server

func loginPageHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "bakinbacon.html")
}

func addEndpoint(w http.ResponseWriter, r *http.Request) {

	j := json.NewEncoder(w)
	
	body, err := ioutil.ReadAll(r.Body)
    if err != nil {
    	j.Encode(map[string]string{"error": err.Error()})
    	return
    }

	if e := storage.DB.AddRPCEndpoint(string(body)); e != nil {
		log.WithError(e).WithField("Endpoint", string(body)).Error("API AddEndpoint")
		j.Encode(map[string]string{"error": err.Error()})
		return
	}
	
	log.WithField("Endpoint", string(body)).Debug("API AddEndpoint")
	j.Encode(map[string]bool{"ok": true})
}

func listEndpoints(w http.ResponseWriter, r *http.Request) {

	endpoints, err := storage.DB.GetRPCEndpoints()
	if err != nil {
		log.WithError(err).Error("Unable to get endpoints")
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	
	log.WithField("Endpoints", endpoints).Debug("API ListEndpoints")
	json.NewEncoder(w).Encode(map[string][]string{"endpoints": endpoints})
}

func Start(shutdownChannel <-chan interface{}, wg *sync.WaitGroup) {

	// Parse template files
	//templates := template.Must(template.ParseFiles(
//		"./static/bakinbacon.tpl",
//	))
	
	// Set things up
	var router = mux.NewRouter()
	router.HandleFunc("/", loginPageHandler)
	router.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		// an example API handler
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	router.HandleFunc("/api/endpoints/add", addEndpoint)
	router.HandleFunc("/api/endpoints/list", listEndpoints)
	
	//router.HandleFunc("/tacos", WS.tacosPageHandler)
	
	//router.HandleFunc("/resetpassword", resetpasswordHandler)
	
	//router.HandleFunc("/login", WS.loginHandler).Methods("POST")
	//router.HandleFunc("/logout", WS.logoutHandler)
	
	router.PathPrefix("/css").Handler(http.StripPrefix("/css", http.FileServer(http.Dir("static/css"))))
	router.PathPrefix("/images").Handler(http.StripPrefix("/images", http.FileServer(http.Dir("static/images"))))
	router.PathPrefix("/js").Handler(http.StripPrefix("/js", http.FileServer(http.Dir("static/js"))))

	httpAddr := BIND_ADDR + ":" + BIND_PORT
	httpSvr = &http.Server{
		Handler: router,
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
