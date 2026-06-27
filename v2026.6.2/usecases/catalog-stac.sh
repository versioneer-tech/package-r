#!/usr/bin/env bash
set -euo pipefail

# title: Catalog and STAC
# order: 30

# Prepare packageR with sample data, a public default share, and a
# `catalog.parquet` file below `/public`.
export FB_ROOT=/tmp/package-r
export FB_DATABASE="$FB_ROOT/filebrowser.db"

./init.sh --add-shares public-share=/public --add-test-data /public --serve

# In another terminal, set request variables.
BASE_URL="${BASE_URL:-http://127.0.0.1:${FB_SERVER_PORT:-8080}}"
ITEM_ID="${ITEM_ID:-67793f0b9478720001790586}"
PUBLIC_SHARE_HASH="${PUBLIC_SHARE_HASH:-public-share}"

# Request the public STAC FeatureCollection.
curl -fsS "$BASE_URL/api/public/catalog/$PUBLIC_SHARE_HASH" \
  | python3 - "$ITEM_ID" <<'PY'
import json
import sys

expected = sys.argv[1]
catalog = json.load(sys.stdin)
assert catalog["type"] == "FeatureCollection", catalog
assert expected in {feature["id"] for feature in catalog["features"]}, catalog
PY

# Request one STAC item through a package path.
curl -fsS "$BASE_URL/api/public/catalog/$PUBLIC_SHARE_HASH/openaerialmap-assets/$ITEM_ID/thumbnail.png" \
  | python3 - "$ITEM_ID" <<'PY'
import json
import sys

expected = sys.argv[1]
item = json.load(sys.stdin)
assert item["type"] == "Feature", item
assert item["id"] == expected, item
assert "thumbnail" in item["assets"], item
assert "presign" in item["assets"]["thumbnail"]["href"], item
PY
