package main

import (
	_ "bytes"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"bakinbacon/nonce"
	"bakinbacon/util"

	"github.com/bakingbacon/go-tezos/v4/crypto"
	"github.com/bakingbacon/go-tezos/v4/keys"

	log "github.com/sirupsen/logrus"
)

func TestMain(m *testing.M) {

	// Connect to node for tests
	// gt, err = gotezos.New("127.0.0.1:18732")
	// if err != nil {
	// 	panic(fmt.Sprintf("Unable to connect to network: %s\n", err))
	// }

	log.SetLevel(log.DebugLevel)

	network = "hangzhounet"

	os.Exit(m.Run())
}

func TestLoadingKey(t *testing.T) {

	// tz1MTZEJE7YH3wzo8YYiAGd8sgiCTxNRHczR
	// pk := "edpkvEbxZAv15SAZAacMAwZxjXToBka4E49b3J1VNrM1qqy5iQfLUx"
	sk := "edsk3yXukqCQXjCnS4KRKEiotS7wRZPoKuimSJmWnfH2m3a2krJVdf"

	wallet, err := keys.FromBase58(sk, keys.Ed25519)
	if err != nil {
		log.WithError(err).Fatal("Failed to load wallet")
		os.Exit(1)
	}

	t.Logf("Baker PKH: %s\n", wallet.PubKey.GetAddress())
}

func TestProofOfWork(t *testing.T) {

	forgedBytes := "00050e7f027173c6c8eda1628b74beba1a4825379d90a818e6c0ea0dba4b8c4dc9f52012c10000000061195ce604e627eb811ac7ec2098304273fea05915c8b02cd9c079e02398204732312bab90000000110000000101000000080000000000050e7e4775bb79657508f01a4efd3e9dd8570a1a6a6b39c45a487cdb56a5c049c18694000142423130000000000000"

	powBytes, _, err := powLoop(forgedBytes, len("000142423130000000000000"))
	if err != nil {
		t.Errorf("PowLoop Failed: %s", err)
	}

	if powBytes[len(forgedBytes)-12:] != "0001e4ee0000" {
		t.Errorf("Incorrect POW")
	}
}

func TestGenericHash(t *testing.T) {

	seed := "e6d84e1e98a65b2f4551be3cf320f2cb2da38ab7925edb2452e90dd5d2eeeead"
	seedBytes, _ := hex.DecodeString(seed)

	nonceHash, err := util.CryptoGenericHash(seedBytes, []byte{})
	if err != nil {
		t.Errorf("Unable to hash rand bytes for nonce")
	}

	// B58 encode seed hash with nonce prefix
	encodedNonce := crypto.B58cencode(nonceHash, nonce.Prefix_nonce)

	t.Logf("Seed: %s\n", seed)
	t.Logf("Nonce: %s\n", encodedNonce)
	t.Logf("Non-Encoded: %s\n", hex.EncodeToString(nonceHash))

	if encodedNonce != "nceVSbP3hcecWHY1dYoNUMfyB7gH9S7KbC4hEz3XZK5QCrc5DfFGm" {
		t.Errorf("Incorrect nonce from seed")
	}
}

func TestNonce(t *testing.T) {

	randBytes := make([]byte, 32)
	if _, err := rand.Read(randBytes); err != nil {
		t.Errorf("Unable to read random bytes: %s", err)
	}
	seed := hex.EncodeToString(randBytes)
	seedBytes, _ := hex.DecodeString(seed)

	randBytesHash, err := util.CryptoGenericHash(randBytes, []byte{})
	if err != nil {
		log.Errorf("Unable to hash rand bytes for nonce: %s", err)
	}

	seedBytesHash, err := util.CryptoGenericHash(seedBytes, []byte{})
	if err != nil {
		log.Errorf("Unable to hash rand bytes for nonce: %s", err)
	}

	// B58 encode seed hash with nonce prefix
	encodedRandBytes := crypto.B58cencode(randBytesHash, nonce.Prefix_nonce)
	encodedSeedBytes := crypto.B58cencode(seedBytesHash, nonce.Prefix_nonce)

	t.Logf("Seed: %s\n", seed)
	t.Logf("ERB:  %s\n", encodedRandBytes)
	t.Logf("ESB:  %s\n", encodedSeedBytes)

	if encodedRandBytes != encodedSeedBytes {
		t.Errorf("Encoded bytes do not match")
	}
}
