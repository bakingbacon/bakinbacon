package baconclient

import (
	"sync"
	"time"

	"github.com/pkg/errors"
	
	"github.com/goat-systems/go-tezos/v4/rpc"
	log "github.com/sirupsen/logrus"

	"goendorse/storage"
)

type BaconClient struct {

	newBlockNotifier  chan *rpc.Block

	Current           *rpc.Client
	rpcClients        []*rpc.Client

	recentBlockHash  string
	recentBlockLevel int
	
	lock sync.Mutex
}

func New() (*BaconClient, error) {

	// Pull endpoints from storage
	endpoints, err := storage.DB.GetRPCEndpoints()
	if err != nil {
		log.WithError(err).Error("Unable to get endpoints")
		return nil, errors.Wrap(err, "Failed DB.GetRPCEndpoints")
	}
	
	if len(endpoints) < 1 {
		// This shouldn't happen, but just in case
		log.Error("No endpoints found")
		return nil, errors.New("No endpoints found")
	}

	clients := make([]*rpc.Client, len(endpoints))
	
	// Foreach endpoint, create an rpc client
	for i, e := range endpoints {
	
		gtRpc, err := rpc.New(e)
		if err != nil {
			log.WithError(err).Error("Error from RPC")
		}
		log.WithField("Endpoint", e).Info("Connected to RPC")

		// Add the client, even if error so we can retry
		clients[i] = gtRpc
	}

	return &BaconClient{
		newBlockNotifier: make(chan *rpc.Block, 1),
		Current:          clients[0],
		rpcClients:       clients,
		recentBlockHash:  "",
		recentBlockLevel: 0,
	}, nil
}

func (b *BaconClient) Run(shutdown <-chan interface{}, wg *sync.WaitGroup) chan *rpc.Block {

	// For each RPC client, thread off a polling monitor.
	// Return the main channel so caller can receive new blocks
	// coming from any monitor.
	
	for _, c := range b.rpcClients {

		wg.Add(1)
		go b.blockWatch(c, shutdown, wg)

		// Small throttle to offset each poller
		<-time.After(2 * time.Second)
	}

	return b.newBlockNotifier
}

func (b *BaconClient) blockWatch(client *rpc.Client, shutdown <-chan interface{}, wg *sync.WaitGroup) {

	defer wg.Done()

	lostTicks := 0

	// Get network constant time_between_blocks and set sleep-ticker to 25%
	timeBetweenBlocks := client.CurrentConstants().TimeBetweenBlocks[0]
	sleepTime := time.Duration(timeBetweenBlocks / 4)
	ticker := time.NewTicker(sleepTime * time.Second)

	log.WithField("Endpoint", client.Host).Info("Running...")

	for {

		var block *rpc.Block
		var err error

		if lostTicks > 4 {
			log.WithField("Endpoint", client.Host).Error("Lost Sync, Marking inactive")
			// TODO: inactive status
		}

		// watch for new head block
		_, block, err = client.Block(&rpc.BlockIDHead{})
		if err != nil {

			log.
				WithField("Endpoint", client.Host).
				WithError(err).
				Error("Unable to get /head block from RPC; Will try again")

		} else {

			// If just fetched block is current with others, then this client
			// is in sync with other clients.
			if b.recentBlockLevel == block.Metadata.Level.Level {
				lostTicks = 0

			} else if b.recentBlockLevel < block.Metadata.Level.Level &&
				b.recentBlockHash != block.Hash {

				// notify new block
				b.newBlockNotifier <- block

				b.lock.Lock()
				b.recentBlockLevel = block.Metadata.Level.Level
				b.recentBlockHash = block.Hash
				b.lock.Unlock()

				log.WithFields(log.Fields{
					"Cycle":   block.Metadata.Level.Cycle,
					"Level":   block.Metadata.Level.Level,
					"Hash":    block.Hash,
					"ChainID": block.ChainID,
				}).Info("New Block")

			} else {
				log.Error("Recent block greater than b.M.L.L")
			}
		}

		// wait here for timer, or shutdown
		select {
		case <-ticker.C:
			log.Debug("tick...")
		case <-shutdown:
			log.WithField("Endpoint", client.Host).Info("Shutting down RPC client")
			return
		}
	}
}
