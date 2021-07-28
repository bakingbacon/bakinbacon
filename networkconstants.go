package main

const (
	NETWORK_MAINNET    = "mainnet"
	NETWORK_GRANADANET = "granadanet"
)

type Constants struct {
	TimeBetweenBlocks          int
	PreservedCycles            int
	BlocksPerCycle             int
	BlocksPerRollSnapshot      int
	BlocksPerCommitment        int
	BlockSecurityDeposit       int
	EndorsementSecurityDeposit int
	ProofOfWorkThreshold       uint64
}

var networkConstants map[string]Constants

func init() {

	networkConstants = make(map[string]Constants)

	// Mainnet
	// curl -Ss https://mainnet-tezos.giganode.io/chains/main/blocks/head/context/constants | \
	// jq -r '[ (.time_between_blocks[0]|tonumber), .preserved_cycles, .blocks_per_cycle, .blocks_per_roll_snapshot, .blocks_per_commitment] | @csv'
	networkConstants[NETWORK_MAINNET] = Constants{
		60, 5, 4096, 256, 32, 512000000, 64000000, 70368744177663,
	}

	networkConstants[NETWORK_GRANADANET] = Constants{
		30, 3, 4096, 256, 32, 640000000, 2500000, 70368744177663,
	}
}
