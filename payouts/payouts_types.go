package payouts

type CycleRewardMetadata struct {
	PayoutCycle        int `json:"c"`   // Rewards cycle
	LevelOfPayoutCycle int `json:"lpc"` // First level of rewards cycle
	SnapshotIndex      int `json:"si"`  // Index of snapshot used for reward cycle
	SnapshotLevel      int `json:"sl"`  // Level of the snapshot used for reward cycle
	UnfrozenLevel      int `json:"ul"`  // Last block of cycle where rewards are unfrozen

	BakerFee           int `json:"f"`   // Fee of baker at time of processing
	NumDelegators      int `json:"nd"`  // Number of delegators

	Balance            int `json:"b"`   // Balance of baker at time of snapshot
	StakingBalance     int `json:"sb"`  // Staking balance of baker (includes bakers own balance)
	DelegatedBalance   int `json:"db"`  // Delegated balance of baker
	BlockRewards       int `json:"br"`  // Rewards for all bakes/endorses
	FeeRewards         int `json:"fr"`  // Rewards for all transaction fees included in our blocks
}

type DelegatorReward struct {
	Delegator string  `json:"d"`
	Balance   int     `json:"b"`
	SharePct  float64 `json:"p"`
	Reward    int     `json:"r"`
	OpHash    string  `json:"o"`
}
