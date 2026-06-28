#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/package_r_harness.sh"

ROOT_DIR="$(package_r_repo_root)"

FB_ADDRESS="${FB_ADDRESS:-127.0.0.1}"
FB_SERVER_PORT="${FB_SERVER_PORT:-8888}"
BASE_URL="${BASE_URL:-http://127.0.0.1:${FB_SERVER_PORT}}"
PUBLIC_SHARE_HASH="${PUBLIC_SHARE_HASH:-public-share}"
ITEM_ID="${ITEM_ID:-67793f0b9478720001790586}"
BUCKET_NAME="${BUCKET_NAME:-package-r-e2e}"
AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-package-r-e2e}"
AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-package-r-e2e-secret}"
AWS_REGION="${AWS_REGION:-us-east-1}"
RCLONE_BIN="${RCLONE_BIN:-rclone}"

tmp_dir=""
rclone_log=""
rclone_pid=""
filebrowser_pid=""

log() {
  printf '[e2e-s3] %s\n' "$*"
}

cleanup() {
  package_r_kill_pid "$filebrowser_pid"
  package_r_kill_pid "$rclone_pid"
  if [ "${PACKAGE_R_E2E_KEEP_TMP:-false}" = "true" ] && [ -n "$tmp_dir" ]; then
    log "keeping temp directory: $tmp_dir"
    return
  fi
  if [ -n "$tmp_dir" ]; then
    rm -rf "$tmp_dir" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log "missing required command: $1"
    exit 127
  fi
}

json_field() {
  field=$1
  python3 -c 'import json, sys; print(json.load(sys.stdin)[sys.argv[1]])' "$field"
}

fetch_and_compare() {
  url=$1
  expected=$2
  output=$3

  curl -fsSL "$url" --output "$output"
  cmp "$expected" "$output"
}

ensure_filebrowser_binary() {
  if [ -n "${FB_FILEBROWSER_BIN:-}" ] && [ -x "$FB_FILEBROWSER_BIN" ]; then
    return
  fi

  FB_FILEBROWSER_BIN="$ROOT_DIR/filebrowser"
  log "building filebrowser backend at $FB_FILEBROWSER_BIN"
  (cd "$ROOT_DIR" && make build-backend-dev)
}

rclone_version_line() {
  "$RCLONE_BIN" version 2>/dev/null | sed -n '1p'
}

rclone_version_supports_query_presign() {
  line=$1
  version="${line#rclone v}"
  version="${version%% *}"
  version="${version%%-*}"
  major="${version%%.*}"
  rest="${version#*.}"
  minor="${rest%%.*}"

  case "$major" in
    '' | *[!0-9]*) return 1 ;;
  esac
  case "$minor" in
    '' | *[!0-9]*) return 1 ;;
  esac

  # rclone v1.65.2 rejects AWS SigV4 query presigned URLs with UnsupportedAlgorithm.
  [ "$major" -gt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -ge 74 ]; }
}

require_compatible_rclone() {
  require_command "$RCLONE_BIN"

  version_line="$(rclone_version_line || true)"
  if ! rclone_version_supports_query_presign "$version_line"; then
    log "local ${version_line:-rclone} is too old; install rclone >= 1.74 or set RCLONE_BIN"
    exit 1
  fi
  log "using local ${version_line:-rclone}"
}

start_local_rclone() {
  S3_PORT="${S3_PORT:-19000}"
  AWS_ENDPOINT_URL="${AWS_ENDPOINT_URL:-http://127.0.0.1:${S3_PORT}}"
  s3_root="$tmp_dir/s3"
  mkdir -p "$s3_root/$BUCKET_NAME/public"
  cp -R "$ROOT_DIR/tests/data/." "$s3_root/$BUCKET_NAME/public/"

  require_compatible_rclone
  rclone_log="$tmp_dir/rclone.log"
  log "starting rclone S3 server with $RCLONE_BIN on $AWS_ENDPOINT_URL"
  "$RCLONE_BIN" serve s3 "$s3_root" \
    --addr "127.0.0.1:${S3_PORT}" \
    --auth-key "$AWS_ACCESS_KEY_ID,$AWS_SECRET_ACCESS_KEY" >"$rclone_log" 2>&1 &
  rclone_pid=$!

  if ! PACKAGE_R_LOG_PREFIX=e2e-s3 \
    PACKAGE_R_WAIT_STATUS=any \
    PACKAGE_R_WAIT_ATTEMPTS=120 \
    PACKAGE_R_WAIT_DELAY=0.25 \
    PACKAGE_R_CURL_TIMEOUT=2 \
    package_r_wait_for_http "$AWS_ENDPOINT_URL" "rclone S3" "$rclone_log" "$rclone_pid"; then
    exit 1
  fi

  EXPECTED_THUMBNAIL="${EXPECTED_THUMBNAIL:-$s3_root/$BUCKET_NAME/public/openaerialmap-assets/$ITEM_ID/thumbnail.png}"
  AUTH_RESOURCE_PATH="${AUTH_RESOURCE_PATH:-/$BUCKET_NAME/public/openaerialmap-assets/$ITEM_ID/thumbnail.png}"
  PUBLIC_SHARE_PATH="${PUBLIC_SHARE_PATH:-openaerialmap-assets/$ITEM_ID/thumbnail.png}"
  CATALOG_URL="${CATALOG_URL:-$BASE_URL/api/public/catalog/$PUBLIC_SHARE_HASH}"
}

