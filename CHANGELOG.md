# Changelog

All notable changes to this project will be documented in this file. See [standard-version](https://github.com/conventional-changelog/standard-version) for commit guidelines.

### 2025.5.2 (2025-05-09)

- fix presigning with empty version identifier
- adapt for roles `admin`, `user`, `browser` and grant source and member management only to `admin`, no share permissions to `browser`
- don't check origin for websockets
- add Home button

### 2025.4.1 (2025-04-06)

- make different share modes explicit in UI
- allow to share with specific groups (as provided through external IdP)
- migrated existing user management to userrolemanagement

### 2025.3.1 (2025-03-31)

- additional pages for member management, source management and runtime settings

### 2025.2.4 (2025-03-03)

- allow to connect to Kubernetes, provide dedicated cli methods wrapping specific Kubernetes calls

### 2025.2.3 (2025-02-17)

- allow package creation, support adding files/folders to package

### 2025.2.2 (2025-02-02)

- introduce auth.mapper config option allowing to map header values from JWT/base64 encoded JSON, use either explicit value or jq like path prefixed with .

### 2024.12.4 (2024-12-31)

- add bucket versioning support

### 2024.12.3 (2024-12-02)

- rebase on File Browser 2.31.2
- introduce "pointer" concept

## [1.4.0](https://github.com/versioneer-tech/package-r/compare/v1.3.0...v1.4.0) (2024-10-28)

- add Source tab to connect additional buckets

## [1.3.0](https://github.com/versioneer-tech/package-r/compare/v1.2.5...v1.3.0) (2024-10-12)

- properly visualize Filesets in Tree component

### [1.2.5](https://github.com/versioneer-tech/package-r/compare/v1.2.0...v1.2.5) (2024-10-04)

- optimize single object API presigning by introducing cache (esp. for k8s objects)

- parallelize direct (1 level down) subpath presigning (but still keeping max. 5000 objects limit below each individual subpath)

## [1.2.0](https://github.com/versioneer-tech/package-r/compare/v1.1.0...v1.2.0) (2024-09-09)

-  code cleanup removing obsolete endpoints (`/raw`, `/preview`, `/image`, ...) and corresponding logic
-  address navigation glitches

## [1.1.0](https://github.com/versioneer-tech/package-r/compare/v1.0.8...v1.1.0) (2024-09-08)

- introduce Source concept to support multiple buckets for browsing and sharing
- k8s-native integration to manage Source manifests and corresponding Secrets

### [1.0.8](https://github.com/versioneer-tech/package-r/compare/v1.0.7...v1.0.8) (2024-08-10)

- allow to put additional description on share link and add to UI

### [1.0.7](https://github.com/versioneer-tech/package-r/compare/v1.0.4...v1.0.7) (2024-08-08)

- don't show meta information for prefixes (folders) on the various pages
- allow to directly open shared items (i.e. individual file like a README) in new window
- show branding name also on shared page

### [1.0.4](https://github.com/versioneer-tech/package-r/compare/v1.0.1...v1.0.4) (2024-08-05)

- introduce configurable branding name

### 1.0.1 (2024-07-19)

- initial version