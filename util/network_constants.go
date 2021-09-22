package util

const (
	Mainnet    = "mainnet"
	Granadanet = "granadanet"
)

type Constants struct {
	TimeBetweenBlocks          int
	BlocksPerCycle             int
	BlocksPerRollSnapshot      int
	BlocksPerCommitment        int
	BlockSecurityDeposit       int
	EndorsementSecurityDeposit int
	ProofOfWorkThreshold       uint64
	InitialEndorsers           int
	GranadaActivationLevel     int
	GranadaActivationCycle     int
	// Granada changed the simple calculations, so we need to
	// know the last level before the change. For mainnet,
	// this happened just before C388 (388 * 4096 - 1)
}

var NetworkConstants map[string]Constants

func init() {

	NetworkConstants = make(map[string]Constants)

	// Mainnet
	// curl -Ss https://mainnet-tezos.giganode.io/chains/main/blocks/head/context/constants | jq -r '[ (.minimal_block_delay|tonumber), .blocks_per_cycle, .blocks_per_roll_snapshot, .blocks_per_commitment, (.block_security_deposit|tonumber), (.endorsement_security_deposit|tonumber), (.proof_of_work_threshold|tonumber), .initial_endorsers] | @csv'
	NetworkConstants[Mainnet] = Constants{
		30, 8192, 512, 64, 64000000, 2500000, 70368744177663, 192, 1589247, 388,
	}

	NetworkConstants[Granadanet] = Constants{
		15, 4096, 256, 32, 640000000, 2500000, 70368744177663, 192, 4095, 2,
	}
}

func IsValidNetwork(maybeNetwork string) bool {
	return maybeNetwork == Mainnet || maybeNetwork == Granadanet
}