# Security policy

## Supported releases

Security fixes apply to the latest stable packageR release.

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

CI tests and scans each production build before release. The release job
publishes the saved image and binary that passed these checks. Daily jobs
scan published releases again with current vulnerability data.

### CI workflows

The [workflows](.github/workflows/) use the same Make targets as local checks.
Jobs run on Ubuntu 24.04. The test job uses Playwright's Ubuntu 24.04 container
so screenshot comparisons use the same browser fonts.

| File | Trigger | Purpose |
| --- | --- | --- |
| [pr-lint.yaml](.github/workflows/pr-lint.yaml) | PR opened or updated, including title edits | Check the PR title against the Conventional Commit format. |
| [pr.yaml](.github/workflows/pr.yaml) | PRs targeting `main`, pushes to `main`, or manual runs | Select and run the required checks. Build and scan a candidate for code changes. |
| [security-check.yaml](.github/workflows/security-check.yaml) | Called by the Tests or release workflow | Scan the supplied candidate and save reports and package inventories. |
| [go-vulnerabilities.yaml](.github/workflows/go-vulnerabilities.yaml) | Called by Tests or the daily workflow | Check Go source with govulncheck and save its report. |
| [tag.yaml](.github/workflows/tag.yaml) | A `vnext.X.Y.Z` or `vnext.X.Y.Z-rc.N` tag is pushed | Verify the tested commit, build and scan a release candidate, then publish the release and documentation. |
| [security-daily.yaml](.github/workflows/security-daily.yaml) | Daily at 04:23 UTC, or manually | Rescan supported releases and check current Go source. |

Full Tests runs include:

- `make lint-files`: file validation, Markdown, Dockerfiles, and secret checks
- `make lint-backend lint-frontend`: Go formatting and lint, frontend lint, and types
- `make docs`: strict documentation and OpenAPI validation
- `make test`: tooling, Go unit and race tests, rclone API tests, and Chromium
- a production candidate build and smoke check, Trivy scans, and govulncheck

Documentation-only changes run file, tooling, and documentation checks. They
skip runtime tests, builds, and vulnerability scans. Manual Tests runs select
all checks. The `required_checks` job fails if a required job fails or is
unexpectedly skipped. To block merges when checks fail, require this status
in the repository's branch rules.

The [test guide](tests/README.md) covers setup and test scope. Timed load tests
run separately with `make performance`.

### Candidate build

A candidate is a saved production build. CI runs it and scans it before the
release job publishes it. The release job uses the saved files without
building them again.

The [build-candidate action](.github/actions/build-candidate/action.yml) runs
`make candidate` with a version and source commit. It uses the Go version in
`go.mod` and the pnpm version in `frontend/package.json`. DuckDB needs CGO,
so the build keeps it enabled.

`make candidate` installs frontend dependencies from the frozen pnpm lockfile,
builds the frontend, embeds it in the Go binary, and builds the Linux AMD64
container image. It saves these files:

| Path under `dist/candidate/` | Contents |
| --- | --- |
| `image.tar` | OCI container image archive |
| `linux-amd64-package-r.tar.gz` | Standalone production binary |
| `security-manifest.json` | Version, full source commit, image digest, and artifact checksums |
| `inputs/` | Go and frontend dependency manifests used by the scans |

Before upload, `make smoke-candidate` runs the saved binary and image against
a temporary local rclone S3 service. Both must start, report the expected
version, pass the health probe, serve the embedded JavaScript bundle, and
read a shared object through a presigned URL. The check removes its processes,
container, image tag, volume, and temporary data when it ends.

The action uploads the directory to GitHub Actions. Later jobs download it
by its artifact ID and check its contents against the saved checksums and
image digest.

