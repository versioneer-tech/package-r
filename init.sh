#!/bin/sh

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

FB_DATABASE=${FB_DATABASE:-/tmp/package-r.db}
FB_ROOT=${FB_ROOT:-/}
FB_SERVER_PORT=${FB_SERVER_PORT:-8888}
FB_FILEBROWSER_BIN=${FB_FILEBROWSER_BIN:-$script_dir/filebrowser}
FB_CREATE_USER_DIR=${FB_CREATE_USER_DIR:-false}
FB_AUTH_METHOD=${FB_AUTH_METHOD:-proxy}
FB_AUTH_HEADER=${FB_AUTH_HEADER:-X-Username}
FB_AUTH_MAPPER=${FB_AUTH_MAPPER:-}
FB_AUTH_JWT_JWKS_URL=${FB_AUTH_JWT_JWKS_URL:-}
FB_AUTH_JWT_ISSUER=${FB_AUTH_JWT_ISSUER:-}
FB_AUTH_JWT_AUDIENCE=${FB_AUTH_JWT_AUDIENCE:-}
FB_AUTH_JWT_ALGORITHMS=${FB_AUTH_JWT_ALGORITHMS:-}
FB_AUTH_JWT_CLOCK_SKEW=${FB_AUTH_JWT_CLOCK_SKEW:-}
FB_BRANDING_NAME=${FB_BRANDING_NAME:-packageR}
FB_SHARELINK_DEFAULT_HASH=${FB_SHARELINK_DEFAULT_HASH:-public-<random>-v1}
FB_CATALOG_DEFAULT_NAME=${FB_CATALOG_DEFAULT_NAME:-catalog.parquet}
FB_CATALOG_PREVIEW_URL=${FB_CATALOG_PREVIEW_URL:-}
FB_CATALOG_ASSET_MAPPINGS=${FB_CATALOG_ASSET_MAPPINGS:-}
FB_ALLOW_CHANGING=${FB_ALLOW_CHANGING:-false}
FB_DEFAULT_SHARES=${FB_DEFAULT_SHARES:-}
FB_DEFAULT_SHARE_PINS=${FB_DEFAULT_SHARE_PINS:-}
FB_PASSWORD=${FB_PASSWORD:-}
export FB_DATABASE FB_ROOT FB_SERVER_PORT FB_FILEBROWSER_BIN
export FB_CREATE_USER_DIR FB_AUTH_METHOD FB_AUTH_HEADER FB_AUTH_MAPPER
export FB_AUTH_JWT_JWKS_URL FB_AUTH_JWT_ISSUER FB_AUTH_JWT_AUDIENCE
export FB_AUTH_JWT_ALGORITHMS FB_AUTH_JWT_CLOCK_SKEW
export FB_BRANDING_NAME FB_SHARELINK_DEFAULT_HASH
export FB_CATALOG_DEFAULT_NAME FB_CATALOG_PREVIEW_URL FB_CATALOG_ASSET_MAPPINGS
export FB_ALLOW_CHANGING FB_DEFAULT_SHARES FB_DEFAULT_SHARE_PINS FB_PASSWORD

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
  --add-share-pins HASH=PIN[;HASH=PIN]
              Protect configured shares in addition to FB_DEFAULT_SHARE_PINS.
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

append_share_pins() {
  value=$1
  if [ -z "$value" ]; then
    return
  fi
  if [ -z "$extra_default_share_pins" ]; then
    extra_default_share_pins=$value
  else
    extra_default_share_pins="${extra_default_share_pins};${value}"
  fi
}

