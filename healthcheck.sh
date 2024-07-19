#!/bin/sh
PORT=${FB_PORT:-$(jq -r .port /.package-r.json)}
ADDRESS=${FB_ADDRESS:-$(jq -r .address /.package-r.json)}
ADDRESS=${ADDRESS:-localhost}
curl -f http://$ADDRESS:$PORT/health || exit 1
