ARG branch=c6edc561556bb99a72dab23618ebdf6c14ea0f24
FROM us-central1-docker.pkg.dev/bedrock-goerli-development/images/contract-artifacts-bedrock:$branch as artifacts
FROM us-central1-docker.pkg.dev/bedrock-goerli-development/images/op-node:$branch

RUN apk add --no-cache python3 bash

RUN echo $branch > /version.txt

COPY --from=artifacts /artifacts /artifacts
COPY ./entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh

# Helper scripts; called from devnet.go
COPY scripts /hive-bin
RUN chmod +x /hive-bin/*

EXPOSE 8545/tcp

WORKDIR /hive/optimism

ENTRYPOINT ["/bin/bash", "/entrypoint.sh"]
