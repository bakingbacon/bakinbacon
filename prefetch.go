package main

import (

	log "github.com/sirupsen/logrus"
	
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
			log.WithField("Cycle", metadataLevel.Cycle).Info("OnDemand-Fetching Endorsing Rights")
			go fetchEndorsingRights(metadataLevel.Cycle)
			break
		case highestFetchedCycle < nextCycle:
			log.WithField("Cycle", nextCycle).Info("OnDemand-Fetching Endorsing Rights")
			go fetchEndorsingRights(nextCycle)
			break
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
			log.WithField("Cycle", metadataLevel.Cycle).Info("OnDemand-Fetching Baking Rights")
			go fetchBakingRights(metadataLevel.Cycle)
		case highestFetchedCycle < nextCycle:
			log.WithField("Cycle", nextCycle).Info("OnDemand-Fetching Baking Rights")
			go fetchBakingRights(nextCycle)
		}
	}
}

// Called on each new block; Only processes every 512 blocks
// Fetches the bake/endorse rights for the next cycle and stores to DB 
func prefetchCycleRights(metadataLevel rpc.Level) {

	// We only prefetch every 512 levels
	if metadataLevel.Level % 512 != 0 {
		return
	}

	nextCycle := metadataLevel.Cycle + 1

	log.WithField("Cycle", nextCycle).Info("Pre-fetching rights for next cycle")

	go fetchEndorsingRights(nextCycle)
	go fetchBakingRights(nextCycle)
}

func fetchEndorsingRights(nextCycle int) {

	if bc.Signer.BakerPkh == "" {
		log.Error("Cannot fetch endorsing rights; No baker configured")
		return
	}

	endorsingRightsFilter := rpc.EndorsingRightsInput{
		BlockID:  &rpc.BlockIDHead{},
		Cycle:    nextCycle,
		Delegate: bc.Signer.BakerPkh,
	}

	resp, endorsingRights, err := bc.Current.EndorsingRights(endorsingRightsFilter)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"Request": resp.Request.URL, "Response": resp.Body(),
		}).Error("Unable to fetch next cycle's endorsing rights")
		return
	}

	if len(endorsingRights) == 0 {
		log.WithField("Cycle", nextCycle).Info("Pre-fetch returned no endorsing rights for next cycle")
	}
	
	// Save rights to DB, even if len == 0 so that it is noted we queried this cycle
	storage.DB.SaveEndorsingRightsForCycle(nextCycle, endorsingRights)
}

func fetchBakingRights(nextCycle int) {

	if bc.Signer.BakerPkh == "" {
		log.Error("Cannot fetch baking rights; No baker configured")
		return
	}

	bakingRightsFilter := rpc.BakingRightsInput{
		BlockID:     &rpc.BlockIDHead{},
		Cycle:       nextCycle,
		Delegate:    bc.Signer.BakerPkh,
	}

	resp, bakingRights, err := bc.Current.BakingRights(bakingRightsFilter)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"Request": resp.Request.URL, "Response": resp.Body(),
		}).Error("Unable to fetch next cycle's baking rights")
		return
	}

	// Got any rights?
	if len(bakingRights) == 0 {
		log.WithFields(log.Fields{
			"Cycle": nextCycle, "MaxPriority": MAX_BAKE_PRIORITY,
		}).Info("Pre-fetch returned no baking rights for cycle")
	}

	// Filter max priority
	var filteredRights []rpc.BakingRights
	for _, r := range bakingRights {
		if r.Priority < MAX_BAKE_PRIORITY {
			filteredRights = append(filteredRights, r)
		}
	}
	
	// Save filtered rights to DB, even if len == 0 so that it is noted we queried this cycle
	storage.DB.SaveBakingRightsForCycle(nextCycle, filteredRights)
}

func getCycleFromLevel(l int) int {
	return int(l / bc.Current.CurrentConstants().BlocksPerCycle)
}
