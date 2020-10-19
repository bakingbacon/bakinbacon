package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/goat-systems/go-tezos/v3/rpc"

	log "github.com/sirupsen/logrus"

	"goendorse/signerclient"
	"goendorse/storage"
)

var (
	gt           *rpc.Client
	wg           sync.WaitGroup
	signerWallet *signerclient.SignerClient
	// wallet       keys.Key

	// Flags
	logDebug          *bool
	dryRunEndorsement *bool
	dryRunBake        *bool
	bakerPkh          *string
	maxBakePriority   *int
	rpcUrl            *string
	signerUrl         *string
)

func main() {

	var err error

	parseArgs()

	// Logging
	setupLogging(*logDebug)

	// Clean exits
	shutdownChannel := SetupCloseChannel()

	// Connection to node
	gt, err = rpc.New(*rpcUrl)
	if err != nil {
		log.WithError(err).Fatalf("Could not connect to network: %s", *rpcUrl)
	}
	log.WithField("Host", *rpcUrl).Info("Connected to RPC server")

	log.WithFields(log.Fields{
		"PreservedCycles":       gt.NetworkConstants.PreservedCycles,
		"BlocksPerCycle":        gt.NetworkConstants.BlocksPerCycle,
		"BlocksPerRollSnapshot": gt.NetworkConstants.BlocksPerRollSnapshot,
	}).Debug("Loaded Network Constants")

	// tz1MTZEJE7YH3wzo8YYiAGd8sgiCTxNRHczR
	// pk := "edpkvEbxZAv15SAZAacMAwZxjXToBka4E49b3J1VNrM1qqy5iQfLUx"
	// sk := "edsk3yXukqCQXjCnS4KRKEiotS7wRZPoKuimSJmWnfH2m3a2krJVdf"
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

	// Signer wallet
	signerWallet, err = signerclient.New(*bakerPkh, *signerUrl)
	if err != nil {
		log.WithError(err).Fatal("Could not connect to signer")
	}
	log.WithFields(log.Fields{
		"Baker": signerWallet.BakerPkh, "PublicKey": signerWallet.BakerPk,
	}).Info("Connected to signer daemon")

	// Launch background thread to check for new /head
	// Returns channel for new head block notifications
	wg.Add(1)
	newHeadNotifier := blockWatcher(shutdownChannel, &wg)

	// loop forever, waiting for new blocks on the channel
	ctx, ctxCancel := context.WithCancel(context.Background())

	Main:
	for {

		select {
		case block := <-newHeadNotifier:

			// New block means to cancel any existing baking work as
			// someone else beat us to it.
			// Noop on very first block from channel
			ctxCancel()

			// Create a new context for this run
			ctx, ctxCancel = context.WithCancel(context.Background())

			wg.Add(1)
			go handleEndorsement(ctx, &wg, *block)

			wg.Add(1)
			go revealNonces(ctx, &wg, *block)

			wg.Add(1)
			go handleBake(ctx, &wg, *block, *maxBakePriority)

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

func blockWatcher(shutdownChannel <-chan interface{}, wg *sync.WaitGroup) chan *rpc.Block {

	// Channel for new head blocks
	newHeadNotifier := make(chan *rpc.Block, 1)

	// Loop forever, checking for new head
	go func(nHN chan<- *rpc.Block, sC <-chan interface{}, wg *sync.WaitGroup) {

		defer wg.Done()

		curHead := &rpc.Block{}

		// Get network constant time_between_blocks and set sleep-ticker to 25%
		timeBetweenBlocks := gt.NetworkConstants.TimeBetweenBlocks[0]
		sleepTime := time.Duration(timeBetweenBlocks / 4)
		ticker := time.NewTicker(sleepTime * time.Second)

		for {

			// watch for new head block
			block, err := gt.Head()
			if err != nil {
				log.WithError(err).Error("Unable to get /head block; Will try again")
			} else {

				if block.Hash != curHead.Hash {

					// notify new block
					nHN <- block

					curHead = block

					log.WithFields(log.Fields{
						"Cycle":   curHead.Metadata.Level.Cycle,
						"Level":   curHead.Metadata.Level.Level,
						"Hash":    curHead.Hash,
						"ChainID": curHead.ChainID,
					}).Info("New Block")
				}
			}

			// wait here for timer, or shutdown
			select {
			case <-ticker.C:
				log.Debug("tick...")
			case <-sC:
				log.Info("Shutting down /head fetch")
				return
			}
		}

	}(newHeadNotifier, shutdownChannel, wg)

	return newHeadNotifier
}

func SetupCloseChannel() chan interface{} {

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
	logDebug = flag.Bool("debug", false, "Enable debug logging")

	dryRunEndorsement = flag.Bool("dry-run-endorse", false, "Compute, but don't inject endorsements")
	dryRunBake = flag.Bool("dry-run-bake", false, "Compute, but don't inject blocks")

	bakerPkh = flag.String("baker", "", "Baker's Public Key Hash")
	maxBakePriority = flag.Int("max-priority", 64, "Maximum allowed priority to bake")

	rpcUrl = flag.String("rpc-url", "http://127.0.0.1:8732", "URL of RPC server")
	signerUrl = flag.String("signer-url", "http://127.0.0.1:8734", "URL of signer")

	flag.Parse()

	// Sanity checks
	if *bakerPkh == "" {
		fmt.Println("Baker's public key hash required")
		flag.PrintDefaults()
		os.Exit(1)
	}

	bakerPhkThree := (*bakerPkh)[0:3]
	if bakerPhkThree != "tz1" && bakerPhkThree != "tz2" && bakerPhkThree != "tz3" {
		fmt.Println("Baker key does not match one of tz1.., tz2.., or tz3..")
		flag.PrintDefaults()
		os.Exit(1)
	}

	isBadUrl := func(u string) bool {
		return u == "" || (u[0:4] != "http" && u[0:5] != "https")
	}

	if isBadUrl(*signerUrl) {
		fmt.Println("Signer URL is required; Ex: http://127.0.0.1:18734")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if isBadUrl(*rpcUrl) {
		fmt.Println("RPC URL is required; Ex: http://127.0.0.1:18734")
		flag.PrintDefaults()
		os.Exit(1)
	}
}
