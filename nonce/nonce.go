package nonce

import (
	"crypto/rand"
	"encoding/hex"

	gotezos "github.com/utdrmac/go-tezos/v2"
	log "github.com/sirupsen/logrus"

	"goendorse/util"
)

var Prefix_nonce []byte = []byte{69, 220, 169}

type Nonce struct {
	Seed        string `json:"seed"`
	NonceHash   string `json:"noncehash"`
	SeedHashHex string `json:"seedhashhex"`

	Level       int    `json:"level"`
	RevealOp    string `json:"revealed"`
}

func GenerateNonce() (Nonce, error) {

	//  Testing:
	// 	  Seed:       e6d84e1e98a65b2f4551be3cf320f2cb2da38ab7925edb2452e90dd5d2eeeead
	// 	  Seed Buf:   230,216,78,30,152,166,91,47,69,81,190,60,243,32,242,203,45,163,138,183,146,94,219,36,82,233,13,213,210,238,238,173
	// 	  Seed Hash:  160,103,236,225,73,68,157,114,194,194,162,215,255,44,50,118,157,176,236,62,104,114,219,193,140,196,133,63,179,229,139,204
	// 	  Nonce Hash: nceVSbP3hcecWHY1dYoNUMfyB7gH9S7KbC4hEz3XZK5QCrc5DfFGm
	// 	  Seed Hex:   a067ece149449d72c2c2a2d7ff2c32769db0ec3e6872dbc18cc4853fb3e58bcc

	// Generate a hexadecimal seed from random bytes
	randBytes := make([]byte, 64)
	if _, err := rand.Read(randBytes); err != nil {
		log.WithError(err).Error("Unable to read random bytes")
		return Nonce{}, err
	}
	seed := hex.EncodeToString(randBytes)[:64]

	seedHash, err := util.CryptoGenericHash(seed, []byte{})
	if err != nil {
		log.WithError(err).Error("Unable to hash seed for nonce")
		return Nonce{}, err
	}

	// B58 encode seed hash with nonce prefix
	nonceHash := gotezos.B58cencode(seedHash, Prefix_nonce)
	seedHashHex := hex.EncodeToString(seedHash)

	n := Nonce{
		Seed:        seed,
		NonceHash:   nonceHash,
		SeedHashHex: seedHashHex,
	}

	return n, nil
}
