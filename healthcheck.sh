#!/bin/sh
port=${PACKAGE_R_SERVER_PORT:-8888}
address=${PACKAGE_R_ADDRESS:-127.0.0.1}

case "$address" in
  ""|0.0.0.0|::|\[::\])
    address=127.0.0.1
    ;;
esac

busybox wget -q -O /dev/null "http://$address:$port/health" || exit 1
