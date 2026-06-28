#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/package_r_harness.sh"

ROOT_DIR="$(package_r_repo_root)"
FB_ADDRESS="${FB_ADDRESS:-127.0.0.1}"
FB_SERVER_PORT="${FB_SERVER_PORT:-18888}"
BASE_URL="http://${FB_ADDRESS}:${FB_SERVER_PORT}"

tmp_dir=""
filebrowser_pid=""

log() {
  printf '[production-smoke] %s\n' "$*"
}

cleanup() {
  package_r_kill_pid "$filebrowser_pid"
  if [ -n "$tmp_dir" ]; then
    rm -rf "$tmp_dir" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

tmp_dir="$(mktemp -d)"

log "building production backend with embedded frontend"
(cd "$ROOT_DIR" && make build-backend)

log "initializing isolated packageR state"
env \
  FB_FILEBROWSER_BIN="$ROOT_DIR/filebrowser" \
  FB_DATABASE="$tmp_dir/filebrowser.db" \
  FB_ROOT="$tmp_dir/root" \
  FB_PASSWORD="package-r-smoke" \
  FB_AUTH_METHOD="proxy" \
  FB_AUTH_HEADER="X-Username" \
  FB_ALLOW_SHARING="true" \
  AWS_ACCESS_KEY_ID="" \
  AWS_SECRET_ACCESS_KEY="" \
  AWS_ENDPOINT_URL="" \
  AWS_REGION="" \
  BUCKET_NAME="" \
  BUCKET_PREFIX="" \
  "$ROOT_DIR/init.sh" --add-shares public-share=/public --add-test-data /public >"$tmp_dir/init.log"

log "starting production binary on $BASE_URL"
export FB_DATABASE="$tmp_dir/filebrowser.db"
export AWS_ACCESS_KEY_ID=""
export AWS_SECRET_ACCESS_KEY=""
export AWS_ENDPOINT_URL=""
export AWS_REGION=""
export BUCKET_NAME=""
export BUCKET_PREFIX=""
package_r_start_filebrowser "$ROOT_DIR/filebrowser" "$FB_ADDRESS" "$FB_SERVER_PORT" "$tmp_dir/filebrowser.log"
filebrowser_pid=$PACKAGE_R_STARTED_PID

PACKAGE_R_LOG_PREFIX=production-smoke \
  PACKAGE_R_WAIT_ATTEMPTS=120 \
  PACKAGE_R_WAIT_DELAY=0.25 \
  PACKAGE_R_CURL_TIMEOUT=2 \
  package_r_wait_for_http "$BASE_URL/health" "packageR health" "$tmp_dir/filebrowser.log" "$filebrowser_pid"

index_html="$tmp_dir/index.html"
curl -fsS "$BASE_URL/" --output "$index_html"
grep -q '<div id="app">' "$index_html"

curl -fsS "$BASE_URL/api/public/share/public-share/" >/dev/null

log "production build smoke checks passed"
