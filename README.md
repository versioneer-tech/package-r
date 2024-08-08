<img src="frontend/public/img/logo.png" height="40"/> 

# packageR 

## Goal

**packageR** is a tool maintained by the [Versioneer team](https://versioneer.at) to provide seamless browsing through items within s3 buckets for authorized users. It enables users to share specific items with anonymous users via a regular HTTP link for a specific duration, optionally protected by a password. This link allows recipients to navigate through the shared items and generate [presigned URLs](https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html) for direct download of one or more items, using e.g. CLI tools like [wget](https://www.gnu.org/software/wget/) facilitating resumption of broken downloads even for large files.

## Roadmap

### v1.x

- Possibility to include metadata to a shared link, shown to authorized users on the share management page as well as to recipients during navigation.
- Allow share links containing even millions of items (currently sharing is limited to a maximum of 5000 items).
- Extend sharing capabilities to support filtering below a prefix path considering a regex.
- Make presigned URLs expiration configurable (currently always valid for 7 days on each PRESIGNED_FILE_LIST generation)

### v2

- Extend sharing capabilities to multiple prefix paths without requiring the generation of multiple HTTP links.
- Provide capabilities to browse through multiple buckets.
- Expose and show item checksums from S3 buckets.

## Setup

- Pre-configured docker images are published to [`ghcr.io/versioneer-tech/package-r`](https://github.com/versioneer-tech/package-r/pkgs/container/package-r)
- As the AWS S3 SDK is used for bucket access, common configuration e.g. through environment variables `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_REGION` applies.
- In its initial version (v1), **packageR** supports connection to a single bucket, configured via the `BUCKET_DEFAULT` environment variable.
- The name of the instance can be configured via the `BRANDING_NAME` environment variable.
- A login password can be configured via the `PASSWORD` environment variable, otherwise the `AWS_SECRET_ACCESS_KEY` must be provided for login. 

## Inheritage

<img src="https://raw.githubusercontent.com/filebrowser/logo/master/banner.png" height="15"/>

**packageR** is built on a fork of [File Browser](https://github.com/filebrowser/filebrowser/). While essential bug fixes relevant to the  File Browser project may be submitted to the original project, new features and capabilities are not planned to be contributed back due to the different scope of this fork.

The following capabilities have been removed from the forked codebase:
- Disk usage information
- Checksum calculation
- Content type inference through probes
- Image size resolution and automated image resizing
- Video subtitle support
- QR code generation

The following capabilities have been suspended within the forked codebase:
- No editing support; all items are treated as read-only
- No shell command execution on trigger events
- No custom styling