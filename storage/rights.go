package storage

import (
	"bytes"

	"github.com/bakingbacon/go-tezos/v4/rpc"

	"github.com/pkg/errors"

	bolt "go.etcd.io/bbolt"
)

const (
	ENDORSING_RIGHTS_BUCKET = "endorsing"
	BAKING_RIGHTS_BUCKET    = "baking"
)

func (s *Storage) SaveEndorsingRightsForCycle(cycle int, endorsingRights []rpc.EndorsingRights) error {

	return s.Update(func(tx *bolt.Tx) error {

		b, err := tx.Bucket([]byte(RIGHTS_BUCKET)).CreateBucketIfNotExists([]byte(ENDORSING_RIGHTS_BUCKET))
		if err != nil {
			return errors.Wrap(err, "Unable to create endorsing rights bucket")
		}

		// Use the bucket's sequence to save the highest cycle for which rights have been fetched
		if err := b.SetSequence(uint64(cycle)); err != nil {
			return err
		}

		// Keys of values are not related to the sequence
		for _, r := range endorsingRights {
			if err := b.Put(Itob(r.Level), Itob(cycle)); err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *Storage) SaveBakingRightsForCycle(cycle int, bakingRights []rpc.BakingRights) error {

	return s.Update(func(tx *bolt.Tx) error {

		b, err := tx.Bucket([]byte(RIGHTS_BUCKET)).CreateBucketIfNotExists([]byte(BAKING_RIGHTS_BUCKET))
		if err != nil {
			return errors.Wrap(err, "Unable to create baking rights bucket")
		}

		// Use the bucket's sequence to save the highest cycle for which rights have been fetched
		if err := b.SetSequence(uint64(cycle)); err != nil {
			return err
		}

		// Keys of values are not related to the sequence
		for _, r := range bakingRights {
			if err := b.Put(Itob(r.Level), Itob(r.Priority)); err != nil {
				return err
			}
		}

		return nil
	})
}

// GetNextEndorsingRight returns the level of the next endorsing opportunity,
// and also the highest cycle for which rights have been previously fetched.
func (s *Storage) GetNextEndorsingRight(curLevel int) (int, int, error) {

	var (
		nextLevel         int
		highestFetchCycle int
	)

	curLevelBytes := Itob(curLevel)

	err := s.View(func(tx *bolt.Tx) error {

		b := tx.Bucket([]byte(RIGHTS_BUCKET)).Bucket([]byte(ENDORSING_RIGHTS_BUCKET))
		if b == nil {
			return errors.New("Endorsing Rights Bucket Not Found")
		}

		highestFetchCycle = int(b.Sequence())

		c := b.Cursor()

		for k, _ := c.First(); k != nil && nextLevel == 0; k, _ = c.Next() {
			switch o := bytes.Compare(curLevelBytes, k); o {
			case 1, 0:
				// k is less than, or equal to current level, loop to next entry
				continue
			case -1:
				nextLevel = Btoi(k)
			}
		}

		return nil
	})

	// Two conditions can happen. We scanned through all rights:
	//  1. .. and found next highest
	//  2. .. or found none

	return nextLevel, highestFetchCycle, err
}

// GetNextBakingRight returns the level of the next baking opportunity, with it's priority,
// and also the highest cycle for which rights have been previously fetched.
func (s *Storage) GetNextBakingRight(curLevel int) (int, int, int, error) {

	var (
		nextLevel         int
		nextPriority      int
		highestFetchCycle int
	)

	curLevelBytes := Itob(curLevel)

	err := s.View(func(tx *bolt.Tx) error {

		b := tx.Bucket([]byte(RIGHTS_BUCKET)).Bucket([]byte(BAKING_RIGHTS_BUCKET))
		if b == nil {
			return errors.New("Endorsing Rights Bucket Not Found")
		}

		highestFetchCycle = int(b.Sequence())

		c := b.Cursor()

		for k, v := c.First(); k != nil && nextLevel == 0; k, v = c.Next() {
			switch o := bytes.Compare(curLevelBytes, k); o {
			case 1, 0:
				// k is less than, or equal to current level, loop to next entry
				continue
			case -1:
				nextLevel = Btoi(k)
				nextPriority = Btoi(v)
			}
		}

		return nil
	})

	// Two conditions can happen. We scanned through all rights:
	//  1. .. and found next highest
	//  2. .. or found none

	return nextLevel, nextPriority, highestFetchCycle, err
}

// GetRecentEndorsement returns the level of the most recent endorsement
func (s *Storage) GetRecentEndorsement() (int, string, error) {

	var (
		recentEndorsementLevel int    = 0
		recentEndorsementHash  string = ""
	)

	err := s.View(func(tx *bolt.Tx) error {

		b := tx.Bucket([]byte(ENDORSING_BUCKET))
		if b == nil {
			return errors.New("Endorsing history bucket not found")
		}

		// The last/highest key is the most recent endorsement
		k, v := b.Cursor().Last()
		if k != nil {
			recentEndorsementLevel = Btoi(k)
			recentEndorsementHash = string(v)
		}

		return nil
	})

	return recentEndorsementLevel, recentEndorsementHash, err
}

// GetRecentBake returns the level of the most recent bake
func (s *Storage) GetRecentBake() (int, string, error) {

	var (
		recentBakeLevel int    = 0
		recentBakeHash  string = ""
	)

	err := s.View(func(tx *bolt.Tx) error {

		b := tx.Bucket([]byte(BAKING_BUCKET))
		if b == nil {
			return errors.New("Baking history bucket not found")
		}

		// The last/highest key is the most recent endorsement
		k, v := b.Cursor().Last()
		if k != nil {
			recentBakeLevel = Btoi(k)
			recentBakeHash = string(v)
		}

		return nil
	})

	return recentBakeLevel, recentBakeHash, err
}
