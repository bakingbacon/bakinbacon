package storage

import (
	"encoding/json"

	"github.com/pkg/errors"

	bolt "go.etcd.io/bbolt"
)

func (s *Storage) SaveNonce(cycle, nonceLevel int, nonceBytes []byte) error {

	// Nonces are stored within a cycle bucket for easy retrieval
	return s.db.Update(func(tx *bolt.Tx) error {
		cb, err := tx.Bucket([]byte(NONCE_BUCKET)).CreateBucketIfNotExists(itob(cycle))
		if err != nil {
			return errors.Wrap(err, "Unable to create nonce-cycle bucket")
		}
		return cb.Put(itob(nonceLevel), nonceBytes)
	})
}

func (s *Storage) GetNoncesForCycle(cycle int) ([]json.RawMessage, error) {

	// Get back all nonces for cycle
	nonces := make([]json.RawMessage, 0)

	err := s.db.Update(func(tx *bolt.Tx) error {
		cb, err := tx.Bucket([]byte(NONCE_BUCKET)).CreateBucketIfNotExists(itob(cycle))
		if err != nil {
			return errors.Wrap(err, "Unable to create nonce-cycle bucket")
		}
		c := cb.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {

			// Unmarshal to raw JSON
			var tmpRaw json.RawMessage
			if err := json.Unmarshal(v, &tmpRaw); err != nil {
				return errors.Wrap(err, "Unable to get nonces from DB")
			}

			nonces = append(nonces, tmpRaw)
		}

		return nil
	})

	return nonces, err
}
