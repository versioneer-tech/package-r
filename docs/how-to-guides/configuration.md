# Configuration

packageR reads server options from command-line flags, `PACKAGE_R_`
environment variables, or its configuration file. Application settings are
stored in the packageR database and managed with `package-r config`.
Object-storage credentials belong to the process and are not stored in the
database or user records.

## Runtime state

packageR is stateless. Object data stays in object storage. The local Bolt
database is a runtime cache for settings, user records, and public shares. It
is not the source of the stored data.

For hosted deployments, use a separate process to prepare the database before
packageR starts. The database does not need a backup or a persistent volume.
The [Kubernetes guide](kubernetes.md) shows an init container and an
`emptyDir` volume.

## Defaults

packageR listens on `127.0.0.1:8888` and uses `/tmp/package-r.db` by default.
Create the database and its first user before you start the server. JSON
username/password authentication is the default.

| Variable | Default | Description |
| --- | --- | --- |
| `PACKAGE_R_DATABASE` | `/tmp/package-r.db` | Bolt runtime-cache path. |
| `PACKAGE_R_ADDRESS` | `127.0.0.1` | Listen address. |
| `PACKAGE_R_PORT` | `8888` | HTTP port. |
| `PACKAGE_R_LOG` | `stdout` | Log output. |
| `PACKAGE_R_BASEURL` | Empty | URL path prefix. |
| `PACKAGE_R_TOKEN_EXPIRATION_TIME` | `2h` | User session lifetime. |

With empty AWS values, packageR uses the ambient AWS credential chain and AWS
S3. Storage requests fail if that chain does not provide usable credentials.

## Object storage

These values configure rclone VFS and presigned links.

| Variable | Default | Required | Description |
| --- | --- | --- | --- |
| `AWS_ACCESS_KEY_ID` | Empty | With static credentials | S3 access key. Set it with `AWS_SECRET_ACCESS_KEY`. |
| `AWS_SECRET_ACCESS_KEY` | Empty | With static credentials | S3 secret key. Set it with `AWS_ACCESS_KEY_ID`. |
| `AWS_SESSION_TOKEN` | Empty | For temporary static keys | Optional session token. |
| `AWS_ROLE_ARN` | Empty | Injected for web identity | Role used by workload identity, including AWS IRSA. |
| `AWS_WEB_IDENTITY_TOKEN_FILE` | Empty | Injected for web identity | Path to the projected workload token. |
| `AWS_ROLE_SESSION_NAME` | Empty | No | Optional web-identity role session name. |
| `PACKAGE_R_ROOT` | `/` | No | S3 service root, or one bucket name. |
| `AWS_ENDPOINT_URL` | Empty | For custom endpoints | S3-compatible API endpoint. An empty value selects AWS S3. |
| `AWS_REGION` | Empty | Service dependent | S3 region. The credential provider can supply it. |
| `XDG_CACHE_HOME` | Platform user cache directory | No | Parent directory for the rclone VFS and DuckDB caches. |

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

Chunked and seek-based uploads use the rclone VFS write cache. Set
`XDG_CACHE_HOME` to choose its parent directory. For example,
`XDG_CACHE_HOME=/var/cache/package-r` places the VFS cache below
`/var/cache/package-r/rclone`. If the variable is unset on Linux, the default
is `$HOME/.cache/rclone`. The directory must be writable and have enough free
space. Storage initialization fails if it is not available.

DuckDB uses the same cache root. With the example above, its `httpfs` extension
is stored under `/var/cache/package-r/package-r/duckdb-extensions`. The first
catalog query downloads the extension if it is not cached. A writable
temporary volume is sufficient. A persistent volume avoids downloading it
again after each restart. Catalog storage must support ranged `GET` requests
and may receive `HEAD` requests.

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

## Authentication and authorization

packageR uses JSON username/password authentication by default. It does not
create a default account. Configure authentication when you prepare the
database. The main process reads the result from that database.

### Username and password

This is the packageR default. Create users with `package-r users add`.
The web interface and HTTP API do not manage user records.

### Trusted identity header

Use a header that contains the username. Include these options in the
`package-r config init` command:

```bash
./package-r config init \
  --auth.method=proxy \
  --auth.header=X-Username \
  --auth.mapper=
```

packageR trusts the complete header value. The upstream proxy must authenticate
the request and replace any client-supplied `X-Username` header.

### Validated JWT

Use `Authorization: Bearer <token>`, a JWKS URL, and the claim that identifies
the user:

```bash
./package-r config init \
  --auth.method=proxy \
  --auth.header=Authorization \
  --auth.mapper=.sub \
  --auth.jwt.jwks-url=https://identity.example/.well-known/jwks.json \
  --auth.jwt.issuer=https://identity.example/ \
  --auth.jwt.audience=package-r
```

The mapper selects the packageR username. It does not limit token validation.
Use `.sub` unless the identity provider defines another stable, unique claim.

If claim mapping is set without a JWKS URL, packageR logs a warning and only
decodes the token. This is allowed for a trusted upstream proxy.

### On-demand users

Add `--signup=true` to the configuration command to enable on-demand user
creation for proxy authentication. The first request from a valid new identity
then creates a user. Add `--create-user-dir=true` to create
`/home/<username>` and hide sibling home directories.

New users have these defaults:

| Setting | Default |
| --- | --- |
| Scope | `/` |
| Browse names and directories | Allowed within the scope and path rules |
| Create, rename, modify, delete | Denied |

Default permissions apply only to new users. Existing users keep their stored
permissions.

User-directory mode does not limit the user to their home directory. The
default scope remains `/`. It hides other home directories and protects
`/home` from recursive changes. Set an explicit user scope or additional rules
to restrict access outside the user's home.

User-directory mode requires one bucket in `PACKAGE_R_ROOT`. packageR refuses
to start when user-directory mode is enabled and `PACKAGE_R_ROOT=/`.

## How access is calculated

packageR applies these access controls in order:

1. The S3 credentials and their storage policy define the maximum access.
2. `PACKAGE_R_ROOT` selects all visible buckets or one bucket.
3. The user scope selects a subtree below that root. Paths cannot escape it.
4. Hidden-file handling, global rules, and user rules filter normalized paths.
   Matching rules are evaluated in order, and the last matching rule wins.
5. User-directory mode denies sibling homes and protects their shared parent
   from recursive changes. This boundary cannot be overridden by a user rule.
6. User permission flags control actions such as create, rename, modify, and
   delete.

## Catalogs

Set a catalog name on each share with `package-r shares add --catalog-name`.
Use `--asset-mappings` on the same command if that share needs URL-to-path
mappings. Without `--catalog-name`, the share has no catalog.

Set the optional STAC Browser URL when you prepare the database:

```bash
./package-r config set \
  --stac-browser-url=https://browser.moregeo.it/external/
```

The default is empty. packageR does not add a STAC Browser link unless you set
this option.

Catalog names and mapping targets must stay inside the shared path. Absolute
names and parent-path escapes are invalid. Most catalogs do not need asset
mappings.

See the [HTTP API](../reference-guides/http-api.md) for route behavior.
