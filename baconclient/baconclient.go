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

	"bakinbacon/baconsigner"
	"bakinbacon/notifications"
	"bakinbacon/storage"
)

const (
	MinBakeBalance = 8001
)

type BaconSlice struct {
	*rpc.Client
	clientID int
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

	timeBetweenBlocks int
	globalShutdown    chan interface{}
	waitGroup         *sync.WaitGroup
}

func New(tbb int, shutdown chan interface{}, wg *sync.WaitGroup) (*BaconClient, error) {
	// Make new client manager
	newBaconClient := &BaconClient{
		NewBlockNotifier:  make(chan *rpc.Block, 1),
		rpcClients:        make([]*BaconSlice, 0),
		Status:            &BaconStatus{},
		timeBetweenBlocks: tbb,
		globalShutdown:    shutdown,
		waitGroup:         wg,
	}

	// Init bacon signer
	signer, err := baconsigner.New()
	if err != nil {
		return nil, errors.Wrap(err, "Cannot init bacon signer")
	}
	newBaconClient.Signer = signer

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
	go b.blockWatch(newBaconSlice)
}

func (b *BaconClient) Shutdown() {
	b.Signer.Close()
}

func (b *BaconClient) ShutdownRpc(rpcId int) error {

	newClients := make([]*BaconSlice, 0)

	// Iterate through list of rpc clients (BaconSlices) and find matching id
	for _, bslice := range b.rpcClients {
		if bslice.clientID == rpcId {
			close(bslice.shutdown)
		} else {
			newClients = append(newClients, bslice) // save those that did not match
		}
	}

	// new list with match removed
	// TODO: Is this a memory leak?
	b.rpcClients = newClients

	return nil
}

func (b *BaconClient) blockWatch(client *BaconSlice) {

	defer b.waitGroup.Done()

	lostTicks := 0

	// Get network constant time_between_blocks and set sleep-ticker to 50%
	sleepTime := time.Duration(b.timeBetweenBlocks / 2)
	ticker := time.NewTicker(sleepTime * time.Second)

	log.WithField("Endpoint", client.Host).Info("Blockwatch running...")

	for {

		var block *rpc.Block
		var err error

		if lostTicks > 4 {
			log.WithField("Endpoint", client.Host).Warn("Lost Sync, Marking inactive")
			client.isActive = false
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
				client.isActive = true

				if client.isActive {

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
				}

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
			log.WithField("Id", client.clientID).Debug("tick...")
		case <-client.shutdown:
			log.WithField("Endpoint", client.Host).Info("Shutting down RPC client")
			return
		case <-b.globalShutdown:
			log.WithField("Endpoint", client.Host).Info("(Global) Shutting down RPC client")
			return
		}
	}
}

func (b *BaconClient) CanBake(silentChecks bool) bool {

	// Always check status of signer, especially important for Ledger
	if err := b.Signer.SignerStatus(silentChecks); err != nil {
		b.Status.SetState(NoSigner)
		b.Status.SetError(err)
		notifications.N.Send(err.Error(), notifications.Signer)
		log.WithError(err).Error("Checking signer status")
		return false
	}

	// The remaining checks of being registered with Tezos network, and having
	// an appropriate balance happen on startup and can be cached
	if b.Status.State == CanBake {
		return true
	}

	// Registered as baker?
	if err := b.CheckBakerRegistered(); err != nil {
		b.Status.SetState(NotRegistered)
		b.Status.SetError(err)
		log.WithError(err).Error("Checking baker registration")
		return false
	}

	// If revealed, balance too low?
	if err := b.CheckDelegateBalance(); err != nil {
		b.Status.SetState(LowBalance)
		b.Status.SetError(err)
		log.WithError(err).Error("Checking baker balance")
		return false
	}

	// TODO: Other checks?

	// If you've passed all the checks, you should be good to bake
	b.Status.SetState(CanBake)
	b.Status.ClearError()

	return true
}

// CheckBakerRegistered Check if the baker is registered as a baker
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

