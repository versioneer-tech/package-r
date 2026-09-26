#!/bin/sh

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

PACKAGE_R_DATABASE=${PACKAGE_R_DATABASE:-/tmp/package-r.db}
PACKAGE_R_ROOT=${PACKAGE_R_ROOT:-/}
PACKAGE_R_SERVER_PORT=${PACKAGE_R_SERVER_PORT:-8888}
PACKAGE_R_BIN=${PACKAGE_R_BIN:-$script_dir/package-r}
PACKAGE_R_CREATE_USER_DIR=${PACKAGE_R_CREATE_USER_DIR:-false}
PACKAGE_R_AUTH_METHOD=${PACKAGE_R_AUTH_METHOD:-proxy}
PACKAGE_R_AUTH_HEADER=${PACKAGE_R_AUTH_HEADER:-X-Username}
PACKAGE_R_AUTH_MAPPER=${PACKAGE_R_AUTH_MAPPER:-}
PACKAGE_R_AUTH_JWT_JWKS_URL=${PACKAGE_R_AUTH_JWT_JWKS_URL:-}
PACKAGE_R_AUTH_JWT_ISSUER=${PACKAGE_R_AUTH_JWT_ISSUER:-}
PACKAGE_R_AUTH_JWT_AUDIENCE=${PACKAGE_R_AUTH_JWT_AUDIENCE:-}
PACKAGE_R_AUTH_JWT_ALGORITHMS=${PACKAGE_R_AUTH_JWT_ALGORITHMS:-}
PACKAGE_R_AUTH_JWT_CLOCK_SKEW=${PACKAGE_R_AUTH_JWT_CLOCK_SKEW:-}
PACKAGE_R_BRANDING_NAME=${PACKAGE_R_BRANDING_NAME:-packageR}
PACKAGE_R_SHARELINK_DEFAULT_HASH=${PACKAGE_R_SHARELINK_DEFAULT_HASH:-public-<random>-v1}
PACKAGE_R_CATALOG_DEFAULT_NAME=${PACKAGE_R_CATALOG_DEFAULT_NAME:-catalog.parquet}
PACKAGE_R_CATALOG_PREVIEW_URL=${PACKAGE_R_CATALOG_PREVIEW_URL:-}
PACKAGE_R_CATALOG_ASSET_MAPPINGS=${PACKAGE_R_CATALOG_ASSET_MAPPINGS:-}
PACKAGE_R_ALLOW_CHANGING=${PACKAGE_R_ALLOW_CHANGING:-false}
PACKAGE_R_DEFAULT_SHARES=${PACKAGE_R_DEFAULT_SHARES:-}
PACKAGE_R_DEFAULT_SHARE_PASSWORDS=${PACKAGE_R_DEFAULT_SHARE_PASSWORDS:-}
PACKAGE_R_PASSWORD=${PACKAGE_R_PASSWORD:-}
export PACKAGE_R_DATABASE PACKAGE_R_ROOT PACKAGE_R_SERVER_PORT PACKAGE_R_BIN
export PACKAGE_R_CREATE_USER_DIR PACKAGE_R_AUTH_METHOD PACKAGE_R_AUTH_HEADER PACKAGE_R_AUTH_MAPPER
export PACKAGE_R_AUTH_JWT_JWKS_URL PACKAGE_R_AUTH_JWT_ISSUER PACKAGE_R_AUTH_JWT_AUDIENCE
export PACKAGE_R_AUTH_JWT_ALGORITHMS PACKAGE_R_AUTH_JWT_CLOCK_SKEW
export PACKAGE_R_BRANDING_NAME PACKAGE_R_SHARELINK_DEFAULT_HASH
export PACKAGE_R_CATALOG_DEFAULT_NAME PACKAGE_R_CATALOG_PREVIEW_URL PACKAGE_R_CATALOG_ASSET_MAPPINGS
export PACKAGE_R_ALLOW_CHANGING PACKAGE_R_DEFAULT_SHARES PACKAGE_R_DEFAULT_SHARE_PASSWORDS PACKAGE_R_PASSWORD

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
              Create one or more default shares in addition to PACKAGE_R_DEFAULT_SHARES.
  --add-share-passwords HASH=PASSWORD[;HASH=PASSWORD]
              Protect shares in addition to PACKAGE_R_DEFAULT_SHARE_PASSWORDS.
  --serve     Start packageR after bootstrap.
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

