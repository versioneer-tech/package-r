#!/bin/sh

log() {
  printf '[init] %s\n' "$*"
}

warn() {
  printf '[init] warning: %s\n' "$*" >&2
}

usage() {
  cat <<'EOF'
Usage: ./init.sh [options]

Options:
  --add-shares HASH=PATH[;HASH=PATH]
              Create one or more default shares in addition to FB_DEFAULT_SHARES.
  --add-test-data PATH
              Copy tests/data into PATH below the configured filebrowser root.
  --serve     Start filebrowser after bootstrap.
  -h, --help  Show this help.
EOF
}

append_shares() {
  value=$1
  if [ -z "$value" ]; then
    return
  fi
  if [ -z "$extra_default_shares" ]; then
    extra_default_shares=$value
  else
    extra_default_shares="${extra_default_shares};${value}"
  fi
}

append_test_data_target() {
  value=$1
  if [ -z "$value" ]; then
    return
  fi
  if [ -z "$test_data_targets" ]; then
    test_data_targets=$value
  else
    test_data_targets="${test_data_targets} ${value}"
  fi
}

log_presign_mode() {
  if [ -z "${AWS_ACCESS_KEY_ID:-}" ] || [ -z "${AWS_SECRET_ACCESS_KEY:-}" ]; then
    log "================================================================"
    log "LOCAL SETUP: AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY are not configured."
    log "Presigned URL requests will return for"
    log "- authenticated resources: ${FB_ADDRESS:-127.0.0.1}:${FB_SERVER_PORT:-8080}/api/raw/<path>"
    log "- public shares:           ${FB_ADDRESS:-127.0.0.1}:${FB_SERVER_PORT:-8080}/api/public/dl/<share-hash>/<path>"
    log "Configure AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY for S3-compatible presigned URLs."
    log "================================================================"
  else
    log "S3-compatible presign credentials detected"
  fi
}

print_filebrowser_banner() {
  log "================================================================"
  log "run via ./filebrowser -d ${FB_DATABASE} -p ${FB_SERVER_PORT:-8080}"
  log "================================================================"
}

ensure_user() {
  username=$1
  shift

  log "Ensuring default user exists: $username"
  if "$filebrowser_bin" users add "$username" "${FB_PASSWORD:-$password}" "$@" > /dev/null 2>&1; then
    log "Default user ready: $username"
  else
    log "Skipping default user bootstrap for $username; it may already exist"
  fi
}

copy_test_data() {
  target=$1
  source=$script_dir/tests/data
  root=${FB_ROOT:-./root}

  if [ ! -d "$source" ]; then
    warn "Test data source not found: $source"
    return 1
  fi

  old_ifs=$IFS
  IFS=/
  for part in $target; do
    if [ "$part" = ".." ]; then
      IFS=$old_ifs
      warn "Skipping test data target outside root: $target"
      return 1
    fi
  done
  IFS=$old_ifs

  target=${target#/}
  if [ -z "$target" ]; then
    destination=$root
  else
    destination=${root%/}/$target
  fi

  log "Copying test data into $destination"
  mkdir -p "$destination"
  cp -R -n "$source"/. "$destination"/
}

extra_default_shares=
test_data_targets=
serve_after_bootstrap=false

while [ "$#" -gt 0 ]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --add-shares)
      shift
      if [ "$#" -eq 0 ]; then
        warn "Missing value for --add-shares"
        usage >&2
        exit 2
      fi
      append_shares "$1"
      ;;
    --add-shares=*)
      append_shares "${1#--add-shares=}"
      ;;
    --add-test-data)
      shift
      if [ "$#" -eq 0 ]; then
        warn "Missing value for --add-test-data"
        usage >&2
        exit 2
      fi
      append_test_data_target "$1"
      ;;
    --add-test-data=*)
      append_test_data_target "${1#--add-test-data=}"
      ;;
    --serve)
      serve_after_bootstrap=true
      ;;
    *)
      warn "Unknown argument: $1"
      usage >&2
      exit 2
      ;;
  esac
  shift
done

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
filebrowser_bin=${FB_FILEBROWSER_BIN:-}
if [ -z "$filebrowser_bin" ]; then
  filebrowser_bin=$script_dir/filebrowser
fi

log "Starting bootstrap"

log "Using ${FB_DATABASE:-./filebrowser.db} for state"
log "Using ${FB_ROOT:-./root} as filebrowser root"

envs=\
"AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID:-},"\
"AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY:-},"\
"AWS_ENDPOINT_URL=${AWS_ENDPOINT_URL:-},"\
"AWS_REGION=${AWS_REGION:-},"\
"BUCKET_NAME=${BUCKET_NAME:-},"\
"BUCKET_PREFIX=${BUCKET_PREFIX:-}"

