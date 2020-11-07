# Benchmark

## Node.js

The code in `addon.js` is from the main README from the official Stremio addon SDK.

On a fresh Ubuntu 20.04 machine run it with:

```bash
# Install Node.js 12, which is the latest LTS release as of writing this.
curl -sL https://deb.nodesource.com/setup_12.x | bash -
apt install -y nodejs

npm install stremio-addon-sdk
node ./addon.js
```

## Go

The code in `addon.go` is does exactly the same as the Node.js addon, but uses the unofficial Go Stremio addon SDK.

On a fresh Ubuntu 20.04 machine run it with:

```bash
# Install Go 1.15, which is the latest version as of writing this
curl -sL -o go.tar.gz https://golang.org/dl/go1.15.3.linux-amd64.tar.gz
tar -C /usr/local -xzf go.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
. ~/.bashrc

apt install -y git

go build -v
./addon
```

## Load test

There are two different kinds of load testing tools:

1. Tools like [wrk2](https://github.com/giltene/wrk2) and [Vegeta](https://github.com/tsenart/vegeta) produce a load based on a constant rate of requests per second that you specifiy
2. Tools like [ab](https://httpd.apache.org/docs/2.4/programs/ab.html) and [Artillery](https://artillery.io/) determine how long it takes to handle a number of requests that you specify

We're more interested in the maximum constant load the service can handle, so we're using `wrk2`.

### Installation

Automated: `./bench.sh -install`

Manual:

```bash
apt install -y build-essential libssl-dev git zlib1g-dev
git clone https://github.com/giltene/wrk2.git
cd wrk2
make
cp wrk /usr/local/bin
```

### Usage

Automated: `./bench.sh http://123.123.123.123:7000/stream/movie/tt1254207.json`

Manual:

```bash
# -t2 for the number of threads, -c for the number of concurrent connections, -d for the duration, -R for the request rate
wrk -t2 -c100 -d30s -R2000 --latency http://123.123.123.123:7000/stream/movie/tt1254207.json
```

> Note: The load test should be run on a different host.

## Automated setup

You can use this script on a new Ubuntu 20.04 machine to update the machine, clone this repository (into the current working directory), install all dependencies and finally reboot the machine:

```bash
#!/bin/bash -i

# We're using `-i` so we can source ~/.bashrc in this script

set -euxo pipefail

apt update
apt upgrade -y
apt install -y git
git clone https://github.com/deflix-tv/go-stremio
cd go-stremio/benchmark

# Set up Node.js
curl -sL https://deb.nodesource.com/setup_12.x | bash -
apt install -y nodejs
npm install stremio-addon-sdk

# Set up Go
curl -sL -o go.tar.gz https://golang.org/dl/go1.15.3.linux-amd64.tar.gz
tar -C /usr/local -xzf go.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
set +ux
. ~/.bashrc
set -ux
go build -v

# Set up wrk2
./bench.sh -install

# Set up ttfok
cd /tmp # We don't want ttfok added to the go.mod
go get github.com/doingodswork/ttfok

set +x
echo ""
echo "Setup successful."
echo ""
echo "After the reboot you can cd into the benchmark directory and run:"
echo "node ./addon.js"
echo "./addon"
echo "./bench.sh http://123.123.123.123:7000/stream/movie/tt1254207.json"
echo "ttfok ./addon http://localhost:7000/stream/movie/tt1254207.json"
echo ""
set -x

reboot
```
