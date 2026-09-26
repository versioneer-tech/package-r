# HTTP API

The packageR API supports object operations, public packages, presigned URLs,
and STAC catalogs. All API paths start with `/api`.

Examples use this base URL:

```bash
PACKAGE_R_URL=http://127.0.0.1:8888
```

URL-encode object path segments. Keep `/` as the path separator. Errors return
an HTTP status code and a short text message.

## Authentication

With proxy authentication, send the trusted identity header to `/api/login`:

```bash
PACKAGE_R_TOKEN=$(curl -fsS \
  -X POST \
  -H 'X-Username: my-user' \
  "$PACKAGE_R_URL/api/login")
```

The reverse proxy must remove a client value before it sets this header. The
login response is a signed packageR token. Send the token in `X-Auth`.

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

The request needs `X-Auth` and permission for the action. These query values
control common operations:

- `checksum=md5|sha1|sha256|sha512` calculates one checksum
- `presign=true` adds a GET `presignedURL` to file metadata
- `followRedirect=true` with `presign=true` returns `307 Temporary Redirect`
- `override=true` permits replacement when the action allows it
- `action=copy|rename` with a URL-encoded `destination` copies or moves an
  object in a `PATCH` request

Presigned object URLs are valid for at most seven days. Directory downloads
support `zip`, `tar`, `targz`, `tarbz2`, `tarxz`, `tarlz4`, and `tarsz` through
the `algo` query value.

## Configured packages

An authenticated user can list public packages that contain an object path:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/share/{path}` | Return read-only public package links for the path. |

The request needs `X-Auth` and access to the object path. The response includes
the package name, description, expiry, and public URL. It does not include
password hashes or access tokens. Configure packages with `package-r shares`.

## Public shares

Public shares are declared when the runtime database is prepared. They do not
need a packageR token.

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

The Parquet catalog does not need full STAC GeoParquet compliance. Each row
must contain:

1. `id`
2. `assets` as a JSON object whose entries contain `href`
3. `datetime` or another valid STAC temporal range, either flattened or in
   `properties`

Rows can also contain `geometry`, `bbox`, `properties`, and `repository`.
packageR moves flattened Item metadata into `properties`, supplies the STAC
version and required empty link arrays, and derives missing geometry from
`bbox` when possible.

packageR checks all assets when it selects rows. Matching `href` values become
public share URLs with `?presign&followRedirect`. Other absolute URLs stay
unchanged. [ADR-002](../architecture/0002-publish-parquet-catalogs-as-stac.md)
defines the matching rules.

The `package-r shares add --asset-mappings` option maps nonstandard URL
prefixes to paths inside that share. Its value is a JSON array of `from` and
`to` strings. Each `to` path must be relative and stay inside the share.

Catalog queries have these limits:

- The catalog must be inside the shared tree
- One catalog cannot exceed 256 MiB
- At most four catalog queries can run at the same time
- DuckDB reads a signed URL with HTTP range requests and does not stage the
  catalog on local disk
- The internal catalog URL is valid for at most 15 minutes
