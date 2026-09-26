# Configuration

packageR reads bootstrap settings from environment variables. Object-storage
credentials and the endpoint belong to the process. `init.sh` does not store
them in the packageR database or user records.

## Object storage

These values configure rclone VFS and presigned links.

| Variable | Required | Description |
| --- | --- | --- |
| `AWS_ACCESS_KEY_ID` | With static credentials | S3 access key. Set it with `AWS_SECRET_ACCESS_KEY`. |
| `AWS_SECRET_ACCESS_KEY` | With static credentials | S3 secret key. Set it with `AWS_ACCESS_KEY_ID`. |
| `AWS_SESSION_TOKEN` | For temporary static keys | Optional session token. |
| `AWS_ROLE_ARN` | Injected for web identity | Role used by workload identity, including AWS IRSA. |
| `AWS_WEB_IDENTITY_TOKEN_FILE` | Injected for web identity | Path to the projected workload token. |
| `AWS_ROLE_SESSION_NAME` | No | Optional web-identity role session name. |
| `PACKAGE_R_ROOT` | No | `/` for the S3 service root, or one bucket name. The default is `/`. |
| `AWS_ENDPOINT_URL` | For custom endpoints | S3-compatible API endpoint. If empty, rclone uses AWS S3. |
| `AWS_REGION` | Service dependent | S3 region. |

Choose one credential mode:

- For workload identity, leave `AWS_ACCESS_KEY_ID` and
  `AWS_SECRET_ACCESS_KEY` unset. rclone uses its S3 credential chain. This can
  use AWS IRSA, a container identity, an instance identity, or a shared
  credential file.
- For static credentials, set both key variables. Set `AWS_SESSION_TOKEN` for
  temporary credentials. Static credentials take priority.

Startup fails when only one static key variable is set. Do not put projected
tokens or static credentials in packageR configuration, logs, or the Bolt
database.

The service keeps one rclone VFS instance. User settings cannot change its
endpoint, credentials, or root. The credential provider refreshes temporary
credentials when the selected identity method supports refresh.

Chunked and seek-based uploads use the rclone VFS write cache. The cache must
be writable and have enough free space. On Linux, it is normally at
`$HOME/.cache/rclone` or below `$XDG_CACHE_HOME`. Storage initialization fails
if the cache is not available.

DuckDB uses `httpfs` to read catalog URLs. The first query installs the
extension if it is not cached. Its cache is under
`package-r/duckdb-extensions` in the user cache directory. Catalog storage
must support ranged `GET` requests and may receive `HEAD` requests.

With `PACKAGE_R_ROOT=/`, the top-level entries are buckets. The credentials
need permission to list buckets. On AWS, this is
`s3:ListAllMyBuckets`. Opening and listing a bucket needs the corresponding
bucket-list permission, such as AWS `s3:ListBucket`. Object actions need the
matching provider permissions.

With `PACKAGE_R_ROOT=my-bucket`, `/` in packageR is the root of `my-bucket`.
packageR does not need permission to list all buckets. It still needs
`s3:ListBucket` and the required object permissions for `my-bucket`.

Browser COG previews use presigned object URLs. Configure S3 CORS for the
packageR and viewer origins. Allow `GET`, `HEAD`, and the `Range` request
header. Expose `Accept-Ranges`, `Content-Range`, `Content-Length`, and `ETag`.
Storage proxies must preserve range requests and `206` responses.

## Application state

| Variable | Description |
| --- | --- |
| `PACKAGE_R_DATABASE` | Temporary Bolt database path. The default is `/tmp/package-r.db`. |
| `PACKAGE_R_SERVER_PORT` | HTTP port. The default is `8888`. |
| `PACKAGE_R_DEFAULT_SHARES` | Semicolon-separated `hash=path` shares to create during bootstrap. |
| `PACKAGE_R_DEFAULT_SHARE_PASSWORDS` | Optional semicolon-separated `hash=password` values for configured shares. |

`init.sh --add-shares hash=path` adds shares for one bootstrap run.
`--add-share-passwords hash=password` adds their passwords. Do not put
password values in logs. Read them from a secret when possible. Use
`init.sh --serve` to start the service after bootstrap.

Keep `PACKAGE_R_DATABASE` on temporary storage. `init.sh` rebuilds this runtime
state. Define public shares and their passwords in bootstrap configuration.

## Authentication and user scope

