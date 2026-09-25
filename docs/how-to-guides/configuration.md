# Configuration

packageR reads bootstrap settings from environment variables. Object-storage
credentials and the endpoint are process-owned. `init.sh` does not copy them
into the File Browser database or identity records.

## Object storage

These values configure both rclone VFS file access and rclone public links.

| Variable                      | Required                  | Description                                                                      |
| ----------------------------- | ------------------------- | -------------------------------------------------------------------------------- |
| `AWS_ACCESS_KEY_ID`           | With static credentials   | S3 access key. Set it together with `AWS_SECRET_ACCESS_KEY`.                      |
| `AWS_SECRET_ACCESS_KEY`       | With static credentials   | S3 secret key. Set it together with `AWS_ACCESS_KEY_ID`.                          |
| `AWS_SESSION_TOKEN`           | For temporary static keys | Optional session token.                                                          |
| `AWS_ROLE_ARN`                | Injected for web identity | Role used by workload identity, including AWS IRSA.                              |
| `AWS_WEB_IDENTITY_TOKEN_FILE` | Injected for web identity | Path to the projected workload token.                                            |
| `AWS_ROLE_SESSION_NAME`       | No                        | Optional web-identity role session name.                                         |
| `FB_ROOT`                     | No                        | `/` for the S3 service root, or one bucket name without `/`. The default is `/`. |
| `AWS_ENDPOINT_URL`            | For custom endpoints      | S3-compatible API endpoint. If this value is empty, rclone uses AWS S3.          |
| `AWS_REGION`                  | Service dependent         | S3 region.                                                                       |

Choose one credential mode:

- For provider-supported workload identity, leave `AWS_ACCESS_KEY_ID` and
  `AWS_SECRET_ACCESS_KEY` unset. rclone uses its S3 ambient credential chain.
  Depending on the provider and runtime, this can use web identity such as AWS
  IRSA, container or instance identities, or a shared credential file. The
  workload platform normally injects the required settings and token.
- For static credentials, set both key variables. Set `AWS_SESSION_TOKEN` for
  temporary keys. A static pair takes precedence over the ambient chain.

Startup fails when only one static key variable is set. Do not put projected
tokens or static credentials in packageR configuration, logs, or the Bolt
database.

The service keeps one rclone VFS instance for the process-owned storage
configuration. `FB_ROOT` in the active server configuration is authoritative.
Identity settings cannot change the object-storage endpoint, credentials, or
root. The ambient credential provider refreshes temporary credentials for the
shared backend when the configured identity mechanism supports refresh.

Chunked and seek-based uploads use rclone's VFS write cache. Ensure that the
process cache directory is writable and has enough temporary space. rclone
uses the operating system cache directory, normally `$HOME/.cache/rclone` on
Linux or the directory below `$XDG_CACHE_HOME` when it is set. packageR stops
storage initialization if this cache is unavailable.

DuckDB uses `httpfs` to read catalog URLs. The first query installs the
extension from DuckDB's core repository if it is not cached. The writable
cache is under `package-r/duckdb-extensions` in the operating system user
cache. Catalog storage must support ranged `GET` requests and can also receive
`HEAD` requests.

With `FB_ROOT=/`, the top-level entries are buckets. The credentials need
the provider's permission to list buckets. On AWS, this is
`s3:ListAllMyBuckets`. Opening and listing a bucket needs the corresponding
bucket-list permission, such as AWS `s3:ListBucket`. Object actions need the
matching provider permissions.

With `FB_ROOT=my-bucket`, `/` in packageR is the root of `my-bucket`.
packageR does not need permission to list all buckets. It still needs
`s3:ListBucket` and the required object permissions for `my-bucket`.

Browser COG previews read presigned object URLs directly. Configure S3 CORS
to let the packageR and viewer origins use `GET` and `HEAD`. Allow the `Range`
request header, and expose `Accept-Ranges`, `Content-Range`, `Content-Length`,
and `ETag`. Any storage proxy must preserve byte-range requests and `206`
responses.

## Application state

| Variable                | Description                                                                |
| ----------------------- | -------------------------------------------------------------------------- |
| `FB_DATABASE`           | Ephemeral Bolt database path. The default is `/tmp/package-r.db`.          |
| `FB_SERVER_PORT`        | HTTP port. The default is `8888`.                                          |
| `FB_DEFAULT_SHARES`     | Semicolon-separated `hash=path` shares to create during bootstrap.         |
| `FB_DEFAULT_SHARE_PINS` | Optional semicolon-separated `hash=pin` protection for configured shares.  |

`init.sh --add-shares hash=path` adds shares to `FB_DEFAULT_SHARES` for one
bootstrap run. `--add-share-pins hash=pin` does the same for PINs. Do not put
PIN values in logs. Inject `FB_DEFAULT_SHARE_PINS` from a secret when possible.
`init.sh --serve` starts the service after bootstrap.

Keep `FB_DATABASE` on ephemeral storage. Bolt contains only runtime state that
`init.sh` reconstructs for a packageR instance. Do not use the database as the
source of truth and do not mount it on persistent storage. Declare public
shares and their PINs in bootstrap configuration.

## Authentication and user scope

