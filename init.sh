#!/bin/sh

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

FB_DATABASE=${FB_DATABASE:-/db/bolt.db}
FB_ROOT=${FB_ROOT:-/workspace}
FB_SERVER_PORT=${FB_SERVER_PORT:-8888}
FB_FILEBROWSER_BIN=${FB_FILEBROWSER_BIN:-$script_dir/filebrowser}
FB_CREATE_USER_DIR=${FB_CREATE_USER_DIR:-true}
FB_AUTH_METHOD=${FB_AUTH_METHOD:-proxy}
FB_AUTH_HEADER=${FB_AUTH_HEADER:-X-Username}
FB_AUTH_MAPPER=${FB_AUTH_MAPPER:-}
FB_BRANDING_NAME=${FB_BRANDING_NAME:-packageR}
FB_SHARELINK_DEFAULT_HASH=${FB_SHARELINK_DEFAULT_HASH:-public-<random>-v1}
FB_CATALOG_DEFAULT_NAME=${FB_CATALOG_DEFAULT_NAME:-catalog.parquet}
FB_CATALOG_PREVIEW_URL=${FB_CATALOG_PREVIEW_URL:-}
FB_ALLOW_SHARING=${FB_ALLOW_SHARING:-false}
FB_ALLOW_CHANGING=${FB_ALLOW_CHANGING:-false}
FB_DEFAULT_SHARES=${FB_DEFAULT_SHARES:-}
FB_PASSWORD=${FB_PASSWORD:-}
export FB_DATABASE FB_ROOT FB_SERVER_PORT FB_FILEBROWSER_BIN
export FB_CREATE_USER_DIR FB_AUTH_METHOD FB_AUTH_HEADER FB_AUTH_MAPPER
export FB_BRANDING_NAME FB_SHARELINK_DEFAULT_HASH
export FB_CATALOG_DEFAULT_NAME FB_CATALOG_PREVIEW_URL
export FB_ALLOW_SHARING FB_ALLOW_CHANGING FB_DEFAULT_SHARES FB_PASSWORD

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
    log "- authenticated resources: ${FB_ADDRESS:-127.0.0.1}:$FB_SERVER_PORT/api/raw/<path>"
    log "- public shares:           ${FB_ADDRESS:-127.0.0.1}:$FB_SERVER_PORT/api/public/dl/<share-hash>/<path>"
    log "Configure AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY for S3-compatible presigned URLs."
    log "================================================================"
  else
    log "S3-compatible presign credentials detected"
  fi
}

print_filebrowser_banner() {
  log "================================================================"
  log "Start via ./filebrowser -d $FB_DATABASE -p $FB_SERVER_PORT"
  log "================================================================"
}

ensure_user() {
  username=$1
  shift

  log "Ensuring default user exists: $username"
  if "$FB_FILEBROWSER_BIN" users add "$username" "${FB_PASSWORD:-$password}" "$@" > /dev/null 2>&1; then
    log "Default user ready: $username"
  else
    log "Skipping default user bootstrap for $username; it may already exist"
  fi
}

copy_test_data() {
  target=$1
  source=$script_dir/tests/data
  root=$FB_ROOT

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

if ! mkdir -p "$FB_ROOT" "$(dirname -- "$FB_DATABASE")"; then
  warn "Failed to prepare FB_ROOT or FB_DATABASE directory; cannot continue"
  exit 1
fi

log "Starting bootstrap"

log "Using $FB_DATABASE for state"
log "Using $FB_ROOT as filebrowser root"

envs=\
"AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID:-},"\
"AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY:-},"\
"AWS_ENDPOINT_URL=${AWS_ENDPOINT_URL:-},"\
"AWS_REGION=${AWS_REGION:-},"\
"BUCKET_NAME=${BUCKET_NAME:-},"\
"BUCKET_PREFIX=${BUCKET_PREFIX:-}"

log_presign_mode

if "$FB_FILEBROWSER_BIN" config init > /dev/null 2>&1; then
  log "Initialized filebrowser database"
else
  log "Filebrowser database already exists; continuing with configuration update"
fi

ALLOW_SHARING=false
if [ "$FB_ALLOW_SHARING" = "true" ]; then
  ALLOW_SHARING=true
  log "Sharing allowed"
fi

ALLOW_CHANGING=false
if [ "$FB_ALLOW_CHANGING" = "true" ]; then
  ALLOW_CHANGING=true
  log "Changing allowed"
fi

set -- config set \
  --address "" \
  --root "$FB_ROOT" \
  --disable-preview-resize \
  --disable-thumbnails \
  --disable-type-detection-by-header \
  --signup=true \
  --create-user-dir="$FB_CREATE_USER_DIR" \
  --auth.method="$FB_AUTH_METHOD" \
  --auth.header="$FB_AUTH_HEADER" \
  --auth.mapper="$FB_AUTH_MAPPER" \
  --branding.name "$FB_BRANDING_NAME" \
  --sharelink.defaultHash "$FB_SHARELINK_DEFAULT_HASH" \
  --catalog.defaultName "$FB_CATALOG_DEFAULT_NAME" \
  --catalog.previewURL "$FB_CATALOG_PREVIEW_URL" \
  --scope "/" \
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
  --commands ""

log "Applying filebrowser configuration"
if "$FB_FILEBROWSER_BIN" "$@" > /dev/null; then
  log "Filebrowser configuration applied"
else
  warn "Failed to apply filebrowser configuration; cannot continue"
  exit 1
fi

password=$(head /dev/urandom | tr -dc 'A-Za-z0-9' | head -c 16)

if [ -n "$FB_PASSWORD" ]; then
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

default_shares=$FB_DEFAULT_SHARES
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
    if "$FB_FILEBROWSER_BIN" shares add "$default_share_owner" "$hash" "$path" > /dev/null; then
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
if "$FB_FILEBROWSER_BIN" shares ls; then
  log "Configured shares listed"
else
  warn "Failed to list configured shares; continuing"
fi

log "Bootstrap complete"
if [ "$serve_after_bootstrap" = "true" ]; then
  log "Starting filebrowser on port $FB_SERVER_PORT"
  exec "$FB_FILEBROWSER_BIN" -p "$FB_SERVER_PORT"
else
  print_filebrowser_banner
fi