default_share_pin() {
  target_hash=$1
  pin_entries=$2

  printf '%s\n' "$pin_entries" | tr ';' '\n' | while IFS= read -r entry || [ -n "$entry" ]; do
    [ -z "$entry" ] && continue
    pin_hash=${entry%%=*}
    pin=${entry#*=}
    if [ -z "$pin_hash" ] || [ -z "$pin" ] || [ "$pin_hash" = "$entry" ]; then
      warn "Skipping invalid FB_DEFAULT_SHARE_PINS entry"
      continue
    fi
    if [ "$pin_hash" = "$target_hash" ]; then
      printf '%s' "$pin"
      return 0
    fi
  done
}

log_object_storage_mode() {
  if [ -z "${AWS_ACCESS_KEY_ID:-}" ] || [ -z "${AWS_SECRET_ACCESS_KEY:-}" ]; then
    warn "Object-storage settings are incomplete; file access needs process S3 settings"
  else
    log "rclone VFS object-storage settings detected"
  fi
}

validate_object_storage_root() {
  case "$FB_ROOT" in
    /)
      if [ "$FB_CREATE_USER_DIR" = "true" ]; then
        warn "FB_CREATE_USER_DIR=true requires FB_ROOT to name one S3 bucket; FB_ROOT=/ exposes the S3 service root"
        return 1
      fi
      ;;
    ""|.|..|*/*)
      warn "FB_ROOT must be / or one S3 bucket name without /"
      return 1
      ;;
  esac
}

print_filebrowser_banner() {
  log "================================================================"
  log "Start via ./filebrowser -d $FB_DATABASE -p $FB_SERVER_PORT"
  log "================================================================"
}

check_filebrowser_binary() {
  if "$FB_FILEBROWSER_BIN" version > /dev/null; then
    return 0
  fi

  warn "Filebrowser binary failed to start; cannot continue"
  return 1
}

filebrowser_config_set_supports() {
  flag=$1
  help_output=$("$FB_FILEBROWSER_BIN" config set --help 2>/dev/null || true)
  case "$help_output" in
    *"$flag"*) return 0 ;;
    *) return 1 ;;
  esac
}

jwt_config_requested() {
  [ -n "$FB_AUTH_JWT_JWKS_URL" ] ||
    [ -n "$FB_AUTH_JWT_ISSUER" ] ||
    [ -n "$FB_AUTH_JWT_AUDIENCE" ] ||
    [ -n "$FB_AUTH_JWT_ALGORITHMS" ] ||
    [ -n "$FB_AUTH_JWT_CLOCK_SKEW" ]
}

user_exists() {
  username=$1
  "$FB_FILEBROWSER_BIN" users find "$username" > /dev/null 2>&1
}

ensure_user() {
  username=$1
  shift

  log "Ensuring default user exists: $username"
  command_output=$("$FB_FILEBROWSER_BIN" users add "$username" "${FB_PASSWORD:-$password}" "$@" 2>&1)
  command_status=$?
  if user_exists "$username"; then
    log "Default user ready: $username"
    return 0
  fi

  if [ "$command_status" -eq 0 ]; then
    warn "Default user bootstrap completed but user was not found: $username"
    return 1
  fi

  warn "Failed to bootstrap default user $username: $command_output"
  return "$command_status"
}

extra_default_shares=
extra_default_share_pins=
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
    --add-share-pins)
      shift
      if [ "$#" -eq 0 ]; then
        warn "Missing value for --add-share-pins"
        usage >&2
        exit 2
      fi
      append_share_pins "$1"
      ;;
    --add-share-pins=*)
      append_share_pins "${1#--add-share-pins=}"
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

if ! validate_object_storage_root; then
  exit 1
fi

if ! mkdir -p "$(dirname -- "$FB_DATABASE")"; then
  warn "Failed to prepare the FB_DATABASE directory; cannot continue"
  exit 1
fi

log "Starting bootstrap"

if ! check_filebrowser_binary; then
  exit 1
fi

log "Using $FB_DATABASE for state"
log "Using $FB_ROOT as the S3 root"

log_object_storage_mode

if "$FB_FILEBROWSER_BIN" config init > /dev/null 2>&1; then
  log "Initialized filebrowser database"
else
  log "Filebrowser database already exists; continuing with configuration update"
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
  --catalog.assetMappings "$FB_CATALOG_ASSET_MAPPINGS" \
  --scope "/" \
  --perm.admin=false \
  --perm.create=$ALLOW_CHANGING \
  --perm.delete=$ALLOW_CHANGING \
  --perm.download=false \
  --perm.execute=false \
  --perm.modify=$ALLOW_CHANGING \
  --perm.rename=$ALLOW_CHANGING \
  --perm.share=false \
  --lockPassword=true \
  --commands ""

if filebrowser_config_set_supports "--auth.jwt.jwks-url"; then
  set -- "$@" \
    --auth.jwt.jwks-url="$FB_AUTH_JWT_JWKS_URL" \
    --auth.jwt.issuer="$FB_AUTH_JWT_ISSUER" \
    --auth.jwt.audience="$FB_AUTH_JWT_AUDIENCE" \
    --auth.jwt.algorithms="$FB_AUTH_JWT_ALGORITHMS" \
    --auth.jwt.clock-skew="$FB_AUTH_JWT_CLOCK_SKEW"
elif jwt_config_requested; then
  warn "JWT proxy auth environment variables require a filebrowser binary that supports --auth.jwt.jwks-url"
  exit 1
fi

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
  --perm.share=false \
  --perm.download=true \
  --lockPassword || exit 1

default_share_owner=admin

default_shares=$FB_DEFAULT_SHARES
if [ -n "$extra_default_shares" ]; then
  if [ -n "$default_shares" ]; then
    default_shares="${default_shares};${extra_default_shares}"
  else
    default_shares=$extra_default_shares
  fi
fi

default_share_pins=$FB_DEFAULT_SHARE_PINS
if [ -n "$extra_default_share_pins" ]; then
  if [ -n "$default_share_pins" ]; then
    default_share_pins="${default_share_pins};${extra_default_share_pins}"
  else
    default_share_pins=$extra_default_share_pins
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
    pin=$(default_share_pin "$hash" "$default_share_pins")
    if FB_SHARE_PIN=$pin "$FB_FILEBROWSER_BIN" shares add "$default_share_owner" "$hash" "$path" > /dev/null; then
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
