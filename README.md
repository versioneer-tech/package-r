<img src="https://raw.githubusercontent.com/versioneer-tech/package-r-design/main/logo.png" height="40"/>

# packageR
<a name="introduction"></a>

**packageR** is a lightweight tool built on top of a fork of [File Browser](https://github.com/filebrowser/filebrowser/), designed to turn large-scale object storage systems into browsable catalogs, making it easy to view and share data packages. It is developed by [Versioneer](https://versioneer.at) and [EOX](https://eox.at).

It allows users to browse files mounted from object storage and share them via direct, secure presigned URLs, without proxying data through the application server.

## Table of Contents
- [Key Features](#key-features)
- [Configuration](#configuration)
- [Usage](#usage)
- [Contributing](#contributing)
- [License](#license)

## Key Features
<a name="key-features"></a>

- **Presigned URL Sharing**: Securely shares files by generating presigned URLs for objects stored in systems like AWS S3, GCS, Azure Blob, or MinIO. `packageR` achieves this by browsing a local filesystem path (configured via `FB_ROOT`) which is a mount of your object storage (e.g., via FUSE, K8s CSI drivers). When a file is accessed, `packageR` uses the provided AWS-compatible credentials and bucket information (`BUCKET_NAME`, `BUCKET_PREFIX`) to generate a direct download URL, avoiding the need to proxy data through the application server.
- **Metadata bundling**: Supports enhancing of datasets with descriptive metadata, attestations, UI hints, and documentation. This facilitates verifiable distribution and integration with external graphical tools.
- **Stateless Operation**: Operates without managing internal application state, aside from the share links themselves. All configurations are applied declaratively at startup.
- **External Authentication**: Leverages proxy-based authentication methods, such as OIDC headers or JWT claims, complemented by a lightweight role mapping system.
- **File Browser Compatibility**: Builds upon the core File Browser user interface and plugin architecture, introducing opinionated enhancements tailored for cloud-native environments.
- **UI Customization**: Offers basic UI branding, such as setting a custom application name (via `FB_BRANDING_NAME`). More advanced visual customizations can be achieved by overriding static assets in a custom Docker image.

## Configuration
<a name="configuration"></a>

All settings are injected via environment variables:

| Variable                                   | Description                                                                                               |
|--------------------------------------------|-----------------------------------------------------------------------------------------------------------|
| `FB_ROOT`                                  | Path inside the container to the root directory for browsing (this should be your mounted object storage). |
| `FB_AUTH_HEADER`                           | HTTP header name from which to extract user identity/role information (e.g., `X-Forwarded-User`, `X-Id-Token`). |
| `FB_AUTH_MAPPER`                           | Mapping strategy for the auth header: `""` (raw value), `".<claim>"` (extract from JSON/JWT), or `<static>` (fixed string). |
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` | Credentials for the S3-compatible object storage, used for signing presigned URLs.                        |
| `AWS_ENDPOINT_URL` / `AWS_REGION`          | Object storage endpoint URL and region configuration.                                                     |
| `BUCKET_NAME`                              | The name of the target object storage bucket.                                                             |
| `BUCKET_PREFIX`                            | Optional path prefix within the target bucket that corresponds to the `FB_ROOT`.                          |
| `FB_BRANDING_NAME`                         | Custom name for the application displayed in the UI.                                                      |

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
  package-r:v2025.6.1
```

This setup allows `packageR` to list and share files from the bucket mount, generating secure presigned URLs pointing to the corresponding objects on the bucket.

## Contributing
<a name="contributing"></a>

We welcome contributions! Please feel free to open an issue or submit a pull request on the GitHub repository.

## License

[Apache 2.0](LICENSE) (Apache License Version 2.0, January 2004) from https://www.apache.org/licenses/LICENSE-2.0