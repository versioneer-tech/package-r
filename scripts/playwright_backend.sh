#!/usr/bin/env bash
set -euo pipefail

script_dir="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(CDPATH= cd -- "$script_dir/.." && pwd)"
tmp_dir="${PACKAGE_R_PLAYWRIGHT_TMPDIR:-$(mktemp -d /tmp/package-r-playwright.XXXXXX)}"
created_tmp=false
if [ -z "${PACKAGE_R_PLAYWRIGHT_TMPDIR:-}" ]; then
  created_tmp=true
fi
backend_log="$tmp_dir/filebrowser.log"
rclone_log="$tmp_dir/rclone.log"
server_pid=""
rclone_pid=""

readonly access_key_id=my-access-key
readonly secret_access_key=my-secret-key
readonly bucket_name=my-bucket
rclone_bin="${RCLONE_BIN:-rclone}"

log() {
  printf '[%s] %s\n' "${PACKAGE_R_LOG_PREFIX:-package-r}" "$*"
}

dump_log() {
  local log_file="${1:-}"

  if [ -n "$log_file" ] && [ -f "$log_file" ]; then
    sed -n '1,160p' "$log_file" >&2 || true
  fi
}

kill_pid() {
  local pid="${1:-}"

  if [ -n "$pid" ]; then
    kill "$pid" >/dev/null 2>&1 || true
    wait "$pid" >/dev/null 2>&1 || true
  fi
}

wait_for_http() {
  local url=$1
  local label=$2
  local log_file="${3:-}"
  local pid="${4:-}"
  local attempts="${PACKAGE_R_WAIT_ATTEMPTS:-120}"
  local delay="${PACKAGE_R_WAIT_DELAY:-0.25}"
  local curl_timeout="${PACKAGE_R_CURL_TIMEOUT:-2}"
  local expected_status="${PACKAGE_R_WAIT_STATUS:-200}"
  local attempt=1
  local code

  log "waiting for $label at $url"
  while [ "$attempt" -le "$attempts" ]; do
    code="$(curl --max-time "$curl_timeout" -sS -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)"
    if { [ "$expected_status" = "any" ] && [ "$code" != "000" ]; } || [ "$code" = "$expected_status" ]; then
      log "ready: $label"
      return 0
    fi
    if [ -n "$pid" ] && ! kill -0 "$pid" >/dev/null 2>&1; then
      log "$label exited before readiness; last HTTP status: ${code:-000}" >&2
      dump_log "$log_file"
      return 1
    fi
    sleep "$delay"
    attempt=$((attempt + 1))
  done

  log "timed out waiting for $label at $url; last HTTP status: ${code:-000}" >&2
  dump_log "$log_file"
  return 1
}

build_backend_if_needed() {
  local binary=$1
  local build_mode="${2:-auto}"

  if [ ! -x "$binary" ] || [ "$build_mode" = "true" ]; then
    export GOCACHE="${GOCACHE:-/tmp/package-r-go-build}"
    make build-backend-dev
  fi
}

start_backend() {
  local binary=$1
  local address=$2
  local port=$3
  local log_file=$4

  "$binary" -a "$address" -p "$port" >"$log_file" 2>&1 &
  server_pid=$!
}

cleanup() {
  local status=$?
  trap - EXIT INT TERM
  kill_pid "$server_pid"
  kill_pid "$rclone_pid"
  if [ "$created_tmp" = "true" ]; then
	rm -rf "$tmp_dir"
  fi
  exit "$status"
}
trap cleanup EXIT INT TERM

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
	printf 'Missing required command: %s\n' "$1" >&2
	exit 1
  fi
}

free_port() {
  python3 - <<'PY'
import socket

with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
    sock.bind(("127.0.0.1", 0))
    print(sock.getsockname()[1])
PY
}

require_compatible_rclone() {
  local output line major minor
  output="$("$rclone_bin" version 2>/dev/null || true)"
  line="${output%%$'\n'*}"
  if [[ "$line" =~ ^rclone[[:space:]]v([0-9]+)\.([0-9]+) ]]; then
	major="${BASH_REMATCH[1]}"
	minor="${BASH_REMATCH[2]}"
	if ((major > 1 || (major == 1 && minor >= 74))); then
	  return 0
	fi
  fi

  printf 'rclone 1.74 or newer is required; found: %s\n' "${line:-unknown version}" >&2
  exit 1
}

