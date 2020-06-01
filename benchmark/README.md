# Benchmark

## Node.js

The code in `addon.js` is from the main README from the official Stremio addon SDK.

On a fresh Ubuntu 20.04 machine run it with:

```bash
apt install -y nodejs npm
npm install stremio-addon-sdk
node ./addon.js
```

## Go

The code in `addon.go` is does exactly the same as the Node.js addon, but uses the unofficial Go Stremio addon SDK.

On a fresh Ubuntu 20.04 machine run it with:

```bash
apt install -y golang
go mod init addon
go get ./...
go build -o addon
./addon
```

## Load test

There are two different kinds of load testing tools:

1. Tools like [wrk2](https://github.com/giltene/wrk2) and [Vegeta](https://github.com/tsenart/vegeta) produce a load based on a constant rate of requests per second that you specifiy
2. Tools like [ab](https://httpd.apache.org/docs/2.4/programs/ab.html) and [Artillery](https://artillery.io/) determine how long it takes to handle a number of requests that you specify

### wrk2

Installation:

```bash
apt install -y build-essential libssl-dev git zlib1g-dev
git clone https://github.com/giltene/wrk2.git
cd wrk2
make
cp wrk /usr/local/bin
```

Usage:

```bash
# -t2 for the number of threads, -c for the number of concurrent connections, -d for the duration, -R for the request rate
wrk -t2 -c100 -d30s -R2000 --latency http://123.123.123.123:7000/stream/movie/tt1254207.json
```

> Note: The load test should be run on a different host.

### Artillery

Installation:

```bash
apt install -y nodejs npm
npm install -g artillery
```

Usage:

```bash
# --count for the number of workers, -n for the number of requests per worker
artillery quick --count 10 -n 20 http://123.123.123.123:7000/stream/movie/tt1254207.json
```

> Note: The load test should be run on a different host.
