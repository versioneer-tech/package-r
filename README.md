# <img src="frontend/public/img/logo.png" height="48" alt="packageR logo" /> packageR

[![CI & Trivy](https://img.shields.io/github/actions/workflow/status/versioneer-tech/package-r/pr.yaml?branch=main&event=push&label=CI%20%26%20Trivy)](https://github.com/versioneer-tech/package-r/actions/workflows/pr.yaml?query=branch%3Amain+event%3Apush)
[![Latest release](https://img.shields.io/github/v/release/versioneer-tech/package-r)](https://github.com/versioneer-tech/package-r/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/versioneer-tech/package-r)](go.mod)
[![License](https://img.shields.io/github/license/versioneer-tech/package-r)](LICENSE)

packageR lets you share files and folders from S3-compatible storage through
a web interface. Recipients open a share link in their browser without an
account. You can protect each share with a password.

The web interface supports directory navigation, uploads, downloads, and
object operations. It previews JSON, Markdown, PDF, common image formats,
and Cloud Optimized GeoTIFF (COG). Parquet catalogs are exposed as public
STAC JSON. Authenticated users and public shares can request time-limited
download URLs.

All users access storage through one S3 identity. Its storage policy controls
which buckets and objects packageR can access. User scopes, path rules, and
permissions further limit what each user can do.

See the [packageR documentation](https://package-r.versioneer.at/) for setup,
configuration, public packages, the HTTP API, and architecture decisions.

## Download and run

### Connect to S3

For an existing S3 service, set its connection details in the terminal where
you will start packageR:

```bash
export AWS_ACCESS_KEY_ID='<your-access-key>'
export AWS_SECRET_ACCESS_KEY='<your-secret-key>'
export AWS_REGION='<your-region>'
export AWS_ENDPOINT_URL='https://<your-s3-endpoint>'
```

For AWS S3, run `unset AWS_ENDPOINT_URL`. The examples use a bucket named
`data`; replace that name with your bucket.

Or start local S3 storage. Install [rclone](https://rclone.org/downloads/)
1.76 and run this in a separate terminal:

```bash
mkdir -p local-s3/data
rclone serve s3 ./local-s3 \
  --addr 127.0.0.1:19100 \
  --auth-key my-access-key,my-secret-key \
  --config /dev/null
```

Keep rclone running. The `local-s3/data` directory is the `data` bucket.
Put files there to make them available to packageR.

In the packageR terminal, set the local connection details:

```bash
export AWS_ACCESS_KEY_ID=my-access-key
export AWS_SECRET_ACCESS_KEY=my-secret-key
export AWS_REGION=us-east-1
export AWS_ENDPOINT_URL=http://127.0.0.1:19100
```

### Binary

The Linux AMD64 binary is built on Ubuntu 24.04. It needs glibc 2.38 or newer
and libstdc++ with `GLIBCXX_3.4.32` support. Use [Docker](#docker) on other hosts.

Download and unpack the latest stable release:

```bash
curl -fL -o package-r.tar.gz \
  https://github.com/versioneer-tech/package-r/releases/latest/download/linux-amd64-package-r.tar.gz
tar -xzf package-r.tar.gz
./package-r version
```

Create the initial user with explicit permissions and publish the `data`
bucket root:

```bash
./package-r config init --root=data
./package-r users add admin my-password \
  --perm.create=true \
  --perm.rename=true \
  --perm.modify=true \
  --perm.delete=true
./package-r shares add admin my-share /

./package-r
```

Open `http://127.0.0.1:8888` and sign in with the initial account created above.
The public package is available without an account at
`http://127.0.0.1:8888/share/my-share/`.

### Docker

The image includes the required libraries. Use the same image and volume for
setup and startup. The volume keeps the database and caches between runs.

This Linux example uses [host networking](https://docs.docker.com/engine/network/drivers/host/)
so packageR and the browser can reach S3 at the same address. Signed download
URLs also use this address.

```bash
image=ghcr.io/versioneer-tech/package-r:latest
docker pull "$image"
docker volume create package-r-state

docker run --rm -v package-r-state:/home/package-r "$image" \
  --database=/home/package-r/package-r.db config init --root=data
docker run --rm -v package-r-state:/home/package-r "$image" \
  --database=/home/package-r/package-r.db users add admin my-password \
  --perm.create=true --perm.rename=true --perm.modify=true --perm.delete=true
docker run --rm -v package-r-state:/home/package-r "$image" \
  --database=/home/package-r/package-r.db shares add admin my-share /

docker run --rm --name package-r \
  --network host \
  -v package-r-state:/home/package-r \
  -e PACKAGE_R_DATABASE=/home/package-r/package-r.db \
  -e PACKAGE_R_ADDRESS=127.0.0.1 -e PACKAGE_R_ROOT=data \
  -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_SESSION_TOKEN \
  -e AWS_REGION -e AWS_ENDPOINT_URL \
  "$image"
```

The browser and public-share URLs are the same as in the binary setup.
For hosted deployments, see the
[Kubernetes operator guide](https://package-r.versioneer.at/latest/how-to-guides/kubernetes/)
for an init-container example.

## Development

- [Contributing](CONTRIBUTING.md): issues and pull requests.
- [Tests and performance](tests/README.md): tools, test data, and Make commands.
- [Local development](.vscode/README.md): VS Code and STAC Browser.
- [Local CI checks](SECURITY.md#run-checks-locally): check selection and the full
  lint, test, candidate, and scan sequence.
- [Security and release checks](SECURITY.md): hooks, releases, dependency updates,
  and cleanup.

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
