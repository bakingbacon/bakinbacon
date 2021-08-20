package nonce

var Prefix_nonce []byte = []byte{69, 220, 169}

type Nonce struct {
	Seed          string `json:"seed"`
	Nonce         []byte `json:"noncehash"`
	EncodedNonce  string `json:"encodednonce"`
	NoPrefixNonce string `json:"noprefixnonce"`

	Level    int    `json:"level"`
	RevealOp string `json:"revealed"`
}
