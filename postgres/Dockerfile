FROM postgres:16-alpine
# SPEC schema + demo seed (applied only on empty data directory).
COPY 001_init.sql /docker-entrypoint-initdb.d/001_init.sql
COPY 002_seed_demo.sql /docker-entrypoint-initdb.d/002_seed_demo.sql
