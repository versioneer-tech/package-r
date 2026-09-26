#!/usr/bin/env bash

set -euo pipefail

INTEGRATION_DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${INTEGRATION_DIR}/../.." && pwd)"
FIXTURE_DIR="${REPO_ROOT}/tests/data"
RCLONE_BIN="${RCLONE_BIN:-rclone}"

readonly ACCESS_KEY_ID=my-access-key
readonly SECRET_ACCESS_KEY=my-secret-key
readonly BUCKET=my-bucket
readonly SHARE_NAME=my-share
readonly SHARE_PASSWORD=1234
readonly SHARED_PREFIX=catalog-sample
readonly ITEM_ID=67793f0b9478720001790586
readonly CURL_CONNECT_TIMEOUT_SECONDS=2
readonly CURL_MAX_TIME_SECONDS=30

tmp_dir=""
rclone_pid=""
package_r_pid=""

log() {
  printf '\n==> %s\n' "$*"
}

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

stop_process() {
  local pid="${1:-}"
  if [[ -n "${pid}" ]]; then
    kill "${pid}" >/dev/null 2>&1 || true
    wait "${pid}" >/dev/null 2>&1 || true
  fi
}

show_log() {
  local name="$1"
  local path="$2"
  if [[ -f "${path}" ]]; then
    printf '\n--- %s ---\n' "${name}" >&2
    sed -n '1,240p' "${path}" >&2 || true
  fi
}

cleanup() {
  local status=$?
  trap - EXIT INT TERM

  stop_process "${package_r_pid}"
  stop_process "${rclone_pid}"

  if ((status != 0)) && [[ -n "${tmp_dir}" ]]; then
    show_log "packageR log" "${tmp_dir}/package-r.log"
    show_log "rclone log" "${tmp_dir}/rclone.log"
    show_log "bootstrap log" "${tmp_dir}/init.log"
  fi

  if [[ -n "${tmp_dir}" && "${PACKAGE_R_KEEP_TEST_DATA:-false}" == "true" ]]; then
    log "Kept integration data at ${tmp_dir}"
  elif [[ -n "${tmp_dir}" ]]; then
    rm -rf "${tmp_dir}"
  fi

  exit "${status}"
}
trap cleanup EXIT INT TERM

wait_for_http() {
  local url="$1"
  local name="$2"
  local log_file="$3"
  local pid="$4"
  local expected_status="$5"
  local status="000"

	local attempt
	for ((attempt = 1; attempt <= 120; attempt++)); do
	  status="$(curl --connect-timeout "${CURL_CONNECT_TIMEOUT_SECONDS}" --max-time 2 \
		-sS -o /dev/null -w '%{http_code}' "${url}" 2>/dev/null || true)"
    if [[ "${expected_status}" == "any" && "${status}" != "000" ]] ||
      [[ "${status}" == "${expected_status}" ]]; then
      return 0
    fi
    if ! kill -0 "${pid}" >/dev/null 2>&1; then
      printf '%s exited before it became ready.\n' "${name}" >&2
      show_log "${name} log" "${log_file}"
      return 1
    fi
    sleep 0.25
  done

  printf 'Timed out waiting for %s at %s; last HTTP status: %s\n' \
    "${name}" "${url}" "${status}" >&2
  show_log "${name} log" "${log_file}"
  return 1
}

curl_test() {
  curl --connect-timeout "${CURL_CONNECT_TIMEOUT_SECONDS}" \
	--max-time "${CURL_MAX_TIME_SECONDS}" "$@"
}

rclone_version_line() {
  local output
  output="$("${RCLONE_BIN}" version 2>/dev/null || true)"
  printf '%s\n' "${output%%$'\n'*}"
}

require_compatible_rclone() {
  local line major minor
  line="$(rclone_version_line)"
  if [[ "${line}" =~ ^rclone[[:space:]]v([0-9]+)\.([0-9]+) ]]; then
	major="${BASH_REMATCH[1]}"
	minor="${BASH_REMATCH[2]}"
	if ((major > 1 || (major == 1 && minor >= 74))); then
	  log "Using ${line}"
	  return 0
	fi
  fi

  printf 'rclone 1.74 or newer is required for query-presigned URLs; found: %s\n' \
	"${line:-unknown version}" >&2
  return 1
}

