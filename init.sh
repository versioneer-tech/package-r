#!/bin/sh

mkdir -p /home || exit 1
mkdir -p /home/default || exit 1

mkdir -p /mounts || exit 1

mkdir -p /secrets || exit 1

./filebrowser config init > /dev/null 2>&1
./filebrowser config set \
  --address "" \
  --root "/" \
  --branding.name ${FB_BRANDING_NAME:-packageR} \
  --branding.files ${FB_BRANDING_FILES:-/package-r} \
  --scope "" \
  --create-user-dir \
  --commands "add-source","establish-sources","remove-source" > /dev/null || exit
  ./filebrowser users add default ${FB_PASSWORD:-changeme} --lockPassword > /dev/null 2>&1 || true # true as it is ok if resource already exists