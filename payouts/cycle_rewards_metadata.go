package payouts

import (
	"encoding/json"

	"github.com/pkg/errors"

	bolt "go.etcd.io/bbolt"

	"bakinbacon/storage"
)

const (
	CALCULATED  = "calc"
	DONE        = "done"
	IN_PROGRESS = "inprog"
	ERROR       = "err"
)

// TODO?
// t := reflect.TypeOf(CycleRewardMetadata{})
// for _, f := range reflect.VisibleFields(t) {
// 	fmt.Println(f.Tag.Get("db"))
// }

type CycleRewardMetadata struct {
	PayoutCycle        int `json:"c"`   // Rewards cycle
	LevelOfPayoutCycle int `json:"lpc"` // First level of rewards cycle
	SnapshotIndex      int `json:"si"`  // Index of snapshot used for reward cycle
	SnapshotLevel      int `json:"sl"`  // Level of the snapshot used for reward cycle
	UnfrozenLevel      int `json:"ul"`  // Last block of cycle where rewards are unfrozen

	BakerFee           float64 `json:"f"`   // Fee of baker at time of processing
	NumDelegators      int     `json:"nd"`  // Number of delegators

	Balance            int `json:"b"`   // Balance of baker at time of snapshot
	StakingBalance     int `json:"sb"`  // Staking balance of baker (includes bakers own balance)
	DelegatedBalance   int `json:"db"`  // Delegated balance of baker
	BlockRewards       int `json:"br"`  // Rewards for all bakes/endorses
	FeeRewards         int `json:"fr"`  // Rewards for all transaction fees included in our blocks

	Status             string `json:"st"`  // One of: calculated, done, or in-progress
}

// GetPayoutsMetadataAll returns a map of CycleRewardsMetadata
func (p *PayoutsHandler) GetPayoutsMetadataAll() (map[int]CycleRewardMetadata, error) {

	payoutsMetadata := make(map[int]CycleRewardMetadata)

	err := p.storage.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DB_PAYOUTS_BUCKET))
		if b == nil {
			return errors.New("Unable to locate cycle payouts bucket")
		}

		c := b.Cursor()

		for k, _ := c.First(); k != nil; k, _ = c.Next() {

			// keys are cycle numbers, which are buckets of data
			cycleBucket := b.Bucket(k)
			cycle := storage.Btoi(k)

			// Get metadata key from bucket
			metadataBytes := cycleBucket.Get([]byte(DB_METADATA))

			// Unmarshal ...
			var tmpMetadata CycleRewardMetadata
			if err := json.Unmarshal(metadataBytes, &tmpMetadata); err != nil {
				return errors.Wrap(err, "Unable to fetch metadata")
			}

			// ... and add to map
			payoutsMetadata[cycle] = tmpMetadata
		}

		return nil
	})

	return payoutsMetadata, err
}

func (p *PayoutsHandler) setCyclePayoutStatus(cycle int, status string) error {

	// Fetch, update, save
	metadata, err := p.GetRewardMetadataForCycle(cycle)
	if err != nil {
		return err
	}
	metadata.Status = status

	if err := p.SaveRewardMetadataForCycle(cycle, metadata); err != nil {
		return err
	}

	return nil
}

// GetRewardMetadataForCycle returns metadata struct for a single cycle
func (p *PayoutsHandler) GetRewardMetadataForCycle(rewardCycle int) (CycleRewardMetadata, error) {

	var cycleMetadata CycleRewardMetadata

	err := p.storage.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DB_PAYOUTS_BUCKET)).Bucket(storage.Itob(rewardCycle))
		if b == nil {
			// No bucket for cycle; Return empty metadata for creation
			return nil
		}

		// Get metadata key from bucket
		cycleMetadataBytes := b.Get([]byte(DB_METADATA))

		// No data, can't unmarshal; Return empty metadata for creation
		if len(cycleMetadataBytes) == 0 {
			return nil
		}

		// Unmarshal ...
		if err := json.Unmarshal(cycleMetadataBytes, &cycleMetadata); err != nil {
			return errors.Wrap(err, "Unable to unmarshal cycle metadata")
		}

		return nil
	})

	return cycleMetadata, err
}

func (p *PayoutsHandler) SaveRewardMetadataForCycle(rewardCycle int, metadata CycleRewardMetadata) error {

	// Marshal to bytes
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return errors.Wrap(err, "Unable to save reward metadata for cycle")
	}

	return p.storage.Update(func(tx *bolt.Tx) error {
		b, err := tx.Bucket([]byte(DB_PAYOUTS_BUCKET)).CreateBucketIfNotExists(storage.Itob(rewardCycle))
		if err != nil {
			return errors.New("Unable to create cycle payouts bucket")
		}

		return b.Put([]byte(DB_METADATA), metadataBytes)
	})
}
