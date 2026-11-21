# Tests

Run tests from the repository root. Go tests need a C compiler for DuckDB.
Integration and browser tests use a temporary copy of `tests/data`, local
rclone S3, and synthetic credentials. They do not use cloud accounts.

| Command | Scope | CI |
| --- | --- | --- |
| `make test-unit` | Go unit tests | Yes |
| `make test-race` | Race detection in authentication, cache, HTTP, logging, metrics, and rclone storage code | Yes |
| `make test-tooling` | CI selection, secret scan boundaries, and release security gates | Yes |
| `make test-integration` | API operations through local rclone S3 | Yes |
| `make test-frontend` | Playwright Chromium browser tests | No |
| `make test-presign-load` | Timed public presign workloads | No |

`make test` checks tools, then runs unit, API integration, and browser tests
in that order. Use `make test-check` to check its prerequisites alone.
Lint, documentation, build, and vulnerability checks are separate commands.
See [release security checks](../SECURITY.md#release-checks) for
`make vulncheck` and Trivy.

## Select checks

Use the PR base commit, or the target branch for other changes:

```bash
python3 scripts/ci-checks.py --base main
```

Without `--head`, this includes committed, staged, unstaged, and untracked
changes. Empty or unavailable comparisons select full checks.

For `docs_only=true`, run `make lint-files test-tooling docs`. Otherwise,
also run the full unit suite and the applicable integration tests below.
CI uses the same selector. Manual CI runs use the full suite. Markdown test
fixtures and frontend assets require full checks.

## Sentinel-2 fixture

API integration and browser tests need the generated Vienna fixture. Create
it once:

```bash
uv run --no-project --with-requirements tests/requirements.txt \
  python scripts/download_sentinel2.py
```

The script needs network access and writes `tests/data/vienna-s2l2a-26`.
See the [catalog guide](../docs/how-to-guides/share-and-preview-data.md#publish-a-stac-compatible-parquet-catalog)
for the source data and format.

## Go unit and race tests

`make test-race` runs the selected packages with `-race` and CGO enabled.
It includes the public presign concurrency limit and password-cache tests.
The timed load test is separate so host load does not affect unit or race
results.

## API integration tests

`tests/integration/run.bash` starts rclone 1.74 or newer and packageR on
loopback addresses. Set `RCLONE_BIN` if rclone is not on `PATH`. The Make
target supplies the pinned STAC validator through uv when needed.

The harness checks:

- user, settings, sign-up, and share-management APIs are unavailable
- the share list exposes safe links within the user's scope
- service-root and bucket listings work with an explicit bucket catalog
- create, copy, rename, delete, and presigned reads work through rclone VFS
- TUS uploads can resume and abort, and use `HEAD` for offsets
- public shares support passwords, temporary tokens, and presigned URLs
- catalog routes return valid STAC Collections, Items, and mapped assets
- metrics omit object paths
- generated homes permit access to the owner's files and deny sibling homes

The API harness selects free ports, waits for readiness, and stops its own
processes. Both harnesses preserve the source fixture.

## Browser tests

Playwright starts rclone, packageR, and Vite on loopback addresses. Tests
cover login, settings, the share list, public shares, previews, and navigation.
Install Chromium once:

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm exec playwright install chromium
cd ..
make test-frontend
```

Browser tests use a temporary config path and require their own servers. They
refuse to reuse a running server. The default ports are `8888` for packageR
and `5173` for Vite. If either port is in use, select free ports:

```bash
PACKAGE_R_PORT=18888 PLAYWRIGHT_FRONTEND_PORT=15173 make test-frontend
```

The screenshot tests maintain the images used by the documentation.

For a manual STAC Browser preview, follow the
[local browser setup](../README.md#use-a-local-stac-browser).
The automated tests do not require a separate STAC Browser checkout.

## Presign load test

`make test-presign-load` enables the `loadtest` build tag. It runs 1,000 and
10,000 password-protected public presign requests with concurrency limits
of 32 and 8. Request start times are spread across 30 seconds. A simulated
object-store operation takes 20 ms; HTTP 429 responses retry after 25 ms.

Run this test alone on an idle host. Each workload must finish within
31 seconds without failed requests. The output reports latency, retries,
and maximum active requests.
