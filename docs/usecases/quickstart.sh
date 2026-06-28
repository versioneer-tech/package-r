#!/usr/bin/env bash
set -euo pipefail

# title: Quickstart
# order: 10

# docs: hide-start
source "$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/usecase_lib.sh"
usecase_init
# docs: hide-end

# Build the local `filebrowser` binary.
# docs: command make build-backend
# docs: hide-start
usecase_build_backend
# docs: hide-end

# Configure the mounted workspace root, database path, and server port for the sample.
export FB_ROOT="${FB_ROOT:-/workspace}"
export FB_DATABASE="${FB_DATABASE:-/db/bolt.db}"
export FB_SERVER_PORT="${FB_SERVER_PORT:-8888}"
BASE_URL="${BASE_URL:-http://127.0.0.1:$FB_SERVER_PORT}"
ITEM_ID="${ITEM_ID:-67793f0b9478720001790586}"
PUBLIC_SHARE_HASH="${PUBLIC_SHARE_HASH:-public-share}"

# Prepare local sample data and a reusable public share.
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

# The prepared `/public` sample data is visible in the authenticated file
# browser UI, and the public share exposes the same files without login.
#
# docs: text ![Authenticated package data listing](../../imgs/screenshots/authenticated-public-listing.png)
# docs: text ![packageR public share directory](../../imgs/screenshots/public-share-directory.png)

# Check the public STAC catalog.
catalog_json="$FB_ROOT/quickstart-catalog.json"
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
PY

# Check one STAC item and its rewritten package asset URL.
item_json="$FB_ROOT/quickstart-item.json"
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
thumbnail = item["assets"]["thumbnail"]["href"]
assert f"/api/public/share/{expected_share}/openaerialmap-assets/{expected_item}/thumbnail.png" in thumbnail, thumbnail
PY

# Check the public share fallback download path for the same thumbnail.
curl -fsSL \
  "$BASE_URL/api/public/share/$PUBLIC_SHARE_HASH/openaerialmap-assets/$ITEM_ID/thumbnail.png?presign=true&followRedirect=true" \
  --output "$FB_ROOT/quickstart-thumbnail.png"
cmp "tests/data/openaerialmap-assets/$ITEM_ID/thumbnail.png" \
  "$FB_ROOT/quickstart-thumbnail.png"

# Authenticated users can also open normal browser previews for previewable
# files in the mounted workspace.
#
# docs: text ![Authenticated image preview](../../imgs/screenshots/authenticated-image-preview.png)
