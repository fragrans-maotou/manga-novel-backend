FROM postgres:16-alpine
# SPEC §4 schema applied on first empty data directory via official entrypoint.
COPY 001_init.sql /docker-entrypoint-initdb.d/001_init.sql
