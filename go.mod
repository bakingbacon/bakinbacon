module bakinbacon

go 1.15

require (
	github.com/Messer4/base58check v0.0.0-20180328134002-7531a92ae9ba
	github.com/bakingbacon/go-tezos/v4 v4.1.0
	github.com/bakingbacon/goledger v1.0.0 // indirect
	github.com/bakingbacon/goledger/ledger-apps/tezos v0.0.0-20210318214534-90bad189f425
	github.com/btcsuite/btcutil v1.0.2
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.1
	go.etcd.io/bbolt v1.3.5
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
)

replace github.com/sirupsen/logrus => /home/drmac/go/src/github.com/sirupsenNEW/logrus

replace github.com/bakingbacon/go-tezos/v4 => ../github.com/bakingbacon/go-tezos
