<img src="https://raw.githubusercontent.com/versioneer-tech/package-r-design/main/logo.png" height="40"/>

# packageR 

## Goal

`packageR` is a tool developed by the [Versioneer](https://versioneer.at) team that allows users to browse their connected remote sources, such as object storage buckets, and curate specific subsets of data. Users can select a group of items and add additional metadata, including files like `README.md` or Jupyter Notebooks.

Data packages created with packageR can be easily shared with collaborators through the concept of "annexing," which involves keeping the actual files in their original remote source while referencing them with pointer files. This method allows users to restructure and filter data, facilitating collaboration by enabling the exchange of just these pointer files. Collaborators can then swap these pointer files (if permissions permit) for temporary URLs that enable direct downloads from the storage layer, eliminating the need to proxy through packageR.

## How It Works

Temporary URLs, also known as [presigned URLs](https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html) for AWS or [Shared Access Signatures (SAS)](https://learn.microsoft.com/en-us/azure/storage/common/storage-sas-overview) for Azure, allow users to share specific items with anonymous users via a standard HTTP link for a limited duration. Pointer files referencing actual files stored remotely can be easily handled and exchanged for such temporary URLs. packageR data packages keeps track of a list of these pointer files, providing a stable listing that ensures reproducibility. To access these data packages, a simple shared HTTP URL (with an optional password protection feature) is sufficient; collaborators can then navigate through the shared URL and download data packages as ZIP files containing both text and pointer files.

Command-line tools like [wget](https://www.gnu.org/software/wget/) can simplify the process of exchanging temporary URLs, enabling users to download large numbers of files with the ability to resume interrupted downloads. Alternatively, users can store this content in Git repositories utilizing [Git Annex](https://git-annex.branchable.com/) or [DVC](https://dvc.org/). This approach allows users to manage data effectively by resolving pointers through commands such as `dvc pull` or `git annex get`, thereby making use of familiar data management practices without the need to learn new concepts.

## Inheritage

<img src="https://raw.githubusercontent.com/filebrowser/logo/master/banner.png" height="15"/>

`packageR` is built on a fork of [File Browser](https://github.com/filebrowser/filebrowser/), which offers a variety of excellent features for browsing files, selecting, and downloading data, as well as sharing data with others when the necessary tools are installed on a server. While essential bug fixes relevant to the File Browser project may be submitted to the original project, new features and capabilities are not planned to be contributed back due to the different scope of this fork.

## Setup

`packageR` is designed to closely adhere to the concepts and structure of [File Browser](https://github.com/filebrowser/filebrowser/), leveraging its established configuration methods and an internal persistence mechanism via BoltDB. It retains most of the functionalities, particularly in browsing and sharing capabilities. Regarding previewing and downloading, it is important to note that non-text content is annexed, meaning that only the pointer files are accessible through `packageR`. However, these pointer files, similar to regular text files, support the usual previewing and downloading processes. Additionally, they allow to generate presigned URLs for client-side direct downloads, bypassing the need for proxying through `packageR`.

To enhance the functionality of remote "Sources" (mounted folders e.g. for S3 Object Storage buckets) and "Packages", `packageR` adheres to specific conventions and initializes these components through the `init.sh` script, which is also used in the Docker entry point.

The following directories in the file system need to be established (i.e., the user running `packageR` must have appropriate permissions):

- **Packages** are stored in `/home/default/packages`.
- **Sources** are mounted at `/mounts`.
- **Credentials** for accessing sources are cached and stored in `/secrets`.

Symbolic linking is employed to facilitate all standard functionality within `packageR`.

The system must have `s3fs` (and thus FUSE) as well as `awscli` installed, along with common tools such as `bash`, `curl`, and `jq`. Functionality has been verified on Ubuntu 22.04 and on Windows systems using WSL2 with Ubuntu 22.04.

> **Note**: Pre-configured Docker images are available at [`ghcr.io/versioneer-tech/package-r`](https://github.com/versioneer-tech/package-r/pkgs/container/package-r). In Local Mode, the docker run command needs to be executed with the --privileged flag.
>
> docker run -e PASSWORD=1234 -p 8080:8080 --privileged  ghcr.io/versioneer-tech/package-r

Secrets are essential for presigning content and must always be accessible to packageR. In configurations where mounts are created directly, credentials are also required for packageR during the mounting process.

In **Local Mode**, when these mounts are set up directly with `s3fs` in the `/mounts` folder, the secrets are explicitly stored in `/secrets`. The CLI tools in the corresponding folder should be added to the system path, enabling users to utilize commands like `add-source` and `remove-source` either directly or via the UI.

In **Kubernetes Mode**, source mounts are managed by a [`sourceD`](https://github.com/versioneer-tech/source-d), a daemon (Kubernetes operator) specifically designed to handle sources such as object storage buckets. This daemon establishes and monitors the mounts (including `s3fs` among others) while providing the necessary credentials for presigning to packageR. In this context, the `/secrets` folder serves solely as a cache.

Both modes are suitable for a variety of deployment and operational scenarios!

## License

[Apache 2.0](LICENSE) (Apache License Version 2.0, January 2004) from https://www.apache.org/licenses/LICENSE-2.0
