#!/bin/bash

set -eu

mkdir -p /config

# Deploy config is provided as first script argument
echo "$1" > /config/hivenet.json

echo "Creating genesis configs."

op-node genesis devnet \
  --artifacts /artifacts/contracts-bedrock,/artifacts/contracts-governance \
  --deploy-config "$1" \
  --outfile.l1 "/config/genesis-l1.json" \
  --outfile.l2 "/config/genesis-l2.json" \
  --outfile.rollup "/config/rollup.json"