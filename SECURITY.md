# Security policy

## Supported releases

Security fixes apply to the latest published packageR release.

## Report a vulnerability

Send security reports to `oss@versioneer.at`. Do not open a public issue for
a vulnerability.

Include this information when possible:

- the affected packageR release or commit
- the expected and actual behavior
- steps to reproduce the problem
- the possible impact
- a proposed fix, if you have one

Do not include live credentials, tokens, presigned URLs, or private data.

## Release checks

CI checks changes before release. A release publishes the image and binary
that passed the security scan. Daily jobs scan published releases again with
current vulnerability data.

### CI workflows

The [workflows](.github/workflows/) use shared Make targets for local and CI
checks. All jobs that select a runner use Ubuntu 24.04.

| File | Trigger | Purpose |
| --- | --- | --- |
| [pr-lint.yaml](.github/workflows/pr-lint.yaml) | PR opened or updated, including title edits | Check the PR title against the Conventional Commit format. |
| [pr.yaml](.github/workflows/pr.yaml) | PRs targeting `main`, pushes to `main`, or manual runs | Select and run the required checks. Build and scan a candidate for code changes. |
| [security-check.yaml](.github/workflows/security-check.yaml) | Called by the Tests or release workflow | Scan the supplied candidate and save reports and package inventories. |
| [go-vulnerabilities.yaml](.github/workflows/go-vulnerabilities.yaml) | Called by Tests or the daily workflow | Check Go source with govulncheck and save its report. |
| [tag.yaml](.github/workflows/tag.yaml) | A `vnext.X.Y.Z` or `vnext.X.Y.Z-rc.N` tag is pushed | Verify the tested commit, build and scan a release candidate, then publish the release and documentation. |
| [security-daily.yaml](.github/workflows/security-daily.yaml) | Daily at 04:23 UTC, or manually | Rescan published releases and check current Go source. |

Full Tests runs include:

- file syntax, whitespace, Markdown, Dockerfile, and secret checks
- tests for CI selection, secret scan boundaries, and release security checks
- strict documentation and OpenAPI validation
- Go formatting and lint, frontend lint, and TypeScript checks
- the full Go unit suite and focused race detection
- API integration tests with local rclone 1.76 S3 and synthetic credentials
- the production candidate build, Trivy scans, and govulncheck

Gitleaks scans selected files with redacted output. Commit hooks check staged
content; CI checks repository files. These checks do not scan full Git history.

Documentation-only changes run file, tooling, and documentation checks. They
skip runtime tests, builds, and vulnerability scans. Manual Tests runs select
all checks. The `required_checks` job fails if a required job fails or is
unexpectedly skipped. Repository rules must require this status to block
merging.

Playwright browser tests and timed presign load tests are explicit local
commands. See [tests/README.md](tests/README.md) for test scope and commands.

### Candidate build

A candidate is a build saved for scanning before publication. The shared
[build-candidate action](.github/actions/build-candidate/action.yml) sets up Go
from `go.mod`, pnpm from `frontend/package.json`, Node.js, uv, and Docker Buildx.
It runs `make candidate` with a version and source commit. CGO stays enabled
for DuckDB.

The Make target builds the frontend with the frozen pnpm lockfile, embeds it
in the production Go binary, and builds the Linux AMD64 image. It saves:

| Path under `dist/candidate/` | Contents |
| --- | --- |
| `image.tar` | OCI container image archive |
| `linux-amd64-package-r.tar.gz` | Standalone production binary |
| `security-manifest.json` | Version, full source commit, image digest, and artifact checksums |
| `inputs/` | Go and frontend dependency manifests used by the scans |

The action uploads this directory and returns its GitHub artifact ID. Later
jobs download that exact upload. The artifact ID identifies the upload; the
image digest identifies the image contents.

The standalone binary needs compatible glibc and libstdc++ libraries on the
host. Use Ubuntu 24.04 or a system with compatible C and C++ libraries.

The runtime image uses Distroless C++ for Debian 13. It includes the C and C++
libraries for DuckDB and runs as UID 1000 and GID 100, without a shell or
BusyBox. Its health check calls the local `/health` endpoint through
`package-r healthcheck`, with a two-second timeout.

### Candidate scans

`make security` installs the pinned Trivy release after a checksum check, then
scans the saved candidate without rebuilding it.

| Scan | What is checked |
| --- | --- |
| Image | OS packages, compiled Go dependencies, and the Go standard library |
| Standalone binary | Compiled Go dependencies and the Go standard library in the release download |
| Frontend | Production dependencies from `frontend/pnpm-lock.yaml` |

The separate frontend scan is needed because the JavaScript assets are
embedded in the Go binary. An image scan cannot recover their full dependency
inventory. The frontend scan excludes development dependencies. If the
lockfile has separate tooling and application documents, the scan uses the
application document and leaves the source lockfile unchanged.

### Go source checks

`make vulncheck` runs the pinned govulncheck release with CGO enabled. It finds
calls to known vulnerable Go symbols. Such calls and scanner errors fail the
job. Full Tests runs check the current commit. Scheduled runs check the
default branch, rather than running call analysis for each published release.

govulncheck uses call analysis and its own exit status. The Trivy severity
policy below does not apply to it.

