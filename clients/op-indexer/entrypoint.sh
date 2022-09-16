#!/bin/bash

# Hive requires us to prefix env vars with "HIVE_"
# Iterate the env, find all HIVE_UNPACK_ vars, and remove the HIVE_UNPACK_ prefix.
while IFS='=' read -r -d '' n v; do
    if [[ "$n" == HIVE_UNPACK_* ]]; then
        name=${n#"HIVE_UNPACK_"}  # remove the HIVE_UNPACK_ prefix
        echo "$name=$v"
        declare -gx "$name=$v"
    fi
done < <(env -0)

indexer --build-env development --eth-network-name devnet --chain-id 901 --db-host 172.17.0.9 --db-port 5432 --db-user postgres --db-password postgres --db-name=indexer --start-block-number 0 --log-terminal --log-level debug
