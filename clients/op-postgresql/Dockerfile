FROM boky/postgresql

RUN postgres --version > /version.txt

ENV PG_TRUST_LOCALNET=true

ENTRYPOINT ["/sbin/entrypoint.sh"]