### Blocking policy and reports

All three Trivy scans use this policy:

| Finding | Result |
| --- | --- |
| CRITICAL, with or without a fix | Block publication |
| HIGH with an available fix | Block publication |
| HIGH without an available fix | Report separately; do not block |
| MEDIUM, LOW, or UNKNOWN | Include in the JSON reports; do not block |
| Image OS at end of support | Block publication |
| Scanner error or missing required inventory, report, or SBOM | Block publication |

The checks require an identified image OS, OS packages, versioned Go standard
libraries, and a frontend production dependency inventory. All three scans
and all three software bills of materials (SBOMs) must be present. The gate
does not use vulnerability ignore lists.

#### DuckDB native library

The image and standalone binary inventories must contain versioned DuckDB
driver, bindings, and Linux AMD64 native-library modules. Their DuckDB
releases must match, and the bindings and native module versions must be
equal. Both Go SBOMs must include the native module at the reported version.

`scan-summary.json` records the native module and DuckDB release version,
using the [upstream version scheme](https://github.com/duckdb/duckdb-go/tree/v2.20000.0#readme).
This verifies the bundled library inventory. It does not add a separate scan
of DuckDB's C++ code or extensions.

### Release gate

A pushed release tag starts this sequence:

1. Verify that the exact tagged commit passed a full Tests run on `main`,
   through a push or manual run. A successful PR run alone is not sufficient.
   For a documentation-only commit, run Tests manually on `main` first.
2. Build a new candidate with the release version. The earlier PR or main
   candidate is not reused.
3. Scan the new candidate and apply the blocking policy.
4. Download the candidate and scan records by artifact ID. Check the image,
   binary archive, dependency inputs, and report checksums with
   `make security-verify`.
5. Copy the checked OCI archive to the registry without rebuilding it.
   Require the registry to preserve its image manifest digest.
6. Create the GitHub release with the binary archive, security manifest,
   three SBOMs, and an archive of the scan records.
7. Build and publish versioned documentation.

The image receives the release tag and an additional `-amd64` tag. Stable
releases also update the `latest` image and documentation aliases. RC releases
are marked as prereleases and do not update those aliases.

Checking jobs use read permissions. Registry and release publication receive
write permissions in the release job. The documentation job has write access
to publish to `gh-pages`.

These steps are separate operations. A failure after image publication can
leave an image published before the GitHub release or documentation is ready.

### Daily release scans

The scheduled workflow scans all published `vnext` releases, including release
candidates. Draft releases are excluded. Each release scan checks the image,
published binary, and frontend production dependencies against current
vulnerability data, using the same blocking policy.

When a release has a security manifest, the scan uses its immutable image
digest and source commit and checks the binary and dependency input hashes.
Without a security manifest, it scans the release image tag, published binary,
and tagged dependency manifests. The reports record the source commit and
image identity. A missing release artifact fails the scan.

The daily job does not rebuild, remove, or replace a release. A blocking
finding fails the workflow. Dependabot opens dependency update PRs through
[its separate configuration](.github/dependabot.yml); it does not repair an
already published artifact.

### Reports and retention

Trivy writes JSON reports and CycloneDX JSON SBOMs to `build/security/`.
Each SBOM lists detected packages and their versions. Reports include all
severities. `scan-summary.json` records the result, scanner and vulnerability
database versions, unfixed HIGH counts and IDs, and report checksums.
govulncheck writes `build/security/govulncheck.txt`.

| Record | Where it is retained |
| --- | --- |
| Candidate image, binary archive, manifest, and dependency inputs | GitHub Actions artifact for seven days |
| Candidate and daily Trivy reports and SBOMs | GitHub Actions artifacts for 90 days, including scan failures |
| govulncheck text report | GitHub Actions artifact for 90 days, including check failures |
| Release binary archive, security manifest, three SBOMs, and scan record archive | GitHub release assets |

### Run the checks locally

Use Linux AMD64 with Go, a C/C++ toolchain, Node.js, pnpm, uv, and Docker Buildx.
Run these shared commands from the repository root:

```bash
make lint-files test-tooling docs
make lint-backend lint-frontend
make test-unit test-race
make test-integration
make vulncheck
make candidate
make security
make security-verify
```

API integration needs rclone 1.76 or newer and the generated Sentinel-2
fixture. Follow [tests/README.md](tests/README.md) for setup and check
selection. Local checks do not publish artifacts.

To rescan a release, put its source checkout at the release commit in
`build/release-source` and its published binary archive in
`build/release-metadata/linux-amd64-package-r.tar.gz`. Put its security manifest
in `build/release-metadata/security-manifest.json` when available, then run:

```bash
make security-rescan RELEASE_IMAGE='ghcr.io/versioneer-tech/package-r@sha256:<digest>'
```

For a release without a security manifest, use its image tag instead of a
digest. Use the matching source commit and binary archive in both cases.

### Limits

The checks depend on the scanners' package detection and vulnerability data.
A successful scan does not prove that a release has no vulnerabilities.
Frontend development dependencies and general build dependencies have no
separate Trivy scan.

Artifact checksums and image digests verify content identity. The workflows
do not sign commits, tags, or images, and do not generate signed build
attestations.
