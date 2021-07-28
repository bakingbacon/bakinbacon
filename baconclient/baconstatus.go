package baconclient

const (

	// Various states for the UI to take action
	CAN_BAKE       = "canbake"
	LOW_BALANCE    = "lowbal"
	NOT_REGISTERED = "noreg"
	NO_SIGNER      = "nosign"
)

type BaconStatus struct {
	Hash          string `json:"hash"`
	Level         int    `json:"level"`
	Cycle         int    `json:"cycle"`
	CyclePosition int    `json:"cycleposition"`

	NextEndorsementLevel int `json:"nel"`
	NextEndorsementCycle int `json:"nec"`

	NextBakingLevel    int `json:"nbl"`
	NextBakingCycle    int `json:"nbc"`
	NextBakingPriority int `json:"nbp"`

	PreviousEndorsementLevel int    `json:"pel"`
	PreviousEndorsementCycle int    `json:"pec"`
	PreviousEndorsementHash  string `json:"peh"`

	PreviousBakeLevel int    `json:"pbl"`
	PreviousBakeCycle int    `json:"pbc"`
	PreviousBakeHash  string `json:"pbh"`

	State    string `json:"state"`
	ErrorMsg string `json:"error"`
}

func (b *BaconStatus) SetNextEndorsement(level, cycle int) {
	b.NextEndorsementLevel = level
	b.NextEndorsementCycle = cycle
}

func (b *BaconStatus) SetNextBake(level, cycle, priority int) {
	b.NextBakingLevel = level
	b.NextBakingCycle = cycle
	b.NextBakingPriority = priority
}

func (b *BaconStatus) SetRecentEndorsement(level, cycle int, hash string) {
	b.PreviousEndorsementLevel = level
	b.PreviousEndorsementCycle = cycle
	b.PreviousEndorsementHash = hash
}

func (b *BaconStatus) SetRecentBake(level, cycle int, hash string) {
	b.PreviousBakeLevel = level
	b.PreviousBakeCycle = cycle
	b.PreviousBakeHash = hash
}

func (b *BaconStatus) SetError(e error) {
	b.ErrorMsg = e.Error()
}

func (b *BaconStatus) ClearError() {
	b.ErrorMsg = ""
}

func (b *BaconStatus) SetState(s string) {
	b.State = s
}
