# Changelog

This changelog covers packageR vNext releases.

### Now based on rclone VFS

packageR now uses an embedded rclone VFS. It does not need an external FUSE
mount or a similar remount.

File Browser names were replaced with packageR names across the project. This
includes the binary, CLI, configuration paths, and environment variables.
Environment variables now use the `PACKAGE_R_*` prefix. Compatibility with the
now-deprecated File Browser project is no longer maintained.

Existing capabilities, including package sharing and dynamic STAC APIs, remain.
See the [documentation](docs/index.md) for details.

## vnext.1.1.0 - proposed

- improve COG previews with raster statistics and automatic contrast stretching

- add configurable presign concurrency

- add configurable log levels and improve catalog and rclone diagnostics

## vnext.1.0.2 - 2026-09-30

- unify browsing capabilities for both authenticated and public (via sharing) cases

- allow cors configuration

- rework test-suite, esp. around STAC capabilities together with stac-browser

- cleanup (mime types, unused methods...)

## vnext.1.0.1 - 2026-09-28

- fix broken presign link in case of password protected share

## vnext.1.0.0 - 2026-09-26

### Now based on rclone VFS

packageR now uses an embedded rclone VFS. It does not need an external FUSE
mount or a similar remount.

File Browser names were replaced with packageR names across the project. This
includes the binary, CLI, configuration paths, and environment variables.
Environment variables now use the `PACKAGE_R_*` prefix. Compatibility with the
now-deprecated File Browser project is no longer maintained.

Existing capabilities, including package sharing and dynamic STAC APIs, remain.
See the [documentation](docs/index.md) for details.