log_presign_mode

if "$filebrowser_bin" config init > /dev/null 2>&1; then
  log "Initialized filebrowser database"
else
  log "Filebrowser database already exists; continuing with configuration update"
fi

ALLOW_SHARING=false
if [ "${FB_ALLOW_SHARING:-false}" = "true" ]; then
  ALLOW_SHARING=true
  log "Sharing allowed"
fi

ALLOW_CHANGING=false
if [ "${FB_ALLOW_CHANGING:-false}" = "true" ]; then
  ALLOW_CHANGING=true
  log "Changing allowed"
fi

log "Applying filebrowser configuration"
if "$filebrowser_bin" config set \
  --address "" \
  --root "${FB_ROOT:-./root}" \
  --disable-preview-resize \
  --disable-thumbnails \
  --disable-type-detection-by-header \
  --signup=true \
  --create-user-dir="${FB_CREATE_USER_DIR:-true}" \
  --auth.method="${FB_AUTH_METHOD:-proxy}" \
  --auth.header="${FB_AUTH_HEADER:-X-Username}" \
  --auth.mapper="${FB_AUTH_MAPPER:-}" \
  --branding.name "${FB_BRANDING_NAME:-packageR}" \
  --branding.files "${FB_BRANDING_FILES:-/package-r}" \
  --sharelink.defaultHash "${FB_SHARELINK_DEFAULT_HASH:-public-<random>-v1}" \
  --catalog.defaultName "${FB_CATALOG_DEFAULT_NAME:-catalog.parquet}" \
  --catalog.previewURL "${FB_CATALOG_PREVIEW_URL:-}" \
  --scope "" \
  --perm.admin=false \
  --perm.create=$ALLOW_CHANGING \
  --perm.delete=$ALLOW_CHANGING \
  --perm.download=false \
  --perm.execute=false \
  --perm.modify=$ALLOW_CHANGING \
  --perm.rename=$ALLOW_CHANGING \
  --perm.share=$ALLOW_SHARING \
  --lockPassword=true \
  --envs="$envs" \
  --commands "" > /dev/null; then
  log "Filebrowser configuration applied"
else
  warn "Failed to apply filebrowser configuration; continuing"
fi

password=$(head /dev/urandom | tr -dc 'A-Za-z0-9' | head -c 16)

if [ -n "${FB_PASSWORD:-}" ]; then
  log "Using FB_PASSWORD for bootstrap users"
else
  log "Generated random password for bootstrap users"
fi

ensure_user admin \
  --scope=/ \
  --perm.admin=true \
  --perm.execute=true \
  --perm.create=true \
  --perm.rename=true \
  --perm.modify=true \
  --perm.delete=true \
  --perm.share=true \
  --perm.download=true \
  --lockPassword \
  --envs="$envs"

for target in $test_data_targets; do
  copy_test_data "$target" || true
done

default_share_owner=admin

default_shares=${FB_DEFAULT_SHARES:-}
if [ -n "$extra_default_shares" ]; then
  if [ -n "$default_shares" ]; then
    default_shares="${default_shares};${extra_default_shares}"
  else
    default_shares=$extra_default_shares
  fi
fi

if [ -n "$default_shares" ]; then
  default_share_count=$(printf '%s' "$default_shares" | tr ';' '\n' | awk 'NF { count++ } END { print count + 0 }')
  log "Processing $default_share_count default share(s) for owner $default_share_owner"

  printf '%s\n' "$default_shares" | tr ';' '\n' | while IFS= read -r share || [ -n "$share" ]; do
    [ -z "$share" ] && continue

    hash=${share%%=*}
    path=${share#*=}

    if [ -z "$hash" ] || [ -z "$path" ] || [ "$hash" = "$share" ]; then
      warn "Skipping invalid FB_DEFAULT_SHARES entry: $share"
      continue
    fi

    log "Ensuring default share exists: hash=$hash path=$path owner=$default_share_owner"
    if "$filebrowser_bin" shares add "$default_share_owner" "$hash" "$path" > /dev/null; then
      log "Default share ready: $hash -> $path"
    else
      warn "Failed to create default share: $hash -> $path"
    fi
  done
  log "Default shares processed"
else
  log "No default shares configured"
fi

log "Listing configured shares"
if "$filebrowser_bin" shares ls; then
  log "Configured shares listed"
else
  warn "Failed to list configured shares; continuing"
fi

log "Bootstrap complete"
if [ "$serve_after_bootstrap" = "true" ]; then
  log "Starting filebrowser on port ${FB_SERVER_PORT:-8080}"
  exec "$filebrowser_bin" -p "${FB_SERVER_PORT:-8080}"
else
  print_filebrowser_banner
fi
