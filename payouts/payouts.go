package payouts

import (
	"context"
	"math"
	"strconv"
	"sync"

	"github.com/pkg/errors"

	"github.com/bakingbacon/go-tezos/v4/rpc"
	"github.com/bakingbacon/go-tezos/v4/forge"
	log "github.com/sirupsen/logrus"

	"bakinbacon/baconclient"
	"bakinbacon/notifications"
	"bakinbacon/storage"
	"bakinbacon/util"
)

type PayoutsHandler struct {
	client        *baconclient.BaconClient
	constants     *util.NetworkConstants
	storage       *storage.Storage
	notifications *notifications.NotificationHandler
}

const (
	REACTIVATION_FEE = 257000
	TXN_FEE          = 395
	GAS_LIMIT        = 1420

	TZ1_BATCH_SIZE = 100
	KT1_BATCH_SIZE = 10

	// DB
	DB_PAYOUTS_BUCKET = "payouts"
	DB_METADATA       = "metadata"
)

var bakerFee = 8.0

func NewPayoutsHandler(bc *baconclient.BaconClient, db *storage.Storage, nc *util.NetworkConstants, nh *notifications.NotificationHandler) (*PayoutsHandler, error) {

	return &PayoutsHandler{
		client:        bc,
		constants:     nc,
		storage:       db,
		notifications: nh,
	}, nil
}

