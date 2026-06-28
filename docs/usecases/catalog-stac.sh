#!/usr/bin/env bash
set -euo pipefail

# title: Catalog and STAC
# order: 30

# docs: hide-start
source "$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/usecase_lib.sh"
usecase_init
# docs: hide-end

# Build the local `filebrowser` binary.
# docs: command make build-backend
# docs: hide-start
usecase_build_backend
# docs: hide-end

# Configure local state with sample package data and a public share.
export FB_ROOT="${FB_ROOT:-/workspace}"
export FB_DATABASE="${FB_DATABASE:-/db/bolt.db}"
export FB_SERVER_PORT="${FB_SERVER_PORT:-8888}"
BASE_URL="${BASE_URL:-http://127.0.0.1:$FB_SERVER_PORT}"
ITEM_ID="${ITEM_ID:-67793f0b9478720001790586}"
PUBLIC_SHARE_HASH="${PUBLIC_SHARE_HASH:-public-share}"

# The catalog endpoint rewrites matching asset links to public packageR HTTP
# URLs. A STAC Browser or similar client can open those assets without S3
# credentials or custom signing logic, as long as it follows HTTP redirects.

# Prepare the `catalog.parquet` file below `/public`.
# docs: command ./init.sh --add-shares public-share=/public --add-test-data /public
# docs: hide-start
./init.sh --add-shares "$PUBLIC_SHARE_HASH=/public" --add-test-data /public
# docs: hide-end

# Start packageR.
# docs: command ./filebrowser -p "$FB_SERVER_PORT"
# docs: hide-start
usecase_start_server "$BASE_URL/api/public/share/$PUBLIC_SHARE_HASH/"
# docs: hide-end

# In another terminal, run the checks below.

# Request the public STAC FeatureCollection.
catalog_json="$FB_ROOT/catalog-stac-catalog.json"
curl -fsS "$BASE_URL/api/public/catalog/$PUBLIC_SHARE_HASH" \
  --output "$catalog_json"
python3 - "$ITEM_ID" "$catalog_json" <<'PY'
import json
import sys
from pathlib import Path

expected, catalog_path = sys.argv[1:]
catalog = json.loads(Path(catalog_path).read_text())
assert catalog["type"] == "FeatureCollection", catalog
assert expected in {feature["id"] for feature in catalog["features"]}, catalog
assert all(feature["type"] == "Feature" for feature in catalog["features"]), catalog
PY

# Request one STAC item through a package asset path.
item_json="$FB_ROOT/catalog-stac-item.json"
curl -fsS "$BASE_URL/api/public/catalog/$PUBLIC_SHARE_HASH/openaerialmap-assets/$ITEM_ID/thumbnail.png" \
  --output "$item_json"
python3 - "$ITEM_ID" "$PUBLIC_SHARE_HASH" "$item_json" <<'PY'
import json
import sys
from pathlib import Path

expected_item, expected_share, item_path = sys.argv[1:]
item = json.loads(Path(item_path).read_text())
assert item["type"] == "Feature", item
assert item["id"] == expected_item, item
assets = item["assets"]
assert "thumbnail" in assets, item
thumbnail = assets["thumbnail"]["href"]
expected_path = f"/api/public/share/{expected_share}/openaerialmap-assets/{expected_item}/thumbnail.png"
assert expected_path in thumbnail, thumbnail
assert "presign" in thumbnail, thumbnail
assert "followRedirect" in thumbnail, thumbnail
PY
