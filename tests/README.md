# Test strategy

packageR has Go unit tests, API integration tests, and browser tests. Lint,
type checks, documentation builds, and release builds are separate checks.

## Unit tests

Go unit tests are next to the packages that they test. They use temporary
directories, memory filesystems, and small test doubles. They do not use
external services.

Run all unit tests:

```bash
make test-unit
```

Use a unit test when you can check behavior at a package boundary.

## API integration tests

The integration harness is `tests/integration/run.bash`. It starts `rclone
serve s3` and packageR on loopback addresses. packageR reads the test bucket
with its embedded rclone VFS. The test uses `PACKAGE_R_ROOT=/` to include S3
service-root navigation.

The harness copies `tests/data` to a temporary bucket and does not change the
source files. It checks:

- directory listing and raw reads
- create, copy, rename, and delete operations
- a two-chunk TUS upload through the VFS write cache
- authenticated and password-protected public presigned URLs
- rejection of runtime share creation
- public share redirects
- catalog access through the VFS
- STAC 1.1 validation of the live catalog endpoint
- a fetch through a rewritten catalog asset URL

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
Vite on loopback addresses. They cover login, settings, public shares,
previews, and other user workflows.

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
