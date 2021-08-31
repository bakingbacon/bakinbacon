package main

import (
	log "github.com/sirupsen/logrus"

	"github.com/pkg/errors"

	"github.com/bakingbacon/go-tezos/v4/rpc"

	"bakinbacon/storage"
)

// Update BaconStatus with the most recent information from DB. This
// is done to initialize BaconStatus with values, otherwise status does
// not update until next bake/endorse.
func updateRecentBaconStatus() {

	// Update baconClient.Status with most recent endorsement
	recentEndorsementLevel, recentEndorsementHash, err := storage.DB.GetRecentEndorsement()
	if err != nil {
		log.WithError(err).Error("Unable to get recent endorsement")
	}

	bc.Status.SetRecentEndorsement(recentEndorsementLevel, getCycleFromLevel(recentEndorsementLevel), recentEndorsementHash)

	// Update baconClient.Status with most recent bake
	recentBakeLevel, recentBakeHash, err := storage.DB.GetRecentBake()
	if err != nil {
		log.WithError(err).Error("Unable to get recent bake")
	}

	bc.Status.SetRecentBake(recentBakeLevel, getCycleFromLevel(recentBakeLevel), recentBakeHash)
}

// Called on each new block; update BaconStatus with next opportunity for bakes/endorses
func updateCycleRightsStatus(metadataLevel rpc.Level) {

	nextCycle := metadataLevel.Cycle + 1

	// Update our baconStatus with next endorsement level and next baking right.
	// If this returns err, it means there was no bucket data which means
	// we have never fetched current cycle rights and should do so asap
	nextEndorsingLevel, highestFetchedCycle, err := storage.DB.GetNextEndorsingRight(metadataLevel.Level)
	if err != nil {
		log.WithError(err).Error("GetNextEndorsingRight")
	}

	// Update BaconClient status, even if next level is 0 (none found)
	nextEndorsingCycle := getCycleFromLevel(nextEndorsingLevel)
	bc.Status.SetNextEndorsement(nextEndorsingLevel, nextEndorsingCycle)

	log.WithFields(log.Fields{
		"Level": nextEndorsingLevel, "Cycle": nextEndorsingCycle,
	}).Trace("Next Endorsing")

	// If next level is 0, check to see if we need to fetch cycle
	if nextEndorsingLevel == 0 {
		switch {
		case highestFetchedCycle < metadataLevel.Cycle:
			log.WithField("Cycle", metadataLevel.Cycle).Info("Fetch Cycle Endorsing Rights")

			go fetchEndorsingRights(metadataLevel, metadataLevel.Cycle)

		case highestFetchedCycle < nextCycle:
			log.WithField("Cycle", nextCycle).Info("Fetch Next Cycle Endorsing Rights")

			go fetchEndorsingRights(metadataLevel, nextCycle)
		}
	}

	//
	// Next baking right; similar logic to above
	//
	nextBakeLevel, nextBakePriority, highestFetchedCycle, err := storage.DB.GetNextBakingRight(metadataLevel.Level)
	if err != nil {
		log.WithError(err).Error("GetNextEndorsingRight")
	}

	// Update BaconClient status, even if next level is 0 (none found)
	nextBakeCycle := getCycleFromLevel(nextBakeLevel)
	bc.Status.SetNextBake(nextBakeLevel, nextBakeCycle, nextBakePriority)

	log.WithFields(log.Fields{
		"Level": nextBakeLevel, "Cycle": nextBakeCycle, "Priority": nextBakePriority,
	}).Trace("Next Baking")

	if nextBakeLevel == 0 {
		switch {
		case highestFetchedCycle < metadataLevel.Cycle:
			log.WithField("Cycle", metadataLevel.Cycle).Info("Fetch Cycle Baking Rights")

			go fetchBakingRights(metadataLevel, metadataLevel.Cycle)

		case highestFetchedCycle < nextCycle:
			log.WithField("Cycle", nextCycle).Info("Fetch Next Cycle Baking Rights")

			go fetchBakingRights(metadataLevel, nextCycle)
		}
	}
}

// Called on each new block; Only processes every 1024 blocks
// Fetches the bake/endorse rights for the next cycle and stores to DB
func prefetchCycleRights(metadataLevel rpc.Level) {

	// We only prefetch every 1024 levels
	if metadataLevel.Level % 1024 != 0 {
		return
	}

	nextCycle := metadataLevel.Cycle + 1

	log.WithField("NextCycle", nextCycle).Info("Pre-fetching rights for next cycle")

	go fetchEndorsingRights(metadataLevel, nextCycle)
	go fetchBakingRights(metadataLevel, nextCycle)
}

