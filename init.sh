#!/bin/sh

mkdir -p /home || exit 1
mkdir -p /home/default || exit 1

mkdir -p /mounts || exit 1

mkdir -p /secrets || exit 1

./filebrowser config init > /dev/null 2>&1
./filebrowser config set \
  --address "" \
  --root "/" \
  --branding.name "packageR" \
  --branding.files "/package-r-design" \
  --scope "" \
  --create-user-dir \
  --commands "add-source","do-echo","do-log","do-presign","establish-sources","remove-source" > /dev/null || exit
  ./filebrowser users add default ${PASSWORD:-changeme} --lockPassword > /dev/null 2>&1 || true # true as it is ok if resource already exists