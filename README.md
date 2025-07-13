<img src="https://raw.githubusercontent.com/versioneer-tech/package-r-design/main/logo.png" height="40"/>

# packageR
<a name="introduction"></a>

`packageR` enables users to browse and explore data items mounted from object storage, enrich them with metadata, and curate shareable data packages. Data access is provided directly via secure, presigned URLs—without routing through the application server. `packageR` is developed by [Versioneer](https://versioneer.at) and [EOX](https://eox.at).

## Table of Contents
- [Background](#background)
- [Key Features](#key-features)
- [Configuration](#configuration)
- [Usage](#usage)
- [Contributing](#contributing)
- [License](#license)

## Background

`packageR` is a lightweight tool built on top of a fork of [File Browser](https://github.com/filebrowser/filebrowser/), designed to manage diverse data formats through a single, intuitive web interface. It addresses the common challenge of juggling different storage systems and editing tools for various data types—streamlining workflows for individuals and teams alike.

- For structured text formats such as Markdown, JSON, and YAML—commonly used for documentation, configuration, and metadata — `packageR` offers an integrated browser-based editor.

- For binary content, including very large files, `packageR` can generate secure, temporary download links (presigned URLs) directly connected to underlying object storage, enabling users to download and open them in their preferred desktop applications. In addition `packageR` provides in-browser previews and selective access to modern, cloud-optimized data formats such as Parquet, Cloud-Optimized GeoTIFF (COG), and Zarr (Upcoming). When available, metadata is displayed without requiring a full download. Direct links to full raw archives are also provided for comprehensive access.

Building on File Browser’s sharing functionality, `packageR` promotes a packaging-oriented approach over traditional file-based workflows to simplify complex data handling and support scalable, cloud-native analysis. It supports modern catalog formats such as the [STAC GeoParquet Specification](https://github.com/stac-utils/stac-geoparquet/blob/main/spec/stac-geoparquet-spec.md), allowing metadata to be embedded directly within Parquet files and previewed using tools like [STAC Browser](https://github.com/radiantearth/stac-browser).

## Key Features
<a name="key-features"></a>

- **Streamlined Data Package Generation**: Curate arbitrary data packages of any size containing both binary and text content. Share them via download links, optionally protected and with customizable expiration settings.

- **Rich Previews**: Inline viewers support modern, streamable data formats such as Parquet, Cloud-Optimized GeoTIFF (COG), and Zarr—enabling easy preview and interactive exploration directly in the browser.

- **Presigned URL Sharing**: Securely share data items by generating presigned URLs for objects stored in systems like AWS S3, GCS, Azure Blob, or MinIO. `packageR` works by browsing a local filesystem path (configured via `FB_ROOT`), which represents a mount of your object storage (e.g., via FUSE or Kubernetes CSI drivers). When a data item is accessed, `packageR` uses AWS-compatible credentials to generate a direct download link directly connected to underlying object storage system—bypassing the `packageR` application.

- **Metadata Bundling**: Enhance datasets with rich metadata, attestations, UI hints, and documentation. This enables verifiable data distribution and smooth integration with external graphical tools.

- **Stateless Operation**: Runs without managing internal application state (aside from share links). All configurations are applied declaratively at startup.

- **External Authentication**: Supports proxy-based authentication mechanisms such as OIDC headers or JWT claims, and includes a lightweight role-mapping system for access control.

- **UI Customization**: Allows basic user interface branding (e.g., setting a custom application name via `FB_BRANDING_NAME`). For more advanced customization, static assets can be overridden in a custom Docker image.


## Configuration
<a name="configuration"></a>

All settings are injected via environment variables:

| Variable                                       | Description                                                                                                  |
|------------------------------------------------|--------------------------------------------------------------------------------------------------------------|
| `FB_ROOT`                                      | Path inside the container (this should be your mounted object storage).                                      |
| `FB_BRANDING_NAME`                             | Custom name for the application displayed in the UI.                                                         |
| `FB_BASEURL`                                   | (Optional) Override base URL if not served from root path.                                                   |
| `FB_AUTH_HEADER`                               | HTTP header name from which to extract user identity/role (e.g., `X-Forwarded-User`, `X-Id-Token`).          |
| `FB_AUTH_MAPPER`                               | Mapping strategy for the auth header: `""` (raw), `".<claim>"` (from JSON/JWT), or `<static>`.               |
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`  | Credentials for the S3-compatible object storage, used for signing presigned URLs.                           |
| `AWS_ENDPOINT_URL` / `AWS_REGION`              | Object storage endpoint URL and region configuration.                                                        |
| `BUCKET_NAME`                                  | (Optional) Name of the target object storage bucket.                                                         |
| `BUCKET_PREFIX`                                | (Optional) Path prefix within the target object storage bucket.                                              |

## Usage
<a name="usage"></a>

If you have mounted an object storage bucket locally (e.g., via FUSE, `s3fs`, or a CSI driver) to a folder on your host machine such as:

```
/workspace/my-bucket
```

You should configure `packageR` to use this path as its browsing root by setting:

```bash
-e FB_ROOT=/workspace/my-bucket
```

When running in Docker, make sure to also mount the corresponding host folder into the container:

```bash
-v /workspace:/workspace
```

This makes `/workspace/my-bucket` available inside the container as `/workspace/my-bucket`.

### Example

```bash
docker run --rm -it \
  -u 1000:1000 \
  -v /workspace:/workspace \
  -e FB_ROOT=/workspace/<my-bucket> \
  -e FB_BRANDING_NAME=Workspace \
  -e AWS_ACCESS_KEY_ID=<my-key> \
  -e AWS_SECRET_ACCESS_KEY=<my-secret> \
  -e AWS_ENDPOINT_URL=<my-endpoint> \
  -e AWS_REGION=<my-region> \
  -e BUCKET_NAME=<my-bucket> \
  -p 8080:8080 \
  package-r:latest
```

This setup allows `packageR` to list and share data items from the bucket mount, generating secure presigned URLs pointing to the corresponding objects on the bucket.

## Contributing
<a name="contributing"></a>

We aim to stay aligned with the [Filebrowser](https://github.com/filebrowser/filebrowser) upstream project. To achieve this, we regularly rebase our `main` branch onto the latest upstream changes.

This rebase-based workflow helps us avoid divergence and maintain compatibility. While we acknowledge that rebasing rewrites history and removes merge traces, we consider this trade-off acceptable to keep our integration clean and manageable.

If you're contributing, please base your work on the current `main` branch and rebase your changes before opening a pull request.

## License

[Apache 2.0](LICENSE) (Apache License Version 2.0, January 2004) from https://www.apache.org/licenses/LICENSE-2.0