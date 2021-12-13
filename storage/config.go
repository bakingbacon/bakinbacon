package storage

import (
	"bytes"
	"strconv"

	"github.com/pkg/errors"

	bolt "go.etcd.io/bbolt"

	"bakinbacon/util"
)

const (
	PUBLIC_KEY_HASH = "pkh"
	BIP_PATH        = "bippath"
	SIGNER_TYPE     = "signertype"
	SIGNER_SK       = "signersk"
	BAKER_FEE       = "bakerfee"
	UI_EXPLORER     = "uiexplorer"
)

func (s *Storage) GetBakerSettings() (map[string]interface{}, error) {

	settings := make(map[string]interface{})

	err := s.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))

		settings[BAKER_FEE]   = strconv.Itoa(Btoi(b.Get([]byte(BAKER_FEE))))
		settings[UI_EXPLORER] = string(b.Get([]byte(UI_EXPLORER)))

		return nil
	})

	return settings, err
}

func (s *Storage) SaveBakerSettings(settings map[string]string) error {

	bakerFee, err := strconv.Atoi(settings[BAKER_FEE])
	if err != nil {
		return err
	}

	return s.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))

		if err := b.Put([]byte(BAKER_FEE), Itob(bakerFee)); err != nil {
			return err
		}

		if err := b.Put([]byte(UI_EXPLORER), []byte(settings[UI_EXPLORER])); err != nil {
			return err
		}

		return nil
	})
}

func (s *Storage) GetDelegate() (string, string, error) {

	var sk, pkh string

	err := s.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		sk = string(b.Get([]byte(SIGNER_SK)))
		pkh = string(b.Get([]byte(PUBLIC_KEY_HASH)))

		return nil
	})

	return sk, pkh, err
}

func (s *Storage) SetDelegate(sk, pkh string) error {

	return s.Update(func(tx *bolt.Tx) error {
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

	var signerType int = 0

	err := s.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		signerTypeBytes := b.Get([]byte(SIGNER_TYPE))
		if signerTypeBytes != nil {
			signerType = Btoi(signerTypeBytes)
		}

		return nil
	})

	return signerType, err
}

func (s *Storage) SetSignerType(signerType int) error {

	return s.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		return b.Put([]byte(SIGNER_TYPE), Itob(signerType))
	})
}

func (s *Storage) GetSignerSk() (string, error) {

	var sk string

	err := s.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		sk = string(b.Get([]byte(SIGNER_SK)))
		return nil
	})

	return sk, err
}

func (s *Storage) SetSignerSk(sk string) error {

	return s.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		return b.Put([]byte(SIGNER_SK), []byte(sk))
	})
}

// Ledger
func (s *Storage) SaveLedgerToDB(pkh, bipPath string, ledgerType int) error {

	return s.Update(func(tx *bolt.Tx) error {

		b := tx.Bucket([]byte(CONFIG_BUCKET))

		// Save signer type as ledger
		if err := b.Put([]byte(SIGNER_TYPE), Itob(ledgerType)); err != nil {
			return err
		}

		// Save PKH
		if err := b.Put([]byte(PUBLIC_KEY_HASH), []byte(pkh)); err != nil {
			return err
		}

		// Save BipPath
		if err := b.Put([]byte(BIP_PATH), []byte(bipPath)); err != nil {
			return err
		}

		return nil
	})
}

func (s *Storage) GetLedgerConfig() (string, string, error) {

	var pkh, bipPath string

	err := s.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET))
		pkh = string(b.Get([]byte(PUBLIC_KEY_HASH)))
		bipPath = string(b.Get([]byte(BIP_PATH)))
		return nil
	})

	return pkh, bipPath, err
}

func (s *Storage) AddRPCEndpoint(endpoint string) (int, error) {

	var rpcId int = 0

	err := s.Update(func(tx *bolt.Tx) error {
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
		rpcId = int(id)

		return b.Put(Itob(int(id)), endpointBytes)
	})

	return rpcId, err
}

func (s *Storage) GetRPCEndpoints() (map[int]string, error) {

	endpoints := make(map[int]string)

	err := s.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET)).Bucket([]byte(ENDPOINTS_BUCKET))
		if b == nil {
			return errors.New("GetRPC - Unable to locate endpoints bucket")
		}

		if err := b.ForEach(func(k, v []byte) error {
			id := Btoi(k)
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

	return s.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET)).Bucket([]byte(ENDPOINTS_BUCKET))
		if b == nil {
			return errors.New("Unable to locate endpoints bucket")
		}

		return b.Delete(Itob(endpointId))
	})
}

func (s *Storage) AddDefaultEndpoints(network string) error {

	// Check the current sequence id for endpoints bucket. If > 2, then
	// this is not a first-time init and we should not add these again

	var currentSeq uint64

	err := s.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CONFIG_BUCKET)).Bucket([]byte(ENDPOINTS_BUCKET))
		if b == nil {
			return errors.New("AddDefaultRPCs - Unable to locate endpoints bucket")
		}
		currentSeq = b.Sequence()

		return nil
	})
	if err != nil {
		return err
	}

	if currentSeq == 0 {

		// Statically add BakinBacon's RPC endpoints
		switch network {
		case util.NETWORK_MAINNET:
			_, _ = s.AddRPCEndpoint("http://mainnet-us.rpc.bakinbacon.io")
			_, _ = s.AddRPCEndpoint("http://mainnet-eu.rpc.bakinbacon.io")

		case util.NETWORK_GRANADANET:
			_, _ = s.AddRPCEndpoint("http://granadanet-us.rpc.bakinbacon.io")
			_, _ = s.AddRPCEndpoint("http://granadanet-eu.rpc.bakinbacon.io")

		case util.NETWORK_HANGZHOUNET:
			_, _ = s.AddRPCEndpoint("http://hangzhounet-us.rpc.bakinbacon.io")

		default:
			return errors.New("Unknown network for storage")
		}
	}

	return nil
}
