#!/usr/bin/env bash
set -euo pipefail

repo_root="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="${PACKAGE_R_PLAYWRIGHT_TMPDIR:-$(mktemp -d /tmp/package-r-playwright.XXXXXX)}"
created_tmp=false
if [ -z "${PACKAGE_R_PLAYWRIGHT_TMPDIR:-}" ]; then
  created_tmp=true
fi

cleanup() {
  if [ "$created_tmp" = "true" ]; then
    rm -rf "$tmp_dir"
  fi
}
trap cleanup EXIT

cd "$repo_root"

export FB_ROOT="${FB_ROOT:-$tmp_dir/root}"
export FB_DATABASE="${FB_DATABASE:-$tmp_dir/filebrowser.db}"
export FB_ADDRESS="${FB_ADDRESS:-127.0.0.1}"
export FB_SERVER_PORT="${FB_SERVER_PORT:-8888}"
export FB_AUTH_METHOD="${FB_AUTH_METHOD:-json}"
export FB_PASSWORD="${FB_PASSWORD:-admin}"
export FB_ALLOW_SHARING="${FB_ALLOW_SHARING:-true}"
export FB_ALLOW_CHANGING="${FB_ALLOW_CHANGING:-true}"
export FB_FILEBROWSER_BIN="${FB_FILEBROWSER_BIN:-$repo_root/filebrowser}"

if [ ! -x "$FB_FILEBROWSER_BIN" ] || [ "${PACKAGE_R_PLAYWRIGHT_BUILD:-auto}" = "true" ]; then
  export GOCACHE="${GOCACHE:-/tmp/package-r-go-build}"
  make build-backend
fi

./init.sh --add-shares public-share=/public --add-test-data /public
"$FB_FILEBROWSER_BIN" -a "$FB_ADDRESS" -p "$FB_SERVER_PORT"
