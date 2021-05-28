package baconclient

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/bakingbacon/go-tezos/v4/forge"
	"github.com/bakingbacon/go-tezos/v4/rpc"
	log "github.com/sirupsen/logrus"

	"bakinbacon/baconclient/baconsigner"
	"bakinbacon/storage"
)

const (
	MIN_BAKE_BALANCE = 8001
)

type BaconSlice struct {
	*rpc.Client
	isActive bool
}

type BaconClient struct {
	newBlockNotifier chan *rpc.Block

	Current    *BaconSlice
	rpcClients []*BaconSlice

	Status *BaconStatus
	Signer *baconsigner.BaconSigner

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

	clients := make([]*BaconSlice, 0)

	// Foreach endpoint, create an rpc client
	for _, e := range endpoints {

		active := true

		gtRpc, err := rpc.New(e)
		if err != nil {
			log.WithError(err).Error("Error from RPC")
			active = false
		}
		log.WithField("Endpoint", e).Info("Connected to RPC")

		// TODO: Add client even if error, but set "inActive" flag
		clients = append(clients, &BaconSlice{
			gtRpc,
			active,
		})
	}

	return &BaconClient{
		newBlockNotifier: make(chan *rpc.Block, 1),
		Current:          clients[0],
		rpcClients:       clients,
		Status:           &BaconStatus{},
		Signer:           baconsigner.New(),
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

func (b *BaconClient) blockWatch(client *BaconSlice, shutdown <-chan interface{}, wg *sync.WaitGroup) {

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
			log.WithField("Endpoint", client.Host).Warn("Lost Sync, Marking inactive")
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
			if b.Status.Level == block.Metadata.Level.Level {
				lostTicks = 0

			} else if block.Metadata.Level.Level > b.Status.Level &&
				block.Hash != b.Status.Hash {

				lostTicks = 0

				// notify new block
				b.newBlockNotifier <- block

				b.lock.Lock()
				b.Status.Hash = block.Hash
				b.Status.Level = block.Metadata.Level.Level
				b.Status.Cycle = block.Metadata.Level.Cycle
				b.Status.CyclePosition = block.Metadata.Level.CyclePosition
				b.Current = client
				b.lock.Unlock()

				log.WithFields(log.Fields{
					"Cycle":   block.Metadata.Level.Cycle,
					"Level":   block.Metadata.Level.Level,
					"Hash":    block.Hash,
					"ChainID": block.ChainID,
				}).Info("New Block")

			} else {
				log.WithFields(log.Fields{
					"Endpoint": client.Host, "Fetched": block.Metadata.Level.Level, "Current": b.Status.Level, "Lost": lostTicks,
				}).Trace("Endpoint Out of Sync")
				lostTicks += 1
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

func (b *BaconClient) CanBake() bool {

	// Passed these checks before? No need to check every new block.
	if b.Status.CanBake() {
		return true
	}

	// Seems we cannot bake; Run through list of checks.

	// Check that signer configuration is good
	if err := b.Signer.SignerStatus(); err != nil {
		b.Status.SetState(NO_SIGNER)
		b.Status.SetError(err)
		log.WithError(err).Error("Checking signer status")
		return false
	}

	// Registered as baker?
	if err := b.CheckBakerRegistered(); err != nil {
		b.Status.SetState(NOT_REGISTERED)
		b.Status.SetError(err)
		log.WithError(err).Error("Checking baker registration")
		return false
	}

	// If revealed, balance too low?
	if err := b.CheckBalance(); err != nil {
		b.Status.SetState(LOW_BALANCE)
		b.Status.SetError(err)
		log.WithError(err).Error("Checking baker balance")
		return false
	}

	// TODO: Other checks?

	// If you've passed all the checks, you should be good to bake
	b.Status.SetState(CAN_BAKE)
	b.Status.ClearError()

	return true
}

// Check if the baker is registered as a baker
func (b *BaconClient) CheckBakerRegistered() error {

	pkh := b.Signer.BakerPkh

	cdi := rpc.ContractInput{
		BlockID:    &rpc.BlockIDHead{},
		ContractID: pkh,
	}

	resp, contract, err := b.Current.Contract(cdi)

	log.WithFields(log.Fields{
		"Request": resp.Request.URL, "Response": string(resp.Body()),
	}).Trace("Fetching delegate info")

	if err != nil {
		return errors.Wrap(err, "Unable to fetch delegate info")
	}

	if contract.Delegate == "" {
		return errors.New("Delegate not registered")
	}

	// Baker PKH should be equal to PKH
	if contract.Delegate == pkh {
		return nil
	}

	return errors.New("Unknown error in determining baker status")
}

// Check if the balance of the baker is > 8001 Tez; Extra 1 Tez is for submitting reveal, if necessary
func (b *BaconClient) CheckBalance() error {

	pkh := b.Signer.BakerPkh

	dbi := rpc.DelegateBalanceInput{
		BlockID:  &rpc.BlockIDHead{},
		Delegate: pkh,
	}

	resp, balance, err := b.Current.DelegateBalance(dbi)

	log.WithFields(log.Fields{
		"Request": resp.Request.URL, "Response": string(resp.Body()),
	}).Trace("Fetching balance")

	if err != nil {
		return errors.Wrap(err, "Unable to fetch balance")
	}

	// Convert string to number
	bal, _ := strconv.Atoi(balance)
	if bal < MIN_BAKE_BALANCE {
		return errors.Errorf("Balance, %d XTZ, is too low", bal/1e6)
	}

	return nil
}

// Check if the baker has revealed their public key. If not, display UI message indicating the need to do this step.
func (b *BaconClient) IsRevealed() (bool, error) {

	pkh := b.Signer.BakerPkh

	cmki := rpc.ContractManagerKeyInput{
		BlockID:    &rpc.BlockIDHead{},
		ContractID: pkh,
	}

	resp, manager, err := b.Current.ContractManagerKey(cmki)

	log.WithFields(log.Fields{
		"Request": resp.Request.URL, "Response": string(resp.Body()), "Manager": manager,
	}).Debug("Fetching manager key")

	if err != nil {
		return false, errors.Wrap(err, "Unable to fetch manager key")
	}

	if manager == "" {
		log.Info("Baker address is not revealed")
		return false, nil
	}

	// Sanity check
	if strings.HasPrefix(manager, "edpk") {
		log.WithField("PK", manager).Info("Found public key for baker")
		return true, nil
	}

	// Something else happened
	return false, errors.New("Unable to determine state of public key reveal")
}

func (b *BaconClient) RegisterBaker() (string, error) {

	var registrationContents []rpc.Content

	pkh := b.Signer.BakerPkh

	// Need counter
	resp, counter, err := b.Current.ContractCounter(rpc.ContractCounterInput{
		BlockID:    &rpc.BlockIDHead{},
		ContractID: pkh,
	})

	log.WithFields(log.Fields{
		"Request": resp.Request.URL, "Response": string(resp.Body()),
	}).Debug("Fetching contract metadata")

	if err != nil {
		return "", errors.Wrap(err, "Unable to fetch contract metadata")
	}

	//
	// Revelation of PK
	// If needed, this operation needs to come first in a multi-op injection

	// Check reveal status because we need to include it if not revealed
	revealed, err := b.IsRevealed()
	if err != nil {
		return "", err
	}

	// Construct the reveal operation, if needed
	if !revealed {

		// Increment counter
		counter += 1

		// Get public key from source
		pk, err := b.Signer.GetPublicKey()
		if err != nil {
			return "", errors.Wrap(err, "Cannot register baker")
		}

		revealOp := rpc.Content{
			Kind:         rpc.REVEAL,
			Source:       pkh,
			Fee:          "359",
			Counter:      strconv.Itoa(counter),
			GasLimit:     "1000",
			StorageLimit: "0",
			PublicKey:    pk,
		}

		registrationContents = append(registrationContents, revealOp)
	}

	//
	// Registration operation
	//

	// Increment counter
	counter += 1

	// Create registration operation
	registrationOp := rpc.Content{
		Kind:         rpc.DELEGATION,
		Source:       pkh,
		Fee:          "358",
		Counter:      strconv.Itoa(counter),
		GasLimit:     "1100",
		StorageLimit: "0",
		Delegate:     pkh,
	}

	registrationContents = append(registrationContents, registrationOp)

	//
	// Forge the operation(s)
	//
	encodedOperation, err := forge.Encode(b.Status.Hash, registrationContents...) // Returns string hex-encoded operation
	if err != nil {
		return "", errors.Wrap(err, "Unable to register baker")
	}

	//
	// Sign the operation
	signerResult, err := b.Signer.SignSetDelegate(encodedOperation)
	if err != nil {
		return "", errors.Wrap(err, "Unable to sign registration")
	}

	//
	// Inject operation
	_, ophash, err := b.Current.InjectionOperation(rpc.InjectionOperationInput{
		Operation: signerResult.SignedOperation,
	})
	if err != nil {
		return "", errors.Wrap(err, "Failed to inject registration")
	}

	// Success
	return ophash, nil
}
