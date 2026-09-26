# packageR

packageR publishes an S3-compatible object prefix as a public package.
Recipients can browse or download the package without an account. An optional
share password controls access.

The web interface also lets signed-in users browse and manage objects. A
Parquet catalog inside a package can provide public [SpatioTemporal Asset
Catalog (STAC)](https://stacspec.org/) JSON.

packageR connects to object storage directly:

```text
Browser -> packageR API -> embedded rclone VFS -> S3-compatible storage
                         -> presigned GET URL
```

Use packageR to:

- publish object prefixes with an optional share password
- browse one bucket or all permitted buckets
- upload, download, copy, move, rename, and delete objects
- inspect metadata and calculate checksums
- create time-limited object URLs
- preview common files and Cloud Optimized GeoTIFFs (COGs)
- publish a Parquet catalog as STAC JSON

One storage identity is shared by all packageR sessions. Its storage policy
sets the maximum access. packageR can reduce that access with user scopes,
path rules, and action permissions. See
[Configuration](how-to-guides/configuration.md) for the full access model.

## Choose a guide

- **New users:** [Work with objects](how-to-guides/work-with-objects.md)
- **Data publishers:**
  [Package, share, and preview data](how-to-guides/share-and-preview-data.md)
- **Operators:** [Run packageR](how-to-guides/run-package-r.md) and
  [Configuration](how-to-guides/configuration.md)
- **API clients:** [HTTP API](reference-guides/http-api.md)

![packageR public share directory](imgs/screenshots/my-share-directory.png)
