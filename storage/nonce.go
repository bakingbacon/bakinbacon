package storage

import (
	"encoding/json"

	"github.com/pkg/errors"

	bolt "go.etcd.io/bbolt"
	log "github.com/sirupsen/logrus"

	"goendorse/nonce"
)

func (s *Storage) SaveNonce(cycle int, n nonce.Nonce) error {

	// Nonces are stored within a cycle bucket for easy retrieval

	nonceBytes, err := json.Marshal(n)
	if err != nil {
		return errors.Wrap(err, "Unable to marshal nonce")
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		cb, err := tx.Bucket([]byte(NONCE_BUCKET)).CreateBucketIfNotExists(itob(cycle))
		if err != nil {
			return errors.Wrap(err, "Unable to create nonce-cycle bucket")
		}
		return cb.Put(itob(n.Level), nonceBytes)
	})
}

func (s *Storage) GetNoncesForCycle(cycle int) ([]nonce.Nonce, error) {

	// Get back all nonces for cycle
	var nonces []nonce.Nonce

	err := s.db.View(func(tx *bolt.Tx) error {
		cb := tx.Bucket([]byte(NONCE_BUCKET)).Bucket(itob(cycle))
		if cb == nil {
			log.WithField("Cycle", cycle).Debug("Nonce cycle bucket not found")
			return errors.New("No cycle bucket found")
		}
		c := cb.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {

			var n nonce.Nonce
			if err := json.Unmarshal(v, &n); err != nil {
				log.WithError(err).Error("Unable to unmarshal nonce")
				continue
			}

			nonces = append(nonces, n)
		}

		return nil
	})

	return nonces, err
}
