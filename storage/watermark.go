package storage

import (
	bolt "go.etcd.io/bbolt"
)

func (s *Storage) GetBakingWatermark() (int, error) {
	return s.getWatermark(BAKING_BUCKET)
}

func (s *Storage) GetEndorsingWatermark() (int, error) {
	return s.getWatermark(ENDORSING_BUCKET)
}

func (s *Storage) getWatermark(wBucket string) (int, error) {

	var watermark uint64

	err := s.db.View(func(tx *bolt.Tx) error {
		watermark = tx.Bucket([]byte(wBucket)).Sequence()
		return nil
	})

	return int(watermark), err
}

func (s *Storage) RecordBakedBlock(level int, blockHash string) error {
	return s.recordOperation(BAKING_BUCKET, level, blockHash)
}

func (s *Storage) RecordEndorsement(level int, endorsementHash string) error {
	return s.recordOperation(ENDORSING_BUCKET, level, endorsementHash)
}

func (s *Storage) recordOperation(opBucket string, level int, opHash string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(opBucket))
		if err := b.SetSequence(uint64(level)); err != nil { // Record our watermark
			return err
		}

		return b.Put(itob(level), []byte(opHash)) // Save the level:opHash
	})
}
