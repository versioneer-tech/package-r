# Run packageR

## Build the service

Build the frontend bundle and Go binary from the repository root:

```bash
make build
```

The result is the `package-r` binary.

## Configure an object store

Set an S3 root and region. This example uses static credentials. Set
`AWS_ENDPOINT_URL` for another S3-compatible service. Leave it empty for AWS
S3.

```bash
export AWS_ACCESS_KEY_ID=my-access-key
export AWS_SECRET_ACCESS_KEY=my-secret-key
export AWS_ENDPOINT_URL=https://objects.example.invalid
export AWS_REGION=us-east-1
export PACKAGE_R_ROOT=my-bucket

export PACKAGE_R_DATABASE=/tmp/package-r.db
export PACKAGE_R_AUTH_METHOD=proxy
export PACKAGE_R_AUTH_HEADER=X-Username
export PACKAGE_R_ALLOW_CHANGING=true
export PACKAGE_R_DEFAULT_SHARES='my-share=/catalog-sample'
export PACKAGE_R_DEFAULT_SHARE_PASSWORDS='my-share=1234'
```

Replace all example values. Do not store credentials in the repository.

For workload identity, omit `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and
`AWS_SESSION_TOKEN`. Let the platform provide the identity. AWS IRSA is one
example. All sessions use this process identity.

The bucket becomes `/` in packageR. For example,
`s3://my-bucket/catalog-sample/report.tif` appears as
`/catalog-sample/report.tif`.

Set `PACKAGE_R_ROOT=/` to show permitted buckets as top-level directories.
This mode needs permission to list buckets. It does not support
`PACKAGE_R_CREATE_USER_DIR=true`.

## Bootstrap and start

Create the configured public shares and start the service:

```bash
./init.sh --serve
```

Open `http://127.0.0.1:8888`. Object actions use rclone VFS. User permissions
and the S3 policy control which actions are available.

Follow [Work with objects](work-with-objects.md) for the browser workflow.

Request a time-limited public object URL:

```bash
curl -fsS \
  'http://127.0.0.1:8888/api/public/share/my-share/report.tif?presign=true'
```

Add `followRedirect=true` to return a temporary redirect to the object URL.
Presigning supports GET requests. See the
[HTTP API](../reference-guides/http-api.md) for route and authentication
details.

## Continue

- Use [Configuration](configuration.md) to set storage, authentication, and
  access controls
- Use [Share and preview data](share-and-preview-data.md) for public packages,
  COGs, and STAC catalogs
- Use [Work with objects](work-with-objects.md) for browser tasks
