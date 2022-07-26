# Pull Geth into a second stage deploy alpine container
FROM alpine:latest as base

# Build Geth in a stock Go builder container
FROM golang:1.18-alpine as builder

RUN apk add --no-cache gcc musl-dev linux-headers git

ARG branch=optimism-prototype

RUN git clone -b $branch https://github.com/ethereum-optimism/reference-optimistic-geth /go-ethereum

RUN cd /go-ethereum && go run build/ci.go install ./cmd/geth

RUN /usr/local/bin/geth console --exec 'console.log(admin.nodeInfo.name)' --maxpeers=0 --nodiscover --dev 2>/dev/null | head -1 > /version.txt

# copy forge
FROM ghcr.io/foundry-rs/foundry:nightly as foundry
FROM base
COPY --from=foundry /usr/local/bin/forge /usr/local/bin/forge

RUN apk add --no-cache yarn git

# Regenerate the L1 genesis file
RUN mkdir /hive
ADD genesis.json /genesis.json

# Helper scripts; called from devnet.go
ADD cat.sh /hive-bin/cat.sh
RUN chmod +x /hive-bin/cat.sh

# Inject the startup script
ADD entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Deploy contracts using Hardhat
RUN git clone https://github.com/ethereum-optimism/optimism /hive/optimism
RUN cd /hive/optimism && git submodule init && git submodule update

# Deps
RUN yarn global add npx -W
RUN cd /hive/optimism && yarn && yarn build
RUN cd /hive/optimism/packages/contracts-bedrock && yarn && yarn build

VOLUME ["/db"]

EXPOSE 9545/tcp
EXPOSE 9546/tcp

ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
