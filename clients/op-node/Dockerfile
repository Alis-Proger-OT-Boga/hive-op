ARG branch=develop
FROM us-docker.pkg.dev/oplabs-tools-artifacts/images/op-node:$branch

RUN apk add bash

COPY ./entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

RUN op-node --version 2>/dev/null | head -1 > /version.txt

ENTRYPOINT ["/bin/bash", "/entrypoint.sh"]