The standalone binary needs the host libraries listed in
[README.md](README.md#binary).

The runtime image uses Distroless C++ for Debian 13. It includes the C and C++
libraries for DuckDB and runs as UID 1000 and GID 100, without a shell or
BusyBox. Its health check calls the local `/health` endpoint through
`package-r healthcheck`, with a two-second timeout.

The build copies Debian's patched `libzstd1` library and its package records
into the runtime image while the base image still contains the affected
version. The candidate scan checks the resulting image inventory.

### Candidate scans

`make security` installs the pinned Trivy release after a checksum check, then
scans the saved candidate without rebuilding it.

| Scan | What is checked |
| --- | --- |
| Image | OS packages, compiled Go dependencies, and the Go standard library |
| Standalone binary | Compiled Go dependencies and the Go standard library in the release download |
| Frontend | Production dependencies from `frontend/pnpm-lock.yaml` |

The image scan cannot identify all dependencies in the embedded JavaScript.
A separate scan reads production dependencies from the frontend lockfile;
it excludes development dependencies. If the lockfile has separate tooling
and application documents, the scan uses the application document and leaves
the source lockfile unchanged.

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

The reports must identify the required packages: the image OS and its packages,
the Go standard library and its version, and frontend production dependencies.
All three scans and all three software bills of materials (SBOMs) must be
present. The gate does not use vulnerability ignore lists.

#### DuckDB native library

The image and standalone binary inventories must contain versioned DuckDB
driver, bindings, and Linux AMD64 native-library modules. Their DuckDB
releases must match, and the bindings and native module versions must be
equal. Both Go SBOMs must include the native module at the reported version.

`scan-summary.json` records the native module and DuckDB release version,
using the [upstream version scheme](https://github.com/duckdb/duckdb-go/tree/v2.20000.0#readme).
These checks confirm which DuckDB library is bundled. They do not scan its C++
code or extensions separately.

### Release gate

A pushed release tag starts this sequence:

1. Verify that the exact tagged commit passed a full Tests run on `main`,
   through a push or manual run. A successful PR run alone is not sufficient.
   For a documentation-only commit, run Tests manually on `main` first.
2. Build a new candidate with the release version and pass its smoke check.
3. Scan the new candidate and apply the blocking policy.
4. Download the candidate and scan records by artifact ID. Check the image,
   binary archive, dependency inputs, and report checksums with
   `make security-verify`.
5. Copy the checked OCI archive to the registry without rebuilding it, and
   preserve its image manifest digest.
6. Create the GitHub release with the binary archive, security manifest,
   three SBOMs, an archive of the scan records, and notes from `CHANGELOG.md`.
7. Build and publish versioned documentation.

The image receives the release tag and an additional `-amd64` tag. Stable
releases also update the `latest` image and documentation aliases. RC releases
are marked as prereleases and do not update those aliases.

Release notes start with the matching `## vnext.X.Y.Z` CHANGELOG section,
followed by GitHub's generated notes and comparison link. RC tags use an exact
RC section if present, otherwise the base version's section. A missing or
empty section stops publication.

Checking jobs have read permissions. The release job can write to the registry
and GitHub releases. The documentation job can write to `gh-pages`.

Publication has several steps. If a later step fails, the image can already
be in the registry while the GitHub release or documentation is still missing.

### Sign commits

List the keys on the computer where you will sign commits:

```sh
gpg --list-secret-keys --keyid-format LONG --fingerprint
```

Choose your signing key and copy the full fingerprint below its `sec` line.
If no key is listed, create one with `gpg --full-generate-key`, then repeat
the command above. Use an email verified on your GitHub account.

Export the public key and configure Git for this checkout. Replace
`YOUR_FULL_KEY_FINGERPRINT` with the fingerprint you copied:

```sh
gpg --armor --export YOUR_FULL_KEY_FINGERPRINT > /tmp/release-signing-key.asc
git config --local gpg.format openpgp
git config --local user.signingkey YOUR_FULL_KEY_FINGERPRINT
git config --local commit.gpgsign true
```

Add the contents of `/tmp/release-signing-key.asc` to your GitHub account.
See [GitHub's GPG setup](https://docs.github.com/en/authentication/managing-commit-signature-verification/adding-a-gpg-key-to-your-github-account).
Keep the private key on your signing device.

This signs new commits and leaves tags unsigned. The release workflow accepts
unsigned tags. It does not verify commit or tag signatures. To require signed
commits on `main`, configure GitHub's
[signed commit rule](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets#require-signed-commits).

### Create a release

Choose an unused version. Add a matching `## vnext.X.Y.Z - YYYY-MM-DD` section
to `CHANGELOG.md`, with a short list of changes since the previous release.
Check the notes that CI will publish. Replace the example version below with
the version you chose:

```sh
release_tag=vnext.1.2.1
python3 scripts/release_notes.py "$release_tag"
```

Commit and push the changes to `main`, then wait for a full successful Tests
run on that commit. For a documentation-only commit, open **Actions → Tests →
Run workflow**, select `main`, and wait for all checks to pass.

From a clean `main` checkout at that same commit, create and push the tag:

```sh
git tag "$release_tag"
git push origin "$release_tag"
```

This tag needs no message. CI reads the release notes from `CHANGELOG.md`.
Editing `CHANGELOG.md` after publication does not change an existing release.

### Daily release scans

The daily workflow scans the latest stable `vnext` release. If a newer version
has release candidates (RCs), it also scans the newest RC. With no stable
release, it scans only the newest RC. It skips drafts, older releases, and
superseded RCs. Versions are compared by number, regardless of publication date.

For a manual scan of older or deployed versions, set the workflow's
`release-tags` input to published tags separated by commas. An empty input
uses the scheduled selection. An unknown tag fails selection.

Each release scan checks the image, published binary, and frontend production
dependencies against current vulnerability data, using the same blocking
policy. A finding in a published artifact can require a new release; changes
to the default branch do not change that artifact.

When a release has a security manifest, the scan uses its immutable image
digest and source commit and checks the binary and dependency input hashes.
Without a security manifest, it scans the release image tag, published binary,
and tagged dependency manifests. The reports record the source commit and
image identity. A missing release artifact fails the scan.

The daily job does not rebuild, remove, or replace a release. A blocking
finding fails the workflow.

### Reports and retention

Trivy writes JSON reports and CycloneDX JSON SBOMs to `build/security/`.
An SBOM lists detected packages and their versions. Scan reports list known
vulnerabilities at all severities. `scan-summary.json` records the result,
scanner and vulnerability database versions, unfixed HIGH counts and IDs,
and report checksums.
govulncheck writes `build/security/govulncheck.txt`.

| Record | Where it is retained |
| --- | --- |
| Candidate image, binary archive, manifest, and dependency inputs | GitHub Actions artifact for seven days |
| Candidate and daily Trivy reports and SBOMs | GitHub Actions artifacts for 90 days, including scan failures |
| govulncheck text report | GitHub Actions artifact for 90 days, including check failures |
| Release binary archive, security manifest, three SBOMs, and scan record archive | GitHub release assets |

### Limits

Scans can report only known vulnerabilities in packages they detect. Frontend
development dependencies and general build dependencies have no separate
Trivy scan.

Checksums and image digests detect changes to saved files and images. They do
not prove who built them. Contributors sign commits locally. CI does not
verify commit or tag signatures, sign images, or generate signed build
attestations.

External Actions use full commit SHAs. Production base images and the
Playwright test image use content digests. Dependabot checks the configured
Action and Docker dependencies for updates. The shared rclone installer checks
its archive against a fixed SHA-256 checksum.

## Developer checks

### Hooks

Install the hooks once per checkout:

```bash
make hooks
```

Commit hooks run the applicable Go and frontend linters, file validation,
markdownlint, Hadolint, and Gitleaks. Gitleaks checks staged content with
redacted output; CI checks repository files. Neither scans full Git history.
Hooks do not run tests or build the application.

Commit messages use Conventional Commits, such as `fix: handle empty catalogs`.
The push hook checks branch names. CI checks the PR title.

### Run checks locally

Use Linux AMD64 with the [required tools](tests/README.md#requirements),
Docker Buildx, and Skopeo. The candidate smoke check needs a local Docker
daemon at `/var/run/docker.sock`. Use the pinned Playwright CI container for
browser screenshot comparisons. Run these commands from the repository root.
The subshell stops at the first failed command:

```bash
(
  set -e

  make check docs
  make setup-tests
  make test
  make vulncheck

  make candidate VERSION="snapshot-$(git rev-parse HEAD)"
  make smoke-candidate
  make security
  make security-verify
)
```

These are the same Make targets used by CI. Local checks do not publish
artifacts. The candidate stays in `dist/candidate/`, and scan records stay
in `build/security/` for inspection.

For change selection, use the known PR base commit or the target branch:

```bash
python3 scripts/ci-checks.py --base main
```

Without `--head`, the selector includes committed, staged, unstaged, and
untracked changes. For `docs_only=true`, run
`make lint-files test-tooling docs`. Otherwise, run the full sequence above.
Dependency files, build configuration, screenshot baselines, unknown files,
and empty or unavailable comparisons select full checks.

To rescan a release, put its source checkout at the release commit in
`build/release-source` and its published binary archive in
`build/release-metadata/linux-amd64-package-r.tar.gz`. Put its security manifest
in `build/release-metadata/security-manifest.json` when available, then run:

```bash
make security-rescan RELEASE_IMAGE='ghcr.io/versioneer-tech/package-r@sha256:<digest>'
```

For a release without a security manifest, use its image tag instead of a
digest. Use the matching source commit and binary archive in both cases.

### Cleanup

`make clean` removes binaries, build and release outputs, generated documentation,
and test reports. It keeps downloaded test data, frontend and Python dependencies,
and local databases. `pnpm run clean` in `frontend` uses `make clean-frontend`.

### Dependency updates

[Dependabot](.github/dependabot.yml) checks Docker images daily. It checks Go,
frontend, Actions, hooks, and Python support dependencies weekly and opens
update PRs. Published releases need a new release to include dependency fixes.
