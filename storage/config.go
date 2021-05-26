package storage

import (
	"bytes"

	"github.com/pkg/errors"

	bolt "go.etcd.io/bbolt"
)

const (
	PUBLIC_KEY_HASH  = "pkh"
	SIGNER_TYPE      = "signertype"
	SIGNER_SK        = "signersk"
)

func (s *Storage) GetDelegate() (string, string, error) {
	var sk, pkh string

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		sk = string(b.Get([]byte(SIGNER_SK)))
		pkh = string(b.Get([]byte(PUBLIC_KEY_HASH)))
		return nil
	})

	return sk, pkh, err
}

func (s *Storage) SetDelegate(sk, pkh string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		if err := b.Put([]byte(SIGNER_SK), []byte(sk)); err != nil {
			return err
		}
		if err := b.Put([]byte(PUBLIC_KEY_HASH), []byte(pkh)); err != nil {
			return err
		}
		return nil
	})
}

func (s *Storage) GetSignerType() (int, error) {
	var st int = 0

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		_st := b.Get([]byte(SIGNER_TYPE))
		if _st != nil {
			st = btoi(_st)
		}
		return nil
	})

	return st, err
}

func (s *Storage) SetSignerType(d int) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		return b.Put([]byte(SIGNER_TYPE), itob(d))
	})
}

func (s *Storage) GetSignerSk() (string, error) {
	var sk string

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		sk = string(b.Get([]byte(SIGNER_SK)))
		return nil
	})

	return sk, err
}

func (s *Storage) SetSignerSk(sk string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		return b.Put([]byte(SIGNER_SK), []byte(sk))
	})
}

func (s *Storage) AddRPCEndpoint(endpoint string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET)).Bucket([]byte(ENDPOINTS_BUCKET))
		if b == nil {
			return errors.New("AddRPC - Unable to locate endpoints bucket")
		}

		var foundDup bool
		endpointBytes := []byte(endpoint)

		if err := b.ForEach(func(k, v []byte) error {
			if bytes.Equal(v, endpointBytes) {
				foundDup = true
			}
			return nil
		}); err != nil {
			return err
		}

		if foundDup {
			// Found duplicate, exit
			return nil
		}

		// else, add
		id, _ := b.NextSequence()

		return b.Put(itob(int(id)), endpointBytes)
	})
}

func (s *Storage) GetRPCEndpoints() ([]string, error) {
	var endpoints []string

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET)).Bucket([]byte(ENDPOINTS_BUCKET))
		if b == nil {
			return errors.New("GetRPC - Unable to locate endpoints bucket")
		}

		if err := b.ForEach(func(k, v []byte) error {
			endpoints = append(endpoints, string(v))
			return nil
		}); err != nil {
			return err
		}

		return nil
	})

	return endpoints, err
}

func (s *Storage) DeleteRPCEndpoint(endpoint string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET)).Bucket([]byte(ENDPOINTS_BUCKET))
		if b == nil {
			return errors.New("Unable to locate endpoints bucket")
		}

		endpointBytes := []byte(endpoint)
		if err := b.ForEach(func(k, v []byte) error {
			if bytes.Equal(v, endpointBytes) {
				return b.Delete(k)
			}
			return nil
		}); err != nil {
			return err
		}

		return nil
	})
}
