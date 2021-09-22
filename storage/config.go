package storage

import (
	"bytes"

	"github.com/pkg/errors"

	bolt "go.etcd.io/bbolt"
)

const (
	PublicKeyHash = "pkh"
	BipPath       = "bippath"
	SignerType    = "signertype"
	SignerSK      = "signersk"
)

func (s *Storage) GetDelegate() (string, string, error) {
	var sk, pkh string

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(ConfigBucket))
		sk = string(b.Get([]byte(SignerSK)))
		pkh = string(b.Get([]byte(PublicKeyHash)))
		return nil
	})

	return sk, pkh, err
}

func (s *Storage) SetDelegate(sk, pkh string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(ConfigBucket))
		if err := b.Put([]byte(SignerSK), []byte(sk)); err != nil {
			return err
		}
		if err := b.Put([]byte(PublicKeyHash), []byte(pkh)); err != nil {
			return err
		}
		return nil
	})
}

func (s *Storage) GetSignerType() (int, error) {
	var signerType int = 0

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ConfigBucket))
		signerTypeBytes := bucket.Get([]byte(SignerType))
		if signerTypeBytes != nil {
			signerType = btoi(signerTypeBytes)
		}
		return nil
	})

	return signerType, err
}

func (s *Storage) SetSignerType(d int) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ConfigBucket))
		return bucket.Put([]byte(SignerType), itob(d))
	})
}

func (s *Storage) GetSignerSk() (signedSK string, err error) {
	err = s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ConfigBucket))
		signedSK = string(bucket.Get([]byte(SignerSK)))
		return err
	})
	return
}

func (s *Storage) SetSignerSk(sk string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(ConfigBucket))
		return b.Put([]byte(SignerSK), []byte(sk))
	})
}

// Ledger
func (s *Storage) SaveLedgerToDB(pkh, bipPath string, ledgerType int) error {
	return s.db.Update(func(tx *bolt.Tx) error {

		b := tx.Bucket([]byte(ConfigBucket))

		// Save signer type as ledger
		if err := b.Put([]byte(SignerType), itob(ledgerType)); err != nil {
			return err
		}

		// Save PKH
		if err := b.Put([]byte(PublicKeyHash), []byte(pkh)); err != nil {
			return err
		}

		// Save BIPPath
		if err := b.Put([]byte(BipPath), []byte(bipPath)); err != nil {
			return err
		}

		return nil
	})
}

func (s *Storage) GetLedgerConfig() (string, string, error) {
	var pkh, bipPath string

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(ConfigBucket))
		pkh = string(b.Get([]byte(PublicKeyHash)))
		bipPath = string(b.Get([]byte(BipPath)))
		return nil
	})

	return pkh, bipPath, err
}

// AddRPCEndpoint
func (s *Storage) AddRPCEndpoint(endpoint string) (int, error) {
	var rpcId int = 0

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(ConfigBucket)).Bucket([]byte(EndpointsBucket))
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
		rpcId = int(id)

		return b.Put(itob(int(id)), endpointBytes)
	})

	return rpcId, err
}

func (s *Storage) GetRPCEndpoints() (map[int]string, error) {
	endpoints := make(map[int]string)

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(ConfigBucket)).Bucket([]byte(EndpointsBucket))
		if b == nil {
			return errors.New("GetRPC - Unable to locate endpoints bucket")
		}

		if err := b.ForEach(func(k, v []byte) error {
			id := btoi(k)
			endpoints[id] = string(v)
			return nil
		}); err != nil {
			return err
		}

		return nil
	})

	return endpoints, err
}

func (s *Storage) DeleteRPCEndpoint(endpointId int) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(ConfigBucket)).Bucket([]byte(EndpointsBucket))
		if b == nil {
			return errors.New("Unable to locate endpoints bucket")
		}

		return b.Delete(itob(endpointId))
	})
}

func (s *Storage) AddDefaultEndpoints(network string) error {

	// Check the current sequence id for endpoints bucket. If > 2, then
	// this is not a first-time init and we should not add these again

	var currentSeq uint64

	if err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(ConfigBucket)).Bucket([]byte(EndpointsBucket))
		if b == nil {
			return errors.New("AddDefaultRPCs - Unable to locate endpoints bucket")
		}
		currentSeq = b.Sequence()
		return nil
	}); err != nil {
		return err
	}

	if currentSeq == 0 {

		// Statically add BakinBacon's RPC endpoints
		switch network {
		case "mainnet":
			_, _ = s.AddRPCEndpoint("http://mainnet-us.rpc.bakinbacon.io")
			_, _ = s.AddRPCEndpoint("http://mainnet-eu.rpc.bakinbacon.io")

		case "granadanet":
			_, _ = s.AddRPCEndpoint("http://granadanet-us.rpc.bakinbacon.io")
			_, _ = s.AddRPCEndpoint("http://granadanet-eu.rpc.bakinbacon.io")

		default:
			return errors.New("Unknown network for storage")
		}
	}

	return nil
}