start_rclone() {
  local rclone_port
  rclone_port="$(free_port)"
  export AWS_ENDPOINT_URL="http://127.0.0.1:${rclone_port}"

  mkdir -p "$tmp_dir/home" "$tmp_dir/rclone/$bucket_name"
  cp -R "$repo_root/tests/data/." "$tmp_dir/rclone/$bucket_name/"

  env -i \
	HOME="$tmp_dir/home" \
	PATH="$PATH" \
	RCLONE_BIN="$rclone_bin" \
	PACKAGE_R_LOCAL_S3_ROOT="$tmp_dir/rclone" \
	PACKAGE_R_LOCAL_S3_ADDRESS="127.0.0.1:${rclone_port}" \
	PACKAGE_R_LOCAL_S3_CONTROL_ADDRESS= \
	AWS_ACCESS_KEY_ID="$access_key_id" \
	AWS_SECRET_ACCESS_KEY="$secret_access_key" \
	"$script_dir/local_s3.sh" serve \
	>"$rclone_log" 2>&1 &
  rclone_pid=$!

  PACKAGE_R_LOG_PREFIX=playwright-rclone \
	PACKAGE_R_WAIT_STATUS=any \
	PACKAGE_R_WAIT_ATTEMPTS=120 \
	PACKAGE_R_WAIT_DELAY=0.25 \
	PACKAGE_R_CURL_TIMEOUT=2 \
	wait_for_http "$AWS_ENDPOINT_URL" "rclone S3" "$rclone_log" "$rclone_pid"
}

wait_for_backend() {
  local ready_url="${FB_PLAYWRIGHT_READY_URL:-http://127.0.0.1:${FB_SERVER_PORT}/health}"

  PACKAGE_R_LOG_PREFIX=playwright-backend \
    PACKAGE_R_WAIT_ATTEMPTS="${PACKAGE_R_PLAYWRIGHT_READY_TIMEOUT:-120}" \
    PACKAGE_R_WAIT_DELAY=1 \
    PACKAGE_R_CURL_TIMEOUT="${PACKAGE_R_PLAYWRIGHT_CURL_TIMEOUT:-2}" \
    wait_for_http "$ready_url" "backend" "$backend_log" "$server_pid"
}

cd "$repo_root"

require_command python3
require_command "$rclone_bin"
require_compatible_rclone
start_rclone

export FB_ROOT="${FB_ROOT:-$bucket_name}"
export FB_DATABASE="${FB_DATABASE:-$tmp_dir/filebrowser.db}"
export FB_ADDRESS="${FB_ADDRESS:-127.0.0.1}"
export FB_SERVER_PORT="${FB_SERVER_PORT:-8888}"
export FB_AUTH_METHOD="${FB_AUTH_METHOD:-json}"
export FB_PASSWORD="${FB_PASSWORD:-admin}"
export FB_ALLOW_CHANGING="${FB_ALLOW_CHANGING:-true}"
export FB_CATALOG_PREVIEW_URL="${FB_CATALOG_PREVIEW_URL:-https://radiantearth.github.io/stac-browser/#/external/}"
export FB_FILEBROWSER_BIN="${FB_FILEBROWSER_BIN:-$repo_root/filebrowser}"
export XDG_CACHE_HOME="${XDG_CACHE_HOME:-$tmp_dir/home/.cache}"
export RCLONE_CONFIG=/dev/null
export AWS_ACCESS_KEY_ID="$access_key_id"
export AWS_SECRET_ACCESS_KEY="$secret_access_key"
export AWS_REGION=us-east-1

mkdir -p "$XDG_CACHE_HOME"

build_backend_if_needed "$FB_FILEBROWSER_BIN" "${PACKAGE_R_PLAYWRIGHT_BUILD:-auto}"

./init.sh --add-shares public-share=/public
start_backend "$FB_FILEBROWSER_BIN" "$FB_ADDRESS" "$FB_SERVER_PORT" "$backend_log"
wait_for_backend
wait "$server_pid"
