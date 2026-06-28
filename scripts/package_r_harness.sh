#!/usr/bin/env bash

package_r_repo_root() {
  CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd
}

package_r_log() {
  printf '[%s] %s\n' "${PACKAGE_R_LOG_PREFIX:-package-r}" "$*"
}

package_r_dump_log() {
  local log_file="${1:-}"

  if [ -n "$log_file" ] && [ -f "$log_file" ]; then
    sed -n '1,160p' "$log_file" >&2 || true
  fi
}

package_r_kill_pid() {
  local pid="${1:-}"

  if [ -n "$pid" ]; then
    kill "$pid" >/dev/null 2>&1 || true
    wait "$pid" >/dev/null 2>&1 || true
  fi
}

package_r_wait_for_http() {
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

  package_r_log "waiting for $label at $url"
  while [ "$attempt" -le "$attempts" ]; do
    code="$(curl --max-time "$curl_timeout" -sS -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)"
    if { [ "$expected_status" = "any" ] && [ "$code" != "000" ]; } || [ "$code" = "$expected_status" ]; then
      package_r_log "ready: $label"
      return 0
    fi
    if [ -n "$pid" ] && ! kill -0 "$pid" >/dev/null 2>&1; then
      package_r_log "$label exited before readiness; last HTTP status: ${code:-000}" >&2
      package_r_dump_log "$log_file"
      return 1
    fi
    sleep "$delay"
    attempt=$((attempt + 1))
  done

  package_r_log "timed out waiting for $label at $url; last HTTP status: ${code:-000}" >&2
  package_r_dump_log "$log_file"
  return 1
}

package_r_build_backend_dev_if_needed() {
  local repo_root=$1
  local binary=$2
  local build_mode="${3:-auto}"

  if [ ! -x "$binary" ] || [ "$build_mode" = "true" ]; then
    export GOCACHE="${GOCACHE:-/tmp/package-r-go-build}"
    (cd "$repo_root" && make build-backend-dev)
  fi
}

package_r_start_filebrowser() {
  local binary=$1
  local address=$2
  local port=$3
  local log_file=$4

  "$binary" -a "$address" -p "$port" >"$log_file" 2>&1 &
  PACKAGE_R_STARTED_PID=$!
}