start_rclone() {
  local attempt
  for ((attempt = 1; attempt <= 3; attempt++)); do
	rclone_port="$(free_port)"
	rclone_url="http://127.0.0.1:${rclone_port}"
	log "Starting rclone S3 on ${rclone_url}"
	env -i \
	  HOME="${tmp_dir}/home" \
	  PATH="${PATH}" \
	  "${RCLONE_BIN}" serve s3 "${tmp_dir}/rclone" \
	  --addr "127.0.0.1:${rclone_port}" \
	  --auth-key "${ACCESS_KEY_ID},${SECRET_ACCESS_KEY}" \
	  --config /dev/null \
	  >"${tmp_dir}/rclone.log" 2>&1 &
	rclone_pid=$!

	if wait_for_http "${rclone_url}" "rclone" "${tmp_dir}/rclone.log" "${rclone_pid}" any; then
	  return 0
	fi
	stop_process "${rclone_pid}"
	rclone_pid=""
	if ((attempt < 3)); then
	  log "Retrying rclone with a new loopback port (${attempt}/3)"
	fi
  done

  return 1
}

json_field() {
  local field="$1"
  python3 -c 'import json, sys; print(json.load(sys.stdin)[sys.argv[1]])' "${field}"
}

fetch_and_compare() {
  local url="$1"
  local expected="$2"
  local output="$3"
	shift 3

	curl_test -fsSL "$@" "${url}" --output "${output}"
  cmp "${expected}" "${output}"
}

for command_name in cmp curl go python3 stac-check; do
  require_command "${command_name}"
done
require_command "${RCLONE_BIN}"
require_compatible_rclone

tmp_dir="$(mktemp -d)"
mkdir -p "${tmp_dir}/home" "${tmp_dir}/rclone/${BUCKET}"
cp -R "${FIXTURE_DIR}/." "${tmp_dir}/rclone/${BUCKET}/"

start_rclone

package_r_port="$(free_port)"
while [[ "${package_r_port}" == "${rclone_port}" ]]; do
  package_r_port="$(free_port)"
done
package_r_url="http://127.0.0.1:${package_r_port}"

log "Building the Go backend"
(
  cd "${REPO_ROOT}"
  CGO_ENABLED=1 go build -tags dev -o "${tmp_dir}/package-r" .
)

common_env=(
  "HOME=${tmp_dir}/home"
  "PATH=${PATH}"
  "RCLONE_CONFIG=/dev/null"
  "PACKAGE_R_BIN=${tmp_dir}/package-r"
  "PACKAGE_R_DATABASE=${tmp_dir}/package-r.db"
  "PACKAGE_R_ROOT=/"
  "PACKAGE_R_SERVER_PORT=${package_r_port}"
  "PACKAGE_R_AUTH_METHOD=proxy"
  "PACKAGE_R_AUTH_HEADER=X-Username"
  "PACKAGE_R_ALLOW_CHANGING=true"
  "PACKAGE_R_DEFAULT_SHARES=${SHARE_NAME}=/${BUCKET}/${SHARED_PREFIX}"
  "PACKAGE_R_DEFAULT_SHARE_PASSWORDS=${SHARE_NAME}=${SHARE_PASSWORD}"
  "PACKAGE_R_PASSWORD=my-password"
  "AWS_ACCESS_KEY_ID=${ACCESS_KEY_ID}"
  "AWS_SECRET_ACCESS_KEY=${SECRET_ACCESS_KEY}"
  "AWS_ENDPOINT_URL=${rclone_url}"
  "AWS_REGION=us-east-1"
)

log "Bootstrapping isolated packageR state"
env -i "${common_env[@]}" \
  "${REPO_ROOT}/init.sh" \
  >"${tmp_dir}/init.log" 2>&1

log "Starting packageR on ${package_r_url}"
env -i "${common_env[@]}" \
  "${tmp_dir}/package-r" -a 127.0.0.1 -p "${package_r_port}" \
  >"${tmp_dir}/package-r.log" 2>&1 &
package_r_pid=$!
wait_for_http "${package_r_url}/health" "packageR" "${tmp_dir}/package-r.log" "${package_r_pid}" 200

token="$(curl_test -fsS -H 'X-Username: admin' "${package_r_url}/api/login")"
auth_header="X-Auth: ${token}"
share_password_header="X-SHARE-PASSWORD: ${SHARE_PASSWORD}"
thumbnail="${FIXTURE_DIR}/${SHARED_PREFIX}/openaerialmap-assets/${ITEM_ID}/thumbnail.png"
resource_path="/${BUCKET}/${SHARED_PREFIX}/openaerialmap-assets/${ITEM_ID}/thumbnail.png"
public_path="openaerialmap-assets/${ITEM_ID}/thumbnail.png"

