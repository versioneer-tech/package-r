<h1><img src="frontend/public/img/logo.png" height="48" alt="packageR logo" /> packageR</h1>

[![Tests](https://github.com/versioneer-tech/package-r/actions/workflows/pr.yaml/badge.svg?branch=main&event=push)](https://github.com/versioneer-tech/package-r/actions/workflows/pr.yaml)
[![Latest release](https://img.shields.io/github/v/release/versioneer-tech/package-r)](https://github.com/versioneer-tech/package-r/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/versioneer-tech/package-r)](go.mod)
[![License](https://img.shields.io/github/license/versioneer-tech/package-r)](LICENSE)

> [!NOTE]
> packageR uses an embedded rclone VFS. It does not require an external FUSE
> mount.

packageR publishes a prefix of S3-compatible objects as a public package.
Recipients can open it in a browser without an account. An optional share
password controls access.

The Go service also provides a web interface for object storage. It can browse
objects and create time-limited download URLs. Production does not need a
separate rclone service.

packageR provides:

- public, browser-accessible packages for configured object prefixes, with no
  recipient login and an optional share password
- directory navigation and common object operations
- uploads, downloads, and access rules
- presigned GET URLs for authenticated files and public shares
- TIFF and Cloud Optimized GeoTIFF (COG) previews
- STAC-compatible Parquet catalogs exposed as public STAC JSON

All sessions use one S3 identity. Its storage policy sets the maximum access.
packageR can reduce access with user scopes, path rules, and action
permissions.

See the [packageR documentation](https://package-r.versioneer.at/) for setup,
configuration, public packages, the HTTP API, and architecture decisions.

## Download and run

Download and unpack the latest prebuilt Linux AMD64 release:

```bash
curl -fL -o package-r.tar.gz \
  https://github.com/versioneer-tech/package-r/releases/latest/download/linux-amd64-package-r.tar.gz
tar -xzf package-r.tar.gz
./package-r version
```

Create the initial user with explicit permissions and add a public share. This
example publishes the `catalog-sample` prefix from the `data` bucket:

```bash
./package-r config init
./package-r users add admin my-password \
  --perm.create=true \
  --perm.rename=true \
  --perm.modify=true \
  --perm.delete=true
./package-r shares add admin my-share /data/catalog-sample \
  --description="Open aerial imagery sample" \
  --catalog-name=catalog.parquet

AWS_ACCESS_KEY_ID=my-access-key \
AWS_SECRET_ACCESS_KEY=my-secret-key \
AWS_REGION=us-east-1 \
  ./package-r
```

If the AWS variables are already exported, run `./package-r` without repeating
them. Replace the example values, and add `AWS_ENDPOINT_URL` for another
S3-compatible service.

Open `http://127.0.0.1:8888` and sign in with the initial account created above.
The public package is available without an account at
`http://127.0.0.1:8888/share/my-share/`.

We also provide a container image at
`ghcr.io/versioneer-tech/package-r:latest`. Its entrypoint starts packageR
directly and expects an initialized database. For hosted deployments, prepare
the database in a separate process before the application starts. See the
[Kubernetes operator guide](https://package-r.versioneer.at/latest/how-to-guides/kubernetes/)
for an init-container example.

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

`serve` stays in the foreground. The `dev` command is for the VS Code task and
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

## Thanks

packageR is based on the now-deprecated
[File Browser](https://github.com/filebrowser/filebrowser) project and still
uses many of its dependencies. It relies heavily on
[rclone](https://rclone.org/) and on tools and standards from the
[Cloud-Native Geospatial Foundation](https://cloudnativegeo.org/) ecosystem.

packageR is used in the
[EOEPCA Reference Architecture](https://eoepca.readthedocs.io/projects/architecture/en/latest/),
an [ESA](https://www.esa.int/)-supported approach to building Earth
observation platforms. packageR helps Earth observation and Earth science
communities inspect, share, and use data assets.

## License

[Apache 2.0](LICENSE)
