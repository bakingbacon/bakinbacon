package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"

	"bakinbacon/baconclient"
	"bakinbacon/storage"
	"bakinbacon/webserver"
)

var (
	bc *baconclient.BaconClient

	// Flags
	logDebug          *bool
	logTrace          *bool
	dryRunEndorsement *bool
	dryRunBake        *bool
	webUiAddr         *string
	webUiPort         *int
)

func main() {

	// Used throughout main
	var (
		err error
		wg  sync.WaitGroup
		ctx context.Context
	)

	parseArgs()

	// Logging
	setupLogging(*logDebug, *logTrace)

	// Clean exits
	shutdownChannel := setupCloseChannel()

	// Set up RPC polling-monitoring
	bc, err = baconclient.New(shutdownChannel, &wg)
	if err != nil {
		log.WithError(err).Fatalf("Could not connect create BaconClient")
	}

	// Web UI
	wg.Add(1)
	webserver.Start(bc, *webUiAddr, *webUiPort, shutdownChannel, &wg)

	// Network constants
	networkConstants := bc.Current.CurrentConstants()
	log.WithFields(log.Fields{ //nolint:wsl
		"PreservedCycles":       networkConstants.PreservedCycles,
		"BlocksPerCycle":        networkConstants.BlocksPerCycle,
		"BlocksPerRollSnapshot": networkConstants.BlocksPerRollSnapshot,
		"BlocksPerCommitment":   networkConstants.BlocksPerCommitment,
	}).Debug("Loaded Network Constants")

	_, ctxCancel := context.WithCancel(context.Background())

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
			if !bc.CanBake() {
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
	logDebug = flag.Bool("debug", false, "Enable debug-level logging")
	logTrace = flag.Bool("trace", false, "Enable trace-level logging")

	dryRunEndorsement = flag.Bool("dry-run-endorse", false, "Compute, but don't inject endorsements")
	dryRunBake = flag.Bool("dry-run-bake", false, "Compute, but don't inject blocks")

	webUiAddr = flag.String("webuiaddr", "127.0.0.1", "Address on which to bind web UI server")
	webUiPort = flag.Int("webuiport", 8082, "Port on which to bind web UI server")

	flag.Parse()
}

// tz1RMmSzPSWPSSaKU193Voh4PosWSZx1C7Hs
// pk := "edpkti2A2ZtvYEfkYaqQ7ESbCrPEYPBacRCBq6Pmxa4E1jTBYqpKG5"
// sk := "edsk3HwPpiN2w34JSoevZ135L9jWpupiqKcYp38SHR5N21XJyK8Ukv"
//
// // Gotezos wallet
// walletInput := keys.NewKeyInput{
// 	EncodedString: sk,
// 	Kind:          keys.Ed25519,
// }
// wallet, err = keys.NewKey(walletInput)
// if err != nil {
// 	log.WithError(err).Fatal("Failed to load wallet")
// }
// log.WithFields(log.Fields{
// 	"Baker": wallet.PubKey.GetPublicKeyHash(), "PublicKey": wallet.PubKey.GetPublicKey(),
// }).Info("Loaded Wallet")
