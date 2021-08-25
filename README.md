<img src="https://bakinbacon.io/img/head_logo.png" width="400px">

---

## Running BakinBacon

_BakinBacon defaults to Granadanet, the current mainnet testing network. Use `-network mainnet` to switch._

1. Download the latest binary for your OS from [bakinbacon/releases](https://github.com/bakingbacon/bakinbacon/releases)
1. Execute: `./bakinbacon-linux-amd64 [-debug] [-trace] [-webuiaddr 127.0.0.1] [-webuiport 8082] [-network mainnet|granadanet]`
1. Open http://127.0.0.1:8082/ in your browser

The following binaries are available as part of our release process:

* bakinbacon-linux-amd64
* bakinbacon-darwin-amd64
* bakinbacon-windows-amd64.exe

If you would like bakinbacon compiled for a different platform, you can build it yourself below, or open an issue and we might be able to add it to our build prcocess.

## Building BakinBacon

If you want to contribute to BakinBacon with pull-requests, you'll need a proper environment set up to build and test BakinBacon.

### Dependencies

* go-1.16+
* nodejs-14.15 (npm 6.14)
* gcc-7.5+ (build-essential package on Ubuntu)
* libhidapi-libusb0, libusb, libusb-dev (For compiling ledger nano support)

### Build Steps

1. Clone the repo
1. `make ui-dev && make ui` (Build the webserver UI, downloading any required npm modules)
1. `make [darwin|windows]` (You can only build darwin on darwin; You can build linux and windows on linux)
1. Run as noted above
