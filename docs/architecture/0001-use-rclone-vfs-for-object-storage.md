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
- The service root uses S3 discovery by default
- A configured bucket list supplies the top-level directories without S3
  bucket discovery; each bucket has its own backend
- An adapter gives the application access to rclone VFS through `afero.Fs`
- All VFS instances use the server's storage configuration
- A user scope selects a path below the backend root
- Object operations use the scoped VFS
- Presigned GET URLs use `PublicLink` from the same rclone backend
- Production does not need an external mount or rclone service
- Integration tests use `rclone serve s3` with temporary data
- DuckDB reads checked catalog files from short-lived signed URLs

The deployment creates the database from configuration before packageR starts.
It can recreate the database when the process is replaced. `PACKAGE_R_ROOT=/`
shows discovered buckets, or the buckets in `PACKAGE_R_BUCKETS` when set.
A bucket name in `PACKAGE_R_ROOT` shows that bucket as `/` and requires an
empty `PACKAGE_R_BUCKETS`. User records cannot change the server's root,
endpoint, credentials, or bucket list. Public shares are set before startup.

Each packageR process uses one S3 identity for all users. Its storage policy
controls which buckets and objects packageR can access. User scopes, path
rules, and permissions further limit what each user can do. User-directory
mode hides other users' homes and protects their parent directory. All users
still use the same storage identity.

## Consequences

File operations and presigned URLs use the same credentials and scoped paths.
The deployment does not need a privileged mount. The browser can use direct
object URLs for formats such as COG.

The application owns the rclone VFS lifetime. Object operations keep S3
semantics. For example, a rename can copy an object and then delete the source.
POSIX ownership and mode changes do not change bucket permissions.

Public links support GET only. DuckDB catalog queries need the `httpfs`
extension and HTTP range requests. Chunked uploads need a local rclone VFS
write cache.
