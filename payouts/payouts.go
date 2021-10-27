package payouts

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"sync"

	_ "github.com/pkg/errors"

	"github.com/bakingbacon/go-tezos/v4/rpc"
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

var bakerFee = 8

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

	// Only handle payouts in levels 32-48 of cycle
// 	cyclePosition := block.Metadata.Level.CyclePosition
// 	if cyclePosition < 32 || cyclePosition > 48 {
// 		return
// 	}

	// Check if payouts already processed for this cycle
	// bb.GetPayoutsForCycle(block.Metadata.Level.Cycle)

	// The current cycle is X. Rewards for cycle X - $preservedCycles are
	// released in the last block of cycle X. BakinBacon will take action
	// after the start of X+1, thus we subtract an additional cycle to
	// determine the payouts cycle
	thisCycle := block.Metadata.Level.Cycle
	payoutCycle := thisCycle - (p.constants.PreservedCycles + 1)

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

	stakingBalance, err := strconv.Atoi(bakerInfo.StakingBalance)
	if err != nil {
		log.WithError(err).Error("Unable to convert baker staking balance")
		return
	}

	delegatedBalance, err := strconv.Atoi(bakerInfo.DelegatedBalance)
	if err != nil {
		log.WithError(err).Error("Unable to convert baker delegated balance")
		return
	}

	cycleRewardMetadata := &CycleRewardMetadata{
		PayoutCycle: payoutCycle,
		LevelOfPayoutCycle: firstLevelPayoutCycle,
		SnapshotIndex: chosenSnapshotIndex,
		SnapshotLevel: snapshotLevel,
		UnfrozenLevel: lastBlockUnfrozen,
		Balance: balance,
		NumDelegators: len(bakerInfo.DelegateContracts),
		StakingBalance: stakingBalance,
		DelegatedBalance: delegatedBalance,
		BlockRewards: blockRewards,
		FeeRewards: feeRewards,
	}

	// Marshal cycle reward metadata
	cycleRewardMetadataBytes, err := json.Marshal(cycleRewardMetadata)
	if err != nil {
		log.WithError(err).Error("Unable to marshal cycle reward metadata")
		return
	}

	// Save to DB
	if err := p.storage.SaveCycleRewardMetadata(payoutCycle, cycleRewardMetadataBytes); err != nil {
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

		// Marshal reward
		rewardRecordBytes, err := json.Marshal(rewardRecord)
		if err != nil {
			log.WithError(err).Error("Unable to marshal reward record")
			return
		}

		// Save reward record to DB
		if err := p.storage.SaveDelegatorReward(payoutCycle, rewardRecord.Delegator, rewardRecordBytes); err != nil {
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

func (p *PayoutsHandler) SendCyclePayouts(rewardCycle int) error {

	log.Info("Payouts sent")
	p.notifications.SendNotification("Payouts sent", notifications.PAYOUTS)

	return nil
}
