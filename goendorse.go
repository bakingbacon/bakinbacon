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

	gotezos "github.com/goat-systems/go-tezos"
	log "github.com/sirupsen/logrus"
	storage "goendorse/storage"
)

const (
	BAKER string = "tz1MTZEJE7YH3wzo8YYiAGd8sgiCTxNRHczR"
)

var (
	gt     *gotezos.GoTezos
	wallet *gotezos.Wallet
)

func main() {

	var err error

	// Args
	logDebug := flag.Bool("debug", false, "Enable debug logging")
	maxBakePriority := flag.Int("max-priority", 64, "Maximum allowed priority to bake")
	rpcHostname := flag.String("node-rpc", "127.0.0.1", "Hostname/IP of RPC server")
	rpcPort := flag.Int("node-port", 8732, "TCP/IP port of RPC server")
	flag.Parse()

	// Logging
	setupLogging(*logDebug)

	// Clean exits
	shutdownChannel := SetupCloseChannel()
	var wg sync.WaitGroup

	// Connection to node
	rpcHostPort := fmt.Sprintf("%s:%d", *rpcHostname, *rpcPort)
	gt, err = gotezos.New(rpcHostPort)
	if err != nil {
		log.WithError(err).Fatalf("Could not connect to network: %s", rpcHostPort)
	}
	log.WithField("Host", rpcHostPort).Info("Connected to RPC server")

	// tz1MTZEJE7YH3wzo8YYiAGd8sgiCTxNRHczR
	pk := "edpkvEbxZAv15SAZAacMAwZxjXToBka4E49b3J1VNrM1qqy5iQfLUx"
	sk := "edsk3yXukqCQXjCnS4KRKEiotS7wRZPoKuimSJmWnfH2m3a2krJVdf"

	wallet, err = gotezos.ImportWallet(BAKER, pk, sk)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.WithField("Baker", wallet.Address).Info("Loaded Wallet")

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

			go handleEndorsement(ctx, *block)
			go handleBake(ctx, *block, *maxBakePriority)

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
