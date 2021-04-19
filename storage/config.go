package storage

import (
	"bytes"
	
	"github.com/pkg/errors"
	
	bolt "go.etcd.io/bbolt"
)

const (
	PUBLIC_KEY_HASH = "pkh"
	ENDPOINTS_BUCKET = "endpoints"
	SIGNER_TYPE = "signertype"
	SIGNER_SK = "signersk"
)

func (s *Storage) GetDelegate() (string, string) {
	var sk, pkh string
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		sk = string(b.Get([]byte(SIGNER_SK)))
		pkh = string(b.Get([]byte(PUBLIC_KEY_HASH)))
		return nil
	})
	return sk, pkh
}

func (s *Storage) SetDelegate(sk, pkh string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		b.Put([]byte(SIGNER_SK), []byte(sk))
		b.Put([]byte(PUBLIC_KEY_HASH), []byte(pkh))
		return nil
	})
}

func (s *Storage) GetSignerType() int {
	var st int = 0
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		_st := b.Get([]byte(SIGNER_TYPE))
		if _st != nil {
			st = btoi(_st)
		}
		return nil
	})
	return st
}

func (s *Storage) SetSignerType(d int) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		b.Put([]byte(SIGNER_TYPE), itob(d))
		return nil
	})
}

func (s *Storage) GetSignerSk() string {
	var sk string
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		sk = string(b.Get([]byte(SIGNER_SK)))
		return nil
	})
	return sk
}

func (s *Storage) SetSignerSk(sk string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		b.Put([]byte(SIGNER_SK), []byte(sk))
		return nil
	})
}

func (s *Storage) AddRPCEndpoint(endpoint string) error {

	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.Bucket([]byte(CONFIG_BUCKET)).CreateBucketIfNotExists([]byte(ENDPOINTS_BUCKET))
		if err != nil {
			return errors.Wrap(err, "Unable to create endpoints bucket")
		}
		
		var foundDup bool
		endpointBytes := []byte(endpoint)

		b.ForEach(func(k, v []byte) error {
			if bytes.Compare(v, endpointBytes) == 0 {
				foundDup = true
			}
			return nil
		})
		
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
			return errors.New("Unable to locate endpoints bucket")
		}

		b.ForEach(func(k, v []byte) error {
			endpoints = append(endpoints, string(v))
			return nil
		})

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
		b.ForEach(func(k, v []byte) error {
			if bytes.Compare(v, endpointBytes) == 0 {
				b.Delete(k)
			}
			return nil
		})
		return nil
	})
}
