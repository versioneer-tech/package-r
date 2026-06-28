# Changelog

All notable changes to this project will be documented in this file. See [standard-version](https://github.com/conventional-changelog/standard-version) for commit guidelines.

### [2026.6.4](https://github.com/versioneer-tech/package-r/compare/v2026.6.3...v2026.6.4) (2026-06-28)

- improve generated user home handling with root-scope defaults, /home isolation rules, .keep markers (with admin bypassing and public-share rule skipping)
- adapt bootstrap via init.sh with flags for shares, sample data and serving, with configurable root, database, auth, sharing and changing defaults
- streamline Docker image and runtime defaults using a slim Debian base, bundled branding assets, non-root execution, /workspace root and port 8888
- add MkDocs documentation site with executable quickstart, public sharing and STAC/catalog use cases
- add layered test coverage for use cases, e2e tests and more
- go 1.26.4 and CI/release tooling updates, including expanded GitHub Actions checks and versioned docs deployment

### [2026.3.1](https://github.com/versioneer-tech/package-r/compare/v2025.7.1...v2026.3.1) (2026-03-30)

- introduce share management via cli, allow automation (GitOps flow) via FB_DEFAULT_SHARES environment variable, enabling PVC-less approach for share management

- improve tif loading behavior

- go 1.24.13 and library updates

### [2025.7.1](https://github.com/versioneer-tech/package-r/compare/v2025.7.1-rc4...v2025.6.4) (2025-07-29)

- go 1.24.4 and library updates

- arrow/duckdb support for catalog functionality via parquet

- preview support using presigned URLs, specific TiffRenderer using geotiff.js for COGs/Tiffs

### [2025.6.4](https://github.com/versioneer-tech/package-r/compare/v2025.6.3...v2025.6.4) (2025-06-12)

- support presign for HEAD method, ensure method is preserved during redirect (307 instead of 302)

### [2025.6.3](https://github.com/versioneer-tech/package-r/compare/v2025.6.2...v2025.6.3) (2025-06-11)

- automatically redirect to presignedURL with followRedirect query

- add description and configurable prefix field for share creation

### [2025.6.2](https://github.com/versioneer-tech/package-r/compare/v2025.6.1...v2025.6.2) (2025-06-03)

- Fix: Click on presigned URLs should not open in new window #18

- Improved handling of previews without Download Permission

- Established `BUCKET_NAME` and `BUCKET_PREFIX` configuration option

### 2025.6.1 (2025-06-01)

- Rebased on File Browser v2.32.0

- Refer to the README for a summary of changes introduced by `packageR` on top of File Browser

**Note:** For earlier `packageR` releases, see the corresponding Git branches.