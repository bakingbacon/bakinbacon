package main

import "testing"

func TestGenerateNonce(t *testing.T) {

	nonceHash, seedHashHex, err := generateNonce("e6d84e1e98a65b2f4551be3cf320f2cb2da38ab7925edb2452e90dd5d2eeeead")
	if err != nil {
		t.Errorf("Error: %w", err)
	}
	if nonceHash != "nceVSbP3hcecWHY1dYoNUMfyB7gH9S7KbC4hEz3XZK5QCrc5DfFGm" {
		t.Errorf("Incorrect hash")
	}
	if seedHashHex != "a067ece149449d72c2c2a2d7ff2c32769db0ec3e6872dbc18cc4853fb3e58bcc" {
		t.Errorf("Incorrect seed hash")
	}
}
