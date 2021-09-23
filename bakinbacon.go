package main

import (
	"bakinbacon/util"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"

	"bakinbacon/baconclient"
	"bakinbacon/notifications"
	"bakinbacon/storage"
	"bakinbacon/webserver"
)

// TODO: Translations (https://www.transifex.com/bakinbacon/bakinbacon-core/content/)

var (
	server *BakinBaconServer
)

type BakinBaconServer struct {
	*baconclient.BaconClient
	*notifications.NotificationHandler
	*webserver.WebServer
	*storage.Storage
	Flags
}

// Flags Sever flags
type Flags struct {
	networkName       string
	logDebug          bool
	logTrace          bool
	dryRunEndorsement bool
	dryRunBake        bool
	webUIAddr         string
	webUIPort         int
	dataDir           string
}

func main() {
	// Used throughout main
	var (
		err  error
		wg   sync.WaitGroup
		ctx  context.Context
		once sync.Once
	)

	server = new(BakinBaconServer)
	server.parseArgs()

	// Logging
	setupLogging(server.logDebug, server.logTrace)

	// Clean exits
	shutdownChannel := setupCloseChannel()

	// Open/Init database
	server.Storage, err = storage.InitStorage(server.dataDir, server.networkName)
	if err != nil {
		log.WithError(err).Fatal("Could not open storage")
	}

	// Start
	log.Infof("=== BakinBacon v1.0 (%s) ===", commitHash)
	log.Infof("=== Network: %s ===", server.networkName)

	// Global Notifications notificationHandler singleton
	server.NotificationHandler, err = notifications.NewHandler(server.Storage)
	if err != nil {
		log.WithError(err).Error("Unable to load notifiers")
	}

	// VERSION checking
	go server.RunVersionCheck()

	// Network constants
	log.WithFields(log.Fields{ //nolint:wsl
		"BlocksPerCycle":      util.NetworkConstants[server.networkName].BlocksPerCycle,
		"BlocksPerCommitment": util.NetworkConstants[server.networkName].BlocksPerCommitment,
		"TimeBetweenBlocks":   util.NetworkConstants[server.networkName].TimeBetweenBlocks,
	}).Debug("Loaded Network Constants")

	// Set up RPC polling-monitoring
	server.BaconClient, err = baconclient.New(server.NotificationHandler, server.Storage, util.NetworkConstants[server.networkName].TimeBetweenBlocks, shutdownChannel, &wg)
	if err != nil {
		log.WithError(err).Fatalf("Cannot create BaconClient")
	}

	// Start web UI
	// Template variables for the UI
	wg.Add(1)
	templateVars := webserver.TemplateVars{
		Network:        server.networkName,
		BlocksPerCycle: util.NetworkConstants[server.networkName].BlocksPerCycle,
		MinBlockTime:   util.NetworkConstants[server.networkName].TimeBetweenBlocks,
		UIBaseURL:      os.Getenv("UI_DEBUG"),
	}

	server.WebServer = new(webserver.WebServer)
	args := webserver.WebServerArgs{
		Network:             server.networkName,
		Client:              server.BaconClient,
		NotificationHandler: server.NotificationHandler,
		Storage:             server.Storage,
		BindAddr:            server.webUIAddr,
		BindPort:            server.webUIPort,
		TemplateVars:        templateVars,
		ShutdownChannel:     shutdownChannel,
		WG:                  &wg,
	}
	once.Do(func() {
		server.WebServer, err = webserver.Start(args)
		if err != nil {
			log.WithError(err).Error()
			os.Exit(1)
		}
	})

	// For canceling when new blocks appear
	_, ctxCancel := context.WithCancel(context.Background())

	// Run checks against our address; silent mode = false
	_ = server.CanBake(false)

	// Update bacon-status with most recent bake/endorse info
	server.updateRecentBaconStatus()

	// loop forever, waiting for new blocks coming from the RPC monitors
Main:
	for {

		select {
		case block := <-server.NewBlockNotifier:
			// NewHandler block means to cancel any existing baking work as someone else beat us to it.
			// Noop on very first block from channel
			ctxCancel()

			// Create a new context for this run
			ctx, ctxCancel = context.WithCancel(context.Background())

			// If we can't bake, no need to do try and do anything else
			// This check is silent = true on success
			if !server.CanBake(true) {
				continue
			}

			wg.Add(1)
			go server.handleEndorsement(ctx, &wg, *block)

			wg.Add(1)
			go server.revealNonces(ctx, &wg, *block)

			wg.Add(1)
			go server.handleBake(ctx, &wg, *block)

			// Utility

			// Update UI with next rights
			go server.updateCycleRightsStatus(block.Metadata.Level)

			// Pre-fetch rights to DB as both backup and for UI display
			go server.prefetchCycleRights(block.Metadata.Level)

		case <-shutdownChannel:
			log.Warn("Shutting things down...")
			ctxCancel()
			server.Shutdown()
			break Main
		}
	}

	// Wait for threads to finish
	wg.Wait()

	// Clean close DB, logs
	server.Storage.Close()
	closeLogging()

	os.Exit(0)
}

func setupCloseChannel() chan interface{} {

	// Create channels for signals
	signalChan := make(chan os.Signal, 1)
	closingChan := make(chan interface{}, 1)

	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-signalChan
		close(closingChan)
	}()

	return closingChan
}

func (s *BakinBaconServer) parseArgs() {

	// Args
	flag.StringVar(&s.networkName, "network", util.GRANADA_NET, fmt.Sprintf("Which network to use: %s", util.AvailableNetworks()))

	flag.BoolVar(&s.logDebug, "debug", false, "Enable debug-level logging")
	flag.BoolVar(&s.logTrace, "trace", false, "Enable trace-level logging")

	flag.BoolVar(&s.dryRunEndorsement, "dry-run-endorse", false, "Compute, but don't inject endorsements")
	flag.BoolVar(&s.dryRunBake, "dry-run-bake", false, "Compute, but don't inject blocks")

	flag.StringVar(&s.webUIAddr, "webuiaddr", "127.0.0.1", "Address on which to bind web UI server")
	flag.IntVar(&s.webUIPort, "webuiport", 8082, "Port on which to bind web UI server")

	flag.StringVar(&s.dataDir, "datadir", "./", "Location of database")

	printVersion := flag.Bool("version", false, "Show version and exit")

	flag.Parse()

	// Sanity
	if !util.IsValidNetwork(s.networkName) {
		log.Errorf("Unknown network: %s", s.networkName)
		flag.Usage()
		os.Exit(1)
	}

	// Handle print version and exit
	if *printVersion {
		log.Printf("Bakin'Bacon %s (%s)", version, commitHash)
		log.Printf("https://github.com/bakingbacon/bakinbacon")
		os.Exit(0)
	}
}
