package storage

import (
	"encoding/binary"
	"fmt"

	bolt "github.com/etcd-io/bbolt"
    log "github.com/sirupsen/logrus"
)

const (
	DATABASE_FILE = "goendorse.db"

	BAKES = "bakes"
)

type Storage struct {
        db *bolt.DB
}

var DB Storage

func init() {

	db, err := bolt.Open(DATABASE_FILE, 0600, nil)
	if err != nil {
		log.Fatal("Failed to init db:", err)
	}
	
	// Ensure some buckets exist, and migrations
	err = db.Update(func(tx *bolt.Tx) error {

		if _, err := tx.CreateBucketIfNotExists([]byte(BAKES)); err != nil {
			return fmt.Errorf("Cannot create baking bucket: %s", err)
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
}

func (s *Storage) Close() {
	s.db.Close()
	log.Info("Database closed")
}

func (s *Storage) GetBakingWatermark() (int) {

	var watermark uint64

	s.db.View(func(tx *bolt.Tx) error {
		watermark = tx.Bucket([]byte(BAKES)).Sequence()
		return nil
	})

	return int(watermark)
}

func (s *Storage) RecordBakedBlock(level int, blockHash string) (error) {

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BAKES))
		b.SetSequence(uint64(level)) // Record our watermark
		return b.Put(itob(level), []byte(blockHash)) // Save the level:blockHash
	})
}

// itob returns an 8-byte big endian representation of v.
func itob(v int) []byte {
    b := make([]byte, 8)
    binary.BigEndian.PutUint64(b, uint64(v))
    return b
}
