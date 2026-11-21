#!/usr/bin/env bash

set -euo pipefail

RCLONE_BIN="${RCLONE_BIN:-rclone}"

required_commands=(
  c++
  cc
  cmp
  curl
  getconf
  go
  jq
  node
  pnpm
  python3
  strings
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

version_at_least() {
  [[ "$1" =~ ^[0-9]+(\.[0-9]+)+$ ]] &&
    printf '%s\n%s\n' "$2" "$1" | sort -VC
}

glibc_version="$(getconf GNU_LIBC_VERSION 2>/dev/null || true)"
glibc_version="${glibc_version#glibc }"
libstdcxx_path="$(c++ -print-file-name=libstdc++.so.6)"
libstdcxx_version=
if [[ -r "${libstdcxx_path}" ]]; then
  libstdcxx_version="$(strings "${libstdcxx_path}" | sed -n 's/^GLIBCXX_\([0-9.]*\)$/\1/p' | sort -V | tail -n 1)"
fi

incompatible_libraries=false
if ! version_at_least "${glibc_version}" 2.38; then
  printf 'make test requires glibc 2.38 or newer (found %s).\n' "${glibc_version:-unavailable}" >&2
  incompatible_libraries=true
fi
if ! version_at_least "${libstdcxx_version}" 3.4.32; then
  printf 'make test requires libstdc++ with GLIBCXX_3.4.32 or newer (found %s).\n' "${libstdcxx_version:+GLIBCXX_}${libstdcxx_version:-unavailable}" >&2
  incompatible_libraries=true
fi
if [[ "${incompatible_libraries}" == true ]]; then
  printf 'Use compatible libraries or the pinned Playwright CI container; see tests/README.md.\n' >&2
  exit 1
fi

tests_dir="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
"${tests_dir}/../scripts/check-rclone.sh" "${RCLONE_BIN}"
node "${tests_dir}/../frontend/scripts/check-playwright.mjs"
printf 'All tools required by make test are available.\n'
