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

	gotezos "github.com/utdrmac/go-tezos/v2"
	log "github.com/sirupsen/logrus"

	"goendorse/storage"
	"goendorse/signerclient"
)

const (
	BAKER string = "tz1MTZEJE7YH3wzo8YYiAGd8sgiCTxNRHczR"
)

var (
	gt           *gotezos.GoTezos
	wallet       *gotezos.Wallet
	wg           sync.WaitGroup
	signerWallet *signerclient.SignerClient

	// Flags
	logDebug        *bool
	bakerPkh        *string
	maxBakePriority *int
	rpcUrl          *string
	signerUrl       *string
)

func main() {

	var err error

	parseArgs()

	// Logging
	setupLogging(*logDebug)

	// Clean exits
	shutdownChannel := SetupCloseChannel()

	// Connection to node
	gt, err = gotezos.New(*rpcUrl)
	if err != nil {
		log.WithError(err).Fatalf("Could not connect to network: %s", *rpcUrl)
	}
	log.WithField("Host", *rpcUrl).Info("Connected to RPC server")

	// tz1MTZEJE7YH3wzo8YYiAGd8sgiCTxNRHczR
	pk := "edpkvEbxZAv15SAZAacMAwZxjXToBka4E49b3J1VNrM1qqy5iQfLUx"
	sk := "edsk3yXukqCQXjCnS4KRKEiotS7wRZPoKuimSJmWnfH2m3a2krJVdf"

	wallet, err = gotezos.ImportWallet(BAKER, pk, sk)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.WithField("Baker", wallet.Address).Info("Loaded Wallet")

	// Signer wallet
	signerWallet, err = signerclient.New(*bakerPkh, *signerUrl)
	if err != nil {
		log.WithError(err).Fatal("Could not connect to signer")
	}
	log.WithFields(log.Fields{
		"Baker": signerWallet.BakerPkh, "PublicKey": signerWallet.BakerPk,
	}).Info("Connected to signer daemon")

	//log.Printf("Constants: PreservedCycles: %d, BlocksPerCycle: %d, BlocksPerRollSnapshot: %d",
	//	gt.Constants.PreservedCycles, gt.Constants.BlocksPerCycle, gt.Constants.BlocksPerRollSnapshot)

	newHeadNotifier := make(chan *gotezos.Block, 1)

	// this go func should loop forever, checking every 20s if a new block has appeared
	wg.Add(1)
	go func(nHN chan<- *gotezos.Block, sC <-chan interface{}, wg *sync.WaitGroup) {

		defer wg.Done()

		curHead := &gotezos.Block{}

		ticker := time.NewTicker(10 * time.Second)

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

					chainid := curHead.ChainID
					hash := curHead.Hash
					level := curHead.Metadata.Level.Level
					cycle := curHead.Metadata.Level.Cycle

					log.WithFields(log.Fields{
						"Cycle": cycle, "Level": level, "Hash": hash, "ChainID": chainid,
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

	}(newHeadNotifier, shutdownChannel, &wg)

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

	if *signerUrl == "" || ((*signerUrl)[0:4] != "http" && (*signerUrl)[0:5] != "https") {
		fmt.Println("Signer URL is required; Ex: http://127.0.0.1:18734")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if *rpcUrl == "" || ((*rpcUrl)[0:4] != "http" && (*rpcUrl)[0:5] != "https") {
		fmt.Println("RPC URL is required; Ex: http://127.0.0.1:18734")
		flag.PrintDefaults()
		os.Exit(1)
	}
}
