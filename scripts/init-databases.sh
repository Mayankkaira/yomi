# Creates the four per-service databases in the shared PostgreSQL instance.
# Runs once on first container start via docker-entrypoint-initdb.d.
# Idempotent: POSTGRES_DB (yomi_auth) is already created by the entrypoint,
# so each CREATE is guarded with "IF NOT EXISTS"-style checks.
#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<-EOSQL
    SELECT 'CREATE DATABASE yomi_auth'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'yomi_auth')\gexec
    SELECT 'CREATE DATABASE yomi_consent'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'yomi_consent')\gexec
    SELECT 'CREATE DATABASE yomi_audit'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'yomi_audit')\gexec
    SELECT 'CREATE DATABASE yomi_institution'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'yomi_institution')\gexec
EOSQL
