package util

import (
	_ "encoding/hex"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"
)

func CryptoGenericHash(bufferBytes []byte, watermark []byte) ([]byte, error) {

	if len(watermark) > 0 {
		bufferBytes = append(watermark, bufferBytes...)
	}

	// Generic hash of 32 bytes
	bufferBytesHashGen, err := blake2b.New(32, []byte{})
	if err != nil {
		return nil, errors.Wrap(err, "Unable create blake2b hash object")
	}

	// Write buffer bytes to hash
	if _, err = bufferBytesHashGen.Write(bufferBytes); err != nil {
		return nil, errors.Wrap(err, "Unable write buffer bytes to hash function")
	}

	// Generate checksum of buffer bytes
	bufferHash := bufferBytesHashGen.Sum([]byte{})

	return bufferHash, nil
}

func StripQuote(s string) string {

	m := strings.TrimSpace(s)
	if len(m) > 0 && m[0] == '"' {
		m = m[1:]
	}

	if len(m) > 0 && m[len(m)-1] == '"' {
		m = m[:len(m)-1]
	}

	return m
}

func AvailableNetworks() string {
	return strings.Join([]string{NETWORK_MAINNET, NETWORK_GRANADANET}, ",")
}