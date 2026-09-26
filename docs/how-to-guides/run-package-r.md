# Run packageR

## Build the service

Build the frontend bundle and Go binary from the repository root:

```bash
make build
```

The result is the `filebrowser` binary.

## Configure an object store

Set one S3 root and region. The example uses static credentials. Set
`AWS_ENDPOINT_URL` for MinIO, Ceph, or another S3-compatible service. Leave it
empty for AWS S3.

```bash
export AWS_ACCESS_KEY_ID=my-access-key
export AWS_SECRET_ACCESS_KEY=my-secret-key
export AWS_ENDPOINT_URL=https://objects.example.invalid
export AWS_REGION=us-east-1
export FB_ROOT=my-bucket

export FB_DATABASE=/tmp/package-r.db
export FB_AUTH_METHOD=proxy
export FB_AUTH_HEADER=X-Username
export FB_ALLOW_CHANGING=true
export FB_DEFAULT_SHARES='my-share=/catalog-sample'
export FB_DEFAULT_SHARE_PINS='my-share=1234'
```

Replace all example values. Do not store credentials in the repository.

For provider-supported workload identity, omit `AWS_ACCESS_KEY_ID`,
`AWS_SECRET_ACCESS_KEY`, and `AWS_SESSION_TOKEN`. Let the platform inject its
identity settings. AWS IRSA is one example. All packageR sessions use this one
process credential source.

The bucket becomes `/` in packageR. For example,
`s3://my-bucket/catalog-sample/report.tif` appears as
`/catalog-sample/report.tif`.

Set `FB_ROOT=/` to show permitted buckets as top-level directories. This mode
needs permission to list buckets. Keep `FB_CREATE_USER_DIR=false` in this
mode.

## Bootstrap and start

Create the configured public shares and start the service:

```bash
./init.sh --serve
```

Open `http://127.0.0.1:8888`. Normal File Browser actions such as browse,
upload, create, rename, copy, move, and delete run through rclone VFS. The
available actions still depend on the packageR user permissions and the S3
credentials.

Follow [Work with objects](work-with-objects.md) for the browser workflow.

Request a time-limited public object URL:

```bash
curl -fsS \
  'http://127.0.0.1:8888/api/public/share/my-share/report.tif?presign=true'
```

Add `followRedirect=true` when the client must follow a temporary redirect to
the object URL. Presigning supports GET requests. See the
[HTTP API](../reference-guides/http-api.md) for authentication and route
details.

## Preview cloud-native data

TIFF and GeoTIFF files use their presigned object URL in the browser viewer.
This supports COG access without sending the full object through packageR.
The object store must allow browser CORS requests from the packageR and viewer
origins. It must also support byte-range requests. See
[Object storage](configuration.md#object-storage) for the required headers.

For a public STAC view, place the configured STAC-compatible Parquet catalog
inside the shared path. A share for `/catalog-sample` with the default catalog
name reads `/catalog-sample/catalog.parquet` through rclone VFS. The public
endpoint is:

```text
/api/public/catalog/my-share
```

packageR validates the catalog through the scoped VFS. DuckDB reads it from a
short-lived signed URL with HTTP range requests.

## Continue

- Use [Configuration](configuration.md) to set storage, authentication, and
  access controls.
- Use [Share and preview data](share-and-preview-data.md) for public shares,
  COGs, and STAC.
