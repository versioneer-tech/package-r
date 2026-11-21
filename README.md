# <img src="frontend/public/img/logo.png" height="48" alt="packageR logo" /> packageR

[![CI & Trivy](https://img.shields.io/github/actions/workflow/status/versioneer-tech/package-r/pr.yaml?branch=main&event=push&label=CI%20%26%20Trivy)](https://github.com/versioneer-tech/package-r/actions/workflows/pr.yaml?query=branch%3Amain+event%3Apush)
[![Latest release](https://img.shields.io/github/v/release/versioneer-tech/package-r)](https://github.com/versioneer-tech/package-r/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/versioneer-tech/package-r)](go.mod)
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

The prebuilt Linux AMD64 binary needs Ubuntu 24.04 or compatible C/C++ libraries.
Download and unpack the latest release:

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

- [Tests and performance](tests/README.md): tools, test data, and Make commands.
- [Local development](.vscode/README.md): VS Code and STAC Browser.
- [Security and release checks](SECURITY.md): hooks, CI, dependency updates, and cleanup.

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
observation platforms.

## License

[Apache 2.0](LICENSE)