init_and_start_local_filebrowser() {
  ensure_filebrowser_binary

  fb_root="$tmp_dir/s3"
  fb_db="$tmp_dir/filebrowser.db"

  log "initializing package-r state in $fb_db with root $fb_root"
  env \
    FB_FILEBROWSER_BIN="$FB_FILEBROWSER_BIN" \
    FB_DATABASE="$fb_db" \
    FB_ROOT="$fb_root" \
    FB_SERVER_PORT="$FB_SERVER_PORT" \
    FB_PASSWORD="package-r-e2e" \
    FB_AUTH_METHOD="proxy" \
    FB_AUTH_HEADER="X-Username" \
    FB_ALLOW_SHARING="true" \
    FB_ALLOW_CHANGING="true" \
    AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY_ID" \
    AWS_SECRET_ACCESS_KEY="$AWS_SECRET_ACCESS_KEY" \
    AWS_ENDPOINT_URL="$AWS_ENDPOINT_URL" \
    AWS_REGION="$AWS_REGION" \
    BUCKET_NAME="$BUCKET_NAME" \
    "$ROOT_DIR/init.sh" --add-shares "$PUBLIC_SHARE_HASH=/$BUCKET_NAME/public" >"$tmp_dir/init.log"

  log "starting package-r on $BASE_URL"
  export FB_DATABASE="$fb_db"
  export AWS_ACCESS_KEY_ID
  export AWS_SECRET_ACCESS_KEY
  export AWS_ENDPOINT_URL
  export AWS_REGION
  export BUCKET_NAME
  package_r_start_filebrowser "$FB_FILEBROWSER_BIN" "$FB_ADDRESS" "$FB_SERVER_PORT" "$tmp_dir/filebrowser.log"
  filebrowser_pid=$PACKAGE_R_STARTED_PID
  if ! PACKAGE_R_LOG_PREFIX=e2e-s3 \
    PACKAGE_R_WAIT_STATUS=any \
    PACKAGE_R_WAIT_ATTEMPTS=120 \
    PACKAGE_R_WAIT_DELAY=0.25 \
    PACKAGE_R_CURL_TIMEOUT=2 \
    package_r_wait_for_http "$BASE_URL" "package-r" "$tmp_dir/filebrowser.log" "$filebrowser_pid"; then
    exit 1
  fi
}

run_presign_checks() {
  require_command curl
  require_command python3

  : "${EXPECTED_THUMBNAIL:?EXPECTED_THUMBNAIL must point at the source object file}"
  : "${AUTH_RESOURCE_PATH:?AUTH_RESOURCE_PATH must be set}"
  : "${PUBLIC_SHARE_PATH:?PUBLIC_SHARE_PATH must be set}"
  : "${CATALOG_URL:?CATALOG_URL must be set}"

  log "checking public catalog endpoint"
  curl -fsS "$CATALOG_URL" | python3 -c 'import json, sys; data = json.load(sys.stdin); assert data["type"] == "FeatureCollection"; assert len(data["features"]) > 0'

  log "checking authenticated resource presign"
  token="$(curl -fsS -H 'X-Username: admin' "$BASE_URL/api/login")"
  auth_presigned_url="$(
    curl -fsS -H "X-Auth: $token" "$BASE_URL/api/resources${AUTH_RESOURCE_PATH}?presign=true" |
      json_field presignedURL
  )"
  fetch_and_compare "$auth_presigned_url" "$EXPECTED_THUMBNAIL" "$tmp_dir/auth-thumbnail.png"

  log "checking public share presign"
  public_presigned_url="$(
    curl -fsS "$BASE_URL/api/public/share/$PUBLIC_SHARE_HASH/$PUBLIC_SHARE_PATH?presign=true" |
      json_field presignedURL
  )"
  fetch_and_compare "$public_presigned_url" "$EXPECTED_THUMBNAIL" "$tmp_dir/public-thumbnail.png"

  log "checking public share presign redirect"
  fetch_and_compare \
    "$BASE_URL/api/public/share/$PUBLIC_SHARE_HASH/$PUBLIC_SHARE_PATH?presign=true&followRedirect=true" \
    "$EXPECTED_THUMBNAIL" \
    "$tmp_dir/public-thumbnail-redirect.png"
}

tmp_dir="$(mktemp -d)"
start_local_rclone
init_and_start_local_filebrowser
run_presign_checks
log "S3 presign e2e checks passed"