log "Checking VFS service-root browse and raw read"
curl_test -fsS -H "${auth_header}" "${package_r_url}/api/resources/" |
  python3 -c 'import json, sys; names = {item["name"] for item in json.load(sys.stdin)["items"]}; assert "my-bucket" in names'
curl_test -fsS -H "${auth_header}" "${package_r_url}/api/resources/${BUCKET}/" |
  python3 -c 'import json, sys; names = {item["name"] for item in json.load(sys.stdin)["items"]}; assert {"catalog-sample", "sample.jpg", "sample.json", "sample.pdf", "sample.txt"} <= names'
curl_test -fsS -H "${auth_header}" "${package_r_url}/api/resources/${BUCKET}/${SHARED_PREFIX}/" |
  python3 -c 'import json, sys; names = {item["name"] for item in json.load(sys.stdin)["items"]}; assert {"catalog.parquet", "openaerialmap-assets"} <= names'
curl_test -fsS -H "${auth_header}" \
  "${package_r_url}/api/raw${resource_path}" \
  --output "${tmp_dir}/raw-thumbnail.png"
cmp "${thumbnail}" "${tmp_dir}/raw-thumbnail.png"

log "Checking VFS create, copy, rename, read, and delete"
printf 'packageR rclone integration\n' >"${tmp_dir}/payload.txt"
curl_test -fsS -X POST -H "${auth_header}" \
  "${package_r_url}/api/resources/${BUCKET}/xyz/" >/dev/null
curl_test -fsS -X POST -H "${auth_header}" \
  --data-binary "@${tmp_dir}/payload.txt" \
  "${package_r_url}/api/resources/${BUCKET}/xyz/created.txt" >/dev/null
curl_test -fsS -X PATCH -H "${auth_header}" \
  "${package_r_url}/api/resources/${BUCKET}/xyz/created.txt?action=copy&destination=%2F${BUCKET}%2Fxyz%2Fcopied.txt" >/dev/null
curl_test -fsS -X PATCH -H "${auth_header}" \
  "${package_r_url}/api/resources/${BUCKET}/xyz/copied.txt?action=rename&destination=%2F${BUCKET}%2Fxyz%2Frenamed.txt" >/dev/null
renamed_url="$(
	curl_test -fsS -H "${auth_header}" \
    "${package_r_url}/api/resources/${BUCKET}/xyz/renamed.txt?presign=true" |
    json_field presignedURL
)"
fetch_and_compare "${renamed_url}" "${tmp_dir}/payload.txt" "${tmp_dir}/renamed.txt"
backing_dir="${tmp_dir}/rclone/${BUCKET}/xyz"
cmp "${tmp_dir}/payload.txt" "${backing_dir}/created.txt"
cmp "${tmp_dir}/payload.txt" "${backing_dir}/renamed.txt"
[[ ! -e "${backing_dir}/copied.txt" ]]

curl_test -fsS -X DELETE -H "${auth_header}" \
  "${package_r_url}/api/resources/${BUCKET}/xyz/created.txt" >/dev/null
curl_test -fsS -X DELETE -H "${auth_header}" \
  "${package_r_url}/api/resources/${BUCKET}/xyz/renamed.txt" >/dev/null
curl_test -fsS -X DELETE -H "${auth_header}" \
  "${package_r_url}/api/resources/${BUCKET}/xyz/" >/dev/null
missing_status="$(curl_test -sS -o /dev/null -w '%{http_code}' -H "${auth_header}" \
  "${package_r_url}/api/resources/${BUCKET}/xyz/renamed.txt")"
[[ "${missing_status}" == "404" ]]
[[ ! -e "${backing_dir}/created.txt" ]]
[[ ! -e "${backing_dir}/renamed.txt" ]]

log "Checking two-chunk TUS upload through the VFS write cache"
tus_path="/${BUCKET}/xyz/chunked.txt"
curl_test -fsS -X POST -H "${auth_header}" \
  "${package_r_url}/api/tus${tus_path}" >/dev/null
