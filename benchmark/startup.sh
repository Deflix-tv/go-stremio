#!/bin/bash

# Only run this when there's no existing background job reported by `jobs`!

set -euxo pipefail

START_DATE=$(date +%s.%N)

if [[ "$1" == "node" ]]; then
    node ./addon.js &
elif [[ "$1" == "go" ]]; then
    ./addon &
else
    echo "You must pass 'node' or 'go' as argument"
    exit 1
fi

# TODO: Node not ready yet! We could loop and do this until response is 200, but sleep only allows granularity down to "second"
curl http://localhost:7000/stream/movie/tt1254207.json

END_DATE=$(date +%s.%N)

kill %1

date -d "0 $END_DATE sec - $START_DATE sec" +"%S.%Ns"
