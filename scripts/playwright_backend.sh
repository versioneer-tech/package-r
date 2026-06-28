#!/usr/bin/env bash
set -euo pipefail

script_dir="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$script_dir/package_r_harness.sh"

repo_root="$(package_r_repo_root)"
tmp_dir="${PACKAGE_R_PLAYWRIGHT_TMPDIR:-$(mktemp -d /tmp/package-r-playwright.XXXXXX)}"
created_tmp=false
if [ -z "${PACKAGE_R_PLAYWRIGHT_TMPDIR:-}" ]; then
  created_tmp=true
fi
backend_log="$tmp_dir/filebrowser.log"
server_pid=""

cleanup() {
  package_r_kill_pid "$server_pid"
  if [ "$created_tmp" = "true" ]; then
    rm -rf "$tmp_dir"
  fi
}
trap cleanup EXIT

wait_for_backend() {
  local ready_url="${FB_PLAYWRIGHT_READY_URL:-http://127.0.0.1:${FB_SERVER_PORT}/health}"

  PACKAGE_R_LOG_PREFIX=playwright-backend \
    PACKAGE_R_WAIT_ATTEMPTS="${PACKAGE_R_PLAYWRIGHT_READY_TIMEOUT:-120}" \
    PACKAGE_R_WAIT_DELAY=1 \
    PACKAGE_R_CURL_TIMEOUT="${PACKAGE_R_PLAYWRIGHT_CURL_TIMEOUT:-2}" \
    package_r_wait_for_http "$ready_url" "backend" "$backend_log" "$server_pid"
}

cd "$repo_root"

export FB_ROOT="${FB_ROOT:-$tmp_dir/root}"
export FB_DATABASE="${FB_DATABASE:-$tmp_dir/filebrowser.db}"
export FB_ADDRESS="${FB_ADDRESS:-127.0.0.1}"
export FB_SERVER_PORT="${FB_SERVER_PORT:-8888}"
export FB_AUTH_METHOD="${FB_AUTH_METHOD:-json}"
export FB_PASSWORD="${FB_PASSWORD:-admin}"
export FB_ALLOW_SHARING="${FB_ALLOW_SHARING:-true}"
export FB_ALLOW_CHANGING="${FB_ALLOW_CHANGING:-true}"
export FB_CATALOG_PREVIEW_URL="${FB_CATALOG_PREVIEW_URL:-https://radiantearth.github.io/stac-browser/#/external/}"
export FB_FILEBROWSER_BIN="${FB_FILEBROWSER_BIN:-$repo_root/filebrowser}"

package_r_build_backend_dev_if_needed "$repo_root" "$FB_FILEBROWSER_BIN" "${PACKAGE_R_PLAYWRIGHT_BUILD:-auto}"

./init.sh --add-shares public-share=/public --add-test-data /public
package_r_start_filebrowser "$FB_FILEBROWSER_BIN" "$FB_ADDRESS" "$FB_SERVER_PORT" "$backend_log"
server_pid=$PACKAGE_R_STARTED_PID
wait_for_backend
wait "$server_pid"
