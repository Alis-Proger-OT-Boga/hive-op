FROM golang:1.18.0-alpine3.15 as builder

RUN apk add --no-cache make gcc musl-dev linux-headers git jq bash

RUN git clone https://github.com/ethereum-optimism/optimistic-specs /app

WORKDIR /app
RUN make bss

FROM alpine:3.15

RUN apk add jq curl

COPY --from=builder /app/bin/bss /usr/local/bin
COPY ./bss-entrypoint.sh /bss-entrypoint.sh
COPY ./rollup.json /rollup.json

RUN /app/bin/bss --version 2>/dev/null | head -1 > /version.txt

ENTRYPOINT ["/bin/sh", "/bss-entrypoint.sh"]
