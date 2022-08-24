ARG branch=develop
FROM us-central1-docker.pkg.dev/bedrock-goerli-development/images/op-node:$branch as opnode
FROM us-central1-docker.pkg.dev/bedrock-goerli-development/images/deployer-bedrock:$branch

RUN npm install http-server -g

RUN echo $branch > /version.txt

ADD entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

COPY --from=opnode /usr/local/bin/op-node /usr/local/bin/op-node

# Helper scripts; called from devnet.go
COPY scripts /hive-bin
RUN chmod +x /hive-bin/*

EXPOSE 8545/tcp

WORKDIR /hive/optimism

ENTRYPOINT ["/bin/bash", "/entrypoint.sh"]
