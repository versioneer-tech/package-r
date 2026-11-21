# <img src="frontend/public/img/logo.png" height="48" alt="packageR logo" /> packageR

[![CI & Trivy](https://img.shields.io/github/actions/workflow/status/versioneer-tech/package-r/pr.yaml?branch=main&event=push&label=CI%20%26%20Trivy)](https://github.com/versioneer-tech/package-r/actions/workflows/pr.yaml?query=branch%3Amain+event%3Apush)
[![Latest release](https://img.shields.io/github/v/release/versioneer-tech/package-r)](https://github.com/versioneer-tech/package-r/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/versioneer-tech/package-r)](go.mod)
[![Dependabot configured](https://img.shields.io/badge/Dependabot-configured-0366d6?logo=dependabot)](.github/dependabot.yml)
[![License](https://img.shields.io/github/license/versioneer-tech/package-r)](LICENSE)

packageR publishes a prefix of S3-compatible objects as a public package.
Recipients can open it in a browser without an account. An optional share
password controls access.

The web interface supports directory navigation, uploads, downloads, and
object operations. It previews JSON, Markdown, PDF, common image formats,
and Cloud Optimized GeoTIFF (COG). Parquet catalogs are exposed as public
STAC JSON. Authenticated users and public shares can request time-limited
download URLs.

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

Use an existing bucket named `data`, or replace that name below. Create the
initial user with explicit permissions and publish the bucket root:

```bash
./package-r config init --root=data
./package-r users add admin my-password \
  --perm.create=true \
  --perm.rename=true \
  --perm.modify=true \
  --perm.delete=true
./package-r shares add admin my-share /

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

Use Go, a C compiler, Node.js, pnpm, uv, and Python 3.11 or newer.
Go checks need CGO for DuckDB. Install the hooks once per checkout, then run
the shared lint and documentation checks:

```bash
make hooks
make check
make docs
```

Hooks run Go and frontend linters, file validation, markdownlint, Hadolint,
and Gitleaks. Secret checks scan staged file contents and redact findings.
Commit messages use Conventional Commits, such as `fix: handle empty catalogs`.
The push hook checks the commit messages in the pushed range.

CI runs the same lint commands, Go unit and race tests, local rclone API tests,
documentation checks, govulncheck, and Trivy. Documentation-only changes run
file, tooling, and documentation checks. Manual CI runs use the full suite.
Playwright and timed load tests run through explicit local commands.
Hooks do not run tests or build the app.

Dependabot checks Docker images daily. It checks Go, frontend, Actions, hooks,
and Python support dependencies weekly.

See [the test guide](tests/README.md) for test commands and fixture setup, and
[release security checks](SECURITY.md#release-checks) for scans,
SBOMs, and the publication gate.

### Use VS Code

Generate the Sentinel-2 fixture with the command in the test guide. Open
**Run and Debug** and start **run packageR**. It starts local S3, builds the
backend, prepares `.vscode/package-r.db`, and shares `vienna-s2l2a-26`. It then
starts the Go debugger and opens `http://localhost:8888`.

The VS Code launch configuration allows cross-origin requests from all origins
for local development. Production CORS remains disabled unless it is configured
explicitly.

Stop the debug session to stop its local S3 server. Rclone output is in
`.vscode/local-s3.log`, and cache data is below `.vscode/cache`. If startup
fails, check that log, the task output, and ports `19100` (S3), `19101`
(rclone control), and `8888` (packageR).

To control local S3 without VS Code, use:

```bash
./scripts/local_s3.sh serve
./scripts/local_s3.sh stop
```

`serve` stays in the foreground.

## Thanks

The OpenAerialMap sample images are licensed under
[CC BY 4.0](https://creativecommons.org/licenses/by/4.0/).

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
