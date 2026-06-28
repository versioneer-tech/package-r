#!/usr/bin/env bash
set -euo pipefail

usecase_repo_root() {
  local script_dir
  script_dir="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
  CDPATH= cd -- "$script_dir/.." && pwd
}

usecase_init() {
  PACKAGE_R_USECASE_REPO_ROOT="${PACKAGE_R_USECASE_REPO_ROOT:-$(usecase_repo_root)}"
  cd "$PACKAGE_R_USECASE_REPO_ROOT"

  export FB_ROOT="${FB_ROOT:-/workspace}"
  export FB_DATABASE="${FB_DATABASE:-/db/bolt.db}"
  export FB_ADDRESS="${FB_ADDRESS:-127.0.0.1}"
  export FB_SERVER_PORT="${FB_SERVER_PORT:-8888}"
  export FB_FILEBROWSER_BIN="${FB_FILEBROWSER_BIN:-$PACKAGE_R_USECASE_REPO_ROOT/filebrowser}"
  export BASE_URL="${BASE_URL:-http://127.0.0.1:$FB_SERVER_PORT}"
  export ITEM_ID="${ITEM_ID:-67793f0b9478720001790586}"
  export PUBLIC_SHARE_HASH="${PUBLIC_SHARE_HASH:-public-share}"
  export AUTH_HEADER="${FB_AUTH_HEADER:-X-Username}"
  export PACKAGE_R_USECASE_SERVER_LOG="${PACKAGE_R_USECASE_SERVER_LOG:-$FB_ROOT/filebrowser.log}"

  mkdir -p "$FB_ROOT" "$(dirname -- "$FB_DATABASE")"
  trap usecase_cleanup EXIT
}

usecase_build_backend() {
  if [ "${PACKAGE_R_USECASE_SKIP_BUILD:-false}" = "true" ]; then
    return
  fi
  export GOCACHE="${GOCACHE:-/tmp/package-r-go-build}"
  make build-backend
}

usecase_start_server() {
  local wait_url=$1
  mkdir -p "$(dirname -- "$PACKAGE_R_USECASE_SERVER_LOG")"
  "$FB_FILEBROWSER_BIN" -a "$FB_ADDRESS" -p "$FB_SERVER_PORT" >"$PACKAGE_R_USECASE_SERVER_LOG" 2>&1 &
  PACKAGE_R_USECASE_SERVER_PID=$!
  usecase_wait_for_url "$wait_url"
}

usecase_wait_for_url() {
  local url=$1
  local attempt
  for attempt in {1..100}; do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return
    fi
    if [ -n "${PACKAGE_R_USECASE_SERVER_PID:-}" ] &&
      ! kill -0 "$PACKAGE_R_USECASE_SERVER_PID" 2>/dev/null; then
      cat "$PACKAGE_R_USECASE_SERVER_LOG" >&2 || true
      return 1
    fi
    sleep 0.1
  done
  cat "$PACKAGE_R_USECASE_SERVER_LOG" >&2 || true
  printf 'Timed out waiting for %s\n' "$url" >&2
  return 1
}

usecase_cleanup() {
  if [ -n "${PACKAGE_R_USECASE_SERVER_PID:-}" ]; then
    kill "$PACKAGE_R_USECASE_SERVER_PID" 2>/dev/null || true
    wait "$PACKAGE_R_USECASE_SERVER_PID" 2>/dev/null || true
  fi
}