func fetchEndorsingRights(metadataLevel rpc.Level, cycleToFetch int) {

	if bc.Signer.BakerPkh == "" {
		log.Error("Cannot fetch endorsing rights; No baker configured")
		return
	}

	// Due to inefficiencies in tezos-node RPC introduced by Granada,
	// we cannot query all rights of a delegate based on cycle.
	// This produces too much load on the node and usually times out.
	//
	// Instead, we make an insane number of fast RPCs to get rights
	// per level for the reminder of this cycle, or for the next cycle.

	blocksPerCycle := networkConstants[network].BlocksPerCycle

	levelToStart, levelToEnd, err := levelToStartEnd(metadataLevel, blocksPerCycle, cycleToFetch)
	if err != nil {
		log.WithError(err).Error("Unable to fetch endorsing rights")
		return
	}

	// Can't have more rights than blocks per cycle; set the
	// capacity of the slice to avoid reallocation on append
	allEndorsingRights := make([]rpc.EndorsingRights, 0, blocksPerCycle)

	// Range from start to end, fetch rights per level
	for level := levelToStart; level < levelToEnd; level++ {

		log.WithField("L", level).Trace("Fetching endorsing rights")

		endorsingRightsFilter := rpc.EndorsingRightsInput{
			BlockID:  &rpc.BlockIDHead{},
			Level:    level,
			Delegate: bc.Signer.BakerPkh,
		}

		resp, endorsingRights, err := bc.Current.EndorsingRights(endorsingRightsFilter)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"Request": resp.Request.URL, "Response": string(resp.Body()),
			}).Error("Unable to fetch endorsing rights")

			return
		}

		// Append this levels' rights, if exists
		if len(endorsingRights) > 0 {
			allEndorsingRights = append(allEndorsingRights, endorsingRights[0])
		}
	}

	log.WithFields(log.Fields{
		"Cycle": cycleToFetch, "LS": levelToStart, "LE": levelToEnd, "Num": len(allEndorsingRights),
	}).Debug("Prefetched Endorsing Rights")

	// Save rights to DB, even if len == 0 so that it is noted we queried this cycle
	if err := storage.DB.SaveEndorsingRightsForCycle(cycleToFetch, allEndorsingRights); err != nil {
		log.WithError(err).Error("Unable to save endorsing rights for cycle")
	}
}

func fetchBakingRights(metadataLevel rpc.Level, cycleToFetch int) {

	if bc.Signer.BakerPkh == "" {
		log.Error("Cannot fetch baking rights; No baker configured")
		return
	}

	blocksPerCycle := networkConstants[network].BlocksPerCycle

	levelToStart, levelToEnd, err := levelToStartEnd(metadataLevel, blocksPerCycle, cycleToFetch)
	if err != nil {
		log.WithError(err).Error("Unable to fetch baking rights")
		return
	}

	allBakingRights := make([]rpc.BakingRights, 0, blocksPerCycle)

	// Range from start to end, fetch rights per level
	for level := levelToStart; level < levelToEnd; level++ {

		bakingRightsFilter := rpc.BakingRightsInput{
			BlockID:  &rpc.BlockIDHead{},
			Level:    level,
			Delegate: bc.Signer.BakerPkh,
		}

		resp, bakingRights, err := bc.Current.BakingRights(bakingRightsFilter)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"Request": resp.Request.URL, "Response": string(resp.Body()),
			}).Error("Unable to fetch next cycle baking rights")

			return
		}

		// If have rights and priority is < max, append to slice
		if len(bakingRights) > 0 {
			if bakingRights[0].Priority < MAX_BAKE_PRIORITY {
				allBakingRights = append(allBakingRights, bakingRights[0])
			}
		}
	}

	// Got any rights?
	log.WithFields(log.Fields{
		"Cycle": cycleToFetch, "LS": levelToStart, "LE": levelToEnd, "Num": len(allBakingRights), "MaxPriority": MAX_BAKE_PRIORITY,
	}).Info("Prefetched Baking Rights")

	// Save filtered rights to DB, even if len == 0 so that it is noted we queried this cycle
	if err := storage.DB.SaveBakingRightsForCycle(cycleToFetch, allBakingRights); err != nil {
		log.WithError(err).Error("Unable to save baking rights for cycle")
	}
}

func levelToStartEnd(metadataLevel rpc.Level, blocksPerCycle, cycleToFetch int) (int, int, error) {

	var levelToStart, levelToEnd int
	levelsRemainingInCycle := (blocksPerCycle - metadataLevel.CyclePosition)

	// Are we fetching remaining rights in this level?
	if cycleToFetch == metadataLevel.Cycle {

		levelToStart = metadataLevel.Level
		levelToEnd = levelToStart + levelsRemainingInCycle + 1

	} else if cycleToFetch == (metadataLevel.Cycle + 1) {

		levelToStart = metadataLevel.Level + levelsRemainingInCycle
		levelToEnd = levelToStart + blocksPerCycle + 1

	} else {
		log.WithFields(log.Fields{
			"CycleToFetch": cycleToFetch, "CurrentCycle": metadataLevel.Cycle,
		}).Error("Unable to fetch endorsing rights")
		return 0, 0, errors.New("Unable to calculate start/end")
	}

	return levelToStart, levelToEnd, nil
}

func getCycleFromLevel(l int) int {

	gal := networkConstants[network].GranadaActivationLevel
	gac := networkConstants[network].GranadaActivationCycle

	// If level is before Granada activation, calculation is simple
	if l <= gal {
		return int(l / networkConstants[network].BlocksPerCycle)
	}

	// If level is after Granada activation, must take in to account the
	// change in number of blocks per cycle
	return int(((l - gal) / networkConstants[network].BlocksPerCycle) + gac)
}
