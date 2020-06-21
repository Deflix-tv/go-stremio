#!/bin/bash

# Runs wrk2 with several request rates until a p99 latency of >=100 is reached.
# Optionally installs wrk2 when the argument "-install" is passed.
#
# Example invokation: "./bench.sh http://123.123.123.123:7000/stream/movie/tt1254207.json"

set -euo pipefail

# Parse arguments ("-install" and the target URL)
if [[ $# -eq 0 ]]; then
    echo "Requires either the target URL or '-install' as argument"
    exit 1
elif [[ $1 == "-install" ]]; then
        apt install -y build-essential libssl-dev git zlib1g-dev
        rm -rf /tmp/wrk2
        cd /tmp
        git clone https://github.com/giltene/wrk2.git
        cd wrk2
        make
        cp wrk /usr/local/bin
        echo "wrk2 was installed successfully"
        exit 0
else
    TARGET_URL=$1
fi

# Determine number of CPU cores
NUM_CPUS=$(grep -c ^processor /proc/cpuinfo)

# Wake up the service :)
echo "Waking up service :)"
wrk -t${NUM_CPUS} -c1000 -d5s -R1000 --latency ${TARGET_URL} > /dev/null
sleep 5s

# Run wrk2 until it reaches a p99 latency of 100.
# First come near the maximum with running for just 10s
REQUEST_RATE=1000
while true; do
    echo "Testing for 10s with ${REQUEST_RATE} requests/s"
    P99=$(wrk -t${NUM_CPUS} -c1000 -d10s -R${REQUEST_RATE} --latency ${TARGET_URL} | grep "99.000%" | tr -d " " | cut -d "%" -f2)
    # P99 is no ms anymore, but s?
    if [[ ${P99} != *"ms"* ]]; then
        break
    fi
    P99=$(echo ${P99} | tr -d "ms" | cut -d "." -f1)
    if [[ ${P99} -ge 100 ]]; then
        break
    fi
    REQUEST_RATE=$(( ${REQUEST_RATE} + 1000 ))
    sleep 5s
done
# Then test more detailed with running for 60s (which depending on the deviation can lead to a couple of thousands of extra requests/s)
while true; do
    sleep 10s
    echo "Testing for 60s with ${REQUEST_RATE} requests/s"
    P99=$(wrk -t${NUM_CPUS} -c1000 -d60s -R${REQUEST_RATE} --latency ${TARGET_URL} | grep "99.000%" | tr -d " " | cut -d "%" -f2)
    # P99 is no ms anymore, but s?
    if [[ ${P99} != *"ms"* ]]; then
        break
    fi
    P99=$(echo ${P99} | tr -d "ms" | cut -d "." -f1)
    if [[ ${P99} -ge 100 ]]; then
        break
    fi
    REQUEST_RATE=$(( ${REQUEST_RATE} + 1000 ))
done
echo ""
echo "Request rate for p99 < 100ms: ${REQUEST_RATE}"
echo ""
