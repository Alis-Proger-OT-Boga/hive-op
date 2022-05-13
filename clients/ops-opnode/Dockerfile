FROM golang:1.18.0-alpine3.15 as builder

RUN apk add --no-cache make gcc musl-dev linux-headers git jq bash

RUN git clone https://github.com/ethereum-optimism/optimistic-specs /app

WORKDIR /app
RUN make opnode

FROM alpine:3.15

RUN apk add jq curl

COPY --from=builder /app/bin/op /usr/local/bin
COPY ./opnode-entrypoint.sh /opnode-entrypoint.sh
COPY ./rollup.json /rollup.json
RUN mkdir /hive /config
COPY p2p-node-key.txt /config
COPY p2p-sequencer-key.txt /config

RUN /app/bin/opnode --version 2>/dev/null | head -1 > /version.txt

EXPOSE 7545/tcp

ENTRYPOINT ["/bin/sh", "/opnode-entrypoint.sh"]
