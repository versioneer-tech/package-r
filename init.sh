#!/bin/sh

mkdir -p /workspace/packages || exit 1
mkdir -p /mounts || exit 1
mkdir -p /secrets || exit 1

./filebrowser config init > /dev/null 2>&1
./filebrowser config set \
  --address "" \
  --root "/" \
  --branding.name ${FB_BRANDING_NAME:-packageR} \
  --branding.files ${FB_BRANDING_FILES:-/package-r} \
  --scope "" \
  --auth.method=proxy \
  --auth.header=x-id-token \
  --auth.mapper=azp-groups \
   --commands "add-source","establish-sources","remove-source","get-members","add-member","get-groups","get-runtime-status","set-runtime-status","ls" > /dev/null || exit
  ./filebrowser users add admin ${FB_PASSWORD:-changeme} --scope=/workspace --perm.admin=true --perm.execute=true --perm.create=true --perm.rename=true --perm.modify=true --perm.delete=true --perm.share=true --perm.download=true --lockPassword > /dev/null 2>&1 || true # true as it is ok if resource already exists
  ./filebrowser users add guest ${FB_PASSWORD:-changeme} --scope=/workspace --perm.admin=false --perm.execute=false --perm.create=false --perm.rename=false --perm.modify=false --perm.delete=false --perm.share=false --perm.download=false --lockPassword > /dev/null 2>&1 || true # true as it is ok if resource already exists
  ./filebrowser users add user ${FB_PASSWORD:-changeme} --scope=/workspace --perm.admin=false --perm.execute=true --perm.create=false --perm.rename=false --perm.modify=false --perm.delete=true --perm.share=true --perm.download=false --lockPassword > /dev/null 2>&1 || true # true as it is ok if resource already exists 