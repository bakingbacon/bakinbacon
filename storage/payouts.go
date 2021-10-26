package storage

import (
	"encoding/json"

	"github.com/pkg/errors"

	bolt "go.etcd.io/bbolt"
)

const (
	METADATA = "metadata"
)

// GetPayoutsMetadata returns a byte-slice of raw JSON from DB
func (s *Storage) GetPayoutsMetadata() (map[int]json.RawMessage, error) {

	payoutsMetadata := make(map[int]json.RawMessage)

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(PAYOUTS_BUCKET))
		if b == nil {
			return errors.New("Unable to locate cycle payouts bucket")
		}

		c := b.Cursor()

		for k, _ := c.First(); k != nil; k, _ = c.Next() {

			// keys are cycle numbers, which are buckets of data
			cycleBucket := b.Bucket(k)
			cycle := btoi(k)

			// Get metadata key from bucket
			metadataBytes := cycleBucket.Get([]byte(METADATA))

			// Unmarshal ...
			var tmpMetadata json.RawMessage
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

func (s *Storage) GetCyclePayouts(cycle int) (map[string]json.RawMessage, error) {

	// key is delegator address
	cyclePayouts := make(map[string]json.RawMessage)

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(PAYOUTS_BUCKET)).Bucket(itob(cycle))
		if b == nil {
			return errors.New("Unable to locate cycle payouts bucket")
		}

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if string(k) == METADATA {
				continue
			}

			// Add the value (JSON object) to the map
			cyclePayouts[string(k)] = v
		}

		return nil
	})

	return cyclePayouts, err
}

func (s *Storage) SaveCycleRewardMetadata(rewardCycle int, metadataBytes []byte) error {

	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.Bucket([]byte(PAYOUTS_BUCKET)).CreateBucketIfNotExists(itob(rewardCycle))
		if err != nil {
			return errors.New("Unable to create cycle payouts bucket")
		}

		return b.Put([]byte(METADATA), metadataBytes)
	})
}

func (s *Storage) SaveDelegatorReward(rewardCycle int, delegator string, rewardRecordBytes []byte) error {

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(PAYOUTS_BUCKET)).Bucket(itob(rewardCycle))
		if b == nil {
			return errors.New("Unable to locate cycle payouts bucket")
		}

		// Store the record as the value of the record address (key)
		// This will allow for easier scanning/searching for a payment record
		return b.Put([]byte(delegator), rewardRecordBytes)
	})
}
