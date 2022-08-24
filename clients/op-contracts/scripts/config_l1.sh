#!/bin/bash

set -eu

cd /opt/optimism/packages/contracts-bedrock

# generate L1 config, redirect standard output to stderr, we output the result as json to stdout later
op-node genesis devnet-l1 --network hivenet --deploy-config deploy-config --outfile genesis-l1.json 1>&2

cat genesis-l1.json