| Variable                 | Description                                                                                                                                                         |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `FB_AUTH_METHOD`         | `proxy` for a trusted identity header, or `none` for a controlled deployment. The bootstrap default is `proxy`.                                                     |
| `FB_AUTH_HEADER`         | Proxy header that contains the user identity. The default is `X-Username`.                                                                                          |
| `FB_AUTH_MAPPER`         | Empty for the raw header, `.<claim>` for a JSON or JWT claim, or a fixed username.                                                                                  |
| `FB_AUTH_JWT_JWKS_URL`   | JWKS URL for strict validation of JWT proxy headers.                                                                                                                |
| `FB_AUTH_JWT_ISSUER`     | Required issuer when JWKS validation is enabled.                                                                                                                    |
| `FB_AUTH_JWT_AUDIENCE`   | Optional expected JWT audience.                                                                                                                                     |
| `FB_AUTH_JWT_ALGORITHMS` | Allowed JWT algorithms. The default is `RS256`.                                                                                                                     |
| `FB_AUTH_JWT_CLOCK_SKEW` | Allowed JWT clock difference. The default is `1m`.                                                                                                                  |
| `FB_CREATE_USER_DIR`     | Protects `/home/<username>` from sibling non-admin identities. The default is `false`. This setting requires one bucket name in `FB_ROOT`. |

When `FB_CREATE_USER_DIR=true`, packageR applies the home boundary to new and
existing non-admin identities. A user can open `/home` to reach
`/home/<username>`, but sibling homes are hidden and denied. The user cannot
delete, copy, rename, or overwrite `/home` or an ancestor because a recursive
operation could affect another user's data. Paths are normalized before these
checks.

The bootstrap default scope is `/`. A user with this scope can access content
outside `/home` when other rules allow it. If changing is enabled, the user can
also change that content. The legacy scope `.` limits the identity to
`/home/<username>`. Other explicit scopes are kept. The home base can be
changed in the global settings, for example from `/home` to `/users`.

packageR refuses to start when `FB_CREATE_USER_DIR=true` and `FB_ROOT=/`.
An S3 service root contains buckets, so `/home/<username>` cannot be a user
directory at that level.

Proxy authentication trusts the configured identity header. A reverse proxy
must remove any client-supplied copy of that header before it sets the
authenticated identity. Set `FB_AUTH_JWT_JWKS_URL` and an issuer when the
header is a JWT and packageR must validate it. Without a JWKS URL, JSON and JWT
claim mapping only decodes the trusted proxy value.

## How access is calculated

The current goal is one S3 storage identity for the packageR process. Its
provider storage policy is the access ceiling for all sessions. packageR
applies narrower identity controls inside that ceiling:

Access is the intersection of these controls, in this order:

1. The S3 credentials and their storage policy define the maximum access.
2. `FB_ROOT` selects all visible buckets or one bucket.
3. The user scope selects a subtree below that root. Paths cannot escape it.
4. Hidden-file handling, global rules, and user rules filter normalized paths.
   Matching rules are evaluated in order, and the last matching rule wins.
5. User-directory mode denies sibling homes and protects their shared parent
   from recursive changes. This boundary cannot be overridden by a user rule.
6. User permission flags control actions such as download, create, rename,
   modify, and delete.

An application admin bypasses hidden-file and application-rule checks. The
admin does not bypass the object-store policy, `FB_ROOT`, or the stored user
scope.

With the bootstrap defaults, user-directory mode is disabled. A provisioned
non-admin identity has scope `/`, so the path checker allows every path below
`FB_ROOT` that the S3 credentials expose. The permission flags still control
permitted actions.

`FB_CREATE_USER_DIR=true` is the built-in automatic per-identity path
isolation below the configured home base. It hides sibling homes but does not
restrict a root-scoped identity to its home. `FB_ALLOW_CHANGING=false` prevents
provisioned non-admin identities from creating, deleting, modifying, or
renaming objects. When it is `true`, those actions are allowed in the user's
own home and on allowed paths outside the home base. They remain denied in
sibling homes and on the shared home parent. The proxy login flow does not
derive different permission profiles from claims. Per-user object-storage
credentials or storage roots are not currently planned.

## Permissions and catalogs

| Variable                  | Description                                                                                     |
| ------------------------- | ----------------------------------------------------------------------------------------------- |
| `FB_ALLOW_CHANGING`       | Enables create, delete, modify, and rename for default non-admin identities. The default is `false`. |
| `FB_CATALOG_DEFAULT_NAME` | Relative STAC-compatible Parquet catalog path inside a share. The default is `catalog.parquet`. |
| `FB_CATALOG_PREVIEW_URL`  | Optional external viewer prefix for public catalog preview links.                               |

Catalog names must stay inside the shared object path. Absolute names and
parent-path escapes are rejected.

## Bootstrap behavior

`init.sh` applies these safety settings:

- It disables commands and command execution.
- It disables generated thumbnails and server-side preview resize.
- It creates or updates the bootstrap runtime configuration.
- It creates the configured public shares and applies their PINs.
- It disables authenticated share creation and deletion.

The selected static or ambient credential source must be available to the
packageR process. A successful bootstrap does not prove that the server can
reach or authorize against S3.

## Runtime limits

- Presigned URLs support GET and have a maximum lifetime of seven days.
- `/api/usage` returns `501 Not Implemented`.
- Object rename and move can use copy and delete operations.

See the [HTTP API](../reference-guides/http-api.md) for route behavior.
