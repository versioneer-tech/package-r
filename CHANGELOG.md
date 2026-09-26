# Changelog

This changelog covers packageR vNext releases.

## [vnext.1.0.0](https://github.com/versioneer-tech/package-r/compare/v2026.6.5...vnext.1.0.0) - 2026-09-26

### Now based on rclone VFS

packageR now uses an embedded rclone VFS. It does not need an external FUSE
mount or a similar remount.

File Browser names were replaced with packageR names across the project. This
includes the binary, CLI, configuration paths, and environment variables.
Environment variables now use the `PACKAGE_R_*` prefix. Compatibility with the
now-deprecated File Browser project is no longer maintained.

Existing capabilities, including package sharing and dynamic STAC APIs, remain.
See the [documentation](docs/index.md) for details.