| Variable | Description |
| --- | --- |
| `PACKAGE_R_AUTH_METHOD` | `proxy` for a trusted identity header, or `none` for a controlled deployment. The default is `proxy`. |
| `PACKAGE_R_AUTH_HEADER` | Proxy header that contains the user identity. The default is `X-Username`. |
| `PACKAGE_R_AUTH_MAPPER` | Empty for the raw header, `.<claim>` for a JSON or JWT claim, or a fixed username. |
| `PACKAGE_R_AUTH_JWT_JWKS_URL` | JWKS URL for strict validation of JWT proxy headers. |
| `PACKAGE_R_AUTH_JWT_ISSUER` | Required issuer when JWKS validation is enabled. |
| `PACKAGE_R_AUTH_JWT_AUDIENCE` | Optional expected JWT audience. |
| `PACKAGE_R_AUTH_JWT_ALGORITHMS` | Allowed JWT algorithms. The default is `RS256`. |
| `PACKAGE_R_AUTH_JWT_CLOCK_SKEW` | Allowed JWT clock difference. The default is `1m`. |
| `PACKAGE_R_CREATE_USER_DIR` | Protects `/home/<username>` from sibling non-admin users. The default is `false`. Requires one bucket in `PACKAGE_R_ROOT`. |

When `PACKAGE_R_CREATE_USER_DIR=true`, packageR protects new and existing
non-admin users. A user can open `/home` and their own home directory. Sibling
homes are hidden. The user cannot change `/home` or an ancestor because a
recursive action could change another user's data.

The default user scope is `/`. Other rules can allow access outside `/home`.
An explicit scope limits a user to that path. Global settings can change the
home base from `/home` to another path, such as `/users`.

packageR refuses to start when `PACKAGE_R_CREATE_USER_DIR=true` and `PACKAGE_R_ROOT=/`.
An S3 service root contains buckets, so `/home/<username>` cannot be a user
directory at that level.

Proxy authentication trusts the configured identity header. The reverse proxy
must remove a client value before it sets this header. To validate a JWT, set
`PACKAGE_R_AUTH_JWT_JWKS_URL` and an issuer. Without a JWKS URL, claim mapping
only decodes the trusted proxy value.

## How access is calculated

packageR applies these access controls in order:

1. The S3 credentials and their storage policy define the maximum access.
2. `PACKAGE_R_ROOT` selects all visible buckets or one bucket.
3. The user scope selects a subtree below that root. Paths cannot escape it.
4. Hidden-file handling, global rules, and user rules filter normalized paths.
   Matching rules are evaluated in order, and the last matching rule wins.
5. User-directory mode denies sibling homes and protects their shared parent
   from recursive changes. This boundary cannot be overridden by a user rule.
6. User permission flags control actions such as download, create, rename,
   modify, and delete.

An admin bypasses hidden-file and application-rule checks. An admin does not
bypass the storage policy, `PACKAGE_R_ROOT`, or their stored scope.

By default, user-directory mode is off and new non-admin users have scope `/`.
Permission flags still control their actions. User-directory mode hides
sibling homes but does not limit a root-scoped user to their home.
`PACKAGE_R_ALLOW_CHANGING=false` prevents create, delete, modify, and rename
actions. When it is `true`, these actions are still denied for sibling homes
and the shared home directory.

## Permissions and catalogs

| Variable | Description |
| --- | --- |
| `PACKAGE_R_ALLOW_CHANGING` | Enables create, delete, modify, and rename for default non-admin users. The default is `false`. |
| `PACKAGE_R_CATALOG_DEFAULT_NAME` | Relative Parquet catalog path inside a share. The default is `catalog.parquet`. |
| `PACKAGE_R_CATALOG_PREVIEW_URL` | Optional external viewer prefix for public catalog preview links. |
| `PACKAGE_R_CATALOG_ASSET_MAPPINGS` | Optional JSON array of `from` URL prefixes and relative `to` paths. |

Catalog names must stay inside the shared path. Absolute names and parent-path
escapes are invalid. Most catalogs do not need asset mappings.

## Bootstrap behavior

`init.sh` applies these settings:

- disables commands and command execution
- disables generated thumbnails and server-side preview resize
- creates or updates the runtime configuration
- creates configured public shares and applies their passwords
- disables authenticated share creation and deletion

The selected credential source must be available to packageR. A successful
bootstrap does not prove that the server can connect to S3.

## Runtime limits

- Presigned URLs support GET and have a maximum lifetime of seven days
- `/api/usage` returns `501 Not Implemented`
- Object rename and move can use copy and delete operations

See the [HTTP API](../reference-guides/http-api.md) for route behavior.
