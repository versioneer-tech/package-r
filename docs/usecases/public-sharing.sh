#!/usr/bin/env bash
set -euo pipefail

# title: Public Sharing
# order: 20

# docs: hide-start
source "$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/usecase_lib.sh"
usecase_init
# docs: hide-end

# Build the local `filebrowser` binary.
# docs: command make build-backend
# docs: hide-start
usecase_build_backend
# docs: hide-end

# Configure local state for a public share. Normal proxy-created users get a
# private generated home directory, while mounted package data elsewhere below
# `FB_ROOT` remains visible through the authenticated resource API.
export FB_ROOT="${FB_ROOT:-/workspace}"
export FB_DATABASE="${FB_DATABASE:-/db/bolt.db}"
export FB_SERVER_PORT="${FB_SERVER_PORT:-8888}"
BASE_URL="${BASE_URL:-http://127.0.0.1:$FB_SERVER_PORT}"
ITEM_ID="${ITEM_ID:-67793f0b9478720001790586}"
PUBLIC_SHARE_HASH="${PUBLIC_SHARE_HASH:-public-share}"
AUTH_HEADER="${FB_AUTH_HEADER:-X-Username}"

# Prepare packageR with sample data, write permissions, and the public share.
# docs: command FB_ALLOW_CHANGING=true ./init.sh --add-shares public-share=/public --add-test-data /public
# docs: hide-start
FB_ALLOW_CHANGING=true ./init.sh --add-shares "$PUBLIC_SHARE_HASH=/public" --add-test-data /public
# docs: hide-end

# Start packageR.
# docs: command ./filebrowser -p "$FB_SERVER_PORT"
# docs: hide-start
usecase_start_server "$BASE_URL/api/public/share/$PUBLIC_SHARE_HASH/"
# docs: hide-end

# In another terminal, run the checks below.

# Login as Admin, Alice, and Bob through the proxy header.
admin_token="$(
  curl -fsS -H "$AUTH_HEADER: admin" \
    "$BASE_URL/api/login"
)"
alice_token="$(
  curl -fsS -H "$AUTH_HEADER: alice" \
    "$BASE_URL/api/login"
)"
bob_token="$(
  curl -fsS -H "$AUTH_HEADER: bob" \
    "$BASE_URL/api/login"
)"

# Bob can write a file in his generated `/home/bob` workspace path. The
# root-scoped admin view verifies the uploaded content because normal users do
# not have download permission by default.
bob_payload="$FB_ROOT/bob-random.txt"
bob_roundtrip="$FB_ROOT/bob-random-roundtrip.txt"
printf 'bob payload\n' > "$bob_payload"
curl -fsS -X POST -H "X-Auth: $bob_token" \
  "$BASE_URL/api/resources/home/bob/public/bob/?override=true" >/dev/null
curl -fsS -X POST -H "X-Auth: $bob_token" \
  --data-binary @"$bob_payload" \
  "$BASE_URL/api/resources/home/bob/public/bob/random.txt?override=true" >/dev/null
curl -fsSL -H "X-Auth: $admin_token" \
  "$BASE_URL/api/resources/home/bob/public/bob/random.txt?presign=true&followRedirect=true" \
  --output "$bob_roundtrip"
cmp "$bob_payload" "$bob_roundtrip"

# Alice and Bob can still inspect mounted sample files outside `/home`; the
# generated rules only hide sibling generated homes.
curl -fsS -H "X-Auth: $bob_token" \
  "$BASE_URL/api/resources/public/openaerialmap-assets/$ITEM_ID/thumbnail.png" \
  >/dev/null
curl -fsS -H "X-Auth: $alice_token" \
  "$BASE_URL/api/resources/public/openaerialmap-assets/$ITEM_ID/thumbnail.png" \
  >/dev/null

# The same sample thumbnail remains reachable through the admin-created public
# share, using the local fallback download URL when object-storage credentials
# are not configured.

# docs: text ![Public share presigned URL fallback](../../imgs/screenshots/public-share-presign.png)
public_thumbnail="$FB_ROOT/public-presigned-thumbnail.png"
curl -fsSL \
  "$BASE_URL/api/public/share/$PUBLIC_SHARE_HASH/openaerialmap-assets/$ITEM_ID/thumbnail.png?presign=true&followRedirect=true" \
  --output "$public_thumbnail"
cmp "tests/data/openaerialmap-assets/$ITEM_ID/thumbnail.png" \
  "$public_thumbnail"
