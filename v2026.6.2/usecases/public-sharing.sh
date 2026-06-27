#!/usr/bin/env bash
set -euo pipefail

# title: Public Sharing
# order: 20

# Prepare packageR with a preconfigured public share. Users can write only in
# their own `/public/<username>` directory. Sharing is disabled for normal
# users; the public share is created by the bootstrap command.
export FB_ROOT=/tmp/package-r
export FB_DATABASE="$FB_ROOT/filebrowser.db"

FB_ALLOW_CHANGING=true ./init.sh --add-shares public-share=/public --add-test-data /public --serve

# In another terminal, set request variables.
BASE_URL="${BASE_URL:-http://127.0.0.1:${FB_SERVER_PORT:-8080}}"
ITEM_ID="${ITEM_ID:-67793f0b9478720001790586}"
PUBLIC_SHARE_HASH="${PUBLIC_SHARE_HASH:-public-share}"
AUTH_HEADER="${FB_AUTH_HEADER:-X-Username}"

# Login as Alice and Bob through the proxy header.
alice_token="$(
  curl -fsS -H "$AUTH_HEADER: alice" \
    "$BASE_URL/api/login"
)"
bob_token="$(
  curl -fsS -H "$AUTH_HEADER: bob" \
    "$BASE_URL/api/login"
)"

# Bob can write in `/public/bob`.
printf 'bob payload\n' > /tmp/package-r/bob-random.txt
curl -fsS -X POST -H "X-Auth: $bob_token" \
  --data-binary @/tmp/package-r/bob-random.txt \
  "$BASE_URL/api/resources/public/bob/random.txt?override=true" >/dev/null
curl -fsSL -H "X-Auth: $bob_token" \
  "$BASE_URL/api/resources/public/bob/random.txt?presign=true&followRedirect=true" \
  --output /tmp/package-r/bob-random-roundtrip.txt
cmp /tmp/package-r/bob-random.txt /tmp/package-r/bob-random-roundtrip.txt

# Bob cannot read or overwrite sample files outside his own public directory.
bob_read_alice_status="$(
  curl -sS -o /dev/null -w '%{http_code}' -H "X-Auth: $bob_token" \
    "$BASE_URL/api/resources/public/openaerialmap-assets/$ITEM_ID/thumbnail.png"
)"
case "$bob_read_alice_status" in
  403 | 404) ;;
  *) printf 'Expected Bob to be denied for shared sample data, got HTTP %s\n' "$bob_read_alice_status" >&2; exit 1 ;;
esac

bob_write_alice_status="$(
  curl -sS -o /dev/null -w '%{http_code}' -X POST -H "X-Auth: $bob_token" \
    --data-binary @/tmp/package-r/bob-random.txt \
    "$BASE_URL/api/resources/public/openaerialmap-assets/$ITEM_ID/thumbnail.png?override=true"
)"
case "$bob_write_alice_status" in
  403 | 404) ;;
  *) printf 'Expected Bob write to shared sample data to be denied, got HTTP %s\n' "$bob_write_alice_status" >&2; exit 1 ;;
esac

# Alice cannot overwrite Bob's files.
alice_write_bob_status="$(
  curl -sS -o /dev/null -w '%{http_code}' -X POST -H "X-Auth: $alice_token" \
    --data-binary @/tmp/package-r/bob-random.txt \
    "$BASE_URL/api/resources/public/bob/random.txt?override=true"
)"
case "$alice_write_bob_status" in
  403 | 404) ;;
  *) printf 'Expected Alice write to Bob data to be denied, got HTTP %s\n' "$alice_write_bob_status" >&2; exit 1 ;;
esac

# Individual files are still reachable through the preconfigured public share.
curl -fsSL \
  "$BASE_URL/api/public/share/$PUBLIC_SHARE_HASH/openaerialmap-assets/$ITEM_ID/thumbnail.png?presign=true&followRedirect=true" \
  --output /tmp/package-r/public-presigned-thumbnail.png
cmp "tests/data/openaerialmap-assets/$ITEM_ID/thumbnail.png" \
  /tmp/package-r/public-presigned-thumbnail.png