append_share_passwords() {
  value=$1
  if [ -z "$value" ]; then
    return
  fi
  if [ -z "$extra_default_share_passwords" ]; then
    extra_default_share_passwords=$value
  else
    extra_default_share_passwords="${extra_default_share_passwords};${value}"
  fi
}

default_share_password() {
  target_hash=$1
  password_entries=$2

  printf '%s\n' "$password_entries" | tr ';' '\n' | while IFS= read -r entry || [ -n "$entry" ]; do
    [ -z "$entry" ] && continue
    password_hash=${entry%%=*}
    password=${entry#*=}
    if [ -z "$password_hash" ] || [ -z "$password" ] || [ "$password_hash" = "$entry" ]; then
      warn "Skipping invalid PACKAGE_R_DEFAULT_SHARE_PASSWORDS entry"
      continue
    fi
    if [ "$password_hash" = "$target_hash" ]; then
      printf '%s' "$password"
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
  case "$PACKAGE_R_ROOT" in
    /)
      if [ "$PACKAGE_R_CREATE_USER_DIR" = "true" ]; then
        warn "PACKAGE_R_CREATE_USER_DIR=true requires PACKAGE_R_ROOT to name one S3 bucket; PACKAGE_R_ROOT=/ exposes the S3 service root"
        return 1
      fi
      ;;
    ""|.|..|*/*)
      warn "PACKAGE_R_ROOT must be / or one S3 bucket name without /"
      return 1
      ;;
  esac
}

print_package_r_banner() {
  log "================================================================"
  log "Start via ./package-r -d $PACKAGE_R_DATABASE -p $PACKAGE_R_SERVER_PORT"
  log "================================================================"
}

check_package_r_binary() {
  if "$PACKAGE_R_BIN" version > /dev/null; then
    return 0
  fi

  warn "packageR binary failed to start; cannot continue"
  return 1
}

package_r_config_set_supports() {
  flag=$1
  help_output=$("$PACKAGE_R_BIN" config set --help 2>/dev/null || true)
  case "$help_output" in
    *"$flag"*) return 0 ;;
    *) return 1 ;;
  esac
}

jwt_config_requested() {
  [ -n "$PACKAGE_R_AUTH_JWT_JWKS_URL" ] ||
    [ -n "$PACKAGE_R_AUTH_JWT_ISSUER" ] ||
    [ -n "$PACKAGE_R_AUTH_JWT_AUDIENCE" ] ||
    [ -n "$PACKAGE_R_AUTH_JWT_ALGORITHMS" ] ||
    [ -n "$PACKAGE_R_AUTH_JWT_CLOCK_SKEW" ]
}

user_exists() {
  username=$1
  "$PACKAGE_R_BIN" users find "$username" > /dev/null 2>&1
}

ensure_user() {
  username=$1
  shift

  log "Ensuring default user exists: $username"
  command_output=$("$PACKAGE_R_BIN" users add "$username" "${PACKAGE_R_PASSWORD:-$password}" "$@" 2>&1)
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
extra_default_share_passwords=
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
    --add-share-passwords)
      shift
      if [ "$#" -eq 0 ]; then
        warn "Missing value for --add-share-passwords"
        usage >&2
        exit 2
      fi
      append_share_passwords "$1"
      ;;
    --add-share-passwords=*)
      append_share_passwords "${1#--add-share-passwords=}"
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

if ! mkdir -p "$(dirname -- "$PACKAGE_R_DATABASE")"; then
  warn "Failed to prepare the PACKAGE_R_DATABASE directory; cannot continue"
  exit 1
fi

log "Starting bootstrap"

if ! check_package_r_binary; then
  exit 1
fi

log "Using $PACKAGE_R_DATABASE for state"
log "Using $PACKAGE_R_ROOT as the S3 root"

log_object_storage_mode

if "$PACKAGE_R_BIN" config init > /dev/null 2>&1; then
  log "Initialized packageR database"
else
  log "packageR database already exists; continuing with configuration update"
fi

ALLOW_CHANGING=false
if [ "$PACKAGE_R_ALLOW_CHANGING" = "true" ]; then
  ALLOW_CHANGING=true
  log "Changing allowed"
fi

set -- config set \
  --address "" \
  --root "$PACKAGE_R_ROOT" \
  --disable-preview-resize \
  --disable-thumbnails \
  --disable-type-detection-by-header \
  --signup=true \
  --create-user-dir="$PACKAGE_R_CREATE_USER_DIR" \
  --auth.method="$PACKAGE_R_AUTH_METHOD" \
  --auth.header="$PACKAGE_R_AUTH_HEADER" \
  --auth.mapper="$PACKAGE_R_AUTH_MAPPER" \
  --branding.name "$PACKAGE_R_BRANDING_NAME" \
  --sharelink.defaultHash "$PACKAGE_R_SHARELINK_DEFAULT_HASH" \
  --catalog.defaultName "$PACKAGE_R_CATALOG_DEFAULT_NAME" \
  --catalog.previewURL "$PACKAGE_R_CATALOG_PREVIEW_URL" \
  --catalog.assetMappings "$PACKAGE_R_CATALOG_ASSET_MAPPINGS" \
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

if package_r_config_set_supports "--auth.jwt.jwks-url"; then
  set -- "$@" \
    --auth.jwt.jwks-url="$PACKAGE_R_AUTH_JWT_JWKS_URL" \
    --auth.jwt.issuer="$PACKAGE_R_AUTH_JWT_ISSUER" \
    --auth.jwt.audience="$PACKAGE_R_AUTH_JWT_AUDIENCE" \
    --auth.jwt.algorithms="$PACKAGE_R_AUTH_JWT_ALGORITHMS" \
    --auth.jwt.clock-skew="$PACKAGE_R_AUTH_JWT_CLOCK_SKEW"
elif jwt_config_requested; then
  warn "JWT proxy auth environment variables require a packageR binary that supports --auth.jwt.jwks-url"
  exit 1
fi

log "Applying packageR configuration"
if "$PACKAGE_R_BIN" "$@" > /dev/null; then
  log "packageR configuration applied"
else
  warn "Failed to apply packageR configuration; cannot continue"
  exit 1
fi

password=$(head /dev/urandom | tr -dc 'A-Za-z0-9' | head -c 16)

if [ -n "$PACKAGE_R_PASSWORD" ]; then
  log "Using PACKAGE_R_PASSWORD for bootstrap users"
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

default_shares=$PACKAGE_R_DEFAULT_SHARES
if [ -n "$extra_default_shares" ]; then
  if [ -n "$default_shares" ]; then
    default_shares="${default_shares};${extra_default_shares}"
  else
    default_shares=$extra_default_shares
  fi
fi

default_share_passwords=$PACKAGE_R_DEFAULT_SHARE_PASSWORDS
if [ -n "$extra_default_share_passwords" ]; then
  if [ -n "$default_share_passwords" ]; then
    default_share_passwords="${default_share_passwords};${extra_default_share_passwords}"
  else
    default_share_passwords=$extra_default_share_passwords
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
      warn "Skipping invalid PACKAGE_R_DEFAULT_SHARES entry: $share"
      continue
    fi

    log "Ensuring default share exists: hash=$hash path=$path owner=$default_share_owner"
    share_password=$(default_share_password "$hash" "$default_share_passwords")
    if PACKAGE_R_SHARE_PASSWORD=$share_password "$PACKAGE_R_BIN" shares add "$default_share_owner" "$hash" "$path" > /dev/null; then
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
if "$PACKAGE_R_BIN" shares ls; then
  log "Configured shares listed"
else
  warn "Failed to list configured shares; continuing"
fi

log "Bootstrap complete"
if [ "$serve_after_bootstrap" = "true" ]; then
  log "Starting packageR on port $PACKAGE_R_SERVER_PORT"
  exec "$PACKAGE_R_BIN" -p "$PACKAGE_R_SERVER_PORT"
else
  print_package_r_banner
fi
