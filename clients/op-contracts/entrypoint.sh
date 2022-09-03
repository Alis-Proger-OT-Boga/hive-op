#!/bin/bash

echo "starting op-contracts"

# this web-browser will keep ensure the container is considered ready and alive by Hive,
# and make it easy to navigate the live container contents.
python3 -m http.server --bind 0.0.0.0 --directory / 8545
