#!/usr/bin/env bash

set -euo pipefail

RCLONE_BIN="${RCLONE_BIN:-rclone}"

required_commands=(
  cc
  cmp
  curl
  go
  node
  pnpm
  python3
  "${RCLONE_BIN}"
)
missing_commands=()

for command_name in "${required_commands[@]}"; do
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    missing_commands+=("${command_name}")
  fi
done

if ! command -v stac-check >/dev/null 2>&1 &&
  ! command -v uv >/dev/null 2>&1; then
  missing_commands+=("stac-check or uv")
fi

if ((${#missing_commands[@]} > 0)); then
  printf 'Missing tools required by make test:\n' >&2
  printf '  - %s\n' "${missing_commands[@]}" >&2
  exit 1
fi

printf 'All tools required by make test are available.\n'
