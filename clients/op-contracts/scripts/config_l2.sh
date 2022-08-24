#!/bin/bash

set -eu

cd /opt/optimism/packages/contracts-bedrock

# we use the L1 RPC to fetch L1 block info in hardhat
export L1_RPC="$1"
echo "configuring L2 genesis using L1 RPC: $L1_RPC" 1>&2

# required for hardhat network definition to work with RPC
export PRIVATE_KEY_DEPLOYER="$2"
export CHAIN_ID="$3"

# redirect standard output to stderr, we output the result as json to stdout later
op-node genesis devnet-l2 --network hivenet --artifacts ./artifacts --deployments ./deployments --rpc-url "$L1_RPC" --outfile genesis-l2.json 1>&2

cat genesis-l2.json
