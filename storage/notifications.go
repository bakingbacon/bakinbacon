package storage

import (
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
)

func (s *Storage) GetNotifiersConfig(notifier string) ([]byte, error) {

	var config []byte

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(ConfigBucket)).Bucket([]byte(NotificationsBucket))
		if b == nil {
			return errors.New("Unable to locate notifications bucket")
		}

		config = b.Get([]byte(notifier))

		return nil
	})

	return config, err
}

func (s *Storage) SaveNotifiersConfig(notifier string, config []byte) error {

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(ConfigBucket)).Bucket([]byte(NotificationsBucket))
		if b == nil {
			return errors.New("Unable to locate notifications bucket")
		}

		return b.Put([]byte(notifier), config)
	})
}
