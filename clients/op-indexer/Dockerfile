FROM debian:bullseye-slim

COPY ./entrypoint.sh /entrypoint.sh
COPY ./indexer /bin
RUN chmod +x /bin/indexer
RUN chmod +x /entrypoint.sh

RUN /bin/indexer --version 2>/dev/null | head -1 > /version.txt

ENTRYPOINT ["/bin/bash", "/entrypoint.sh"]
