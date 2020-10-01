package storage

import (
	bolt "go.etcd.io/bbolt"
)

func (s *Storage) GetBakingWatermark() (int) {

	var watermark uint64

	s.db.View(func(tx *bolt.Tx) error {
		watermark = tx.Bucket([]byte(BAKING_BUCKET)).Sequence()
		return nil
	})

	return int(watermark)
}

func (s *Storage) RecordBakedBlock(level int, blockHash string) (error) {

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BAKING_BUCKET))
		b.SetSequence(uint64(level)) // Record our watermark
		return b.Put(itob(level), []byte(blockHash)) // Save the level:blockHash
	})
}
