# Test Strategy

packageR uses layered tests. Small tests check one piece of code. Browser and
S3 tests check real flows. The goal is to catch most mistakes quickly, then use
slower tests only where they add real confidence.

This directory mainly holds shared fixture data in `tests/data`. The test code
itself lives in several places.

## Test Layers

| Layer | Where | Main command | What it checks |
| --- | --- | --- | --- |
| Backend tests | `*_test.go` files next to Go packages | `make test-backend` | Go logic, settings, rules, shares, catalog, public routes, presign fallback, file helpers |
| Frontend type check | `frontend/src` | `make test-frontend` | TypeScript and Vue types compile |
| Browser tests | `frontend/tests/*.spec.ts` | `make test-frontend-e2e` | Login, settings, public shares, previews, screenshots, visible UI behavior |
| Executable use cases | `docs/usecases/*.sh` | `make test-usecases` | The documented quickstart, public sharing, and STAC/catalog flows really work |
| Local S3 e2e | `scripts/e2e-s3.sh` | `make test-e2e-s3` | packageR can presign and redirect files from an S3-compatible backend |
| Kubernetes S3 e2e | `scripts/e2e-s3-k8s.sh` | `scripts/e2e-s3-k8s.sh` | The same S3 checks while packageR runs in kind with csi-rclone |

## What Each Layer Is For

Backend tests are the first choice for Go behavior. Add these when changing
rules, settings, HTTP handlers, catalog code, share paths, presigning, files, or
image helpers. They should be close to the package they test.

`make test-frontend` is a type check. It does not click through the UI. Use it
for frontend code changes, but do not treat it as browser coverage.

Browser tests run with Playwright. The make target builds a dev-tag backend,
starts packageR through `scripts/playwright_backend.sh`, starts Vite, and runs
Chromium tests. These tests are for behavior a user can see: login, settings,
public share pages, file info, previews, and screenshot-backed use cases.

Executable use cases are both docs and tests. The shell files under
`docs/usecases` are rendered into documentation, and `scripts/test_usecases.py`
runs them against a temporary packageR server. Use this layer when a documented
workflow changes.

The S3 e2e test starts a local `rclone serve s3` server and a local packageR
server. It uses `tests/data` as bucket content, then checks public catalog
output, authenticated presign, public-share presign, and redirect downloads.
Use this layer for changes that touch object storage, S3 signing, public share
downloads, or catalog asset URLs.

The Kubernetes S3 e2e test is the same scenario inside a kind cluster. It uses
the packageR Kubernetes/csi-rclone manifest, installs the EOEPCA csi-rclone
chart, uploads `tests/data` to an in-cluster rclone S3 server, and runs the
same catalog, presign, and redirect checks. It is a manual script for now, not
a Make target or GitHub Actions job.

## Main Commands

Run the common fast suite:

```bash
make test
```

This runs frontend typecheck, backend Go tests, and executable use cases. It
does not run Playwright browser tests or the S3 e2e test.

Run focused checks:

```bash
make test-backend
make test-frontend
make test-usecases
make test-frontend-e2e
make test-e2e-s3
```

For S3 e2e, local rclone must be available and must be at least v1.74:

```bash
RCLONE_BIN="$PWD/tools/bin/rclone" make test-e2e-s3
```

Run the manual kind/csi-rclone S3 check:

```bash
RCLONE_BIN="$PWD/tools/bin/rclone" scripts/e2e-s3-k8s.sh
```

Check generated use-case docs after editing `docs/usecases/*.sh`:

```bash
python3 scripts/render_usecases.py --check
```

## CI Strategy

GitHub Actions runs the layers as separate jobs:

- `test-frontend`
- `test-frontend-e2e`
- `test-backend`
- `test-usecases`
- `test-e2e-s3`

This keeps failures easier to read. The aggregate `test` job only waits for
those jobs to finish.

CI also runs frontend and backend lint jobs separately from tests.

The Kubernetes S3 e2e script is not part of CI yet.

## When To Add Which Test

Add a Go test when the change is backend logic or an API response.

Add a Playwright test when the change affects what users see or click.

Add or update a use-case script when the public documentation workflow changes.

Add S3 e2e coverage when the behavior depends on real S3-style presigned URLs
or redirect downloads.

Run the Kubernetes S3 e2e script when the Kubernetes deployment, csi-rclone
mounting, container image, or S3 behavior in-cluster is part of the change.

Add fixture files under `tests/data` only when a test needs stable sample
package data. Do not put secrets, real bucket credentials, or private URLs in
fixtures.

## Known Gaps

There are no separate frontend unit tests right now. Frontend confidence comes
from TypeScript checks and Playwright browser tests.

`make test` is useful locally, but it is not the whole CI gate. For changes that
touch UI flows or S3 behavior, also run the matching e2e command.
