FROM boky/postgresql

RUN postgres --version > /version.txt

ENV PG_TRUST_LOCALNET=true
ENV DB_NAME=indexer

ENTRYPOINT ["/sbin/entrypoint.sh"]
