#!/bin/sh
./filebrowser config init > /dev/null 2>&1

./filebrowser config set \
  --address "" \
  --scope "" \
  --disable-preview-resize \
  --disable-thumbnails \
  --disable-type-detection-by-header \
  --signup=false \
  --auth.method=proxy \
  --auth.header=${FB_AUTH_HEADER:-"X-Username"} \
  --auth.mapper=${FB_AUTH_MAPPER:-""} \
  --branding.name ${FB_BRANDING_NAME:-packageR} \
  --branding.files ${FB_BRANDING_FILES:-/package-r} \
  --commands "" > /dev/null

envs=\
"AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID},"\
"AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY},"\
"AWS_ENDPOINT_URL=${AWS_ENDPOINT_URL},"\
"AWS_REGION=${AWS_REGION},"\
"BUCKET_NAME=${BUCKET_NAME},"\
"BUCKET_PREFIX=${BUCKET_PREFIX}"

password=$(head /dev/urandom | tr -dc 'A-Za-z0-9' | head -c 16)

  ./filebrowser users add admin "${FB_PASSWORD:-$password}" \
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
  --envs="$envs"  > /dev/null 2>&1 || true # true as it is ok if resource already exists

./filebrowser users add reader-noshare "${FB_PASSWORD:-$password}" \
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
  --envs="$envs"  > /dev/null 2>&1 || true # true as it is ok if resource already exists

./filebrowser users add reader-share "${FB_PASSWORD:-$password}" \
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
  --envs="$envs"  > /dev/null 2>&1 || true # true as it is ok if resource already exists

./filebrowser users add writer-noshare "${FB_PASSWORD:-$password}" \
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
  --envs="$envs"  > /dev/null 2>&1 || true # true as it is ok if resource already exists

./filebrowser users add writer-share "${FB_PASSWORD:-$password}" \
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
  --envs="$envs"  > /dev/null 2>&1 || true # true as it is ok if resource already exists