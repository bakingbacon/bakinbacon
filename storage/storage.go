package storage

import (
	"encoding/binary"
	"time"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

const (
	DATABASE_FILE = "bakinbacon.db"

	BAKING_BUCKET        = "bakes"
	ENDORSING_BUCKET     = "endorses"
	NONCE_BUCKET         = "nonces"
	CONFIG_BUCKET        = "config"
	RIGHTS_BUCKET        = "rights"
	ENDPOINTS_BUCKET     = "endpoints"
	NOTIFICATIONS_BUCKET = "notifs"
	PAYOUTS_BUCKET       = "payouts"
)

type Storage struct {
	*bolt.DB
}

func InitStorage(dataDir, network string) (*Storage, error) {

	db, err := bolt.Open(dataDir+DATABASE_FILE, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to init db")
	}

	// Ensure some buckets exist, and migrations
	err = db.Update(func(tx *bolt.Tx) error {

		// Config bucket
		cfgBkt, err := tx.CreateBucketIfNotExists([]byte(CONFIG_BUCKET))
		if err != nil {
			return errors.Wrap(err, "Cannot create config bucket")
		}

		// Nested bucket inside config
		if _, err := cfgBkt.CreateBucketIfNotExists([]byte(ENDPOINTS_BUCKET)); err != nil {
			return errors.Wrap(err, "Cannot create endpoints bucket")
		}

		// Nested bucket inside config
		if _, err := cfgBkt.CreateBucketIfNotExists([]byte(NOTIFICATIONS_BUCKET)); err != nil {
			return errors.Wrap(err, "Cannot create notifications bucket")
		}

		//
		// Root buckets
		if _, err := tx.CreateBucketIfNotExists([]byte(ENDORSING_BUCKET)); err != nil {
			return errors.Wrap(err, "Cannot create endorsing bucket")
		}

		if _, err := tx.CreateBucketIfNotExists([]byte(BAKING_BUCKET)); err != nil {
			return errors.Wrap(err, "Cannot create baking bucket")
		}

		if _, err := tx.CreateBucketIfNotExists([]byte(NONCE_BUCKET)); err != nil {
			return errors.Wrap(err, "Cannot create nonce bucket")
		}

		if _, err := tx.CreateBucketIfNotExists([]byte(RIGHTS_BUCKET)); err != nil {
			return errors.Wrap(err, "Cannot create rights bucket")
		}

		if _, err := tx.CreateBucketIfNotExists([]byte(PAYOUTS_BUCKET)); err != nil {
			return errors.Wrap(err, "Cannot create payouts bucket")
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// set variable so main program can access
	storage := &Storage{
		DB: db,
	}

	// Add the default endpoints only on brand new setup
	if err := storage.AddDefaultEndpoints(network); err != nil {
		log.WithError(err).Error("Could not add default endpoints")
		return nil, errors.Wrap(err, "Could not add default endpoints")
	}

	return storage, err
}

func (s *Storage) CloseDb() {
	s.Close()
	log.Info("Database closed")
}

// Itob returns an 8-byte big endian representation of v.
func Itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func Btoi(b []byte) int {
	if b != nil {
		return int(binary.BigEndian.Uint64(b))
	}
	return 0
}
