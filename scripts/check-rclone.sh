#!/usr/bin/env bash
set -euo pipefail

rclone_bin="${1:-rclone}"
output="$("$rclone_bin" --config /dev/null version 2>/dev/null)" || output=""
line="${output%%$'\n'*}"

if [[ "$line" =~ ^rclone[[:space:]]v([0-9]+)\.([0-9]+) ]]; then
  major="${BASH_REMATCH[1]}"
  minor="${BASH_REMATCH[2]}"
  if ((major > 1 || (major == 1 && minor >= 76))); then
    printf '%s\n' "$line"
    exit 0
  fi
fi

printf 'rclone 1.76 or newer is required; found: %s\n' "${line:-unknown version}" >&2
exit 1
