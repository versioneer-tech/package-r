#!/bin/sh

log() {
  printf '[init] %s\n' "$*"
}

warn() {
  printf '[init] warning: %s\n' "$*" >&2
}

ensure_user() {
  username=$1
  shift

  log "Ensuring default user exists: $username"
  if ./filebrowser users add "$username" "${FB_PASSWORD:-$password}" "$@" > /dev/null 2>&1; then
    log "Default user ready: $username"
  else
    log "Skipping default user bootstrap for $username; it may already exist"
  fi
}

log "Starting bootstrap"

if ./filebrowser config init > /dev/null 2>&1; then
  log "Initialized filebrowser database"
else
  log "Filebrowser database already exists; continuing with configuration update"
fi

log "Applying filebrowser configuration"
if ./filebrowser config set \
  --address "" \
  --scope "" \
  --disable-preview-resize \
  --disable-thumbnails \
  --disable-type-detection-by-header \
  --signup=false \
  --auth.method=${FB_AUTH_METHOD:-"proxy"} \
  --auth.header=${FB_AUTH_HEADER:-"X-Username"} \
  --auth.mapper=${FB_AUTH_MAPPER:-""} \
  --branding.name ${FB_BRANDING_NAME:-packageR} \
  --branding.files ${FB_BRANDING_FILES:-/package-r} \
  --sharelink.defaultHash ${FB_SHARELINK_DEFAULT_HASH:-"public-<random>-v1"} \
  --catalog.baseurl ${FB_CATALOG_BASE_URL:-""} \
  --catalog.defaultName ${FB_CATALOG_DEFAULT_NAME:-"catalog.v1.parquet"} \
  --catalog.previewURL ${FB_CATALOG_PREVIEW_URL:-""} \
  --commands "" > /dev/null; then
  log "Filebrowser configuration applied"
else
  warn "Failed to apply filebrowser configuration; continuing"
fi

envs=\
"AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID},"\
"AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY},"\
"AWS_ENDPOINT_URL=${AWS_ENDPOINT_URL},"\
"AWS_REGION=${AWS_REGION},"\
"BUCKET_NAME=${BUCKET_NAME},"\
"BUCKET_PREFIX=${BUCKET_PREFIX}"

password=$(head /dev/urandom | tr -dc 'A-Za-z0-9' | head -c 16)

if [ -n "${FB_PASSWORD:-}" ]; then
  log "Using FB_PASSWORD for bootstrap users"
else
  log "Generated random password for bootstrap users"
fi

log "Ensuring bootstrap users exist"
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

ensure_user reader-noshare \
  --scope=/ \
  --perm.admin=false \
  --perm.execute=false \
  --perm.create=false \
  --perm.rename=false \
  --perm.modify=false \
  --perm.delete=false \
  --perm.share=false \
  --perm.download=false \
  --lockPassword \
  --envs="$envs"

ensure_user reader-share \
  --scope=/ \
  --perm.admin=false \
  --perm.execute=false \
  --perm.create=false \
  --perm.rename=false \
  --perm.modify=false \
  --perm.delete=false \
  --perm.share=true \
  --perm.download=false \
  --lockPassword \
  --envs="$envs"

ensure_user writer-noshare \
  --scope=/ \
  --perm.admin=false \
  --perm.execute=false \
  --perm.create=true \
  --perm.rename=true \
  --perm.modify=true \
  --perm.delete=true \
  --perm.share=false \
  --perm.download=false \
  --lockPassword \
  --envs="$envs"

ensure_user writer-share \
  --scope=/ \
  --perm.admin=false \
  --perm.execute=false \
  --perm.create=true \
  --perm.rename=true \
  --perm.modify=true \
  --perm.delete=true \
  --perm.share=true \
  --perm.download=false \
  --lockPassword \
  --envs="$envs"
log "Bootstrap users processed"

default_share_owner=admin

if [ -n "${FB_DEFAULT_SHARES:-}" ]; then
  default_share_count=$(printf '%s' "$FB_DEFAULT_SHARES" | tr ';' '\n' | awk 'NF { count++ } END { print count + 0 }')
  log "Processing $default_share_count default share(s) for owner $default_share_owner"

  printf '%s\n' "$FB_DEFAULT_SHARES" | tr ';' '\n' | while IFS= read -r share || [ -n "$share" ]; do
    [ -z "$share" ] && continue

    hash=${share%%=*}
    path=${share#*=}

    if [ -z "$hash" ] || [ -z "$path" ] || [ "$hash" = "$share" ]; then
      warn "Skipping invalid FB_DEFAULT_SHARES entry: $share"
      continue
    fi

    log "Ensuring default share exists: hash=$hash path=$path owner=$default_share_owner"
    if ./filebrowser shares add "$default_share_owner" "$hash" "$path" > /dev/null; then
      log "Default share ready: $hash -> $path"
    else
      warn "Failed to create default share: $hash -> $path"
    fi
  done
  log "Default shares processed"
else
  log "No default shares configured via FB_DEFAULT_SHARES"
fi

log "Listing configured shares"
if ./filebrowser shares ls; then
  log "Configured shares listed"
else
  warn "Failed to list configured shares; continuing"
fi

log "Bootstrap complete"
