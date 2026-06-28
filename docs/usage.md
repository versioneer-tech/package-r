# Usage

### Local Setup Without S3

Build the local `filebrowser` binary from the repository root:

```bash
make build
```

Prepare local fixture data and start packageR:

```bash
export FB_ROOT="${FB_ROOT:-/workspace}"
export FB_DATABASE="${FB_DATABASE:-/db/bolt.db}"

./init.sh --add-shares public-share=/public --add-test-data /public --serve
```

This copies `tests/data` below `/workspace/public`, stores runtime state in
`/db/bolt.db`, and creates a reusable public share. Make sure `/workspace` and
`/db` exist and are writable by the user running packageR.

In the recommended declarative setup, packageR can recreate configuration and
default shares during startup. The database does not contain object data; keep
`/db` durable only if users, shares, or settings created at runtime should
survive a restart.

!!! note
    This setup is not backed by S3 or another object store. Presigned URL
    actions still exercise the packageR flow, but they fall back to
    packageR/File Browser URLs instead of S3-compatible object-storage URLs.

For other local checks, use `--add-test-data <path>` to copy the fixture
contents into another folder inside `FB_ROOT`.

### Local Setup With S3

For an S3-backed setup, packageR needs two views of the same data:

- a filesystem view mounted at `FB_ROOT`, used for browsing, sharing,
  previews, and catalogs. Write access can be enabled as well, making it easy
  to curate additional metadata next to the data, while bulk data uploads
  should generally remain the responsibility of the underlying storage and
  ingestion workflows.
- S3-compatible signing settings, used to create presigned URLs for clients.

Use an existing S3-compatible bucket, such as AWS S3 or self-hosted MinIO, or
start a local S3-compatible server for testing. The example below uses
`rclone serve s3`, tested with rclone `v1.74.3`.

In one terminal, create a small local bucket with one sample file and expose it
through rclone's authenticated S3 API:

```bash
mkdir -p /tmp/rclone-s3/bucket1
echo "hello" > /tmp/rclone-s3/bucket1/test.txt

rclone serve s3 /tmp/rclone-s3 \
  --addr :9000 \
  --auth-key ACCESS_KEY_ID,SECRET_ACCESS_KEY
```

`rclone serve s3` is an S3 API endpoint, not a public static HTTP file server.
With `--auth-key`, a plain browser request such as
`http://localhost:9000/bucket1/test.txt` or `http://localhost:9000/a` is
unsigned and can return:

 To sanity-check the rclone S3
server directly, use the same credentials with an S3 client:

```bash
rclone cat :s3:bucket1/test.txt \
  --s3-provider Other \
  --s3-endpoint http://127.0.0.1:9000 \
  --s3-access-key-id ACCESS_KEY_ID \
  --s3-secret-access-key SECRET_ACCESS_KEY \
  --s3-region default \
  --s3-force-path-style
```

In another terminal, mount that bucket into packageR's workspace path:

```bash
mkdir -p /tmp/workspace

rclone mount :s3:bucket1 /tmp/workspace \
  --uid 1000 \
  --gid 100 \
  --umask 022 \
  --allow-other \
  --s3-provider Other \
  --s3-endpoint http://127.0.0.1:9000 \
  --s3-access-key-id ACCESS_KEY_ID \
  --s3-secret-access-key SECRET_ACCESS_KEY \
  --s3-region default \
  --s3-force-path-style
```

Leave the S3 server and the rclone mount running. Then start packageR against
the mounted bucket in another terminal:

```bash
export FB_ROOT=/tmp/workspace
export FB_DATABASE=/tmp/db/bolt.db
export AWS_ACCESS_KEY_ID=ACCESS_KEY_ID
export AWS_SECRET_ACCESS_KEY=SECRET_ACCESS_KEY
export AWS_ENDPOINT_URL=http://127.0.0.1:9000
export AWS_REGION=any
export BUCKET_NAME=bucket1

./init.sh --add-shares public-share=/ --serve
```

Then validate access through packageR, which signs the S3 request and redirects
the client to the presigned object URL:

```bash
curl -fsSL \
  "http://127.0.0.1:8888/api/public/share/public-share/test.txt?presign=true&followRedirect=true"
```

For AWS S3, MinIO, or another S3-compatible service, use the same packageR
environment variables and replace the endpoint, region, bucket name, and
credentials. The mounted filesystem and the signing settings must describe the
same bucket content; packageR does not read rclone or MinIO configuration.

### Docker Setup With S3

Docker follows the same model: mount the bucket into the container at
`/workspace`, keep packageR state in `/db/bolt.db`, and pass the S3-compatible
credentials used for presigning. For the local rclone example, these are the
same `ACCESS_KEY_ID` and `SECRET_ACCESS_KEY` values passed to `--auth-key`.
The bind mount below assumes `/workspace` is the mounted bucket path on the
Docker host.

```bash
docker run --rm -it \
  -v /tmp/workspace:/workspace \
  -e FB_ROOT=/workspace \
  -e FB_DATABASE=/db/bolt.db \
  -e FB_DEFAULT_SHARES='public-share=/' \
  -e AWS_ACCESS_KEY_ID=ACCESS_KEY_ID \
  -e AWS_SECRET_ACCESS_KEY=SECRET_ACCESS_KEY \
  -e AWS_ENDPOINT_URL=http://127.0.0.1:9000 \
  -e AWS_REGION=any \
  -e BUCKET_NAME=bucket1 \
  -p 8888:8888 \
  package-r:latest
```

The image runs as UID `1000` and GID `100` by default. Make sure bind-mounted
folders are readable by that user/group, and writable when packageR should
create generated user homes, shares, or uploaded files.

If the S3 endpoint is not reachable from clients as `http://127.0.0.1:9000`,
set `AWS_ENDPOINT_URL` to the URL that clients should use for presigned URLs.

## Shares and Catalogs

Share paths are filesystem paths inside `FB_ROOT`. Public share URLs are only
the external access paths.

For example, with `FB_ROOT=/workspace`, sharing `/a/b` as `yyy` exposes
`/workspace/a/b/c.txt` as `/share/yyy/c.txt`.