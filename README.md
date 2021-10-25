<img src="https://bakinbacon.io/img/head_logo.png" width="400px">

---

## Running BakinBacon

_BakinBacon defaults to Granadanet, the current mainnet testing network. Use `-network mainnet` to switch._

1. Download the latest binary for your OS from [bakinbacon/releases](https://github.com/bakingbacon/bakinbacon/releases)
1. Open a terminal, shell, cmd, powershell, etc and execute the binary for your operating system: 

    Example: `./bakinbacon-linux-amd64 [-debug] [-trace] [-webuiaddr 127.0.0.1] [-webuiport 8082] [-network mainnet|granadanet]`

3. Open http://127.0.0.1:8082/ in your browser

The following binaries are available as part of our release process:

* bakinbacon-linux-amd64
* bakinbacon-darwin-amd64
* bakinbacon-windows-amd64.exe

If you would like bakinbacon compiled for a different platform, you can build it yourself below, or open an issue and we might be able to add it to our build prcocess.

### Testing Tokens

The Tezos network requires 8000 XTZ at stake in order to be considered a baker. Please use the [hangzhou faucet](https://faucet.hangzhounet.teztnets.xyz/) to acquire testing tokens. These tokens are only valid on the Hangzhou testing network and will not work on mainnet.

## Building BakinBacon

If you want to contribute to BakinBacon with pull-requests, you'll need a proper environment set up to build and test BakinBacon.

### Dependencies

* go-1.16+
* nodejs-14.15 (npm 6.14)
* gcc-7.5+, make (build-essential package on Ubuntu)
* libhidapi-libusb0, libusb, libusb-dev (For compiling ledger nano support)

### Ledger Usage

If you want to use a Ledger device with BakinBacon, you will need to [download and install](https://www.ledger.com/ledger-live/download) Ledger Live, and install **BOTH** Tezos Wallet and Tezos Baker apps to your device. We **DO NOT** recommend any version higher than 2.2.9 as they are buggy and prone to device freeze.
* If using a ledger on linux, you'll need to add the [udev rules](https://support.ledger.com/hc/en-us/articles/115005165269-Fix-USB-connection-issues-with-Ledger-Live).

### Build Steps

1. Clone the repo
1. `make ui-dev && make ui` (Build the webserver UI, downloading any required npm modules)
1. `make [darwin|windows]` (You can only build darwin on darwin; You can build linux and windows on linux)
1. Run as noted above
