#!/bin/bash

set -exu

erigon=/usr/local/bin/erigon

VERBOSITY=${HIVE_ETH1_LOGLEVEL:-3}

ERIGON_DATA_DIR=/db

CHAIN_ID=$(cat /genesis.json | jq -r .config.chainId)

if [ ! -d "$ERIGON_DATA_DIR" ]; then
	echo "$ERIGON_DATA_DIR missing, running init"
	echo "Initializing genesis."
	$erigon --verbosity="$VERBOSITY" init \
		--datadir="$ERIGON_DATA_DIR" \
		"/genesis.json"
else
	echo "$ERIGON_DATA_DIR exists."
fi

$erigon \
  --datadir /erigon-hive-datadir \
  --verbosity=$HIVE_LOGLEVEL \
  --http \
  --http.addr=0.0.0.0 \
  --http.port=8545 \
  --http.api=web3,debug,eth,txpool,net,engine,admin \
	--ws \
	--authrpc.jwtsecret="/hive/input/jwt-secret.txt" \
	--authrpc.port=8551 \
	--authrpc.addr=0.0.0.0 \
  --nat=none --experimental.overlay \
  --nodiscover \
	--networkid="$CHAIN_ID" \
	--mine \
	--miner.etherbase="$HIVE_ETHERBASE" \
	"$@"

