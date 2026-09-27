# HTTP API

The packageR API supports object operations, public packages, presigned URLs,
and STAC catalogs. All API paths start with `/api`.

Examples use this base URL:

```bash
PACKAGE_R_URL=http://127.0.0.1:8888
```

URL-encode object path segments. Keep `/` as the path separator. Errors return
an HTTP status code and a short text message.

The [OpenAPI 3.1 specification](openapi.yaml) provides the supported API in a
machine-readable format.

## Authentication

packageR supports two authentication methods. Both methods use `/api/login`
and return a signed packageR session token.

### Username and password

The default `json` method accepts a JSON body:

```bash
PACKAGE_R_TOKEN=$(curl -fsS \
  -X POST \
  -H 'Content-Type: application/json' \
  --data '{"username":"my-user","password":"my-password"}' \
  "$PACKAGE_R_URL/api/login")
```

### Proxy authentication

The `proxy` method reads the configured identity header. The header can
contain a trusted username or a JWT.

For a trusted username header:

```bash
PACKAGE_R_TOKEN=$(curl -fsS \
  -X POST \
  -H 'X-Username: my-user' \
  "$PACKAGE_R_URL/api/login")
```

The reverse proxy must remove any client-supplied value before it sets the
trusted header.

For a validated JWT:

```bash
PACKAGE_R_TOKEN=$(curl -fsS \
  -X POST \
  -H 'Authorization: Bearer <identity-token>' \
  "$PACKAGE_R_URL/api/login")
```

The proxy authentication settings select the header, username claim, and JWT
validation rules. The identity JWT is only the login credential. A successful
login returns a separate packageR session token. See
[Configuration](../how-to-guides/configuration.md#authentication-and-authorization)
for the required proxy settings.

Send the packageR session token with authenticated API requests:

```bash
curl -fsS \
  -H "X-Auth: $PACKAGE_R_TOKEN" \
  "$PACKAGE_R_URL/api/resources/"
```

| Method | Path | Result |
| --- | --- | --- |
| `POST` | `/api/login` | Authenticate with the configured method and return a packageR session token. |
| `POST` | `/api/renew` | Replace a valid packageR session token. |

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

The request needs `X-Auth` and permission for the action. These query values
control common operations:

- `checksum=md5|sha1|sha256|sha512` calculates one checksum
- `presign=true` adds a GET `presignedURL` to file metadata
- `followRedirect=true` with `presign=true` returns `307 Temporary Redirect`
- `override=true` permits replacement when the action allows it
- `action=copy|rename` with a URL-encoded `destination` copies or moves an
  object in a `PATCH` request

Presigned object URLs send file content directly from object storage. They are
valid for at most seven days.

## Chunked uploads

Use `/api/tus/{path}` for resumable uploads through the rclone VFS write
cache.

| Method | Purpose |
| --- | --- |
| `POST` | Create the upload target. Use `override=true` to replace an existing file. |
| `HEAD` | Return the current byte position in `Upload-Offset`. |
| `PATCH` | Append `application/offset+octet-stream` data at `Upload-Offset`. |
| `DELETE` | Abort the upload and remove the partial object. |

The web interface sends `DELETE` when a user cancels an upload. A successful
request removes the partial object. If a `PATCH` is interrupted or `DELETE`
fails, the partial object can remain. An API client can use `HEAD` to find its
offset and continue with `PATCH`, or delete it.

## Shares

Shares are declared when the runtime database is prepared. Authenticated users
can list the public shares available in their scope:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/shares` | Return safe, read-only public share links. |

The response includes the source root, path, expiry, description, catalog,
asset mappings, and whether the share has a password.

## Public shares

Share resources do not require a packageR session token.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` or `HEAD` | `/api/public/share/{hash}/{path}` | List or inspect a shared resource. |

For a protected share, send the URL-encoded share password in
`X-SHARE-PASSWORD` with each request. No preliminary request is necessary.

```bash
curl -fsS \
  -H 'X-SHARE-PASSWORD: my-password' \
  "$PACKAGE_R_URL/api/public/share/my-share/"
```

Public file metadata supports the same checksum and presign query values as an
authenticated resource. With `presign=true` and `follow=true`, the response is
a `307 Temporary Redirect` to object storage. Public shares do not provide a
packageR download or directory archive endpoint.

## Public catalogs

For a share with a Parquet catalog, use these paths to get the complete catalog
or the entries below a package path:

```text
/api/public/catalog/{hash}
/api/public/catalog/{hash}/{path}
```

The route accepts `GET` and `HEAD`. For a protected share, send
`X-SHARE-PASSWORD` with each request. One matching row returns a STAC
`Feature`. Zero or multiple rows return a versioned `FeatureCollection` with a
`links` array.
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

Asset mappings map nonstandard URL prefixes to paths inside a share. Each
mapping contains `from` and `to` strings. The `to` path must be relative and
stay inside the share.

Catalog queries have these requirements:

- The catalog must be inside the shared tree
- DuckDB reads a signed URL with HTTP range requests and does not stage the
  catalog on local disk
- The internal catalog URL is valid for at most 15 minutes
