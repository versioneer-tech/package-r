# Tests and performance

## Tests

Run this from the repository root on Linux:

```bash
make setup-tests && make test
```

Setup installs frontend dependencies, installs Chromium and its system libraries
if Chromium cannot launch, and downloads missing Sentinel-2 test data. The
system library install can ask for `sudo`. Run setup again after a Playwright
update.

### Requirements

Install these tools before setup:

- glibc 2.38 or newer and libstdc++ with `GLIBCXX_3.4.32` support.
- Go, a C/C++ toolchain, and binutils. DuckDB needs CGO.
- Node.js and pnpm, as specified in `frontend/package.json`.
- Python 3.11 or newer, uv, and jq.
- rclone 1.76 or newer. Set `RCLONE_BIN` to an absolute path if it is not on `PATH`.

`make test-check` checks the C/C++ libraries and tools, then launches headless
Chromium. An old or missing library, Playwright package, or browser
stops `make test` before the suites or services start. Individual suites can
run on other systems. They build against the host's libraries and do not prove
that the prebuilt release binary can run there. If you set
`PLAYWRIGHT_BROWSERS_PATH`, use the same value for setup and tests.

### Test suites

`make test` runs every `test-*` target below, in order, and stops on failure.
CI uses the same `make setup-tests` and `make test` commands. Each suite can
also run on its own:

| Command | Checks |
| --- | --- |
| `make test-check` | C/C++ library versions, required tools, and a headless Chromium launch |
| `make test-tooling` | Repository tools, hooks, CI selection, and release gates |
| `make test-unit` | All Go unit tests |
| `make test-race` | Race detection in authentication, cache, HTTP, logging, metrics, and rclone storage code |
| `make test-integration` | API operations through local rclone S3 |
| `make test-frontend` | Playwright Chromium browser tests |

### Test data

API and browser tests use a temporary copy of `tests/data`, local rclone S3,
and synthetic credentials. They preserve the source data and do not use cloud
accounts.

`make setup-tests` downloads the Vienna Sentinel-2 fixture to
`tests/data/vienna-s2l2a-26` if its Parquet catalog is missing. To download only
this fixture:

```bash
uv run --no-project --with-requirements tests/requirements.txt \
  python scripts/download_sentinel2.py
```

The download needs network access. See the
[catalog guide](../docs/how-to-guides/share-and-preview-data.md#publish-a-stac-compatible-parquet-catalog)
for the source data and format.

### API integration tests

`make test-integration` starts rclone S3 and packageR on loopback addresses.
It supplies the pinned STAC validator through uv when needed. The harness
selects free ports, waits for readiness, and stops its own processes.

It checks:

- user, settings, sign-up, and share-management APIs are unavailable
- the share list exposes safe links within the user's scope
- service-root and bucket listings work with an explicit bucket catalog
- create, copy, rename, delete, and presigned reads work through rclone VFS
- TUS uploads can resume and abort, and use `HEAD` for offsets
- public shares support passwords, temporary tokens, and presigned URLs
- catalog routes return valid STAC Collections, Items, and mapped assets
- metrics omit object paths
- generated homes permit access to the owner's files and deny sibling homes

### Browser tests

To prepare and run only the browser suite:

```bash
make setup-tests && make test-frontend
```

Playwright starts rclone, packageR, and Vite on loopback addresses. It checks
login, settings, the share list, public shares, previews, and navigation.
The backend uses a temporary config path. The tests require their own servers
and refuse to reuse a running server.

The default ports are `8888` for packageR and `5173` for Vite. If either is in
use, select free ports:

```bash
PACKAGE_R_PORT=18888 PLAYWRIGHT_FRONTEND_PORT=15173 make test-frontend
```

Screenshot tests maintain the images used by the documentation. CI uses
Playwright's pinned Ubuntu 24.04 (`noble`) container. Use the same container
to compare or update these images. On other operating systems, the six
screenshot tests are skipped with a warning; the other browser tests still run.
Follow
Playwright's [rendering environment guidance](https://playwright.dev/docs/test-snapshots)
for consistent comparisons. After an intended visual change, update the images
in the CI environment and review the diff:

```bash
make test-frontend PLAYWRIGHT_ARGS=--update-snapshots=changed
```

The HTML report is in `frontend/playwright-report/`. Failure details are in
`frontend/test-results/`. CI saves both for seven days when tests fail.
Open the report with `(cd frontend && pnpm exec playwright show-report)`.

The tests do not need a separate STAC Browser checkout. For a manual preview,
follow the [local STAC Browser setup](../.vscode/README.md#use-a-local-stac-browser).

### Select checks for a change

Use the PR base commit, or the target branch for other changes:

```bash
python3 scripts/ci-checks.py --base main
```

Without `--head`, this includes committed, staged, unstaged, and untracked
changes. Empty or unavailable comparisons select full checks.

For `docs_only=true`, run `make lint-files test-tooling docs`. Otherwise,
also run `make test` and the applicable lint, build, and security checks.
CI uses the same selector. Manual CI runs use the full suite. Browser screenshot
baselines, Markdown test fixtures, and frontend assets require full checks.

Lint, documentation, build, and vulnerability checks have separate Make targets.
See [release security checks](../SECURITY.md#release-checks) for `make vulncheck`
and Trivy.

## Performance (load tests)

Run this separately on an idle host, from the repository root:

```bash
make performance
```

It needs Go and the C/C++ toolchain. It uses synthetic data and a simulated
object store, so no setup target, Sentinel-2 data, rclone, or Playwright is
needed. These timed workloads run only through `make performance`.

The command enables the `loadtest` build tag and runs four workloads: 1,000
and 10,000 password-protected public presign requests, each with concurrency
limits of 32 and 8. Request start times are spread across 30 seconds. The
simulated object-store operation takes 20 ms; HTTP 429 responses retry after
25 ms.

Each workload must finish within 31 seconds without failed requests. The
output reports latency, retries, and maximum active requests. The full run
takes about two minutes.
