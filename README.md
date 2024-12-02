<img src="https://raw.githubusercontent.com/versioneer-tech/package-r-design/main/logo.png" height="40"/>

# packageR 

## Goal

`packageR` is a lightweight tool developed by the [Versioneer](https://versioneer.at) team, designed to facilitate secure and seamless data sharing from object storage systems using **temporary URLs**. It eliminates the need for direct access to object storage or proxying data through intermediary systems by delegating access control directly to the object storage layer’s existing capabilities. Additionally, it allows users to bundle additional context or metadata with shared datasets, making sharing comprehensive and user-friendly.

## Why `packageR` Exists

Modern organizations increasingly rely on **object storage** (like S3, GCS, or Azure Blob Storage) to manage vast amounts of data. While object storage is excellent for large-scale storage and retrieval, sharing this data comes with several challenges. `packageR` is designed to address these challenges:

1. **Secure Sharing Without Direct Access or Proxying**
   - Whether deployed in the cloud or hosted on-premises, object storage systems are typically managed centrally by IT or Ops teams, who may be hesitant to provide direct access to storage for external collaborators or even internal teams.
   - Traditional approaches to data sharing often involve **proxying data** through additional systems or services, which introduces unnecessary network hops, latency, and maintenance overhead.
   - Temporary URLs (e.g., presigned URLs in S3 or similar mechanisms in GCS/Azure) enable access control delegation directly to object storage, eliminating the need for additional infrastructure.
   > **PackageR simplifies the process of generating and managing these temporary URLs.**

2. **Data Sharing Without Local Copies**
   - Users appreciate the abstraction of a filesystem, as browsing through directories provides an intuitive overview of stored data. Mounting object storage into a filesystem is one approach, ideal for cases requiring direct data access. However, when the goal is simply to share data, this approach is unnecessary.  
   - For sharing data via temporary URLs, all you need is:  
     - **Credentials**: Used to generate access tokens or URLs for the data. These can be pre-configured within the tool, eliminating the need for IT or Ops teams to share them with you.
     - **Object Identifiers**: URLs or paths to the data in object storage.  
   > **PackageR streamlines the sharing process by enabling users to browse and select files for sharing, either through mounted filesystems or by providing text or JSON files that describe object identifiers.**

3. **Bundling Context and Metadata**
   - Shared datasets often require additional context, such as README files, schema definitions, or metadata (e.g., descriptions, owners, or licensing information).
   - This contextual information typically exists outside the object storage system, resulting in fragmented or incomplete sharing practices.
   > **PackageR allows users to bundle this information with the data, ensuring a comprehensive and self-contained sharing experience.**

## How It Works

`packageR` follows common and user-friendly practices, using symbolic links and relying on a few naming conventions to reference object storage items through so-called pointer files. These symbolic links enable users to restructure and filter datasets while facilitating seamless collaboration through the exchange of these pointer files. Additionally, users can create and attach supplementary files, such as README files, to provide extra context and share them alongside the data.

Collaborators can download these packages and exchange pointer files for temporary URLs, providing secure, time-limited access to specific data items. Temporary URLs, such as [Presigned URLs](https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html) (AWS) or [Shared Access Signatures (SAS)](https://learn.microsoft.com/en-us/azure/storage/common/storage-sas-overview) (Azure), allow direct HTTP access to the data.

Command-line tools like [wget](https://www.gnu.org/software/wget/) simplify the exchange of temporary URLs, enabling users to download large datasets with support for resuming interrupted transfers.

## Inheritage

<img src="https://raw.githubusercontent.com/filebrowser/logo/master/banner.png" height="15"/>

`packageR` is built on a fork of [File Browser](https://github.com/filebrowser/filebrowser/), a tool offering robust features for browsing, selecting, and sharing files. While essential bug fixes may be contributed back to the File Browser project, new features unique to `packageR` will not, due to its distinct scope and functionality.

`packageR` is designed to closely follow the principles of [File Browser](https://github.com/filebrowser/filebrowser/), utilizing its configuration methods and internal persistence with BoltDB as well as the capabilities to execute whitelisted shell commands window. It retains the core browsing and sharing functionalities while focusing on handling pointer files for non-text content. These pointer files can be previewed and downloaded like regular text files, with support for generating temporary URLs for direct client-side downloads.

## Setup

`packageR` uses the following directory conventions, initialized via the `init.sh` script (used in the Docker entry point):

- **Packages**: Stored in `~/packages`
- **Sources**: Stored in `~/sources`, which may optionally link to mounted buckets (e.g. via `s3fs`) or be materialized as symbolic links based on text or JSON files describing object identifiers
- **Credentials**: Stored in `/secrets` as individual files with the following structure:

```bash
$ tree /secrets
.
├── bucket-a
│   ├── AWS_ACCESS_KEY_ID
│   ├── AWS_ENDPOINT_URL
│   ├── AWS_REGION
│   ├── AWS_SECRET_ACCESS_KEY
│   └── BUCKET_NAME
├── bucket-b
│   ├── AWS_ACCESS_KEY_ID
│   ├── AWS_ENDPOINT_URL
│   ├── AWS_REGION
│   └── AWS_SECRET_ACCESS_KEY
```

Note: The `BUCKET_NAME` file is optional. By default, the folder name (e.g., `bucket-a`) will be used instead.

Symbolic linking is employed to facilitate all standard functionality within `packageR`.

Secrets are read by `packageR` from the `/secrets` folder to generate temporary URLs in the code. These secrets can be mounted by the IT or operations teams, or added by users via the `add-source` bash script located in the `cli` folder, which creates the necessary folder structure. 

Sources can be mounted by the IT or operations teams under `/mounts`, or added by users via reference files, typically in the form of a text file containing relative paths:

Example `bucket-a.source`:
```
a/a1.tif
a/a2.tif
b/b.tif
```

Or `bucket-b.source` in JSON format:

```json
[
   {
      "url": "a/a1.tif"
   },
   {
      "url": "a/a2.tif"
   },
   {
      "name": "bbb.tif",
      "url": "b/b.tif"
   }
]
```

Note: After creating a file in the `~/sources` directory, the entries are parsed, and symbolic links are automatically created. To trigger the creation process again (e.g., after adding entries to the reference files), simply delete the folder associated with the bucket name.

`packageR` functionality has been verified on:
- Ubuntu 22.04
- Windows systems using WSL2 with Ubuntu 22.04

## Current Limitations and Roadmap

- Although the inherited File Browser tool offers pluggable authentication mechanisms, we have currently limited ourselves to the default `json` strategy with a preconfigured `default` user (password provided via the `PASSWORD` environment variable). The folder setup is thus `/home/default/source` and `/home/default/packages`. Since there is no immediate need for fine-grained authorization within `packageR`, we may enhance the authentication in the future by extending proxy header authentication, but this is still under discussion.

- At present, only obfuscated URLs using distinct hashes are supported for sharing. In future releases, `packageR` may integrate with identity systems like Keycloak to enable sharing data with specific users or groups.

## License

[Apache 2.0](LICENSE) (Apache License Version 2.0, January 2004) from https://www.apache.org/licenses/LICENSE-2.0

