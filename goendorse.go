package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	gotezos "github.com/goat-systems/go-tezos"
	log "github.com/sirupsen/logrus"
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

	// Connection to node
	rpcHostPort := fmt.Sprintf("%s:%d", *rpcHostname, *rpcPort)
	gt, err = gotezos.New(rpcHostPort)
	if err != nil {
		log.WithError(err).Fatalf("Could not connect to network: %s", rpcHostPort)
	}
	log.WithField("RPCServer", rpcHostPort).Info("Connected to RPC")

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
	go func(cH chan<- *gotezos.Block) {

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
					cH <- block

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

			// wait here for timer
			<-ticker.C
			log.Debug("tick...")
		}

	}(newHeadNotifier)

	// loop forever, waiting for new blocks on the channel
	ctx, ctxCancel := context.WithCancel(context.Background())

	for block := range newHeadNotifier {

		// New block means to cancel any existing baking work as
		// someone else beat us to it.
		// Noop on very first block from channel
		ctxCancel()

		// Create a new context for this run
		ctx, ctxCancel = context.WithCancel(context.Background())

		go handleEndorsement(ctx, *block)
		go handleBake(ctx, *block, *maxBakePriority)
	}
}
