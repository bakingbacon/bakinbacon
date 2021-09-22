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

var (
	bc *baconclient.BaconClient

	// Flags
	network           string
	logDebug          *bool
	logTrace          *bool
	dryRunEndorsement *bool
	dryRunBake        *bool
	webUIAddr         *string
	webUIPort         *int
	dataDir           *string
)

// TODO: Translations (https://www.transifex.com/bakinbacon/bakinbacon-core/content/)

func main() {

	// Used throughout main
	var (
		err  error
		wg   sync.WaitGroup
		ctx  context.Context
		once sync.Once
	)

	parseArgs()

	// Logging
	setupLogging(*logDebug, *logTrace)

	// Clean exits
	shutdownChannel := setupCloseChannel()

	// Open/Init database
	if err := storage.InitStorage(*dataDir, network); err != nil {
		log.WithError(err).Fatal("Could not open storage")
	}

	// Start
	log.Infof("=== BakinBacon v1.0 (%s) ===", commitHash)
	log.Infof("=== Network: %s ===", network)

	// Global Notifications handler singleton
	if err := notifications.New(); err != nil {
		log.WithError(err).Error("Unable to load notifiers")
	}

	// Version checking
	go RunVersionCheck()

	// Network constants
	log.WithFields(log.Fields{ //nolint:wsl
		"BlocksPerCycle":      util.NetworkConstants[network].BlocksPerCycle,
		"BlocksPerCommitment": util.NetworkConstants[network].BlocksPerCommitment,
		"TimeBetweenBlocks":   util.NetworkConstants[network].TimeBetweenBlocks,
	}).Debug("Loaded Network Constants")

	// Set up RPC polling-monitoring
	bc, err = baconclient.New(util.NetworkConstants[network].TimeBetweenBlocks, shutdownChannel, &wg)
	if err != nil {
		log.WithError(err).Fatalf("Cannot create BaconClient")
	}

	// Start web UI
	// Template variables for the UI
	wg.Add(1)
	templateVars := webserver.TemplateVars{
		Network:        network,
		BlocksPerCycle: util.NetworkConstants[network].BlocksPerCycle,
		MinBlockTime:   util.NetworkConstants[network].TimeBetweenBlocks,
		UIBaseURL:      os.Getenv("UI_DEBUG"),
	}

	var ws *webserver.WebServer
	args := webserver.WebServerArgs{
		Network:         network,
		Client:          bc,
		BindAddr:        *webUIAddr,
		BindPort:        *webUIPort,
		TemplateVars:    templateVars,
		ShutdownChannel: shutdownChannel,
		WG:              &wg,
	}
	once.Do(func() {
		ws, err = webserver.Start(args)
		if err != nil {
			log.WithError(err).Error()
			os.Exit(1)
		}
	})

	// For canceling when new blocks appear
	_, ctxCancel := context.WithCancel(context.Background())

	// Run checks against our address; silent mode = false
	_ = bc.CanBake(false)

	// Update bacon-status with most recent bake/endorse info
	updateRecentBaconStatus()

	// loop forever, waiting for new blocks coming from the RPC monitors
Main:
	for {

		select {
		case block := <-bc.NewBlockNotifier:

			// New block means to cancel any existing baking work as someone else beat us to it.
			// Noop on very first block from channel
			ctxCancel()

			// Create a new context for this run
			ctx, ctxCancel = context.WithCancel(context.Background())

			// If we can't bake, no need to do try and do anything else
			// This check is silent = true on success
			if !bc.CanBake(true) {
				continue
			}

			wg.Add(1)
			go handleEndorsement(ctx, &wg, *block)

			wg.Add(1)
			go revealNonces(ctx, &wg, *block)

			wg.Add(1)
			go handleBake(ctx, &wg, *block)

			//
			// Utility
			//

			// Update UI with next rights
			go updateCycleRightsStatus(block.Metadata.Level)

			// Pre-fetch rights to DB as both backup and for UI display
			go prefetchCycleRights(block.Metadata.Level)

		case <-shutdownChannel:
			log.Warn("Shutting things down...")
			ctxCancel()
			bc.Shutdown()
			break Main
		}
	}

	// Wait for threads to finish
	wg.Wait()

	// Clean close DB, logs
	storage.DB.Close()
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

func parseArgs() {

	// Args
	flag.StringVar(&network, "network", util.Granadanet, fmt.Sprintf("Which network to use: %s", util.AvailableNetworks()))

	logDebug = flag.Bool("debug", false, "Enable debug-level logging")
	logTrace = flag.Bool("trace", false, "Enable trace-level logging")

	dryRunEndorsement = flag.Bool("dry-run-endorse", false, "Compute, but don't inject endorsements")
	dryRunBake = flag.Bool("dry-run-bake", false, "Compute, but don't inject blocks")

	webUIAddr = flag.String("webuiaddr", "127.0.0.1", "Address on which to bind web UI server")
	webUIPort = flag.Int("webuiport", 8082, "Port on which to bind web UI server")

	dataDir = flag.String("datadir", "./", "Location of database")

	printVersion := flag.Bool("version", false, "Show version and exit")

	flag.Parse()

	// Sanity
	if !util.IsValidNetwork(network) {
		log.Errorf("unknown network: %s", network)
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