// HandlePayouts At the beginning of each cycle, collect information about the baker's delegators and calculate
// how much is owed to each based on the baker's fee and % share of each delegator in the overall staking balance
func (p *PayoutsHandler) HandlePayouts(ctx context.Context, wg *sync.WaitGroup, block rpc.Block) {

	// Decrement waitGroup on exit
	defer wg.Done()

	// Handle panic gracefully
	defer func() {
		if r := recover(); r != nil {
			log.WithField("Message", r).Error("Panic recovered in HandlePayouts")
		}
	}()

	// Only calculate payouts in levels 32-48 of cycle
// 	cyclePosition := block.Metadata.Level.CyclePosition
// 	if cyclePosition < 32 || cyclePosition > 48 {
// 		return
// 	}

	// The current cycle is X. Rewards for cycle X - $preservedCycles are
	// released in the last block of cycle X. BakinBacon will take action
	// after the start of X+1, thus we subtract an additional cycle to
	// determine the payouts cycle
	thisCycle := block.Metadata.Level.Cycle
	payoutCycle := thisCycle - (p.constants.PreservedCycles + 1)

	// Check if payouts have already calculated, or processed for this cycle
	cycleRewardMetadata, err := p.GetRewardMetadataForCycle(payoutCycle)
	if err != nil {
		log.WithError(err).Error("Unable to get payouts metadata from DB")
		return
	}

	// If no status for cycle (ie: nothing from DB), process this reward cycle, otherwise, return
	switch cycleRewardMetadata.Status {
	case CALCULATED:
	case DONE:
	case IN_PROGRESS:
		log.WithField("RewardCycle", payoutCycle).Info("Reward metadata already processed")
		return
	default:
		log.WithField("RewardCycle", payoutCycle).Info("Processing cycle reward metadata")
	}

	// Begin calculations

	// Calculate the first block of the payout cycle so we can determine the chosen snapshot index
	firstLevelPayoutCycle := p.constants.GranadaActivationLevel + ((payoutCycle - p.constants.GranadaActivationCycle - p.constants.PreservedCycles) * p.constants.BlocksPerCycle + 1)

	// Get the snapshot index for the payouts cycle
	resp, cycle, err := p.client.Current.GetCycleAtHash(strconv.Itoa(firstLevelPayoutCycle), payoutCycle)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"Request": resp.Request.URL, "Response": string(resp.Body()),
		}).Error("Unable to GetCycleAtHash for payouts")
		return
	}

	chosenSnapshotIndex := cycle.RollSnapshot

	snapshotLevel := p.constants.GranadaActivationLevel + ((payoutCycle - p.constants.GranadaActivationCycle - p.constants.PreservedCycles - 2) * p.constants.BlocksPerCycle + (chosenSnapshotIndex + 1) * p.constants.BlocksPerRollSnapshot)

	// This is the last block of the cycle which contains reward payout information
	// in the form of a 'balance_update'
	lastBlockUnfrozen := p.constants.GranadaActivationLevel + ((payoutCycle - p.constants.GranadaActivationCycle + p.constants.PreservedCycles + 1) * p.constants.BlocksPerCycle)

	cycleRewardMetadata.PayoutCycle = payoutCycle
	cycleRewardMetadata.LevelOfPayoutCycle = firstLevelPayoutCycle
	cycleRewardMetadata.SnapshotIndex = chosenSnapshotIndex
	cycleRewardMetadata.SnapshotLevel = snapshotLevel
	cycleRewardMetadata.UnfrozenLevel = lastBlockUnfrozen

	log.WithFields(log.Fields{
		"ThisCycle": thisCycle, "RewardCycle": payoutCycle,
		"SnapshotIndex": chosenSnapshotIndex, "SnapshotLevel": snapshotLevel,
		"LastBlockUnfrozen": lastBlockUnfrozen, "FirstLevelPayoutCycle": firstLevelPayoutCycle,
	}).Info("Cycle Rewards Info")

	// Need bakers public key hash to query balance information
	_, pkh, err := p.client.Signer.GetPublicKey()
	if err != nil {
		log.WithError(err).Error("Cannot get public key for payouts")
		return
	}

	// Get unfrozen rewards
	blockRewards, feeRewards, err := p.getUnfrozenRewards(pkh, payoutCycle, lastBlockUnfrozen)
	if err != nil {
		log.WithError(err).Error("getUnfrozenRewards failure")
		return
	}

	totalBakerRewards := float64(blockRewards + feeRewards)
	bakerFeePct := float64(1 - (bakerFee / 100))

	cycleRewardMetadata.BakerFee = bakerFee
	cycleRewardMetadata.BlockRewards = blockRewards
	cycleRewardMetadata.FeeRewards = feeRewards

	// Query the snapshot level and get all delegators and their staking balances
	snapshotBlockID := rpc.BlockIDLevel(snapshotLevel)
	bakerInput := rpc.DelegateInput{
		BlockID: &snapshotBlockID,
		Delegate: pkh,
	}

	resp, bakerInfo, err := p.client.Current.Delegate(bakerInput)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"Request": resp.Request.URL, "Response": string(resp.Body()),
		}).Error("Cannot get delegates info")
		return
	}

	// For saving metadata to DB
	balance, err := strconv.Atoi(bakerInfo.Balance)
	if err != nil {
		log.WithError(err).Error("Unable to convert baker balance")
		return
	}
	cycleRewardMetadata.Balance = balance

	stakingBalance, err := strconv.Atoi(bakerInfo.StakingBalance)
	if err != nil {
		log.WithError(err).Error("Unable to convert baker staking balance")
		return
	}
	cycleRewardMetadata.StakingBalance = stakingBalance

	delegatedBalance, err := strconv.Atoi(bakerInfo.DelegatedBalance)
	if err != nil {
		log.WithError(err).Error("Unable to convert baker delegated balance")
		return
	}
	cycleRewardMetadata.DelegatedBalance = delegatedBalance

	cycleRewardMetadata.NumDelegators = len(bakerInfo.DelegateContracts)
	cycleRewardMetadata.Status = CALCULATED

	// Save to DB
	if err := p.SaveRewardMetadataForCycle(payoutCycle, cycleRewardMetadata); err != nil {
		log.WithError(err).Error("Cannot save rewards metadata to DB")
		return
	}

	log.Infof("Baker Rewards Info: B: %d FB: %s SB: %d DB: %d TR: %.0f",
		balance, bakerInfo.FrozenBalance, stakingBalance, delegatedBalance, totalBakerRewards)

	stakeBalance64 := float64(stakingBalance)

	// For each delegator, get their balance as of the snapshot block
	for _, delegatorAddress := range bakerInfo.DelegateContracts {

		// Skip ourselves
		if delegatorAddress == pkh {
			continue
		}

		// Reward record
		rewardRecord := &DelegatorReward{
			Delegator: delegatorAddress,
		}

		// Fetch delegator balance from RPC
		delegatorInput := rpc.ContractBalanceInput{
			BlockID: &snapshotBlockID,
			ContractID: delegatorAddress,
		}

		resp, _delegatorBalance, err := p.client.Current.ContractBalance(delegatorInput)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"Request": resp.Request.URL, "Response": string(resp.Body()),
			}).Error("Cannot get delegator info")
			return
		}

		// Parse string'd balance to integer (mutez)
		delegatorBalance, err := strconv.Atoi(_delegatorBalance)
		if err != nil {
			log.WithError(err).Error("Cannot parse delegator balance.")
			return
		}
		rewardRecord.Balance = delegatorBalance

		// There's a lot of int/float business going on here. This is golang issues around
		// division and multiplication along with attempts to keep monetary calculations
		// based in mutez (int)

		// Calculate delegator share %, rounded to 6 decimal places
		rewardRecord.SharePct = math.Round((float64(delegatorBalance) / stakeBalance64) * 1000000) / 1000000

		// Calculate delegator share of the rewards in mutez
		rewardShareRevenue := int(totalBakerRewards * rewardRecord.SharePct)

		// Subtract baker fee
		rewardRecord.Reward = int(float64(rewardShareRevenue) * bakerFeePct)

		log.Infof("Delegator Rewards: D: %s, Bal: %d, SharePct: %.6f, RewardShareRev: %d, RewardShareNet: %.6f",
			rewardRecord.Delegator, rewardRecord.Balance, rewardRecord.SharePct, rewardShareRevenue, float64(rewardRecord.Reward)/1e6)

		// Save reward record to DB
		if err := p.SaveDelegatorReward(payoutCycle, rewardRecord); err != nil {
			log.WithError(err).Error("Unable to save delegator reward to DB. Aborting payouts.")
			return
		}
	}
}

