<img src="frontend/public/img/logo.png" height="40"/>

# packageR

`packageR` is a File Browser-derived application designed for data that lives in object
storage but is made visible to packageR as a regular file tree (e.g. via FUSE).

packageR uses that filesystem view for browsing, inspection, and curation, then
adds presigned URL access, streaming previews, public STAC catalog endpoints,
and more. See the [feature summary](https://package-r.versioneer.at/latest/features/)
and follow the walkthroughs for
[public sharing](https://package-r.versioneer.at/latest/generated/usecases/public-sharing/)
and [catalog handling via STAC](https://package-r.versioneer.at/latest/generated/usecases/catalog-stac/).

Full documentation is available at
[package-r.versioneer.at](https://package-r.versioneer.at/), including the necessary
[configuration](https://package-r.versioneer.at/latest/configuration/) for runtime
environment variables and bootstrap settings.

## Getting Started

Build the local `filebrowser` binary from the repository root:

```bash
make build
```

Start packageR with a public share and local test data:

```bash
export FB_ROOT="${FB_ROOT:-/workspace}"
export FB_DATABASE="${FB_DATABASE:-/db/bolt.db}"

./init.sh --add-shares public-share=/public --add-test-data /public --serve
```

Local commands run from the current repository directory. The Docker image uses
the same mounted data and database paths, with `/home/package-r` as the runtime
home directory. Outside Docker, make sure `/workspace` and `/db` exist and are
writable by the user running packageR.

In another terminal, check the public package and STAC catalog endpoints:

```bash
BASE_URL="${BASE_URL:-http://127.0.0.1:${FB_SERVER_PORT:-8888}}"
ITEM_ID=67793f0b9478720001790586

curl -sS "$BASE_URL/api/public/catalog/public-share"
curl -sS "$BASE_URL/api/public/catalog/public-share/openaerialmap-assets/$ITEM_ID/thumbnail.png"
curl -sSI "$BASE_URL/api/public/share/public-share/openaerialmap-assets/$ITEM_ID/thumbnail.png?presign=true&followRedirect=true"
```

The local sample is not backed by object storage, so presign checks return
packageR/File Browser URLs. See the
[Quickstart](https://package-r.versioneer.at/latest/generated/usecases/quickstart/)
docs page for the full walkthrough.

## Contributing

packageR is fork-derived from
[File Browser](https://github.com/filebrowser/filebrowser). We aim to stay
aligned with upstream where practical, so packageR-specific changes should stay
narrow and easy to rebase.

Contributors should base work on the current `main` branch and rebase changes
before opening a pull request.

Run the backend tests from the repository root:

```bash
go test -v ./...
```

For documentation changes, build the docs locally:

```bash
uv run --with-requirements docs/requirements.txt mkdocs build --strict
```

## License

[Apache 2.0](LICENSE) (Apache License Version 2.0, January 2004) from https://www.apache.org/licenses/LICENSE-2.0
