package main

import (
	"encoding/hex"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"
)

func cryptoGenericHash(buffer string) ([]byte, error) {

	// Convert hex buffer to bytes
	bufferBytes, err := hex.DecodeString(buffer)
	if err != nil {
		return []byte{0}, errors.Wrap(err, "Unable to hex decode buffer bytes")
	}

	// Generic hash of 32 bytes
	bufferBytesHashGen, err := blake2b.New(32, []byte{})
	if err != nil {
		return []byte{0}, errors.Wrap(err, "Unable create blake2b hash object")
	}

	// Write buffer bytes to hash
	_, err = bufferBytesHashGen.Write(bufferBytes)
	if err != nil {
		return []byte{0}, errors.Wrap(err, "Unable write buffer bytes to hash function")
	}

	// Generate checksum of buffer bytes
	bufferHash := bufferBytesHashGen.Sum([]byte{})

	return bufferHash, nil
}

func stripQuote(s string) string {
	m := strings.TrimSpace(s)
	if len(m) > 0 && m[0] == '"' {
		m = m[1:]
	}
	if len(m) > 0 && m[len(m)-1] == '"' {
		m = m[:len(m)-1]
	}
	return m
}
