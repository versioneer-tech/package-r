# ADR-002: Publish Parquet catalogs as STAC

- Status: Accepted
- Date: 2026-08-30

## Context

packageR packages an object-storage prefix as a public share. A recipient can
open the package without a packageR login. A share can also contain a Parquet
catalog that describes its objects.

Catalog producers use different asset `href` forms:

- a path relative to the shared directory;
- a path from an object-storage root;
- an absolute HTTP(S) object URL;
- an `s3://` URL; or
- a CDN URL whose path does not match the object path.

The public response must be valid STAC JSON even when the source Parquet file
is not full STAC GeoParquet. Asset URLs must remain inside the share unless the
catalog intentionally refers to an external resource.

## Decision

A share stores its catalog path. Asset selection and URL resolution are
automatic.

packageR reads each candidate Parquet row and checks every asset. For a request
below `/api/public/catalog/{share}`, a row matches when any internal asset is
at the requested relative path or below it. The full catalog endpoint includes
all rows that contain an asset `href`.

packageR resolves and rewrites an asset in this order:

1. Apply the first matching explicit asset mapping.
2. Resolve a safe relative `href` from the share root.
3. Match a root-relative object path against the shared path.
4. Match the path of an absolute HTTP(S) URL against the shared path.
5. Match an `s3://` bucket and object path against the shared path.
6. Leave the URL unchanged when it does not identify an object in the share.

Path matching uses path-segment boundaries. Relative paths and mapped paths
cannot use `..` to leave the share. HTTP(S) URLs are not treated as internal
for a service-root share because their bucket identity is ambiguous. An
`s3://` URL includes the bucket and can be resolved at the service root.

An explicit mapping is a deployment setting, not share state. The
`FB_CATALOG_ASSET_MAPPINGS` value is a JSON array:

```json
[
  {
    "from": "https://imagery.example.org/openaerialmap/",
    "to": "openaerialmap-assets"
  }
]
```

The `from` value is an exact string prefix. The `to` value is a path relative
to every configured share. Mappings exist only for URL layouts that automatic
matching cannot identify.

For each returned row, packageR supplies or normalizes the fields required for
a STAC Item. It moves flattened metadata into `properties`, supplies
`stac_version`, normalizes geometry and bounding boxes when possible, and adds
links. Multiple or zero matches produce a STAC `FeatureCollection`. One match
produces a STAC `Feature`. Responses use `application/geo+json`.

The catalog file must be inside the share. packageR checks the file through the
scoped rclone VFS and gives DuckDB a short-lived signed read URL. DuckDB uses
HTTP range requests. Catalog size, query concurrency, and signed-URL lifetime
remain bounded.

## Consequences

The common share configuration contains only the share name, object path,
catalog filename, and optional share password. Catalog producers can use
relative, HTTP(S), or S3 asset URLs without per-share URL settings. External
asset URLs remain external.

Filtering all asset keys removes schema-specific configuration. It also means
that DuckDB reads candidate rows before packageR applies path filtering. The
catalog size and concurrency limits bound this cost. A future optimization can
push general asset matching into DuckDB without adding an asset-key field to
the public model.

Explicit mappings apply to all configured shares in one packageR process.
Operators must use a narrow `from` prefix and a `to` path that matches the
layout inside each share.

## Alternatives considered

Require relative asset paths. This is simple but rejects common STAC catalogs
that use absolute object or CDN URLs.

Rewrite every absolute URL. This can redirect an external asset through a
share where that object does not exist. packageR instead rewrites only URLs it
can resolve inside the share or through an explicit mapping.
