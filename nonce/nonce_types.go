package nonce

var Prefix_nonce []byte = []byte{69, 220, 169}

type Nonce struct {
	Seed        string `json:"seed"`
	NonceHash   string `json:"noncehash"`
	SeedHashHex string `json:"seedhashhex"`

	Level       int    `json:"level"`
	RevealOp    string `json:"revealed"`
}

