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
	MIN_BAKE_BALANCE            = 8001
	DEFAULT_TIME_BETWEEN_BLOCKS = 60
)

type BaconSlice struct {
	*rpc.Client
	clientId int
	isActive bool
	shutdown chan interface{}
}

type BaconClient struct {
	NewBlockNotifier chan *rpc.Block

	Current    *BaconSlice
	rpcClients []*BaconSlice

	Status *BaconStatus
	Signer *baconsigner.BaconSigner

	lock sync.Mutex

	shutdown  chan interface{}
	waitGroup *sync.WaitGroup
}

func New(globalShutdown chan interface{}, wg *sync.WaitGroup) (*BaconClient, error) {

	newBaconClient := &BaconClient{
		NewBlockNotifier: make(chan *rpc.Block, 1),
		rpcClients:       make([]*BaconSlice, 0),
		Status:           &BaconStatus{},
		Signer:           baconsigner.New(),
		shutdown:         globalShutdown,
		waitGroup:        wg,
	}

	// Pull endpoints from storage
	endpoints, err := storage.DB.GetRPCEndpoints()
	if err != nil {
		log.WithError(err).Error("Unable to get endpoints")
		return nil, errors.Wrap(err, "Failed DB.GetRPCEndpoints")
	}

	if len(endpoints) < 1 {
		// This shouldn't happen, but just in case
		log.Error("No endpoints found. Please add an RPC endpoint.")
	}

	// For each RPC client, thread off a polling monitor.
	for id, e := range endpoints {
		newBaconClient.AddRpc(id, e)

		// Small throttle to offset each poller
		<-time.After(2 * time.Second)
	}

	return newBaconClient, nil
}

func (b *BaconClient) AddRpc(rpcId int, rpcEndpointUrl string) {

	active := true

	gtRpc, err := rpc.New(rpcEndpointUrl)
	if err != nil {
		log.WithField("Endpoint", rpcEndpointUrl).WithError(err).Error("Error from RPC")
		active = false
	} else {
		log.WithField("Endpoint", rpcEndpointUrl).Info("Connected to RPC")
	}

	newBaconSlice := &BaconSlice{
		gtRpc,
		rpcId,
		active,
		make(chan interface{}, 1), // For shutting down individual BaconSlices
	}

	// Add client to list
	b.rpcClients = append(b.rpcClients, newBaconSlice)

	// Launch client
	b.waitGroup.Add(1)
	go b.blockWatch(newBaconSlice, b.shutdown, b.waitGroup)
}

func (b *BaconClient) ShutdownRpc(rpcId int) error {

	newClients := make([]*BaconSlice, 0)

	// Iterate through list of rpc clients (BaconSlices) and find matching id
	for _, r := range b.rpcClients {
		if r.clientId == rpcId {
			close(r.shutdown)
		} else {
			newClients = append(newClients, r) // save those that did not match
		}
	}

	// new list with match removed
	b.rpcClients = newClients

	return nil
}

func (b *BaconClient) blockWatch(client *BaconSlice, globalShutdown chan interface{}, wg *sync.WaitGroup) {

	defer wg.Done()

	lostTicks := 0

	// Get network constant time_between_blocks and set sleep-ticker to 25%
	timeBetweenBlocks := DEFAULT_TIME_BETWEEN_BLOCKS
	if client.isActive {
		// If an active RPC, get TBB from network constants
		// TODO: Update TBB once constants are loaded from inactive RPC
		timeBetweenBlocks = client.CurrentConstants().TimeBetweenBlocks[0]
	}
	sleepTime := time.Duration(timeBetweenBlocks / 4)
	ticker := time.NewTicker(sleepTime * time.Second)

	log.WithField("Endpoint", client.Host).Info("Blockwatch running...")

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
				Error("Unable to get /head; Will try again")

		} else {

			// If just fetched block is current with others, then this client
			// is in sync with other clients.
			if b.Status.Level == block.Metadata.Level.Level {
				lostTicks = 0

			} else if block.Metadata.Level.Level > b.Status.Level &&
				block.Hash != b.Status.Hash {

				lostTicks = 0

				// notify new block
				b.NewBlockNotifier <- block

				b.lock.Lock()
				b.Status.Hash = block.Hash
				b.Status.Level = block.Metadata.Level.Level
				b.Status.Cycle = block.Metadata.Level.Cycle
				b.Status.CyclePosition = block.Metadata.Level.CyclePosition

				if b.Current != client {
					log.WithField("Endpoint", client.Host).Warn("Switched active RPC")
					b.Current = client
				}

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
			log.WithField("Id", client.clientId).Debug("tick...")
		case <-client.shutdown:
			log.WithField("Endpoint", client.Host).Info("Shutting down RPC client")
			return
		case <-globalShutdown:
			log.WithField("Endpoint", client.Host).Info("(Global) Shutting down RPC client")
			return
		}
	}
}

func (b *BaconClient) CanBake() bool {

	// Passed these checks before? No need to check every new block.
	if b.Status.State == CAN_BAKE {
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
	if err := b.CheckDelegateBalance(); err != nil {
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
func (b *BaconClient) CheckDelegateBalance() error {

	dbi := rpc.DelegateBalanceInput{
		BlockID:  &rpc.BlockIDHead{},
		Delegate: b.Signer.BakerPkh,
	}

	resp, delegateBalance, err := b.Current.DelegateBalance(dbi)

	log.WithFields(log.Fields{
		"Request": resp.Request.URL, "Response": string(resp.Body()),
	}).Trace("Fetching delegate balance")

	if err != nil {
		return errors.Wrap(err, "Unable to fetch delegate balance")
	}

	// Convert string to number
	balance, _ := strconv.Atoi(delegateBalance)
	if err != nil {
		return errors.Wrap(err, "Unable to parse delegate balance")
	}

	if balance < MIN_BAKE_BALANCE {
		return errors.Errorf("Balance, %d XTZ, is too low", balance/1e6)
	}

	return nil
}

// Returns spendable balance to determine if we can post bond
func (b *BaconClient) GetSpendableBalance() (int, error) {

	di := rpc.DelegateInput{
		BlockID:  &rpc.BlockIDHead{},
		Delegate: b.Signer.BakerPkh,
	}

	resp, delegateInfo, err := b.Current.Delegate(di)

	log.WithFields(log.Fields{
		"Request": resp.Request.URL, "Response": string(resp.Body()),
	}).Trace("Fetching delegate balances")

	if err != nil {
		return 0, errors.Wrap(err, "Unable to fetch balances (delegate info)")
	}

	// Spendable balance is "total balance" - frozen balance
	balance, err := strconv.Atoi(delegateInfo.Balance)
	if err != nil {
		return 0, errors.Wrap(err, "Unable to parse balance")
	}

	frozen, _ := strconv.Atoi(delegateInfo.FrozenBalance)
	if err != nil {
		return 0, errors.Wrap(err, "Unable to parse frozen balance")
	}

	return balance - frozen, nil
}

// Check if the baker has revealed their public key. If not, display UI message indicating the need to do this step.
func (b *BaconClient) IsRevealed() (bool, error) {

	cmki := rpc.ContractManagerKeyInput{
		BlockID:    &rpc.BlockIDHead{},
		ContractID: b.Signer.BakerPkh,
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
