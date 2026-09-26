# ADR-002: Publish Parquet catalogs as STAC

- Status: Accepted
- Date: 2026-08-30

## Context

packageR packages an object-storage prefix as a public share. A recipient can
open the package without a packageR login. A share can also contain a Parquet
catalog that describes its objects.

Catalogs use different asset `href` forms:

- a path relative to the shared directory
- a path from an object-storage root
- an absolute HTTP(S) object URL
- an `s3://` URL
- a CDN URL whose path does not match the object path

The public response must be valid STAC JSON even when the source Parquet file
is not full STAC GeoParquet. Asset URLs must remain inside the share unless the
catalog intentionally refers to an external resource.

## Decision

A share stores its catalog path. Asset selection and URL resolution are
automatic.

packageR checks every asset in each candidate row. A row matches a catalog
path when an internal asset is at or below that path. The full catalog endpoint
includes all rows that contain an asset `href`.

packageR resolves and rewrites an asset in this order:

1. Apply the first matching explicit asset mapping.
2. Resolve a safe relative `href` from the share root.
3. Match a root-relative object path against the shared path.
4. Match the path of an absolute HTTP(S) URL against the shared path.
5. Match an `s3://` bucket and object path against the shared path.
6. Leave the URL unchanged when it does not identify an object in the share.

Path matching uses path boundaries. Relative and mapped paths cannot use `..`
to leave the share. An HTTP(S) URL is external in service-root mode because it
does not identify a bucket. An `s3://` URL includes the bucket, so packageR can
resolve it in service-root mode.

An explicit mapping is an application setting, not share state. The
`PACKAGE_R_CATALOG_ASSET_MAPPINGS` value is a JSON array:

```json
[
  {
    "from": "s3://data/",
    "to": "."
  }
]
```

The `from` value is an exact string prefix. The `to` value is relative to each
configured share. Here, `s3://data/item.tif` maps to `item.tif` at the share
root. Keep a trailing slash on URI prefixes to prevent partial bucket-name
matches. Use mappings only when automatic matching cannot identify the object
path.

For each returned row, packageR supplies or normalizes required STAC Item
fields. It moves flattened metadata into `properties`, supplies
`stac_version`, normalizes geometry and bounding boxes when possible, and adds
links. One match produces a STAC `Feature`. Zero or multiple matches produce a
STAC `FeatureCollection`. Responses use `application/geo+json`.

The catalog file must be inside the share. packageR checks the file through the
scoped rclone VFS and gives DuckDB a short-lived signed read URL. DuckDB uses
HTTP range requests. Catalog size, query concurrency, and signed-URL lifetime
remain bounded.

## Consequences

Share configuration contains the share name, object path, catalog filename,
and optional password. Catalogs can use relative, HTTP(S), or S3 asset URLs.
External asset URLs remain external.

Checking all asset keys avoids schema-specific settings. DuckDB must read
candidate rows before packageR filters them by path. Catalog size and query
limits bound this work.

Explicit mappings apply to all configured shares in one process. Operators
must use a narrow `from` prefix and the correct relative `to` path.

## Alternatives considered

Require relative asset paths. This rejects common STAC catalogs that use
absolute object or CDN URLs.

Rewrite every absolute URL. This can send an external asset through a share
where the object does not exist. packageR rewrites only URLs that it can
resolve inside the share or through an explicit mapping.
