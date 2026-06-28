# Configuration

packageR keeps runtime configuration declarative. The container bootstrap and
File Browser configuration are driven by environment variables.

## Bootstrap and Runtime

| Variable | Description |
| --- | --- |
| `FB_DATABASE` | File Browser database path. packageR examples use `/db/bolt.db`, and the Docker image sets this path by default. |
| `FB_ROOT` | Filesystem tree that packageR browses. packageR examples use `/workspace`, usually with the object-storage bucket mounted at that path. The Docker image sets this path by default. |
| `FB_PASSWORD` | Optional bootstrap password for default users. If unset, `init.sh` generates a random password. |
| `HOME` | Runtime home directory. Local examples run packageR from the current repository directory; the Docker image sets `/home/package-r`. |

`init.sh` also accepts quickstart options: `--add-shares hash=path;hash=path...` creates
startup shares, `--add-test-data <path>` copies `tests/data` below `FB_ROOT`,
and `--serve` starts File Browser after bootstrap. Startup shares can also be provided via
`FB_DEFAULT_SHARES` environment variable with same pattern.

## Authentication and User Scope

| Variable | Description |
| --- | --- |
| `FB_AUTH_METHOD` | Authentication method configured by bootstrap, out of 'none','json','proxy'. Defaults to `proxy`. |
| `FB_AUTH_HEADER` | HTTP header used to extract the user identity or role. Defaults to `X-Username` in `init.sh`. |
| `FB_AUTH_MAPPER` | Mapping strategy for the auth header: empty uses the raw value, `.<claim>` extracts from JSON/JWT payloads, or any other value is used as a static username. |
| `FB_CREATE_USER_DIR` | Whether packageR should create generated user directories under the configured user-home base path. Defaults to `true` in `init.sh`; the settings default user-home base path is `/home`. packageR creates `/home/<username>/.keep` so each user's empty workspace exists on normal filesystems and object-store-backed mounts. The packageR default scope `/` keeps generated users at full-bucket scope while generated access rules hide sibling homes. Explicit legacy scope `.` stores the generated `/home/<username>` directory as the user's home-only scope. |

## Catalogs

| Variable | Description |
| --- | --- |
| `FB_CATALOG_DEFAULT_NAME` | Relative catalog path used inside each default share. Defaults to `catalog.parquet`; absolute paths and `..` escapes are rejected. |
| `FB_CATALOG_PREVIEW_URL` | Optional external preview URL used by public share preview links. |

## Permissions

| Variable | Description |
| --- | --- |
| `FB_ALLOW_CHANGING` | Enables create, delete, modify, and rename defaults for non-admin users when set to `true`. Defaults to `false`. |

## Object-Storage Signing

| Variable | Description |
| --- | --- |
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` | Credentials for S3-compatible presigned URL generation. |
| `AWS_ENDPOINT_URL` / `AWS_REGION` | S3-compatible endpoint and region used for signing. |
| `BUCKET_NAME` | Optional target bucket name for presigned object keys. |
| `BUCKET_PREFIX` | Optional path prefix inside the target bucket. |

These values are stored in the default admin user's environment map during
bootstrap and are also used as process-environment fallbacks.

See the [feature summary](features.md) for the supported presigning and
access-rule behavior.
