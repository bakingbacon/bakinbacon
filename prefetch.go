package main

import (
	"bakinbacon/util"
	log "github.com/sirupsen/logrus"

	"github.com/pkg/errors"

	"github.com/bakingbacon/go-tezos/v4/rpc"
)

// Update BaconStatus with the most recent information from DB. This
// is done to initialize BaconStatus with values, otherwise status does
// not update until next bake/endorse.
func (s *BakinBaconServer) updateRecentBaconStatus() {
	// Update baconClient.Status with most recent endorsement
	recentEndorsementLevel, recentEndorsementHash, err := s.GetRecentEndorsement()
	if err != nil {
		log.WithError(err).Error("Unable to get recent endorsement")
	}

	s.Status.SetRecentEndorsement(recentEndorsementLevel, s.getCycleFromLevel(recentEndorsementLevel), recentEndorsementHash)

	// Update baconClient.Status with most recent bake
	recentBakeLevel, recentBakeHash, err := s.GetRecentBake()
	if err != nil {
		log.WithError(err).Error("Unable to get recent bake")
	}

	s.Status.SetRecentBake(recentBakeLevel, s.getCycleFromLevel(recentBakeLevel), recentBakeHash)
}

// Called on each new block; update BaconStatus with next opportunity for bakes/endorses
func (s *BakinBaconServer) updateCycleRightsStatus(metadataLevel rpc.Level) {

	nextCycle := metadataLevel.Cycle + 1

	// Update our baconStatus with next endorsement level and next baking right.
	// If this returns err, it means there was no bucket data which means
	// we have never fetched current cycle rights and should do so asap
	nextEndorsingLevel, highestFetchedCycle, err := s.GetNextEndorsingRight(metadataLevel.Level)
	if err != nil {
		log.WithError(err).Error("GetNextEndorsingRight")
	}

	// Update BaconClient status, even if next level is 0 (none found)
	nextEndorsingCycle := s.getCycleFromLevel(nextEndorsingLevel)
	s.Status.SetNextEndorsement(nextEndorsingLevel, nextEndorsingCycle)

	log.WithFields(log.Fields{
		"Level": nextEndorsingLevel, "Cycle": nextEndorsingCycle,
	}).Trace("Next Endorsing")

	// If next level is 0, check to see if we need to fetch cycle
	if nextEndorsingLevel == 0 {
		switch {
		case highestFetchedCycle < metadataLevel.Cycle:
			log.WithField("Cycle", metadataLevel.Cycle).Info("Fetch Cycle Endorsing Rights")

			go s.fetchEndorsingRights(metadataLevel, metadataLevel.Cycle)

		case highestFetchedCycle < nextCycle:
			log.WithField("Cycle", nextCycle).Info("Fetch Next Cycle Endorsing Rights")

			go s.fetchEndorsingRights(metadataLevel, nextCycle)
		}
	}

	// Next baking right; similar logic to above
	nextBakeLevel, nextBakePriority, highestFetchedCycle, err := s.GetNextBakingRight(metadataLevel.Level)
	if err != nil {
		log.WithError(err).Error("GetNextEndorsingRight")
	}

	// Update BaconClient status, even if next level is 0 (none found)
	nextBakeCycle := s.getCycleFromLevel(nextBakeLevel)
	s.Status.SetNextBake(nextBakeLevel, nextBakeCycle, nextBakePriority)

	log.WithFields(log.Fields{
		"Level": nextBakeLevel, "Cycle": nextBakeCycle, "Priority": nextBakePriority,
	}).Trace("Next Baking")

	if nextBakeLevel == 0 {
		switch {
		case highestFetchedCycle < metadataLevel.Cycle:
			log.WithField("Cycle", metadataLevel.Cycle).Info("Fetch Cycle Baking Rights")

			go s.fetchBakingRights(metadataLevel, metadataLevel.Cycle)

		case highestFetchedCycle < nextCycle:
			log.WithField("Cycle", nextCycle).Info("Fetch Next Cycle Baking Rights")

			go s.fetchBakingRights(metadataLevel, nextCycle)
		}
	}
}

