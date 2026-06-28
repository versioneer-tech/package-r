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

!!! note
    This setup is not backed by S3 or another object store. Presigned URL
    actions still exercise the packageR flow, but they fall back to
    packageR/File Browser URLs instead of S3-compatible object-storage URLs.

For other local checks, use `--add-test-data <path>` to copy the fixture
contents into another folder inside `FB_ROOT`.

### Local Setup With S3

For an S3-backed setup, packageR needs two views of the same data:

- a filesystem view mounted at `FB_ROOT`, used for browsing, shares, previews,
  and catalogs;
- S3-compatible signing settings, used to create presigned URLs for clients.

Use an existing S3-compatible bucket, such as AWS S3 or MinIO, or start a local
S3-compatible server for testing. This example uses `rclone serve s3`.

In one terminal, create a small local bucket and expose it through rclone's S3
server:

```bash
mkdir -p /tmp/rclone-s3/bucket1
echo "hello" > /tmp/rclone-s3/bucket1/test.txt

rclone serve s3 /tmp/rclone-s3 \
  --addr :9000 \
  --auth-key ACCESS_KEY_ID,SECRET_ACCESS_KEY
```

In another terminal, mount that bucket into packageR's workspace path:

```bash
mkdir -p /workspace

rclone mount :s3:bucket1 /workspace \
  --s3-provider Other \
  --s3-endpoint http://127.0.0.1:9000 \
  --s3-access-key-id ACCESS_KEY_ID \
  --s3-secret-access-key SECRET_ACCESS_KEY \
  --s3-region us-east-1 \
  --s3-force-path-style \
  --vfs-cache-mode writes
```

Then start packageR against the mounted bucket:

```bash
export FB_ROOT=/workspace
export FB_DATABASE=/db/bolt.db
export AWS_ACCESS_KEY_ID=ACCESS_KEY_ID
export AWS_SECRET_ACCESS_KEY=SECRET_ACCESS_KEY
export AWS_ENDPOINT_URL=http://127.0.0.1:9000
export AWS_REGION=us-east-1
export BUCKET_NAME=bucket1

./init.sh --add-shares public-share=/ --serve
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

```bash
docker run --rm -it \
  -v /workspace:/workspace \
  -v package-r-db:/db \
  -e FB_ROOT=/workspace \
  -e FB_DATABASE=/db/bolt.db \
  -e FB_DEFAULT_SHARES='public-share=/' \
  -e AWS_ACCESS_KEY_ID=ACCESS_KEY_ID \
  -e AWS_SECRET_ACCESS_KEY=SECRET_ACCESS_KEY \
  -e AWS_ENDPOINT_URL=http://127.0.0.1:9000 \
  -e AWS_REGION=us-east-1 \
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
`/workspace/a/b/c.txt` as `/share/yyy/c.txt`. If the catalog name is
`catalog.parquet`, packageR reads it from `/workspace/a/b/catalog.parquet`.

## Kubernetes Bucket Mount Health

If packageR runs on Kubernetes with a bucket-backed PVC, for example via a CSI
FUSE mount, the mount can become stale. A typical error is:

```text
Transport endpoint is not connected
```

You can verify this inside the pod:

```bash
stat /workspace
```

If the mount is broken, it returns an error similar to:

```text
stat: cannot statx '/workspace': Transport endpoint is not connected
```

Restarting the container may not fix a stale FUSE mount. In that case, recreate
the pod. The full csi-rclone example includes readiness and liveness probes
that check `/workspace` and keep packageR running as UID `1000` and GID `100`:
[`docs/examples/kubernetes-csi-rclone.yaml`](examples/kubernetes-csi-rclone.yaml).
