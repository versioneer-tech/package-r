#!/usr/bin/env bash

set -euo pipefail

script_dir="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(CDPATH= cd -- "$script_dir/.." && pwd)"
state_dir="$repo_root/.vscode"
log_file="$state_dir/local-s3.log"
rclone_bin="${RCLONE_BIN:-rclone}"
serve_root="${PACKAGE_R_LOCAL_S3_ROOT:-$repo_root/tests}"
address="${PACKAGE_R_LOCAL_S3_ADDRESS:-127.0.0.1:19100}"
endpoint="http://$address"
control_address="${PACKAGE_R_LOCAL_S3_CONTROL_ADDRESS-127.0.0.1:19101}"
control_endpoint="http://$control_address"
access_key_id="${AWS_ACCESS_KEY_ID:-my-access-key}"
secret_access_key="${AWS_SECRET_ACCESS_KEY:-my-secret-key}"
server_owned=false

usage() {
  printf 'Usage: %s run|serve|stop\n' "$0" >&2
}

control_ready() {
  [ -n "$control_address" ] || return 1
  curl --max-time 1 -fsS -X POST "$control_endpoint/rc/noop" >/dev/null 2>&1
}

s3_ready() {
  curl --max-time 1 -sS -o /dev/null "$endpoint" 2>/dev/null
}

stop_server() {
  local attempt

  if control_ready; then
    curl --max-time 2 -fsS -X POST "$control_endpoint/core/quit" >/dev/null
    attempt=1
    while [ "$attempt" -le 40 ]; do
      if ! control_ready && ! s3_ready; then
        break
      fi
      sleep 0.1
      attempt=$((attempt + 1))
    done
    printf '[local-s3] stopped\n'
  elif s3_ready; then
    printf '[local-s3] cannot stop the server at %s because its control endpoint is unavailable\n' "$endpoint" >&2
    return 1
  else
    printf '[local-s3] not running\n'
  fi
}

serve_s3() {
  local -a args

  args=(
    serve s3 ":local:$serve_root"
    --addr "$address"
    --auth-key "$access_key_id,$secret_access_key"
    --config /dev/null
  )
  if [ -n "$control_address" ]; then
    args+=(--rc --rc-addr "$control_address" --rc-no-auth)
  fi
  exec "$rclone_bin" "${args[@]}"
}

cleanup() {
  local status=$?

  trap - EXIT INT TERM
  if [ "$server_owned" = "true" ]; then
    stop_server >/dev/null 2>&1 || true
  fi
  exit "$status"
}

run_local_environment() {
  local attempt
  local pid=""
  local status

  for command_name in "$rclone_bin" curl make; do
    if ! command -v "$command_name" >/dev/null 2>&1; then
      printf '[local-dev] %s is required\n' "$command_name" >&2
      return 1
    fi
  done

  mkdir -p "$state_dir"
  printf '[local-dev] preparing\n'
  if control_ready && s3_ready; then
    printf '[local-s3] already serving %s at %s\n' "$serve_root" "$endpoint"
  elif s3_ready; then
    printf '[local-s3] port %s is used by a server without the expected rclone control endpoint\n' "$address" >&2
    return 1
  else
    "$script_dir/local_s3.sh" serve \
      >"$log_file" 2>&1 &
    pid=$!
    server_owned=true

    attempt=1
    while [ "$attempt" -le 120 ]; do
      if ! kill -0 "$pid" >/dev/null 2>&1; then
        printf '[local-s3] rclone exited before it was ready\n' >&2
        sed -n '1,160p' "$log_file" >&2 || true
        return 1
      fi
      if control_ready && s3_ready; then
        break
      fi
      sleep 0.25
      attempt=$((attempt + 1))
    done
    if ! control_ready || ! s3_ready; then
      printf '[local-s3] timed out while starting rclone\n' >&2
      sed -n '1,160p' "$log_file" >&2 || true
      return 1
    fi
    printf '[local-s3] serving %s at %s\n' "$serve_root" "$endpoint"
  fi

  make -C "$repo_root" build-backend-dev
  FB_FILEBROWSER_BIN="${FB_FILEBROWSER_BIN:-$repo_root/filebrowser}" \
    "$repo_root/init.sh" --add-shares my-share=/public

  printf '[local-dev] ready\n'
  if [ -n "$pid" ]; then
    if wait "$pid"; then
      status=0
    else
      status=$?
    fi
    if [ "$status" -eq 130 ] || [ "$status" -eq 143 ]; then
      return 0
    fi
    return "$status"
  fi

  while control_ready && s3_ready; do
    sleep 1
  done
  return 0
}

if [ "${1:-}" = "run" ]; then
  trap cleanup EXIT INT TERM
  run_local_environment
else
  case "${1:-}" in
    serve) serve_s3 ;;
    stop) stop_server ;;
    *)
      usage
      exit 2
      ;;
  esac
fi
