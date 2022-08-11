# Hive Optimism RPC test suite

This test suite is a copy of the ETH L1 RPC test suite adapted for Optimism L2.
It tests several real-world scenarios such as sending value transactions,
deploying a contract or interacting with one.

    hive --sim optimism/rpc --client=go-ethereum,op-geth,op-proposer,op-batcher,op-node,op-contracts --docker.output
