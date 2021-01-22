module goendorse

go 1.15

require (
	github.com/Messer4/base58check v0.0.0-20180328134002-7531a92ae9ba
	github.com/btcsuite/btcutil v1.0.2
	github.com/goat-systems/go-tezos/v4 v4.0.0-00010101000000-000000000000
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/securecookie v1.1.1
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.0
	go.etcd.io/bbolt v1.3.5
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a
)

replace github.com/goat-systems/go-tezos/v4 => /home/drmac/go/src/github.com/utdrmac/go-tezos

replace github.com/sirupsen/logrus => /home/drmac/go/src/github.com/sirupsenNEW/logrus
