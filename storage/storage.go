package storage

import (
	"encoding/binary"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

const (
	DATABASE_FILE = "bakinbacon.db"

	BAKING_BUCKET    = "bakes"
	ENDORSING_BUCKET = "endorses"
	NONCE_BUCKET     = "nonces"
	CONFIG_BUCKET    = "config"
	RIGHTS_BUCKET    = "rights"
	ENDPOINTS_BUCKET = "endpoints"
)

type Storage struct {
	db *bolt.DB
}

var DB Storage

func init() {

	db, err := bolt.Open(DATABASE_FILE, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatal("Failed to init db:", err)
	}

	// Ensure some buckets exist, and migrations
	err = db.Update(func(tx *bolt.Tx) error {

		// Config bucket
		cfgBkt, err := tx.CreateBucketIfNotExists([]byte(CONFIG_BUCKET))
		if err != nil {
			return fmt.Errorf("Cannot create config bucket: %s", err)
		}

		// Nested bucket inside config
		if _, err := cfgBkt.CreateBucketIfNotExists([]byte(ENDPOINTS_BUCKET)); err != nil {
			return fmt.Errorf("Cannot create endpoints bucket: %s", err)
		}

		// Root buckets
		if _, err := tx.CreateBucketIfNotExists([]byte(ENDORSING_BUCKET)); err != nil {
			return fmt.Errorf("Cannot create endorsing bucket: %s", err)
		}

		if _, err := tx.CreateBucketIfNotExists([]byte(BAKING_BUCKET)); err != nil {
			return fmt.Errorf("Cannot create baking bucket: %s", err)
		}

		if _, err := tx.CreateBucketIfNotExists([]byte(NONCE_BUCKET)); err != nil {
			return fmt.Errorf("Cannot create nonce bucket: %s", err)
		}

		if _, err := tx.CreateBucketIfNotExists([]byte(RIGHTS_BUCKET)); err != nil {
			return fmt.Errorf("Cannot create rights bucket: %s", err)
		}

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	// set variable so main program can access
	DB = Storage{
		db: db,
	}

	// Statically add BakinBacon's RPC endpoints
	_, _ = DB.AddRPCEndpoint("http://florencenet-us.rpc.bakinbacon.io")
	_, _ = DB.AddRPCEndpoint("http://florencenet-eu.rpc.bakinbacon.io")
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
