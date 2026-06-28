#!/usr/bin/env bash
set -euo pipefail

repo_root="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="${PACKAGE_R_PLAYWRIGHT_TMPDIR:-$(mktemp -d /tmp/package-r-playwright.XXXXXX)}"
created_tmp=false
if [ -z "${PACKAGE_R_PLAYWRIGHT_TMPDIR:-}" ]; then
  created_tmp=true
fi
backend_log="$tmp_dir/filebrowser.log"
server_pid=""

cleanup() {
  if [ -n "$server_pid" ] && kill -0 "$server_pid" >/dev/null 2>&1; then
    kill "$server_pid" >/dev/null 2>&1 || true
    wait "$server_pid" >/dev/null 2>&1 || true
  fi
  if [ "$created_tmp" = "true" ]; then
    rm -rf "$tmp_dir"
  fi
}
trap cleanup EXIT

dump_backend_log() {
  if [ -f "$backend_log" ]; then
    sed -n '1,160p' "$backend_log" >&2 || true
  fi
}

wait_for_backend() {
  local ready_url="${FB_PLAYWRIGHT_READY_URL:-http://127.0.0.1:${FB_SERVER_PORT}/api/public/share/public-share/}"
  local timeout="${PACKAGE_R_PLAYWRIGHT_READY_TIMEOUT:-120}"
  local attempt=1
  local code

  printf '[playwright-backend] waiting for %s\n' "$ready_url"
  while [ "$attempt" -le "$timeout" ]; do
    code="$(curl -sS -o /dev/null -w '%{http_code}' "$ready_url" 2>/dev/null || true)"
    if [ "$code" = "200" ]; then
      printf '[playwright-backend] ready: %s\n' "$ready_url"
      return 0
    fi
    if ! kill -0 "$server_pid" >/dev/null 2>&1; then
      printf '[playwright-backend] backend exited before readiness; last HTTP status: %s\n' "${code:-000}" >&2
      dump_backend_log
      return 1
    fi
    if [ $((attempt % 20)) -eq 0 ]; then
      printf '[playwright-backend] still waiting; last HTTP status: %s\n' "${code:-000}" >&2
    fi
    sleep 1
    attempt=$((attempt + 1))
  done

  printf '[playwright-backend] timed out waiting for %s; last HTTP status: %s\n' "$ready_url" "${code:-000}" >&2
  dump_backend_log
  return 1
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

if [ ! -x "$FB_FILEBROWSER_BIN" ] || [ "${PACKAGE_R_PLAYWRIGHT_BUILD:-auto}" = "true" ]; then
  export GOCACHE="${GOCACHE:-/tmp/package-r-go-build}"
  make build-backend-dev
fi

./init.sh --add-shares public-share=/public --add-test-data /public
"$FB_FILEBROWSER_BIN" -a "$FB_ADDRESS" -p "$FB_SERVER_PORT" >"$backend_log" 2>&1 &
server_pid=$!
wait_for_backend
wait "$server_pid"
