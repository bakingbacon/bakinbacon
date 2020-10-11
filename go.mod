module goendorse

go 1.15

require (
	github.com/Messer4/base58check v0.0.0-20180328134002-7531a92ae9ba
	github.com/btcsuite/btcutil v1.0.2
	github.com/goat-systems/go-tezos/v3 v3.0.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.0
	go.etcd.io/bbolt v1.3.5
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a
)

replace github.com/goat-systems/go-tezos/v3 => /home/drmac/go/src/github.com/utdrmac/go-tezos
