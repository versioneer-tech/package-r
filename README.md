<img src="https://raw.githubusercontent.com/versioneer-tech/package-r-design/main/logo.png" height="40"/>

# packageR
<a name="introduction"></a>

**packageR** is a lightweight tool built on top of a fork of [File Browser](https://github.com/filebrowser/filebrowser/), designed to turn large-scale object storage systems into browsable catalogs, making it easy to view and share data packages. It is developed by [Versioneer](https://versioneer.at) and [EOX](https://eox.at).

It allows users to browse data items mounted from object storage, enrich them with metadata and share them via direct, secure presigned URLs, without proxying data through the application server.

## Table of Contents
- [Backround](#backround)
- [Key Features](#key-features)
- [Configuration](#configuration)
- [Usage](#usage)
- [Contributing](#contributing)
- [License](#license)

## Backround

PackageR is a tool built on Filebrowser and lets you manage all kinds of data formats in a single web interface. It was created to solve the problem of having to struggle with different storage options and editing tools for different data types, which often complicates workflows for teams and individual users.

With packageR, you can access text files (e.g. Markdown, JSON, YAML), binary files (e.g. Excel tables, ZIP archives, images, high‑resolution satellite imagery), complex data packages (such as scientific archives with RO‑Crate metadata), and other modern formats (e.g., Parquet, Cloud Optimized GeoTIFF, and Zarr)- all without switching between separate applications.

Smaller files open directly in a lightweight inline editor in your browser, while larger or proprietary formats (i.e., files that require specific software to open, such as Excel or Photoshop) are offered via signed download URLs so you can open them in your preferred desktop software.

For cloud-native and packaged data, packageR provides in-browser previews and partial access where possible — supporting formats like Parquet, Cloud‑Optimized GeoTIFF (COG), and Zarr. When metadata is available (e.g., RO‑Crate metadata.json, manifest files), it is displayed without requiring a full download, while direct links to raw archives remain available for full access when needed.

By unifying these capabilities in one intuitive interface, packageR minimizes load times and unnecessary indirection, and supports the FAIR principles - keeping your data findable, accessible, interoperable, and reusable. Whether you’re glancing at a single file or conducting large‑scale analyses, Package‑R saves you time and reduces complexity.


## Key Features
<a name="key-features"></a>

- **Presigned URL Sharing**: Securely shares data items by generating presigned URLs for objects stored in systems like AWS S3, GCS, Azure Blob, or MinIO. `packageR` achieves this by browsing a local filesystem path (configured via `FB_ROOT`) which is a mount of your object storage (e.g., via FUSE, K8s CSI drivers). When a data item is accessed, `packageR` uses the provided AWS-compatible credentials to generate a direct download URL, avoiding the need to proxy data through the application server.
- **Metadata bundling**: Supports enhancing of datasets with descriptive metadata, attestations, UI hints, and documentation. This facilitates verifiable distribution and integration with external graphical tools.
- **Stateless Operation**: Operates without managing internal application state, aside from the share links themselves. All configurations are applied declaratively at startup.
- **External Authentication**: Leverages proxy-based authentication methods, such as OIDC headers or JWT claims, complemented by a lightweight role mapping system.
- **File Browser Compatibility**: Builds upon the core File Browser user interface and plugin architecture, introducing opinionated enhancements tailored for cloud-native environments.
- **UI Customization**: Offers basic UI branding, such as setting a custom application name (via `FB_BRANDING_NAME`). More advanced visual customizations can be achieved by overriding static assets in a custom Docker image.

## Configuration
<a name="configuration"></a>

> ⚠️ **Important Notice**  
> As of the latest rebase on **31 May 2025**, the repository was aligned with the latest upstream changes and underwent a cleanup of obsolete configuration options as well as the removal of outdated issues.  
> This was done to reduce confusion caused by outdated guidance and to ensure that all relevant information is now accurately reflected in the `packageR` documentation.


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
  package-r:v2025.6.2
```

This setup allows `packageR` to list and share data items from the bucket mount, generating secure presigned URLs pointing to the corresponding objects on the bucket.

## Contributing
<a name="contributing"></a>

We aim to stay aligned with the [Filebrowser](https://github.com/filebrowser/filebrowser) upstream project. To achieve this, we regularly rebase our `main` branch onto the latest upstream changes.

This rebase-based workflow helps us avoid divergence and maintain compatibility. While we acknowledge that rebasing rewrites history and removes merge traces, we consider this trade-off acceptable to keep our integration clean and manageable.

If you're contributing, please base your work on the current `main` branch and rebase your changes before opening a pull request.

## License

[Apache 2.0](LICENSE) (Apache License Version 2.0, January 2004) from https://www.apache.org/licenses/LICENSE-2.0