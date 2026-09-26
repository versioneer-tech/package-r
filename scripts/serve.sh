#!/bin/sh

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

log() {
  printf '[serve] %s\n' "$*"
}

warn() {
  printf '[serve] warning: %s\n' "$*" >&2
}

package_r() {
  "${SERVE_PACKAGE_R_BIN:-$script_dir/../package-r}" "$@"
}

auth_mapper() {
  if [ "${PACKAGE_R_AUTH_HEADER:-Authorization}" = "Authorization" ]; then
    printf '%s' "${PACKAGE_R_AUTH_MAPPER-.sub}"
  else
    printf '%s' "${PACKAGE_R_AUTH_MAPPER-}"
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
      warn "Skipping invalid SERVE_PACKAGE_R_DEFAULT_SHARE_PASSWORDS entry"
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
  case "${PACKAGE_R_ROOT:-/}" in
    /)
      if [ "${PACKAGE_R_CREATE_USER_DIR:-false}" = "true" ]; then
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

check_package_r_binary() {
  if package_r version > /dev/null; then
    return 0
  fi

  warn "packageR binary failed to start; cannot continue"
  return 1
}

package_r_config_set_supports() {
  flag=$1
  help_output=$(package_r config set --help 2>/dev/null || true)
  case "$help_output" in
    *"$flag"*) return 0 ;;
    *) return 1 ;;
  esac
}

jwt_config_requested() {
  [ -n "${PACKAGE_R_AUTH_JWT_JWKS_URL:-}" ] ||
    [ -n "${PACKAGE_R_AUTH_JWT_ISSUER:-}" ] ||
    [ -n "${PACKAGE_R_AUTH_JWT_AUDIENCE:-}" ] ||
    [ -n "${PACKAGE_R_AUTH_JWT_ALGORITHMS:-}" ] ||
    [ -n "${PACKAGE_R_AUTH_JWT_CLOCK_SKEW:-}" ]
}

user_exists() {
  username=$1
  package_r users find "$username" > /dev/null 2>&1
}

ensure_user() {
  username=$1
  shift

  log "Ensuring default user exists: $username"
  command_output=$(package_r users add "$username" "${SERVE_PACKAGE_R_PASSWORD:-$password}" "$@" 2>&1)
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

if [ "$#" -ne 0 ]; then
  warn "serve.sh does not accept arguments; use environment variables"
  exit 2
fi

if ! validate_object_storage_root; then
  exit 1
fi

if ! mkdir -p "$(dirname -- "${PACKAGE_R_DATABASE:-/tmp/package-r.db}")"; then
  warn "Failed to prepare the PACKAGE_R_DATABASE directory; cannot continue"
  exit 1
fi

log "Starting bootstrap"

if ! check_package_r_binary; then
  exit 1
fi

log "Using ${PACKAGE_R_DATABASE:-/tmp/package-r.db} for state"
log "Using ${PACKAGE_R_ROOT:-/} as the S3 root"

log_object_storage_mode

if package_r config init > /dev/null 2>&1; then
  log "Initialized packageR database"
else
  log "packageR database already exists; continuing with configuration update"
fi

ALLOW_CHANGING=false
if [ "${SERVE_PACKAGE_R_ALLOW_CHANGING:-false}" = "true" ]; then
  ALLOW_CHANGING=true
  log "Changing allowed"
fi

set -- config set \
  --address "" \
  --root "${PACKAGE_R_ROOT:-/}" \
  --disable-preview-resize \
  --disable-thumbnails \
  --disable-type-detection-by-header \
  --signup="${PACKAGE_R_SIGNUP:-true}" \
  --create-user-dir="${PACKAGE_R_CREATE_USER_DIR:-false}" \
  --auth.method="${PACKAGE_R_AUTH_METHOD:-json}" \
  --auth.header="${PACKAGE_R_AUTH_HEADER:-Authorization}" \
  --auth.mapper="$(auth_mapper)" \
  --branding.name "${PACKAGE_R_BRANDING_NAME:-packageR}" \
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
    --auth.jwt.jwks-url="${PACKAGE_R_AUTH_JWT_JWKS_URL:-}" \
    --auth.jwt.issuer="${PACKAGE_R_AUTH_JWT_ISSUER:-}" \
    --auth.jwt.audience="${PACKAGE_R_AUTH_JWT_AUDIENCE:-}" \
    --auth.jwt.algorithms="${PACKAGE_R_AUTH_JWT_ALGORITHMS:-}" \
    --auth.jwt.clock-skew="${PACKAGE_R_AUTH_JWT_CLOCK_SKEW:-}"
elif jwt_config_requested; then
  warn "JWT proxy auth environment variables require a packageR binary that supports --auth.jwt.jwks-url"
  exit 1
fi

log "Applying packageR configuration"
if package_r "$@" > /dev/null; then
  log "packageR configuration applied"
else
  warn "Failed to apply packageR configuration; cannot continue"
  exit 1
fi

password=$(head /dev/urandom | tr -dc 'A-Za-z0-9' | head -c 16)

if [ -n "${SERVE_PACKAGE_R_PASSWORD:-}" ]; then
  log "Using SERVE_PACKAGE_R_PASSWORD for bootstrap users"
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

default_shares=${SERVE_PACKAGE_R_DEFAULT_SHARES:-}
default_share_passwords=${SERVE_PACKAGE_R_DEFAULT_SHARE_PASSWORDS:-}

if [ -n "$default_shares" ]; then
  default_share_count=$(printf '%s' "$default_shares" | tr ';' '\n' | awk 'NF { count++ } END { print count + 0 }')
  log "Processing $default_share_count default share(s) for owner $default_share_owner"

  printf '%s\n' "$default_shares" | tr ';' '\n' | while IFS= read -r share || [ -n "$share" ]; do
    [ -z "$share" ] && continue

    hash=${share%%=*}
    path=${share#*=}

    if [ -z "$hash" ] || [ -z "$path" ] || [ "$hash" = "$share" ]; then
      warn "Skipping invalid SERVE_PACKAGE_R_DEFAULT_SHARES entry: $share"
      continue
    fi

    log "Ensuring default share exists: hash=$hash path=$path owner=$default_share_owner"
    share_password=$(default_share_password "$hash" "$default_share_passwords")
    if PACKAGE_R_SHARE_PASSWORD=$share_password package_r shares add "$default_share_owner" "$hash" "$path" > /dev/null; then
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
if package_r shares ls; then
  log "Configured shares listed"
else
  warn "Failed to list configured shares; continuing"
fi

log "Bootstrap complete"
log "Starting packageR on port ${PACKAGE_R_PORT:-8888}"
exec "${SERVE_PACKAGE_R_BIN:-$script_dir/../package-r}" -p "${PACKAGE_R_PORT:-8888}"
