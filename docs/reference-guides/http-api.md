# HTTP API

packageR exposes an HTTP API for object operations, configured public shares,
presigned URLs, and STAC catalog responses. API paths start with `/api`.

Examples use this base URL:

```bash
PACKAGE_R_URL=http://127.0.0.1:8888
```

Encode object path segments as URL data, but keep `/` as the path separator.
Errors use an HTTP status code and a short plain-text response.

## Authentication

With proxy authentication, send the trusted identity header to `/api/login`:

```bash
PACKAGE_R_TOKEN=$(curl -fsS \
  -X POST \
  -H 'X-Username: my-user' \
  "$PACKAGE_R_URL/api/login")
```

The reverse proxy must remove client-supplied copies of this header before it
sets the authenticated identity. The login response is a signed packageR
token. Send it in `X-Auth` for authenticated requests.

| Method | Path | Result |
| --- | --- | --- |
| `POST` | `/api/login` | Return a token for a trusted proxy identity. |
| `POST` | `/api/renew` | Replace a valid packageR token. |

## Resources

Resource paths are below the authenticated identity's scope and the configured
storage root.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/resources/{path}` | List a directory or get file metadata. |
| `POST` | `/api/resources/{path}` | Upload a file. A trailing `/` creates a directory. |
| `PUT` | `/api/resources/{path}` | Replace an existing file. |
| `PATCH` | `/api/resources/{path}` | Copy or move an object. |
| `DELETE` | `/api/resources/{path}` | Delete a file or directory tree. |
| `GET` | `/api/raw/{path}` | Download a file or directory archive. |

The request needs `X-Auth` and permission for the action. Important query
values are:

- `checksum=md5|sha1|sha256|sha512` calculates one checksum.
- `presign=true` adds a GET `presignedURL` to file metadata.
- `followRedirect=true` with `presign=true` returns `307 Temporary Redirect`.
- `override=true` permits replacement when the action allows it.
- `action=copy|rename` with a URL-encoded `destination` copies or moves an
  object in a `PATCH` request.

Presigned object URLs are valid for at most seven days. Directory downloads
support `zip`, `tar`, `targz`, `tarbz2`, `tarxz`, `tarlz4`, and `tarsz` through
the `algo` query value.

## Configured packages

An authenticated user can discover the public packages that contain an object
path:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/share/{path}` | Return read-only public package links for the path. |

The request needs `X-Auth` and access to the object path. The response contains
only the package name, description, expiry, and public URL. It does not expose
share-password hashes or access tokens. Create, update, and delete methods are
not available. Configure packages through `init.sh`.

## Public shares

Public shares are declared during bootstrap. They do not need a packageR
token.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` or `HEAD` | `/api/public/share/{hash}/{path}` | List or inspect a shared resource. |
| `GET` | `/api/public/dl/{hash}/{path}` | Download a shared file or directory archive. |

For a share protected by a share password, send the URL-encoded share password
in `X-SHARE-PASSWORD`. A successful response contains a temporary share token.
Public file metadata supports the same checksum and presign query values as
an authenticated resource.

## Public catalogs

Request a share's complete catalog or select entries below a package path:

```text
/api/public/catalog/{hash}
/api/public/catalog/{hash}/{path}
```

The route accepts `GET` and `HEAD`. Shares protected by a share password use
`X-SHARE-PASSWORD`. One matching row returns a STAC `Feature`; zero or
multiple rows return a versioned `FeatureCollection` with a `links` array.
Responses use the `application/geo+json` media type.

The STAC-compatible Parquet catalog does not need full STAC GeoParquet
compliance. Each row must contain:

1. `id`;
2. `assets` as a JSON object whose asset entries contain `href`; and
3. `datetime` or another valid STAC temporal range, either flattened or in
   `properties`.

Rows can also contain `geometry`, `bbox`, `properties`, and `repository`.
packageR moves flattened Item metadata into `properties`, supplies the STAC
version and required empty link arrays, and derives missing geometry from
`bbox` when possible.

packageR checks all assets when it selects catalog rows. Matching asset `href`
values become public share URLs with `?presign&followRedirect`. This includes
relative paths, root-relative paths, and absolute HTTP(S) or `s3://` URLs whose
object path contains the shared path. Other absolute URLs remain unchanged.

`FB_CATALOG_ASSET_MAPPINGS` can map a nonstandard URL prefix to a relative path
inside the share. The value is a JSON array of `from` and `to` strings.
Mappings take priority over automatic matching. The `to` path cannot be
absolute or contain a parent-path escape.

Catalog queries have these limits:

- The catalog must be inside the shared tree.
- One catalog cannot exceed 256 MiB.
- At most four catalog queries can run at the same time.
- DuckDB reads a signed URL with HTTP range requests and does not stage the
  catalog on local disk.
- The internal catalog URL is valid for at most 15 minutes.