// CheckDelegateBalance Check if the balance of the baker is > 8001 Tez; Extra 1 Tez is for submitting reveal, if necessary
func (b *BaconClient) CheckDelegateBalance() error {
	dbi := rpc.DelegateBalanceInput{
		BlockID:  new(rpc.BlockIDHead),
		Delegate: b.Signer.BakerPkh,
	}

	resp, delegateBalance, err := b.Current.DelegateBalance(dbi)

	if resp != nil {
		log.WithFields(log.Fields{
			"Request": resp.Request.URL, "Response": string(resp.Body()),
		}).Trace("Fetching delegate balance")
	}

	if err != nil {
		return errors.Wrap(err, "Unable to fetch delegate balance")
	}

	// Convert string to number
	balance, err := strconv.Atoi(delegateBalance)
	if err != nil {
		return errors.Wrap(err, "Unable to parse delegate balance")
	}

	if balance < MinBakeBalance {
		return errors.Errorf("Balance, %d XTZ, is too low", balance/1e6)
	}

	return nil
}

// GetSpendableBalance Returns spendable balance to determine if we can post bond
func (b *BaconClient) GetSpendableBalance() (int, error) {
	di := rpc.DelegateInput{
		BlockID:  new(rpc.BlockIDHead),
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

	frozen, err := strconv.Atoi(delegateInfo.FrozenBalance)
	if err != nil {
		return 0, errors.Wrap(err, "Unable to parse frozen balance")
	}

	return balance - frozen, nil
}

// IsRevealed Check if the baker has revealed their public key. If not, display UI message indicating the need to do this step.
func (b *BaconClient) IsRevealed() (bool, error) {

	cmki := rpc.ContractManagerKeyInput{
		BlockID:    new(rpc.BlockIDHead),
		ContractID: b.Signer.BakerPkh,
	}

	resp, manager, err := b.Current.ContractManagerKey(cmki)

	if resp != nil {
		log.WithFields(log.Fields{
			"Request": resp.Request.URL, "Response": string(resp.Body()), "Manager": manager,
		}).Debug("Fetching manager key")
	}

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

	if resp != nil {
		log.WithFields(log.Fields{
			"Request": resp.Request.URL, "Response": string(resp.Body()),
		}).Debug("Fetching contract metadata")
	}

	if err != nil {
		return "", errors.Wrap(err, "Unable to fetch contract metadata")
	}

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
		pk, _, err := b.Signer.GetPublicKey()
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

	// Registration operation

	// Increment counter
	counter += 1

	// Create registration operation
	registrationOp := rpc.Content{
		Kind:         rpc.DELEGATION,
		Source:       pkh,
		Fee:          "359",
		Counter:      strconv.Itoa(counter),
		GasLimit:     "1100",
		StorageLimit: "0",
		Delegate:     pkh,
	}

	registrationContents = append(registrationContents, registrationOp)

	// Forge the operation(s)
	// Returns string hex-encoded operation
	encodedOperation, err := forge.Encode(b.Status.Hash, registrationContents...)
	if err != nil {
		return "", errors.Wrap(err, "Unable to register baker")
	}

	// Sign the operation
	signerResult, err := b.Signer.SignSetDelegate(encodedOperation)
	if err != nil {
		return "", errors.Wrap(err, "Unable to sign registration")
	}

	// Inject operation
	_, opHash, err := b.Current.InjectionOperation(rpc.InjectionOperationInput{
		Operation: signerResult.SignedOperation,
	})
	if err != nil {
		return "", errors.Wrap(err, "Failed to inject registration")
	}

	// Success
	return opHash, nil
}

func (b *BaconClient) UpvoteProposal(proposal string, period int) (string, error) {
	pkh := b.Signer.BakerPkh

	proposalVote := rpc.Content{
		Kind:      rpc.PROPOSALS,
		Source:    pkh,
		Period:    period,
		Proposals: []string{proposal},
	}

	log.WithField("RPC", proposalVote).Debug("RPC-PROPOSAL")

	encodedOperation, err := forge.Encode(b.Status.Hash, proposalVote)
	if err != nil {
		return "", err
	}

	// Sign the operation
	signerResult, err := b.Signer.SignProposalVote(encodedOperation)
	if err != nil {
		return "", errors.Wrap(err, "Unable to sign upvote")
	}

	// Inject operation
	_, opHash, err := b.Current.InjectionOperation(rpc.InjectionOperationInput{
		Operation: signerResult.SignedOperation,
	})
	if err != nil {
		return "", errors.Wrap(err, "Failed to inject proposal upvote")
	}

	// Success
	return opHash, nil
}
