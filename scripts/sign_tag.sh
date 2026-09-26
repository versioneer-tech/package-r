#!/usr/bin/env bash
set -euo pipefail

missing=()
for command_name in git gpg pnpm; do
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    missing+=("${command_name}")
  fi
done

if [[ "${#missing[@]}" -ne 0 ]]; then
  printf 'Missing required commands: %s\n' "${missing[*]}" >&2
  exit 1
fi

if [[ "$#" -ne 1 ]]; then
  printf 'Usage: %s vnext.<major>.<minor>.<patch>[-rc.<number>]\n' "$0" >&2
  exit 1
fi

readonly release_tag="$1"
readonly base_ref='refs/remotes/origin/main'

if [[ ! "${release_tag}" =~ ^vnext\.[0-9]+\.[0-9]+\.[0-9]+(-rc\.[0-9]+)?$ ]]; then
  printf 'Invalid release tag: %s\n' "${release_tag}" >&2
  exit 1
fi

if [[ "$(git branch --show-current)" != 'main' ]]; then
  printf 'Releases must be tagged from the main branch.\n' >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  printf 'Commit all changes before creating a release tag.\n' >&2
  exit 1
fi

if ! git rev-parse --verify --quiet "${base_ref}" >/dev/null; then
  printf 'Remote branch not found. Fetch origin before creating a release tag.\n' >&2
  exit 1
fi

if ! git merge-base --is-ancestor "${base_ref}" HEAD; then
  printf 'Local main has diverged from origin/main. Fetch and reconcile it first.\n' >&2
  exit 1
fi

if git rev-parse --verify --quiet "refs/tags/${release_tag}" >/dev/null; then
  printf 'Tag already exists: %s\n' "${release_tag}" >&2
  exit 1
fi

if [[ "$(git rev-list --count "${base_ref}..HEAD")" == '0' ]]; then
  printf 'No local commits to lint after origin/main.\n'
else
  pnpm dlx \
    --package @commitlint/cli \
    --package @commitlint/config-conventional \
    commitlint \
    --extends @commitlint/config-conventional \
    --from "${base_ref}" \
    --to HEAD \
    --verbose
fi

printf 'Create signed tag %s at %s?\n' \
  "${release_tag}" "$(git rev-parse --short HEAD)"
read -r -p 'Continue (y/n)? ' -n 1 reply
printf '\n'

if [[ ! "${reply}" =~ ^[Yy]$ ]]; then
  printf 'Tag creation cancelled.\n'
  exit 0
fi

git tag --sign --message "Release ${release_tag}" "${release_tag}"

printf 'Created %s locally.\n' "${release_tag}"
if [[ "$(git rev-parse HEAD)" != "$(git rev-parse "${base_ref}")" ]]; then
  printf 'Push main and wait for the Tests workflow to pass:\n'
  printf '  git push origin main\n'
fi
printf 'Then publish the tag:\n'
printf '  git push origin %s\n' "${release_tag}"
