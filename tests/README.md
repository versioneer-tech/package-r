# Test strategy

packageR has Go unit tests, API integration tests, and browser tests. Lint,
type checks, documentation builds, and release builds are separate checks.

## Sentinel-2 fixture

The integration and browser tests use a generated fixture that is
excluded from Git. Before you run these tests, generate it with:

```bash
uv run \
  --with pystac-client --with pystac --with requests \
  --with stac-geoparquet --with pyarrow --with geopandas --with rasterio \
  python scripts/download_sentinel2.py
```

This command downloads approximately 150 MB of data clipped to Vienna. It
selects scenes from February, May, and August 2026 in the Earth Search
Sentinel-2 Collection 1 Level-2A catalog. It also downloads Vienna district
boundaries from City of Vienna Open Government Data and dissolves them into a
single city boundary.

## Unit tests

Go unit tests are next to the packages that they test. They use temporary
directories, memory filesystems, and small test doubles. They do not use
external services.

Run all unit tests:

```bash
make test-unit
```

#### Current public presign baseline

`TestPublicSharePresignConcurrentHierarchy` is the synthetic baseline for
concurrent password-protected public-share presign requests. Both workloads
start downloads at deterministic random times from `t0` through `t0+30s` and
must finish within 31 seconds. `w1000` sends 1,000 requests, and `w10000` sends
10,000 requests. The test uses a real bcrypt password hash, a 20 ms simulated
public-link operation, one warm-up request, and a 25 ms client retry delay after
an HTTP 429 response. The warm-up also populates the successful-password cache.
Client request counts and overall durations include retries.

| Workload | Presign limit | Downloads | Client requests | HTTP 429 | Errors | Maximum active | Overall duration | Presign minimum | Presign average | Presign maximum |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `w1000` | 32 | 1,000 | 1,000 | 0 | 0 | 5 | 29.98 s | 20.16 ms | 20.92 ms | 31.31 ms |
| `w10000` | 32 | 10,000 | 10,000 | 0 | 0 | 19 | 30.02 s | 20.03 ms | 20.78 ms | 31.31 ms |
| `w1000` | 8 | 1,000 | 1,000 | 0 | 0 | 5 | 30.01 s | 20.11 ms | 21.15 ms | 51.53 ms |
| `w10000` | 8 | 10,000 | 37,642 | 27,642 | 0 | 8 | 30.10 s | 20.04 ms | 20.82 ms | 71.51 ms |

With limit 32, both workloads complete without retries. With limit 8, `w1000`
also completes without retries. `w10000` receives 27,642 HTTP 429 responses and
retries them. All downloads finish within 31 seconds, and no request fails.

Run only this baseline with:

```bash
go test ./http -run '^TestPublicSharePresignConcurrentHierarchy$' -count=1 -v
```

Use a unit test when you can check behavior at a package boundary.

## API integration tests

The integration harness is `tests/integration/run.bash`. It starts `rclone
serve s3` and packageR on loopback addresses. packageR reads the test bucket
with its embedded rclone VFS. The test uses `PACKAGE_R_ROOT=/` and an explicit
`PACKAGE_R_BUCKETS` catalog to check service-root navigation without bucket
discovery.

The harness copies `tests/data` to a temporary bucket and does not change the
source files. It checks:

- user, settings, and sign-up management HTTP APIs are unavailable
- `GET /api/shares` returns safe links, while share-management writes are
  unavailable
- S3 service-root and bucket listings work through the VFS
- create, copy, rename, presigned read, and delete operations work
- a two-chunk TUS upload works through the VFS write cache
- TUS accepts `HEAD` and rejects its old `GET` alias
- aborting a TUS upload removes its partial object
- authenticated presigned URLs work without proxy-download audit records
- password-protected public presigned URLs and temporary-token redirects work
- runtime share creation is unavailable
- public catalogs load through the VFS with the GeoJSON media type
- the live catalog endpoint passes STAC 1.1 validation
- catalog Collection and Item metadata and links are correct
- explicit catalog asset mappings return the expected object
- Prometheus metrics expose HTTP, authentication, upload, presign, rclone, and
  VFS values without object paths
- generated users can read and write their own home
- generated users cannot read or write a sibling home

Run the test with rclone 1.74 or newer:

```bash
make test-integration
```

Set `RCLONE_BIN` when rclone is not on `PATH`. The test uses fixed credentials
and does not need cloud access. If `stac-check` is not on `PATH`, the Make
target runs the pinned validator with `uv`.

Use an integration test for behavior that needs rclone VFS, the S3 protocol,
presigned URLs, or several API operations.

## Browser tests

Browser tests use Playwright and Chromium. They start rclone, packageR, and
Vite on loopback addresses. They cover login, personal settings, the read-only
share list, public shares, previews, and other user workflows.

To test opening generated STAC Browser links on localhost, make an existing
[STAC Browser](https://github.com/radiantearth/stac-browser) checkout available,
for example at `~/stac-browser`.

```bash
pnpm --prefix ~/stac-browser install
pnpm --prefix ~/stac-browser start
```

The server listens on `http://localhost:8080`. Configure packageR with
`http://localhost:8080/external/` as its STAC Browser URL. This local server is
required for a manual end-to-end preview test because an HTTPS page cannot read
the HTTP packageR endpoint on localhost.

Install the Chromium runtime once, then run the suite:

```bash
cd frontend
pnpm exec playwright install chromium
cd ..
make test-frontend
```

Use Playwright only for behavior that a user can see or control. Test backend
rules with Go unit or API integration tests.

## Run all tests

```bash
make test
```

This command checks the required tools before it starts the tests. Run
`make test-check` to check the tools without running tests. Use a focused
target when you change only one part of the application.
