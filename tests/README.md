# Test Strategy

packageR has two backend test levels: unit tests and local integration tests.
Frontend behavior is tested with Playwright. Lint, type checks, documentation
builds, and release builds are verification tasks, not separate test suites.

## Unit Tests

Go unit tests are next to the packages that they test. They use temporary
directories, memory filesystems, and small test doubles. They must not require
an external service.

Run all unit tests:

```bash
make test-unit
```

Add a unit test when behavior can be checked at a package boundary. Prefer a
small focused test over another process-level scenario.

## API Integration Tests

The integration harness is `tests/integration/run.bash`. It starts `rclone
serve s3` and packageR on loopback addresses. packageR accesses the test bucket
through its in-process rclone VFS. The API test uses `FB_ROOT=/` so it also
checks bucket navigation from the S3 service root.

The harness copies `tests/data` to a temporary bucket. The `public` directory
contains the catalog and its assets. Root-level sample files cover common file
types. The harness does not change the source fixtures. It checks:

- directory browse and raw reads;
- create, copy, rename, and delete operations;
- a two-chunk TUS upload through the VFS write cache;
- authenticated and share-password-protected public presigned URLs;
- rejection of authenticated runtime share creation;
- public share redirects; and
- catalog access through the VFS;
- STAC 1.1 validation of the live catalog endpoint; and
- one rewritten catalog asset fetch.

Install the pinned STAC validator and run the test with rclone 1.74 or newer:

```bash
python3 -m pip install -r tests/requirements.txt
make test-integration
```

Set `RCLONE_BIN` when rclone is not on `PATH`. The harness uses fixed synthetic
credentials and does not need cloud credentials. `stac-check` can retrieve the
schemas referenced by catalog extensions during validation.

Add an integration check only when the behavior depends on the real rclone VFS,
the S3 protocol, presigned URLs, or several API operations working together.

## Frontend Integration Tests

Frontend tests run locally only. They use Playwright and Chromium and start
local rclone, the Go development backend, and Vite on loopback addresses. They
cover login, settings, public shares, previews, and visible user flows.

Install the Chromium runtime once, then run the suite:

```bash
cd frontend
pnpm exec playwright install chromium
cd ..
make test-frontend
```

Add a Playwright test only for behavior that a user can see or operate. Keep
backend rules in Go unit tests or in the rclone integration harness.

## Run All Tests

```bash
make test
```

This runs Go unit tests, the local rclone integration harness, and Playwright.
Use the focused targets during development when only one level changed.
