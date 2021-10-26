package main

import (
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
	"bakinbacon/util"
	"bakinbacon/webserver"
)

var (
	bakinbacon *BakinBacon
)

type BakinBacon struct {
	*baconclient.BaconClient
	*notifications.NotificationHandler
	*storage.Storage
	*util.NetworkConstants
	Flags
}

//nolint:structcheck
type Flags struct {
	network           string
	logDebug          bool
	logTrace          bool
	dryRunEndorsement bool
	dryRunBake        bool
	webUiAddr         string
	webUiPort         int
	dataDir           string
}

// TODO: Translations (https://www.transifex.com/bakinbacon/bakinbacon-core/content/)

func main() {

	// Used throughout main
	var (
		err error
		wg  sync.WaitGroup
		ctx context.Context
	)

	// Init the main server
	bakinbacon = &BakinBacon{}
	bakinbacon.parseArgs()

	// Logging
	setupLogging(bakinbacon.logDebug, bakinbacon.logTrace)

	// Clean exits
	shutdownChannel := setupCloseChannel()

	// Open/Init database
	bakinbacon.Storage, err = storage.InitStorage(bakinbacon.dataDir, bakinbacon.network)
	if err != nil {
		log.WithError(err).Fatal("Could not open storage")
	}

	// Start
	log.Infof("=== BakinBacon v1.0 (%s) ===", commitHash)
	log.Infof("=== Network: %s ===", bakinbacon.network)

	// Global Notifications handler singleton
	bakinbacon.NotificationHandler, err = notifications.NewHandler(bakinbacon.Storage)
	if err != nil {
		log.WithError(err).Error("Unable to load notifiers")
	}

	// Network constants
	bakinbacon.NetworkConstants, err = util.GetNetworkConstants(bakinbacon.network)
	if err != nil {
		log.WithError(err).Fatal("Cannot load network constants")
	}

	log.WithFields(log.Fields{ //nolint:wsl
		"BlocksPerCycle":      bakinbacon.NetworkConstants.BlocksPerCycle,
		"BlocksPerCommitment": bakinbacon.NetworkConstants.BlocksPerCommitment,
		"TimeBetweenBlocks":   bakinbacon.NetworkConstants.TimeBetweenBlocks,
	}).Debug("Loaded Network Constants")

	// Set up RPC polling-monitoring
	bakinbacon.BaconClient, err = baconclient.New(bakinbacon.NotificationHandler, bakinbacon.Storage, bakinbacon.NetworkConstants, shutdownChannel, &wg)
	if err != nil {
		log.WithError(err).Fatalf("Cannot create BaconClient")
	}

	// Version checking
	go bakinbacon.RunVersionCheck()

	// Start web UI
	// Template variables for the UI
	templateVars := webserver.TemplateVars{
		Network:        bakinbacon.network,
		BlocksPerCycle: bakinbacon.NetworkConstants.BlocksPerCycle,
		MinBlockTime:   bakinbacon.NetworkConstants.TimeBetweenBlocks,
		UiBaseUrl:      os.Getenv("UI_DEBUG"),
	}

	// Args for web server
	webServerArgs := webserver.WebServerArgs{
		Client:              bakinbacon.BaconClient,
		NotificationHandler: bakinbacon.NotificationHandler,
		Storage:             bakinbacon.Storage,
		BindAddr:            bakinbacon.webUiAddr,
		BindPort:            bakinbacon.webUiPort,
		TemplateVars:        templateVars,
		ShutdownChannel:     shutdownChannel,
		WG:                  &wg,
	}
	if err := webserver.Start(webServerArgs); err != nil {
		log.WithError(err).Error("Unable to start webserver UI")
		os.Exit(1)
	}

	// For canceling when new blocks appear
	_, ctxCancel := context.WithCancel(context.Background())

	// Run checks against our address; silent mode = false
	_ = bakinbacon.CanBake(false)

	// Update bacon-status with most recent bake/endorse info
	bakinbacon.updateRecentBaconStatus()

	// loop forever, waiting for new blocks coming from the RPC monitors
	Main:
	for {

		select {
		case block := <-bakinbacon.NewBlockNotifier:

			// New block means to cancel any existing baking work as someone else beat us to it.
			// Noop on very first block from channel
			ctxCancel()

			// Create a new context for this run
			ctx, ctxCancel = context.WithCancel(context.Background())

			// If we can't bake, no need to do try and do anything else
			// This check is silent = true on success
			if !bakinbacon.CanBake(true) {
				continue
			}

			wg.Add(1)
			go bakinbacon.handleEndorsement(ctx, &wg, *block)

			wg.Add(1)
			go bakinbacon.revealNonces(ctx, &wg, *block)

			wg.Add(1)
			go bakinbacon.handleBake(ctx, &wg, *block)

			//
			// Utility
			//

			// Update UI with next rights
			go bakinbacon.updateCycleRightsStatus(block.Metadata.Level)

			// Pre-fetch rights to DB as both backup and for UI display
			go bakinbacon.prefetchCycleRights(block.Metadata.Level)

		case <-shutdownChannel:
			log.Warn("Shutting things down...")
			ctxCancel()
			bakinbacon.BaconClient.Shutdown()
			break Main
		}
	}

	// Wait for threads to finish
	wg.Wait()

	// Clean close DB, logs
	bakinbacon.Storage.Close()
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

func (bb *BakinBacon) parseArgs() {

	// Args
	flag.StringVar(&bb.network, "network", util.NETWORK_HANGZHOUNET, fmt.Sprintf("Which network to use: %s", util.AvailableNetworks()))

	flag.BoolVar(&bb.logDebug, "debug", false, "Enable debug-level logging")
	flag.BoolVar(&bb.logTrace, "trace", false, "Enable trace-level logging")

	flag.BoolVar(&bb.dryRunEndorsement, "dry-run-endorse", false, "Compute, but don't inject endorsements")
	flag.BoolVar(&bb.dryRunBake, "dry-run-bake", false, "Compute, but don't inject blocks")

	flag.StringVar(&bb.webUiAddr, "webuiaddr", "127.0.0.1", "Address on which to bind web UI server")
	flag.IntVar(&bb.webUiPort, "webuiport", 8082, "Port on which to bind web UI server")

	flag.StringVar(&bb.dataDir, "datadir", "./", "Location of database")

	printVersion := flag.Bool("version", false, "Show version and exit")

	flag.Parse()

	// Sanity
	if !util.IsValidNetwork(bb.network) {
		log.Errorf("Unknown network: %s", bb.network)
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
