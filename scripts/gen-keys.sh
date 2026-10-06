#!/usr/bin/env bash
# Generates the RSA key pair used for JWT signing/verification.
# Output: keys/jwt-private.pem, keys/jwt-public.pem
set -euo pipefail

cd "$(dirname "$0")/.."
mkdir -p keys

if [[ ! -f keys/jwt-private.pem ]]; then
  openssl genrsa -out keys/jwt-private.pem 2048
  openssl rsa -in keys/jwt-private.pem -pubout -out keys/jwt-public.pem
  chmod 600 keys/jwt-private.pem
  echo "JWT keys written to keys/"
else
  echo "JWT keys already exist; skipping generation."
fi