printf 'first chunk\n' >"${tmp_dir}/chunk-one.txt"
printf 'second chunk\n' >"${tmp_dir}/chunk-two.txt"
curl_test -fsS -X PATCH -H "${auth_header}" \
  -H 'Content-Type: application/offset+octet-stream' \
  -H 'Upload-Offset: 0' \
  --data-binary "@${tmp_dir}/chunk-one.txt" \
  "${package_r_url}/api/tus${tus_path}" >/dev/null
first_chunk_size="$(wc -c <"${tmp_dir}/chunk-one.txt" | tr -d ' ')"
curl_test -fsS -X PATCH -H "${auth_header}" \
  -H 'Content-Type: application/offset+octet-stream' \
  -H "Upload-Offset: ${first_chunk_size}" \
  --data-binary "@${tmp_dir}/chunk-two.txt" \
  "${package_r_url}/api/tus${tus_path}" >/dev/null
cat "${tmp_dir}/chunk-one.txt" "${tmp_dir}/chunk-two.txt" >"${tmp_dir}/chunked-expected.txt"
curl_test -fsS -H "${auth_header}" \
  "${package_r_url}/api/raw${tus_path}" \
  --output "${tmp_dir}/chunked-actual.txt"
cmp "${tmp_dir}/chunked-expected.txt" "${tmp_dir}/chunked-actual.txt"
curl_test -fsS -X DELETE -H "${auth_header}" \
  "${package_r_url}/api/resources${tus_path}" >/dev/null
curl_test -fsS -X DELETE -H "${auth_header}" \
  "${package_r_url}/api/resources/${BUCKET}/xyz/" >/dev/null

log "Checking authenticated and public rclone presigned URLs"
share_create_status="$(curl_test -sS -o /dev/null -w '%{http_code}' \
  -X POST -H "${auth_header}" "${package_r_url}/api/share/${BUCKET}/${SHARED_PREFIX}")"
[[ "${share_create_status}" == "404" ]]
authenticated_url="$(
	curl_test -fsS -H "${auth_header}" \
    "${package_r_url}/api/resources${resource_path}?presign=true" |
    json_field presignedURL
)"
fetch_and_compare "${authenticated_url}" "${thumbnail}" "${tmp_dir}/authenticated-thumbnail.png"

pin_required_status="$(curl_test -sS -o /dev/null -w '%{http_code}' \
  "${package_r_url}/api/public/share/${SHARE_NAME}/${public_path}")"
[[ "${pin_required_status}" == "401" ]]
public_url="$(
	curl_test -fsS -H "${share_password_header}" \
    "${package_r_url}/api/public/share/${SHARE_NAME}/${public_path}?presign=true" |
    json_field presignedURL
)"
fetch_and_compare "${public_url}" "${thumbnail}" "${tmp_dir}/share-thumbnail.png"

redirect_status="$(curl_test -sS -o /dev/null -w '%{http_code}' \
  -H "${share_password_header}" \
  "${package_r_url}/api/public/share/${SHARE_NAME}/${public_path}?presign=true&followRedirect=true")"
[[ "${redirect_status}" == "307" ]]
fetch_and_compare \
  "${package_r_url}/api/public/share/${SHARE_NAME}/${public_path}?presign=true&followRedirect=true" \
  "${thumbnail}" \
  "${tmp_dir}/share-redirect-thumbnail.png" \
  -H "${share_password_header}"

log "Checking public catalog access"
catalog_url="${package_r_url}/api/public/catalog/${SHARE_NAME}"
catalog_content_type="$(curl_test -fsS -H "${share_password_header}" \
  --write-out '%{content_type}' \
  "${catalog_url}" \
  --output "${tmp_dir}/catalog.json")"
[[ "${catalog_content_type}" == application/geo+json* ]]
stac-check "${catalog_url}" \
  --item-collection \
  --links \
  --no-assets-urls \
  --header X-SHARE-PASSWORD "${SHARE_PASSWORD}"
catalog_asset_url="$(python3 -c '
import json, sys
value = json.load(open(sys.argv[1], encoding="utf-8"))
assert value["type"] == "FeatureCollection" and value["features"]
for feature in value["features"]:
    asset = feature.get("assets", {}).get("thumbnail")
    if asset:
        print(asset["href"])
        break
else:
    raise AssertionError("catalog has no thumbnail asset")
' "${tmp_dir}/catalog.json")"
fetch_and_compare "${catalog_asset_url}" "${thumbnail}" "${tmp_dir}/catalog-thumbnail.png" \
  -H "${share_password_header}"

log "Integration tests passed"