// getUnfrozenRewards Get the metadata of the last block of the cycle where the rewards are unfrozen.
// Search for update records for our baker for fees and base rewards
func (p *PayoutsHandler) getUnfrozenRewards(bakerAddress string, rewardCycle int, unfrozenLevel int) (int, int, error) {

	unfrozenBlockID := rpc.BlockIDLevel(unfrozenLevel)
	resp, unfrozenBlockMetadata, err := p.client.Current.Metadata(&unfrozenBlockID)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"Request": resp.Request.URL, "Response": string(resp.Body()),
		}).Error("Unable to get unfrozen rewards metadata for payouts")

		return 0, 0, err
	}

	// loop over balance_updates, looking for our baker
	var unfrozenRewards, unfrozenFees int

	for _, update := range unfrozenBlockMetadata.BalanceUpdates {

		// nolint:nestif
		if update.Kind == "freezer" && update.Delegate == bakerAddress && update.Cycle == rewardCycle {

			if update.Category == "rewards" {

				tmp, err := strconv.Atoi(update.Change)
				if err != nil {
					return 0, 0, err
				}
				unfrozenRewards = tmp * -1

			} else if update.Category == "fees" {

				tmp, err := strconv.Atoi(update.Change)
				if err != nil {
					return 0, 0, err
				}
				unfrozenFees = tmp * -1
			}
		}
	}

	return unfrozenRewards, unfrozenFees, nil
}

// SendCyclePayouts will launch a func to run in the background which will batch up all unpaid
// rewards for rewardCycle and send them to the node for injection.
func (p *PayoutsHandler) SendCyclePayouts(rewardCycle int) error {

	// Get metadata for this payout
	cycleRewardMetadata, err := p.GetRewardMetadataForCycle(rewardCycle)
	if err != nil {
		return errors.Wrap(err, "Unable to get payouts metadata from DB")
	}

	// Update status of cycle payouts
	cycleRewardMetadata.Status = IN_PROGRESS

	// Save to DB
	if err := p.SaveRewardMetadataForCycle(rewardCycle, cycleRewardMetadata); err != nil {
		return errors.Wrap(err, "Cannot save rewards metadata to DB")
	}

	// Need baker's public key hash to create transactions
	_, pkh, err := p.client.Signer.GetPublicKey()
	if err != nil {
		return errors.Wrap(err, "Cannot get public key for payouts")
	}

	// Need baker's current txn counter
	bakerCounterInput := rpc.ContractCounterInput{
		BlockID:    &rpc.BlockIDHead{},
		ContractID: pkh,
	}

	resp, counter, err := p.client.Current.ContractCounter(bakerCounterInput)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"Request": resp.Request.URL, "Response": string(resp.Body()),
		}).Error("Baker txn counter")
		return errors.Wrap(err, "Cannot get baker txn counter")
	}

	// background function to create, sign, and inject transactions
	go p.createInjectRewards(rewardCycle, pkh, counter)

	return nil
}

