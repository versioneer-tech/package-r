# Usage

## Local Setup

Build the local `filebrowser` binary from the repository root:

```bash
make build
```

Prepare the local sample data and start packageR:

```bash
./init.sh --add-shares public-share=/public --add-test-data /public --serve
```

This copies `tests/data` below the configured root's `/public` folder and
creates reusable local state. The sample
`catalog.parquet` uses relative asset paths, so it works from the copied
location without rewriting. This setup is useful for checking the UI, public
shares, and STAC catalog routes.

For other local checks, use `--add-test-data <path>` to copy the fixture
contents into another folder inside `FB_ROOT`.

!!! note
    Local sample setup is not backed by object storage. Presigned URL actions
    still work as application flows, but they return packageR/File Browser URLs
    instead of S3-compatible object-storage URLs.

### Local Setup With a FUSE Mount

For a local setup backed by object storage, mount the bucket first and point
`FB_ROOT` at the mounted folder. For example, with a configured rclone remote:

```bash
mkdir -p /tmp/package-r-bucket
rclone mount package-r-s3:my-bucket /tmp/package-r-bucket
```

Then start packageR against that mounted folder in another terminal:

```bash
export FB_ROOT=/tmp/package-r-bucket
export AWS_ACCESS_KEY_ID=<access-key>
export AWS_SECRET_ACCESS_KEY=<secret-key>
export AWS_ENDPOINT_URL=<s3-endpoint-url>
export AWS_REGION=<region>
export BUCKET_NAME=my-bucket

./init.sh --add-shares public-share=/public --serve
```

The FUSE mount gives packageR a normal-looking file tree for browsing,
previews, shares, and catalogs. S3-compatible presigned URLs require
`AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`; configure
`AWS_ENDPOINT_URL`, `AWS_REGION`, `BUCKET_NAME`, and `BUCKET_PREFIX` as needed
so packageR signs URLs for the same object-storage location that is mounted.
Even if the rclone remote already has credentials, packageR needs these
AWS-compatible signing values separately because it does not read rclone's
remote configuration.

The same model works with `s3fs` or other FUSE mounts. In Kubernetes, the
mounted bucket may simply be injected as a folder through a CSI/FUSE volume or
another platform storage integration; packageR only needs that folder as
`FB_ROOT`.

## Shares and Catalogs

Share paths are filesystem paths inside `FB_ROOT`. Public share URLs are only
the external access paths.

For example, with `FB_ROOT=/workspace`, sharing `/a/b` as `yyy` exposes
`/workspace/a/b/c.txt` as `/share/yyy/c.txt`. If the catalog name is
`catalog.parquet`, packageR reads it from `/workspace/a/b/catalog.parquet`.

## Docker Example

```bash
docker run --rm -it \
  -u 1000:1000 \
  -v /workspace:/workspace \
  -e FB_ROOT=/workspace/<my-bucket> \
  -e FB_BRANDING_NAME=Workspace \
  -e FB_DEFAULT_SHARES='public-my-bucket=/workspace/<my-bucket>/public' \
  -e AWS_ACCESS_KEY_ID=<my-key> \
  -e AWS_SECRET_ACCESS_KEY=<my-secret> \
  -e AWS_ENDPOINT_URL=<my-endpoint> \
  -e AWS_REGION=<my-region> \
  -e BUCKET_NAME=<my-bucket> \
  -p 8080:8080 \
  package-r:latest
```

This lets packageR list and share data from the mounted folder while generating
presigned URLs that point clients directly at the bucket.

If you want startup-created default shares, set `FB_DEFAULT_SHARES` as a
semicolon-separated list.

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

Add readiness and liveness probes that check the mount:

```yaml
readinessProbe:
  exec:
    command:
      - /bin/sh
      - -lc
      - |
        out="$( (stat /workspace >/dev/null) 2>&1 || true )"
        if echo "$out" | grep -qi "Transport endpoint is not connected"; then
          exit 1
        fi
        exit 0
  periodSeconds: 10

livenessProbe:
  exec:
    command:
      - /bin/sh
      - -lc
      - |
        out="$( (stat /workspace >/dev/null) 2>&1 || true )"
        if echo "$out" | grep -qi "Transport endpoint is not connected"; then
          exit 1
        fi
        exit 0
  periodSeconds: 20
```

Restarting the container may not fix a stale FUSE mount. In that case, recreate
the pod.
