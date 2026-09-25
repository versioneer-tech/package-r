# ADR-001: Use rclone VFS for object storage

- Status: Accepted
- Date: 2026-09-24

## Context

packageR must let users browse and manage S3-compatible object data. It must
also create direct, time-limited URLs for previews and downloads. File access
and URL generation must use the same storage root, endpoint, credentials, and
user scope.

An external object-storage mount gives the application a filesystem view, but
it separates file access from URL generation. It also adds a runtime service,
privileged mount behavior, or platform storage integration.

## Decision

packageR embeds rclone in the Go process.

- rclone creates an S3 backend at the service root or at one bucket.
- The service root supports navigation across all accessible buckets.
- rclone VFS is adapted to the `afero.Fs` interface used by File Browser.
- One VFS instance is reused for the process-owned storage configuration. It
  uses either a standard S3 access-key pair or a provider-supported ambient
  credential chain, including workload identity such as AWS IRSA.
- A user's logical scope is applied once below the backend root.
- Browse, read, create, upload, copy, move, rename, and delete operations use
  this scoped VFS.
- Presigned GET URLs use `PublicLink` from the same rclone backend.
- Production does not use an external storage mount or rclone service.
- Integration tests use `rclone serve s3` over temporary test data.
- Parquet catalog paths and sizes are checked through the scoped VFS. DuckDB
  reads each catalog from a short-lived signed URL with HTTP range requests.

The File Browser database is ephemeral runtime state that bootstrap
configuration reconstructs. `FB_ROOT=/` exposes permitted buckets. A bucket
name in `FB_ROOT` exposes that bucket as `/`. Object-storage credentials, the
endpoint, and `FB_ROOT` are not taken from user records. Public shares are
declared before startup instead of through the authenticated API.

The current target is one S3 storage identity per packageR process. The
provider's storage policy is the access ceiling. packageR can narrow access
with scopes, rules, and action permissions. User-directory mode denies access
to sibling homes and prevents recursive changes to their shared parent. It
does not limit access to content outside the home base or select credentials.
Action permissions define read-only versus write access. Per-user
object-storage credentials are a possible future evolution and are not
currently planned.

## Consequences

File operations and presigned URLs now share one configuration and path model.
The deployment has no object-mount lifecycle and needs no privileged mount
access. S3-compatible services can use the same API contract, and the browser
can use direct object URLs for formats such as COG.

The application owns the lifetime of its rclone VFS instances. Object-store
operations keep object semantics: a rename can be a copy followed by a delete,
and POSIX ownership or mode changes are compatibility operations rather than
bucket permissions.

Current public links support GET only. DuckDB catalog queries need the
`httpfs` extension and object storage
that supports HTTP range requests. Bucket capacity is not the same as local
host disk capacity, so the local disk-usage API is unavailable. Chunked object
uploads require a local rclone VFS write cache.
