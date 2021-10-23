package main

import (
	"context"
	"math"
	"strconv"
	"sync"

	_ "github.com/pkg/errors"

	"github.com/bakingbacon/go-tezos/v4/rpc"
	log "github.com/sirupsen/logrus"

	_ "bakinbacon/baconsigner"
	_ "bakinbacon/notifications"
	"bakinbacon/payouts"
	"bakinbacon/storage"
	"bakinbacon/util"
)

var bakerFee = 8

// HandlePayouts At the beginning of each cycle, collect information about the baker's delegators and calculate
// how much is owed to each based on the baker's fee and % share of each delegator in the overall staking balance
func handlePayouts(ctx context.Context, wg *sync.WaitGroup, block rpc.Block, networkConstants util.Constants) {

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
	// storage.DB.GetPayoutsForCycle(block.Metadata.Level.Cycle)

	// The current cycle is X. Rewards for cycle X - $preservedCycles are
	// released in the last block of cycle X. BakinBacon will take action
	// after the start of X+1, thus we subtract an additional cycle to
	// determine the payouts cycle
	thisCycle := block.Metadata.Level.Cycle
	payoutCycle := thisCycle - (networkConstants.PreservedCycles + 1)

	// Calculate the first block of the payout cycle so we can determine the chosen snapshot index
	firstLevelPayoutCycle := networkConstants.GranadaActivationLevel + ((payoutCycle - networkConstants.GranadaActivationCycle - networkConstants.PreservedCycles) * networkConstants.BlocksPerCycle + 1)

	// Get the snapshot index for the payouts cycle
	resp, cycle, err := bc.Current.GetCycleAtHash(strconv.Itoa(firstLevelPayoutCycle), payoutCycle)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"Request": resp.Request.URL, "Response": string(resp.Body()),
		}).Error("Unable to GetCycleAtHash for payouts")
		return
	}

	chosenSnapshotIndex := cycle.RollSnapshot

	snapshotLevel := networkConstants.GranadaActivationLevel + ((payoutCycle - networkConstants.GranadaActivationCycle - networkConstants.PreservedCycles - 2) * networkConstants.BlocksPerCycle + (chosenSnapshotIndex + 1) * networkConstants.BlocksPerRollSnapshot)

	// This is the last block of the cycle which contains reward payout information
	// in the form of a 'balance_update'
	lastBlockUnfrozen := networkConstants.GranadaActivationLevel + ((payoutCycle - networkConstants.GranadaActivationCycle + networkConstants.PreservedCycles + 1) * networkConstants.BlocksPerCycle)

	log.WithFields(log.Fields{
		"ThisCycle": thisCycle, "RewardCycle": payoutCycle,
		"SnapshotIndex": chosenSnapshotIndex, "SnapshotLevel": snapshotLevel,
		"LastBlockUnfrozen": lastBlockUnfrozen, "FirstLevelPayoutCycle": firstLevelPayoutCycle,
	}).Info("Cycle Rewards Info")

	// Need bakers public key hash to query balance information
	_, pkh, err := bc.Signer.GetPublicKey()
	if err != nil {
		log.WithError(err).Error("Cannot get public key for payouts")
		return
	}

	// Get unfrozen rewards
	blockRewards, feeRewards, err := getUnfrozenRewards(pkh, payoutCycle, lastBlockUnfrozen)
	if err != nil {
		log.WithError(err).Error("getUnfrozenRewards failure")
		return
	}

	totalRewards := float64(blockRewards + feeRewards)
	bakerFeePct := float64(1 - (bakerFee / 100))

	// Query the snapshot level and get all delegators and their staking balances
	snapshotBlockID := rpc.BlockIDLevel(snapshotLevel)
	bakerInput := rpc.DelegateInput{
		BlockID: &snapshotBlockID,
		Delegate: pkh,
	}

	resp, bakerInfo, err := bc.Current.Delegate(bakerInput)
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

	cycleRewardMetadata := &payouts.CycleRewardMetadata{
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

	// Save to DB
	if err := storage.DB.SaveCycleRewardMetadata(payoutCycle, cycleRewardMetadata); err != nil {
		log.WithError(err).Error("Cannot save rewards metadata to DB")
		return
	}

	log.WithFields(log.Fields{
		"B": balance, "FB": bakerInfo.FrozenBalance, "SB": stakingBalance, "DB": delegatedBalance, "TR": totalRewards,
	}).Info("Baker Rewards Info")

	stakeBalance64 := float64(stakingBalance)

	// For each delegator, get their balance as of the snapshot block
	for _, delegatorAddress := range bakerInfo.DelegateContracts {

		// Skip ourselves
		if delegatorAddress == pkh {
			continue
		}

		// Reward record
		reward := &payouts.DelegatorReward{
			Delegator: delegatorAddress,
		}

		// Fetch delegator balance from RPC
		delegatorInput := rpc.ContractBalanceInput{
			BlockID: &snapshotBlockID,
			ContractID: delegatorAddress,
		}

		resp, _delegatorBalance, err := bc.Current.ContractBalance(delegatorInput)
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
		reward.Balance = delegatorBalance

		// Calculate delegator share %, rounded to 6 decimal places
		reward.SharePct = math.Round((float64(delegatorBalance) / stakeBalance64) * 1000000) / 1000000

		// Calculate delegator share of the rewards in mutez (integer)
		rewardShare := int(totalRewards * reward.SharePct)

		// Subtract baker fee
		reward.Reward = int(float64(rewardShare) * bakerFeePct)

		log.Infof("Delegator Rewards: D: %s, Bal: %d, SharePct: %.6f, RewardShare: %d, Reward: %.6f",
			reward.Delegator, reward.Balance, reward.SharePct, rewardShare, (reward.Reward / 1e6))

		// Save reward record to DB
		if err := storage.DB.SaveDelegatorReward(payoutCycle, reward); err != nil {
			log.WithError(err).Error("Unable to save delegator reward to DB. Aborting payouts.")
			return
		}
	}
}

// getUnfrozenRewards Get the metadata of the last block of the cycle where the rewards are unfrozen.
// Search for update records for our baker for fees and base rewards
func getUnfrozenRewards(bakerAddress string, rewardCycle int, unfrozenLevel int) (int, int, error) {

	unfrozenBlockID := rpc.BlockIDLevel(unfrozenLevel)
	resp, unfrozenBlockMetadata, err := bc.Current.Metadata(&unfrozenBlockID)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"Request": resp.Request.URL, "Response": string(resp.Body()),
		}).Error("Unable to get unfrozen rewards metadata for payouts")
		return 0, 0, err
	}

	// loop over balance_updates, looking for our baker
	var unfrozenRewards, unfrozenFees int

	for _, update := range unfrozenBlockMetadata.BalanceUpdates {

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
