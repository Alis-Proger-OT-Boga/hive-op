#!/bin/sh
set -exu

# Generate the rollup config.

pusd /app/packages/contracts-bedrock
L2OO_STARTING_BLOCK_TIMESTAMP=$GENESIS_TIMESTAMP npx hardhat rollup-config --outfile /hive/rollup.json
popd

exec op-node \
    $HIVE_L1_ETH_RPC_FLAG \
    $HIVE_L2_ENGINE_RPC_FLAG \
    --l2.jwt-secret=/config/test-jwt-secret.txt \
    $HIVE_SEQUENCER_ENABLED_FLAG \
    $HIVE_SEQUENCER_KEY_FLAG \
    --sequencer.l1-confs=0 \
    --verifier.l1-confs=0 \
    --rollup.config=/hive/rollup.json \
    --rpc.addr=0.0.0.0 \
    --rpc.port=7545 \
    --p2p.listen.ip=0.0.0.0 \
    --p2p.listen.tcp=9003 \
    --p2p.listen.udp=9003 \
    $HIVE_P2P_STATIC_FLAG \
    --snapshotlog.file=/snapshot.log \
    --p2p.priv.path=/config/p2p-node-key.txt
