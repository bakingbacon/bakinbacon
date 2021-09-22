package storage

import (
	"encoding/binary"
	"time"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

const (
	DatabaseFile = "bakinbacon.db"

	BakingBucket        = "bakes"
	EndorsingBucket     = "endorses"
	NonceBucket         = "nonces"
	ConfigBucket        = "config"
	RightsBucket        = "rights"
	EndpointsBucket     = "endpoints"
	NotificationsBucket = "notifs"
)

type Storage struct {
	db *bolt.DB
}

func InitStorage(dataDir, network string) (*Storage, error) {
	db, err := bolt.Open(dataDir+DatabaseFile, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to init db")
	}

	// Ensure some buckets exist, and migrations
	err = db.Update(func(tx *bolt.Tx) error {
		// Config bucket
		cfgBkt, err := tx.CreateBucketIfNotExists([]byte(ConfigBucket))
		if err != nil {
			return errors.Wrap(err, "Cannot create config bucket")
		}

		// Nested bucket inside config
		if _, err := cfgBkt.CreateBucketIfNotExists([]byte(EndpointsBucket)); err != nil {
			return errors.Wrap(err, "Cannot create endpoints bucket")
		}

		// Nested bucket inside config
		if _, err := cfgBkt.CreateBucketIfNotExists([]byte(NotificationsBucket)); err != nil {
			return errors.Wrap(err, "Cannot create notifications bucket")
		}

		//
		// Root buckets
		if _, err := tx.CreateBucketIfNotExists([]byte(EndorsingBucket)); err != nil {
			return errors.Wrap(err, "Cannot create endorsing bucket")
		}

		if _, err := tx.CreateBucketIfNotExists([]byte(BakingBucket)); err != nil {
			return errors.Wrap(err, "Cannot create baking bucket")
		}

		if _, err := tx.CreateBucketIfNotExists([]byte(NonceBucket)); err != nil {
			return errors.Wrap(err, "Cannot create nonce bucket")
		}

		if _, err := tx.CreateBucketIfNotExists([]byte(RightsBucket)); err != nil {
			return errors.Wrap(err, "Cannot create rights bucket")
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// set variable so main program can access
	storage := &Storage{
		db: db,
	}

	// Add the default endpoints only on brand-new setup
	if err := storage.AddDefaultEndpoints(network); err != nil {
		log.WithError(err).Error("could not add default endpoints")
		return nil, err
	}
	return storage, err
}

func (s *Storage) Close() {
	s.db.Close()
	log.Info("Database closed")
}

// itob returns an 8-byte big endian representation of v.
func itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))

	return b
}

func btoi(b []byte) int {
	return int(binary.BigEndian.Uint64(b))
}
