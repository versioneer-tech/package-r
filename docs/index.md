# packageR

`packageR` started as a fork of the excellent
[File Browser](https://github.com/filebrowser/filebrowser) project. Most core
File Browser concepts still apply: users, permissions, shares, a familiar file
tree, and browser-based inspection of text and common media.

packageR is meant for data that lives in object storage and is made visible to
packageR as a regular file tree. That filesystem view can come from different
deployment choices, where an object-storage bucket is mounted into the runtime,
often through FUSE or the platform's storage integration.

On top of that filesystem view, packageR adds bucket-aware features: presigned
URLs for direct access, streaming previews for supported formats, and catalogs
for domain workflows. STAC is the main geospatial catalog workflow.

Large uploads and downloads should usually go directly to the bucket with
S3-compatible tools. packageR is mainly for browsing, inspection, curation,
sharing, preview, and catalog access.

Another conceptual difference is operational. packageR treats the File Browser
database as ephemeral cache: only files in the mounted filesystem are
persistent, while runtime state such as configured shares comes from startup
configuration. This lets operators treat packageR as stateless, with no
application database to back up; the persistent system is the mounted object
store plus credentials for presigned URLs. A curated `init.sh` bootstrap provides
recommended defaults and helper flags, while still letting deployments tweak
settings through environment variables.

Use the [feature summary](features.md) for the packageR-specific
behavior layered on top of File Browser.

The [quickstart](generated/usecases/quickstart.md) starts a local sample. The
walkthroughs also cover public sharing, presigned URLs, access boundaries, and
STAC catalog access.
