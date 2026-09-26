# ADR-001: Use rclone VFS for object storage

- Status: Accepted
- Date: 2026-08-30

## Context

packageR must let users browse and manage S3-compatible object data. It must
also create time-limited URLs for previews and downloads. File access and URL
generation must use the same root, endpoint, credentials, and user scope.

An external mount gives the application a filesystem view. It does not create
signed object URLs. It also needs another service, a privileged mount, or
platform storage support.

## Decision

packageR embeds rclone in the Go process.

- rclone creates an S3 backend at the service root or one bucket
- The service root shows all buckets that the credentials can access
- An adapter exposes rclone VFS as the inherited `afero.Fs` interface
- One VFS instance holds the process storage configuration
- A user scope selects a path below the backend root
- Object operations use the scoped VFS
- Presigned GET URLs use `PublicLink` from the same rclone backend
- Production does not need an external mount or rclone service
- Integration tests use `rclone serve s3` with temporary data
- DuckDB reads checked catalog files from short-lived signed URLs

Bootstrap configuration rebuilds the packageR database. The database is a
disposable runtime cache. `PACKAGE_R_ROOT=/` exposes permitted buckets. A
bucket name exposes that bucket as `/`. User records do not set the storage
root, endpoint, or credentials. Public shares are set before startup.

One S3 identity is used by each packageR process. Its storage policy sets the
maximum access. User scopes, path rules, and action permissions can reduce
this access. User-directory mode hides sibling homes and protects their shared
parent. It does not select different storage credentials.

## Consequences

File operations and presigned URLs share one configuration and path model.
The deployment does not need a privileged mount. The browser can use direct
object URLs for formats such as COG.

The application owns the rclone VFS lifetime. Object operations keep S3
semantics. For example, a rename can copy an object and then delete the source.
POSIX ownership and mode changes do not change bucket permissions.

Public links support GET only. DuckDB catalog queries need the `httpfs`
extension and HTTP range requests. Chunked uploads need a local rclone VFS
write cache.
