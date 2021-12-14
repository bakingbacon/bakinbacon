package payouts

import (
	"context"
	"fmt"
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
	Disabled      bool
}

const (
	REACTIVATION_FEE     = 277000
	REACTIVATION_STORAGE = 300

	TXN_FEE              = 395
	GAS_LIMIT            = 1520 // tezos-client uses 1420 + 100
	STORAGE_LIMIT        = 0

	TZ1_BATCH_SIZE = 100
	KT1_BATCH_SIZE = 10

	// DB
	DB_PAYOUTS_BUCKET = "payouts"
	DB_METADATA       = "metadata"
)

func NewPayoutsHandler(bc *baconclient.BaconClient, db *storage.Storage, nc *util.NetworkConstants, nh *notifications.NotificationHandler, noPayouts bool) (*PayoutsHandler, error) {

	return &PayoutsHandler{
		client:        bc,
		constants:     nc,
		storage:       db,
		notifications: nh,
		Disabled:      noPayouts,
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

	// Only calculate payouts in levels 32-64 of cycle
	cyclePosition := block.Metadata.Level.CyclePosition
	if cyclePosition < 32 || cyclePosition > 64 {
		return
	}

	// disabled?
	if p.Disabled {
		log.Info("Payouts functionality is disabled")
		return
	}

	// The current cycle is X. Rewards for cycle X - $preservedCycles are
	// released in the last block of cycle X. BakinBacon will take action
	// after the start of X+1, thus we subtract an additional cycle to
	// determine the payouts cycle
	thisCycle := block.Metadata.Level.Cycle
	payoutCycle := thisCycle - (p.constants.PreservedCycles + 1)

	// Check if payouts have already calculated, or processed for this cycle
	cycleRewardMetadata, err := p.GetRewardMetadataForCycle(payoutCycle)
	if err != nil {
		log.WithError(err).WithField("PC", payoutCycle).Error("Unable to get payouts metadata from DB")
		return
	}

	// If no status for cycle (ie: nothing from DB), process this reward cycle, otherwise, return
	switch cycleRewardMetadata.Status {
	case CALCULATED, DONE, IN_PROGRESS:
		log.WithField("RewardCycle", payoutCycle).Info("Reward metadata for cycle already processed")
		return
	default:
		msg := fmt.Sprintf("Calculating rewards for cycle %d", payoutCycle)
		p.notifications.SendNotification(msg, notifications.PAYOUTS)
		log.WithField("RewardCycle", payoutCycle).Info(msg)
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

	// Get baker settings to determine their fee
	bakerSettings, err := p.storage.GetBakerSettings()
	if err != nil {
		log.WithError(err).Error("Unable to get baker settings for payouts")
		return
	}

	bakerFee, err := strconv.ParseFloat(bakerSettings["bakerfee"].(string), 64)
	if err != nil {
		log.WithError(err).Error("Cannot convert baker fee for payouts")
		return
	}

	totalBakerRewards := float64(blockRewards + feeRewards)
	bakerFeePct := float64(1 - (cycleRewardMetadata.BakerFee / 100))

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

	// Subtract ourselves since bakers delegate to themselves
	cycleRewardMetadata.NumDelegators = len(bakerInfo.DelegateContracts) - 1
	cycleRewardMetadata.Status = CALCULATED

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
		rewardRecord := DelegatorReward{
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

	// No delegators to process? Then done!
	if cycleRewardMetadata.NumDelegators == 0 {
		cycleRewardMetadata.Status = DONE
	}

	// Save status to DB and send notification
	if err := p.SaveRewardMetadataForCycle(payoutCycle, cycleRewardMetadata); err != nil {
		msg := "Cannot save cycle rewards metadata to DB"
		log.WithError(err).Error(msg)
		p.notifications.SendNotification(msg, notifications.PAYOUTS)

		return
	}

	msg := fmt.Sprintf("Rewards calculations for cycle %d are complete. Use the UI to submit transactions.", payoutCycle)
	log.Info(msg)
	p.notifications.SendNotification(msg, notifications.PAYOUTS)
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

	// Set payouts processing state
	if err := p.setCyclePayoutStatus(rewardCycle, IN_PROGRESS); err != nil {
		return errors.Wrap(err, "Cannot update rewards metadata status")
	}

	// Need baker's public key hash to create transactions
	_, pkh, err := p.client.Signer.GetPublicKey()
	if err != nil {
		return errors.Wrap(err, "Cannot get public key for payouts")
	}

	// Need baker's current txn counter
	bakerTxnCounterInput := rpc.ContractCounterInput{
		BlockID:    &rpc.BlockIDHead{},
		ContractID: pkh,
	}

	resp, counter, err := p.client.Current.ContractCounter(bakerTxnCounterInput)
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

func (p *PayoutsHandler) createInjectRewards(rewardsCycle int, bakerPkh string, bakerTxnCounter int) {

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

	log.WithFields(log.Fields{
		"RewardCycle": rewardsCycle, "NumRewards": numRewardsData, "NumBatches": numBatches, "BatchSize": TZ1_BATCH_SIZE,
	}).Info("Creating rewards payouts batches")

	var batchCounter, rewardsCounter, curBatchSize int

	// Cant get index, val from this range due to being a map. Have to counter++ it.
	for _, r := range rewardsData {

		delegatorNetReward := r.Reward
		txnFee       := TXN_FEE
		storageLimit := STORAGE_LIMIT
		gasLimit     := GAS_LIMIT

		// Need the current balance of the delegator. If balance is 0,
		// we will deduct reactivation cost from their reward before sending

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

		// Need reactivation
		if delegatorBalance == 0 {

			// Charge reactivation to delegator
			delegatorNetReward -= REACTIVATION_FEE

			// Need storage to reactivate
			storageLimit += REACTIVATION_STORAGE

			// Increased storage requires additional fee in the txn
			txnFee += REACTIVATION_FEE

			log.WithFields(log.Fields{
				"D": r.Delegator, "B": delegatorBalance,
			}).Trace("Delegator needs reactivation")
		}

		// Delegator's pay for the transaction fee
		delegatorNetReward -= txnFee

		// Baker fee already deducted during handlePayouts()

		// Anything to actually pay? If so, construct the transaction and append to batch
		if delegatorNetReward > 0 {

			bakerTxnCounter++

			txn := rpc.Transaction{
				Kind:         rpc.TRANSACTION,
				Source:       bakerPkh,
				Destination:  r.Delegator,
				Amount:       strconv.Itoa(delegatorNetReward),
				Fee:          strconv.Itoa(txnFee),
				GasLimit:     strconv.Itoa(gasLimit),
				StorageLimit: strconv.Itoa(storageLimit),
				Counter:      strconv.Itoa(bakerTxnCounter),
			}

			// Convert to generic 'content' before appending to batch
			curBatch = append(curBatch, txn.ToContent())
			curBatchSize++

		} else {
			log.WithField("D", r.Delegator).Debug("Delegator reward is 0 XTZ; No payout required.")
		}

		// Count how many rewards we've processed
		rewardsCounter++

		// If our current batch equals batch size, or we've finished processing all rewards,
		// then add this batch to bucket and keep looping
		if curBatchSize == TZ1_BATCH_SIZE || rewardsCounter == numRewardsData {

			txnBatches[batchCounter] = curBatch
			curBatch = nil
			curBatchSize = 0

			// Go to next batch
			batchCounter++
		}
	}

	// any rewards?
	if len(txnBatches) < 1 {

		msg := fmt.Sprintf("No rewards to send for cycle %d", rewardsCycle)
		log.Info(msg)
		p.notifications.SendNotification(msg, notifications.PAYOUTS)

	} else {

		log.Info("Rewards payouts batches created; Sign/Inject Phase...")

		// Now that we have all the batches, sign and send them

		for i, batch := range txnBatches {

			// Forge the entire batch as 1 operation
			batchOp, err := forge.Encode(p.client.HeadHash(), batch...)
			if err != nil {
				log.WithError(err).Error("Unable to forge-encode payouts batch")
				if err := p.setCyclePayoutStatus(rewardsCycle, ERROR); err != nil {
					log.WithError(err).Error("Unable to update cycle status to ERROR")
				}
				return
			}

			// Sign the operation
			signerResult, err := p.client.Signer.SignTransaction(batchOp)
			if err != nil {
				log.WithError(err).Error("Unable to sign payouts batch")
				if err := p.setCyclePayoutStatus(rewardsCycle, ERROR); err != nil {
					log.WithError(err).Error("Unable to update cycle status to ERROR")
				}
				return
			}

			// Inject operation
			_, opHash, err := p.client.Current.InjectionOperation(rpc.InjectionOperationInput{
				Operation: signerResult.SignedOperation,
			})
			if err != nil {
				log.WithError(err).Error("Failed to inject batch transaction")
				if err := p.setCyclePayoutStatus(rewardsCycle, ERROR); err != nil {
					log.WithError(err).Error("Unable to update cycle status to ERROR")
				}
				return
			}

			// Update database with opHash for each delegator reward
			for _, c := range batch {

				if err := p.updateDelegatorRewardOpHash(c.Destination, rewardsCycle, opHash); err != nil {

					log.WithError(err).WithFields(log.Fields{
						"RewardCycle": rewardsCycle, "OpHash": opHash,
					}).Error("Unable to update reward opHash")

					if err := p.setCyclePayoutStatus(rewardsCycle, ERROR); err != nil {
						log.WithError(err).Error("Unable to update cycle status to ERROR")
					}

					continue
				}

				log.WithFields(log.Fields{
					"D": c.Destination, "A": c.Amount, "F": c.Fee,
				}).Debugf("Cycle %d Rewards Payout Batch #%d", rewardsCycle, i+1)
			}

			// Give some logging
			log.WithField("OpHash", opHash).Infof("Payouts Batch #%d Injected", i+1)
		}
	}

	// After all batches processed, update cycle payout status
	if err := p.setCyclePayoutStatus(rewardsCycle, DONE); err != nil {
		log.WithError(err).Error("Unable to update cycle status to DONE")
	}
}
