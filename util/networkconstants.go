package util

import (
	"fmt"
)

const (
	NETWORK_MAINNET     = "mainnet"
	NETWORK_GRANADANET  = "granadanet"
	NETWORK_HANGZHOUNET = "hangzhounet"
)

type NetworkConstants struct {
	TimeBetweenBlocks          int
	BlocksPerCycle             int
	BlocksPerRollSnapshot      int
	BlocksPerCommitment        int
	BlockGasLimit              int
	BlockSecurityDeposit       int
	EndorsementSecurityDeposit int
	ProofOfWorkThreshold       uint64
	PreservedCycles            int
	InitialEndorsers           int
	GranadaActivationLevel     int
	GranadaActivationCycle     int
	// Granada changed the simple calculations, so we need to
	// know the last level before the change. For mainnet,
	// this happened just before C388 (388 * 4096 - 1)
}

// For updating, mainnet example
// curl -Ss https://mainnet-tezos.giganode.io/chains/main/blocks/head/context/constants | jq -r '[ (.minimal_block_delay|tonumber), .blocks_per_cycle, .blocks_per_roll_snapshot, .blocks_per_commitment, (.hard_gas_limit_per_block|tonumber), (.block_security_deposit|tonumber), (.endorsement_security_deposit|tonumber), (.proof_of_work_threshold|tonumber), .preserved_cycles, .initial_endorsers] | @csv'

func GetNetworkConstants(network string) (*NetworkConstants, error) {

	switch network {
	case NETWORK_MAINNET:
		return &NetworkConstants{
			30, 8192, 512, 64, 5200000, 64000000, 2500000, 70368744177663, 5, 192, 1589247, 388,
		}, nil
	case NETWORK_GRANADANET:
		return &NetworkConstants{
			15, 4096, 256, 32, 5200000, 640000000, 2500000, 70368744177663, 3, 192, 4095, 2,
		}, nil
	case NETWORK_HANGZHOUNET:
		return &NetworkConstants{
			15, 4096, 256, 32, 5200000, 640000000, 2500000, 70368744177663, 3, 192, 0, 0,
		}, nil
	}

	// Unknown network
	return nil, fmt.Errorf("No such network '%s' exists", network)
}

func IsValidNetwork(maybeNetwork string) bool {
	return maybeNetwork == NETWORK_MAINNET || maybeNetwork == NETWORK_GRANADANET || maybeNetwork == NETWORK_HANGZHOUNET
}
