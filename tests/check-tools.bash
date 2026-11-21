#!/usr/bin/env bash

set -euo pipefail

RCLONE_BIN="${RCLONE_BIN:-rclone}"

required_commands=(
  cc
  cmp
  curl
  go
  jq
  node
  pnpm
  python3
  uv
  "${RCLONE_BIN}"
)
missing_commands=()

for command_name in "${required_commands[@]}"; do
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    missing_commands+=("${command_name}")
  fi
done

if ((${#missing_commands[@]} > 0)); then
  printf 'Missing tools required by make test:\n' >&2
  printf '  - %s\n' "${missing_commands[@]}" >&2
  exit 1
fi

tests_dir="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
"${tests_dir}/../scripts/check-rclone.sh" "${RCLONE_BIN}"
node "${tests_dir}/../frontend/scripts/check-playwright.mjs"
printf 'All tools required by make test are available.\n'