func (p *PayoutsHandler) createInjectRewards(rewardsCycle int, bakerPkh string, bakerCounter int) {

	// A batch bucket
	var curBatch []rpc.Content

	// Fetch individual reward data
	rewardsData, err := p.GetDelegatorRewardAllForCycle(rewardsCycle)
	if err != nil {
		log.WithError(err).Error("Cannot get payouts from DB")
		return
	}

	// How many batches do we need?
	numRewardsData := len(rewardsData)
	numBatches := int(math.Ceil(float64(numRewardsData) / TZ1_BATCH_SIZE))
	txnBatches := make([][]rpc.Content, numBatches)

	var batchCounter, rewardsCounter int

	// Cant get index, val from this range due to being a map. Have to counter++ it.
	for _, r := range rewardsData {

		// Need the current balance of the delegator. If balance is 0,
		// we will deduct reactivation cost from their reward before sending

		delegatorNetReward := r.Reward

		cbi := rpc.ContractBalanceInput{
			BlockID:    &rpc.BlockIDHead{},
			ContractID: r.Delegator,
		}

		resp, _delegatorBalance, err := p.client.Current.ContractBalance(cbi)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"Request": resp.Request.URL, "Response": string(resp.Body()),
			}).Error("Cannot get delegator current balance")
			continue
		}

		// Parse string'd balance to integer (mutez)
		delegatorBalance, err := strconv.Atoi(_delegatorBalance)
		if err != nil {
			log.WithError(err).Error("Cannot parse delegator balance.")
			continue
		}

		if delegatorBalance == 0 {
			delegatorNetReward -= REACTIVATION_FEE
			log.WithFields(log.Fields{
				"D": r.Delegator, "B": delegatorBalance,
			}).Trace("Delegator needs reactivation")
		}

		// Delegator's pay for the transaction fee
		delegatorNetReward -= TXN_FEE

		// Baker fee already deducted during handlePayouts()

		// Anything to actually pay? If so, construct the transaction and append to batch
		if delegatorNetReward > 0 {

			bakerCounter++

			txn := rpc.Transaction{
				Source:      bakerPkh,
				Destination: r.Delegator,
				Amount:      strconv.Itoa(delegatorNetReward),
				Fee:         strconv.Itoa(TXN_FEE),
				GasLimit:    strconv.Itoa(GAS_LIMIT),
				Counter:     strconv.Itoa(bakerCounter),
			}

			// Convert to generic 'content' before appending to batch
			curBatch = append(curBatch, txn.ToContent())
		}

		// Count how many rewards we've processed
		rewardsCounter++

		// If our current batch equals batch size, add batch to bucket and reset
		if rewardsCounter == TZ1_BATCH_SIZE || rewardsCounter == numRewardsData {
			txnBatches[batchCounter] = curBatch
			curBatch = nil

			// Go to next batch
			batchCounter++
		}
	}

	// Now that we have all the batches, sign and send them

	for i, batch := range txnBatches {

		// Forge the entire batch as 1 operation
		batchOp, err := forge.Encode(p.client.HeadHash(), batch...)
		if err != nil {
			log.WithError(err).Error("Unable to forge-encode payouts batch")
			continue
		}

		// Sign the operation
		signerResult, err := p.client.Signer.SignTransaction(batchOp)
		if err != nil {
			log.WithError(err).Error("Unable to sign payouts batch")
			continue
		}

		// Inject operation
		_, opHash, err := p.client.Current.InjectionOperation(rpc.InjectionOperationInput{
			Operation: signerResult.SignedOperation,
		})
		if err != nil {
			log.WithError(err).Error("Failed to inject registration")
			continue
		}

		// Update database with opHash for each delegator reward
		for _, c := range batch {

			if err := p.updateDelegatorRewardOpHash(c.Destination, rewardsCycle, opHash); err != nil {
				log.WithError(err).Error("Unable to update reward opHash")
				continue
			}

			log.WithFields(log.Fields{
				"D": c.Destination, "A": c.Amount, "F": c.Fee,
			}).Debugf("Cycle %d Rewards Payout Batch #%d", rewardsCycle, i)
		}

		// Give some logging
		log.WithField("OpHash", opHash).Infof("Payouts Batch #%d Injected", i)
	}

	// After all batches processed, update cycle payout status
	if err := p.setCyclePayoutStatus(rewardsCycle, DONE); err != nil {
		log.WithError(err).Error("Unable to update cycle status to DONE")
	}
}
