#!/usr/bin/env bash
set -euo pipefail

# title: Quickstart
# order: 10

# Build the local `filebrowser` binary.
make build

# Prepare the local sample data and start packageR.
./init.sh --add-shares public-share=/public --add-test-data /public --serve

# In another terminal, check the public package and STAC catalog endpoints.
BASE_URL="${BASE_URL:-http://127.0.0.1:${FB_SERVER_PORT:-8080}}"
ITEM_ID="${ITEM_ID:-67793f0b9478720001790586}"

curl -sS "$BASE_URL/api/public/catalog/public-share"
curl -sS "$BASE_URL/api/public/catalog/public-share/openaerialmap-assets/$ITEM_ID/thumbnail.png"
curl -sSI "$BASE_URL/api/public/share/public-share/openaerialmap-assets/$ITEM_ID/thumbnail.png?presign=true&followRedirect=true"
