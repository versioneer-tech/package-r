<img src="frontend/public/img/logo.png" height="40" alt="packageR"/>

# packageR

> [!NOTE]
> This is the third packageR iteration: v1 used the AWS S3 SDK directly, v2
> required an external S3 mount, and the current implementation uses an
> embedded rclone VFS.

packageR makes it easy to package and distribute a prefix of S3-compatible
bucket objects. An operator maps the prefix to a public package name, and
recipients open the package in a browser without an account or login. The name
can be simple, such as `public`, arbitrary within the supported URL format, or
hard to guess. An optional share password provides access protection. If the
share contains a Parquet catalog that lists its objects, packageR can also
expose it as [SpatioTemporal Asset Catalog
(STAC)](https://stacspec.org/)-compatible JSON.

packageR also provides a web file browser. The Go service opens the S3 service
root or one bucket through an embedded rclone VFS. It uses the same rclone
backend to browse objects and to create time-limited download URLs.
Production does not need an object-storage mount or a separate rclone service.

The current design uses one process-owned S3 credential source. It can use a
standard S3 access-key pair or provider-supported workload identity, such as
AWS IRSA. All sessions use that storage identity. packageR can narrow access
with identity scopes, path rules, and action permissions. User-directory mode
protects sibling home directories and their shared parent. Action permissions
separately control changes to allowed paths. User-directory mode does not
select credentials. Per-user object-storage credentials are a possible future
direction and are not currently planned.

packageR provides:

- public, browser-accessible packages for configured object prefixes, with no
  recipient login and an optional share password;
- directory navigation and common File Browser operations;
- uploads, downloads, and access rules;
- presigned GET URLs for authenticated files and public shares;
- TIFF and Cloud Optimized GeoTIFF (COG) previews; and
- STAC-compatible Parquet catalogs exposed as public STAC JSON.

See the [packageR documentation](https://package-r.versioneer.at/) for
[packaging and sharing data](https://package-r.versioneer.at/latest/how-to-guides/share-and-preview-data/),
[configuration](https://package-r.versioneer.at/latest/how-to-guides/configuration/),
[operation](https://package-r.versioneer.at/latest/how-to-guides/run-package-r/),
the [HTTP API](https://package-r.versioneer.at/latest/reference-guides/http-api/),
the
[rclone VFS architecture decision](https://package-r.versioneer.at/latest/architecture/0001-use-rclone-vfs-for-object-storage/),
and the
[STAC catalog architecture decision](https://package-r.versioneer.at/latest/architecture/0002-publish-parquet-catalogs-as-stac/).

## Download and run

Download and unpack the latest prebuilt Linux AMD64 release:

```bash
curl -fL -o package-r.tar.gz \
  https://github.com/versioneer-tech/package-r/releases/latest/download/linux-amd64-package-r.tar.gz
tar -xzf package-r.tar.gz
./filebrowser version
```

We also provide a container image at
`ghcr.io/versioneer-tech/package-r:latest`. Run it with your S3 settings:

```bash
docker run --rm -p 127.0.0.1:8888:8888 \
  -e AWS_ACCESS_KEY_ID=my-access-key \
  -e AWS_SECRET_ACCESS_KEY=my-secret-key \
  -e AWS_REGION=us-east-1 \
  -e FB_ROOT=my-bucket \
  -e FB_AUTH_METHOD=none \
  -e FB_DEFAULT_SHARES='my-share=/catalog-sample' \
  ghcr.io/versioneer-tech/package-r:latest
```

Replace the example values, and add `AWS_ENDPOINT_URL` for another
S3-compatible service. See [Run packageR](https://package-r.versioneer.at/latest/how-to-guides/run-package-r/)
for the full configuration.

## Development

Local development uses `rclone serve s3` as a disposable S3-compatible API.
Production does not need this process.

### Use VS Code

Open **Run and Debug** and start **run packageR**. The launch configuration:

1. Starts local S3 on `127.0.0.1:19100`.
2. Builds the development backend.
3. Bootstraps `.vscode/package-r.db` and shares `/catalog-sample` as
   `my-share`.
   This directory contains the test catalog and its assets. The bucket root
   contains image, JSON, PDF, and text fixtures for manual checks.
4. Starts packageR on `127.0.0.1:8888` under the Go debugger.
5. Opens `http://localhost:8888` when the server is ready.

Stop the debug session to stop its local S3 server. Rclone output is in
`.vscode/local-s3.log`, and cache data is below `.vscode/cache`.

To control local S3 without VS Code, use:

```bash
./scripts/local_s3.sh serve
./scripts/local_s3.sh stop
```

`serve` stays in the foreground. The `run` command is for the VS Code task and
also stays active so that VS Code can manage its lifetime.

### Run tests

Run the Go unit tests and local integration tests:

```bash
make test-unit
make test-integration
```

The integration harness starts `rclone serve s3` on a loopback address. It
copies `tests/data` into a temporary test bucket and does not need cloud
credentials.

Run the local-only frontend tests with Playwright and Chromium:

```bash
make test-frontend
```

See [tests/README.md](tests/README.md) for the complete test strategy.

Each integration harness selects free ports, starts its dependencies, waits
for readiness, and stops the processes that it owns.

### Build the documentation

Build all pages and fail on warnings:

```bash
uv run mkdocs build --strict
```

You can also use `make docs`. The first run creates `.venv` and installs the
locked dependencies from `pyproject.toml`.

### Troubleshoot startup

If packageR does not open on port `8888`, check `.vscode/local-s3.log` and the
VS Code task output. Confirm that ports `19100`, `19101`, and `8888` are free.
Stop a previous local S3 process with:

```bash
./scripts/local_s3.sh stop
```

The launch fails if port `19100` has a different server or if the rclone
control endpoint on port `19101` is not available.

packageR is derived from
[File Browser](https://github.com/filebrowser/filebrowser). Keep packageR
changes narrow where practical so that upstream updates remain manageable.

## License

[Apache 2.0](LICENSE)