// Called on each new block; Only processes every 1024 blocks
// Fetches the bake/endorse rights for the next cycle and stores to DB
func (s *BakinBaconServer) prefetchCycleRights(metadataLevel rpc.Level) {

	// We only prefetch every 1024 levels
	if metadataLevel.Level % 1024 != 0 {
		return
	}

	nextCycle := metadataLevel.Cycle + 1

	log.WithField("NextCycle", nextCycle).Info("Pre-fetching rights for next cycle")

	go s.fetchEndorsingRights(metadataLevel, nextCycle)
	go s.fetchBakingRights(metadataLevel, nextCycle)
}

func (s *BakinBaconServer) fetchEndorsingRights(metadataLevel rpc.Level, cycleToFetch int) {

	if s.Signer.BakerPkh == "" {
		log.Error("Cannot fetch endorsing rights; No baker configured")
		return
	}

	// Due to inefficiencies in tezos-node RPC introduced by Granada,
	// we cannot query all rights of a delegate based on cycle.
	// This produces too much load on the node and usually times out.
	//
	// Instead, we make an insane number of fast RPCs to get rights
	// per level for the reminder of this cycle, or for the next cycle.

	blocksPerCycle := util.NetworkConstants[s.networkName].BlocksPerCycle

	levelToStart, levelToEnd, err := s.levelToStartEnd(metadataLevel, blocksPerCycle, cycleToFetch)
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
			Delegate: s.Signer.BakerPkh,
		}

		resp, endorsingRights, err := s.Current.EndorsingRights(endorsingRightsFilter)
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
	if err := s.SaveEndorsingRightsForCycle(cycleToFetch, allEndorsingRights); err != nil {
		log.WithError(err).Error("Unable to save endorsing rights for cycle")
	}
}

func (s *BakinBaconServer) fetchBakingRights(metadataLevel rpc.Level, cycleToFetch int) {
	if s.Signer.BakerPkh == "" {
		log.Error("Cannot fetch baking rights; No baker configured")
		return
	}

	blocksPerCycle := util.NetworkConstants[s.networkName].BlocksPerCycle

	levelToStart, levelToEnd, err := s.levelToStartEnd(metadataLevel, blocksPerCycle, cycleToFetch)
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
			Delegate: s.Signer.BakerPkh,
		}

		resp, bakingRights, err := s.Current.BakingRights(bakingRightsFilter)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"Request": resp.Request.URL, "Response": string(resp.Body()),
			}).Error("Unable to fetch next cycle baking rights")

			return
		}

		// If we have rights and priority is < max, append to slice
		if len(bakingRights) > 0 {
			if bakingRights[0].Priority < MaxBakePriority {
				allBakingRights = append(allBakingRights, bakingRights[0])
			}
		}
	}

	// Got any rights?
	log.WithFields(log.Fields{
		"Cycle": cycleToFetch, "LS": levelToStart, "LE": levelToEnd, "Num": len(allBakingRights), "MaxPriority": MaxBakePriority,
	}).Info("Prefetched Baking Rights")

	// Save filtered rights to DB, even if len == 0 so that it is noted we queried this cycle
	if err := s.SaveBakingRightsForCycle(cycleToFetch, allBakingRights); err != nil {
		log.WithError(err).Error("Unable to save baking rights for cycle")
	}
}

func (s *BakinBaconServer) levelToStartEnd(metadataLevel rpc.Level, blocksPerCycle, cycleToFetch int) (int, int, error) {

	var levelToStart, levelToEnd int
	levelsRemainingInCycle := blocksPerCycle - metadataLevel.CyclePosition

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

func (s *BakinBaconServer) getCycleFromLevel(l int) int {

	gal := util.NetworkConstants[s.networkName].GranadaActivationLevel
	gac := util.NetworkConstants[s.networkName].GranadaActivationCycle

	// If level is before Granada activation, calculation is simple
	if l <= gal {
		return int(l / util.NetworkConstants[s.networkName].BlocksPerCycle)
	}

	// If level is after Granada activation, must take into account the
	// change in number of blocks per cycle
	return int(((l - gal) / util.NetworkConstants[s.networkName].BlocksPerCycle) + gac)
}
