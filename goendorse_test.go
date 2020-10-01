package main

import (
	"encoding/hex"

	gotezos "github.com/goat-systems/go-tezos/v2"
	log "github.com/sirupsen/logrus"
	"testing"
)

const SEED string = "3c51b0c9f14eb473bd9affdd01b5429679a6e73c553cae76561ce08046510b09"

func init() {

	var err error

	// Connect to node for tests
// 	gt, err = gotezos.New("127.0.0.1:18732")
// 	if err != nil {
// 		panic(fmt.Sprintf("Unable to connect to network: %s\n", err))
// 	}

	log.SetLevel(log.DebugLevel)

	// tz1MTZEJE7YH3wzo8YYiAGd8sgiCTxNRHczR
	pk := "edpkvEbxZAv15SAZAacMAwZxjXToBka4E49b3J1VNrM1qqy5iQfLUx"
	sk := "edsk3yXukqCQXjCnS4KRKEiotS7wRZPoKuimSJmWnfH2m3a2krJVdf"

	wallet, err = gotezos.ImportWallet(BAKER, pk, sk)
	if err != nil {
		log.Fatal(err.Error())
	}
}


func TestPow(t *testing.T) {

	forgedBytes := "000b28bc020aa7c9617eec24986aeabc2ee633a415e3353b83e10ca53d78896960890e9b0b000000005f63ab820466f3ae293bf159d678bea8eefffde4d12a2a6128d74d555f2356a71a485cf0620000001100000001010000000800000000000b28bb3ce664f245ce743c12a4f945a29db2e6363c50019ff3e0539e7c26849e245b83"

	powBytes, attempts, err := powLoop(forgedBytes, 1, SEED)
	if err != nil {
		t.Errorf("PowLoop Failed: %s", err)
	}

	if powBytes != forgedBytes+"000100bc03030003dd18ff3c51b0c9f14eb473bd9affdd01b5429679a6e73c553cae76561ce08046510b09" {
		t.Errorf("Incorrect POW")
	}

	t.Logf("POW Attempts: %d\n", attempts)
}

func TestGenericHash(t *testing.T) {

	t.Logf("Seed: %s\n", SEED)

	seedHash, err := cryptoGenericHash(SEED)
	if err != nil {
		t.Errorf("Unable to hash seed for nonce")
	}
	t.Logf("Seed Hash: %v\n", seedHash)

	// B58 encode seed hash with nonce prefix
	nonceHash := gotezos.B58cencode(seedHash, Prefix_nonce)
	seedHashHex := hex.EncodeToString(seedHash)

	t.Logf("Nonce Hash: %s\n", nonceHash)
	if nonceHash != "nceVuHM4VHi6c1JsgbEwHXDdLFJKoTxuM4jz1eWCGxv6pLRhv1Kdp" {
		t.Errorf("Incorrect hash")
	}

	t.Logf("Seed Hex: %s\n", seedHashHex)
	if seedHashHex != "dd01ba02e9826494b92cf433c7266560f669aa2a7f5d9a65a6bdaf2172bdffdc" {
		t.Errorf("Incorrect seed hash")
	}
}
