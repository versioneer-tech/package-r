# Package, share, and preview data

packageR turns an object prefix into a browser-accessible package. Recipients
can browse and download the package without an account or login. Public
packages are deployment configuration. Users can view configured package links
in the web interface, but they cannot create, change, or delete them there.

## Package an object prefix

Set `FB_DEFAULT_SHARES` before packageR starts. Each entry maps a public package
name to an object path. When `FB_ROOT` selects one bucket, the object path is a
prefix in that bucket:

```bash
export FB_DEFAULT_SHARES='public=/deliverables/26-06;f4ae91c8d2=/reports/report.tif'
./init.sh --serve
```

A prefix package lets a public visitor navigate below the configured path. A
single-object package opens that object. The visitor cannot navigate above the
configured path.

The package name becomes part of its URL. It can be a simple name such as
`public`, a hard-to-guess value, or another deployment-specific name. It must
contain 1 to 20 lowercase letters, digits, dots, or hyphens.

A hard-to-guess name can reduce accidental discovery, but it is not an access
control. Add a share password when recipients must authenticate before they
open the package.

![Public share directory](../imgs/screenshots/my-share-directory.png)

## Protect a package with a share password

Map a configured share to a share password with `FB_DEFAULT_SHARE_PINS`:

```bash
export FB_DEFAULT_SHARE_PINS='f4ae91c8d2=1234'
```

The browser asks for this share password before it opens the package. API
clients send it in the `X-SHARE-PASSWORD` header. Inject share-password values
from a deployment secret and do not write them to logs.

## Use a direct object URL

For an authenticated file or a public shared file, select **Show** next to
**Presigned URL**. packageR creates a direct S3 GET URL. This avoids routing
the object body through packageR.

The API can also return a `307 Temporary Redirect` to the direct URL. See the
[HTTP API](../reference-guides/http-api.md#resources).

## Preview a COG

Open a TIFF or GeoTIFF object from the file list or public share. The viewer
uses the presigned URL and byte-range requests to load the required image
parts.

If the viewer does not load, ask the operator to check:

- S3 CORS access for the packageR origin;
- `GET`, `HEAD`, and the `Range` request header;
- exposed range and object metadata response headers; and
- support for HTTP `206 Partial Content` through any storage proxy.

![Authenticated image preview](../imgs/screenshots/authenticated-image-preview.png)

## Publish a STAC-compatible Parquet catalog

Place the Parquet catalog inside the directory that you share. Each row must
contain `id` and an `assets` JSON object with `href` values. Rows can also
contain `geometry`, `bbox`, `properties`, and `repository`. The catalog does
not need full STAC GeoParquet compliance.

Relative asset paths are recommended. packageR resolves them from the shared
directory. It also detects root-relative paths and absolute HTTP(S) or `s3://`
URLs when their object path contains the shared path. It rewrites these assets
to the public share endpoint. It leaves unrelated absolute URLs unchanged, so
externally hosted assets keep their original URL.

packageR checks every asset in a row. You do not have to select one asset key
for package matching. A request below the catalog endpoint selects a row when
any internal asset is at that path or below it.

Most catalogs do not need asset mapping configuration. If a CDN URL does not
contain the shared object path, set `FB_CATALOG_ASSET_MAPPINGS` to a JSON array.
Each entry replaces an exact URL prefix with a path relative to the share:

```yaml
FB_CATALOG_ASSET_MAPPINGS: >-
  [{"from":"https://imagery.example.org/openaerialmap/","to":"openaerialmap-assets"}]
```

For this example,
`https://imagery.example.org/openaerialmap/67793f0b9478720001790586/thumbnail.png`
maps to
`openaerialmap-assets/67793f0b9478720001790586/thumbnail.png` inside each
configured share. This layout matches the included OpenAerialMap test data.
The `to` value must be relative and cannot leave the share. Explicit mappings
take priority over automatic matching.

Set `FB_CATALOG_DEFAULT_NAME` to the catalog's relative path before bootstrap.
The value applies to each configured share. The default is `catalog.parquet`.

For a share named `my-share`, request the full catalog:

```text
/api/public/catalog/my-share
```

Add a path below the share to select matching catalog entries:

```text
/api/public/catalog/my-share/path/to/package
```

The response is a STAC `Feature` or versioned `FeatureCollection`. packageR
rewrites matching asset links to public share URLs that can redirect to
presigned S3 URLs.

Use the absolute endpoint URL as the external catalog URL in a STAC Browser.
Set `FB_CATALOG_PREVIEW_URL` to the browser's external-catalog prefix so that
packageR also provides a **Preview URL** that opens the current package path in
that browser.

See the [HTTP API](../reference-guides/http-api.md#public-catalogs) for the
contract and limits. See
[ADR-002](../architecture/0002-publish-parquet-catalogs-as-stac.md) for the
catalog and asset-resolution design.
