# Changelog

All notable changes to this project will be documented in this file. See [standard-version](https://github.com/conventional-changelog/standard-version) for commit guidelines.

### [1.2.2](https://github.com/versioneer-tech/package-r/compare/v1.2.1...v1.2.2) (2024-09-19)

- optimize single object API presigning by introducing cache (esp. for k8s objects)

- parallelize direct (1 level down) subpath presigning (but still keeping max. 5000 objects limit below each individual subpath)

### [1.2.1](https://github.com/versioneer-tech/package-r/compare/v1.2.0...v1.2.1) (2024-09-11)

- concept of FileSets/ObjectSet providing a view on top of the owning Source, may be backed e.g. by different infrastructure or only expose a subset but presigning of the items still works against owning Source

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