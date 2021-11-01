package payouts

import (
	"encoding/json"

	"github.com/pkg/errors"

	bolt "go.etcd.io/bbolt"

	"bakinbacon/storage"
)

type DelegatorReward struct {
	Delegator string  `json:"d"`
	Balance   int     `json:"b"`
	SharePct  float64 `json:"p"`
	Reward    int     `json:"r"`
	OpHash    string  `json:"o"`
}

func (p *PayoutsHandler) GetDelegatorRewardForCycle(address string, cycle int) (*DelegatorReward, error) {

	var delegatorReward *DelegatorReward

	err := p.storage.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DB_PAYOUTS_BUCKET)).Bucket(storage.Itob(cycle))
		if b == nil {
			return errors.New("Unable to locate cycle payouts bucket")
		}

		delegatorRewardBytes := b.Get([]byte(address))
		if err := json.Unmarshal(delegatorRewardBytes, delegatorReward); err != nil {
			return errors.Wrap(err, "Unable to decode delegator reward info")
		}

		return nil
	})

	return delegatorReward, err
}

func (p *PayoutsHandler) SaveDelegatorReward(rewardCycle int, rewardRecord *DelegatorReward) error {

	return p.storage.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DB_PAYOUTS_BUCKET)).Bucket(storage.Itob(rewardCycle))
		if b == nil {
			return errors.New("Unable to locate cycle payouts bucket")
		}

		rewardRecordBytes, err := json.Marshal(rewardRecord)
		if err != nil {
			return errors.Wrap(err, "Unable to encode delegator reward")
		}

		// Store the record as the value of the record address (key)
		// This will allow for easier scanning/searching for a payment record
		return b.Put([]byte(rewardRecord.Delegator), rewardRecordBytes)
	})
}

// Returns all rewards data for each delegator for a specific cycle. Mainly used by the UI when
// refreshing paid/unpaid status during a payouts action
func (p *PayoutsHandler) GetDelegatorRewardAllForCycle(cycle int) (map[string]DelegatorReward, error) {

	// key is delegator address
	delegatorRewards := make(map[string]DelegatorReward)

	err := p.storage.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DB_PAYOUTS_BUCKET)).Bucket(storage.Itob(cycle))
		if b == nil {
			return errors.New("Unable to locate cycle payouts bucket")
		}

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if string(k) == DB_METADATA {
				continue
			}

			// Add the value (JSON object) to the map
			var tmpRewards DelegatorReward
			if err := json.Unmarshal(v, &tmpRewards); err != nil {
				return errors.Wrap(err, "Unable to parse delegator rewards")
			}
			delegatorRewards[string(k)] = tmpRewards
		}

		return nil
	})

	return delegatorRewards, err
}

// updateDelegatorRewardOpHash will fetch a DelegatorReward struct from DB via cycle and address. It then
// unmarshals the bytes to a new struct, updates, re-marshals and saves back to DB, effectively updating the record.
func (p *PayoutsHandler) updateDelegatorRewardOpHash(address string, cycle int, opHash string) error {

	// Fetch from DB
	delegatorReward, err := p.GetDelegatorRewardForCycle(address, cycle)
	if err != nil {
		return err
	}

	delegatorReward.OpHash = opHash

	return p.SaveDelegatorReward(cycle, delegatorReward)
}
