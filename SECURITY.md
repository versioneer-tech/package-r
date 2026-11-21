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

Pull requests and release tags use the same candidate build and security check.
The build runs on Ubuntu 22.04 with CGO enabled for DuckDB.

The runtime image uses Distroless C++ for Debian 13. It includes the C and C++
libraries for DuckDB and runs as UID 1000 and GID 100. The image has no shell or
BusyBox. Docker runs `package-r healthcheck` to check the local `/health`
endpoint. The probe uses `PACKAGE_R_ADDRESS` and `PACKAGE_R_PORT`, with loopback
for wildcard addresses, and fails after two seconds.

### Run the checks locally

Use Linux AMD64 with Go, a C compiler, Node.js, pnpm, uv, and Docker Buildx.
Run these commands from the repository root:

```bash
make candidate
make security
make security-verify
```

`make candidate` builds the frontend, production binary, binary archive, and
OCI image archive. It also saves the dependency manifests in `dist/candidate`.
`make security` installs the pinned Trivy release after a checksum check and
scans the existing candidate. It does not rebuild the candidate.

The scans cover:

- OS packages and Go dependencies in the candidate image
- Go dependencies and the Go standard library in the standalone release binary
- frontend production dependencies from the pnpm lockfile

The frontend scan excludes development dependencies. For a lockfile with
separate tooling and application documents, it scans the application document.
The source lockfile is unchanged.

### Go source checks

PR and main-branch CI run `make vulncheck`. The daily security workflow also
runs it against the default branch. It uses the pinned govulncheck release
to find calls to known vulnerable Go code, with CGO enabled. Calls to vulnerable
symbols and scanner errors fail the job. The text report is retained for
90 days.

This check uses govulncheck's call analysis and exit status. The severity
policy below applies to the Trivy scans.

### Blocking policy and reports

CRITICAL findings block publication, with or without a fix. HIGH findings
block only when a fix is available. The scan summary reports unfixed HIGH
findings in separate counts and ID lists. An image OS at end of support, a
scanner error, or a missing package inventory also blocks publication. The
gate requires all three scans and all three SBOMs. It does not use
vulnerability ignore lists.

The image and standalone binary inventories must contain versioned DuckDB
driver, bindings, and Linux AMD64 native-library modules. Their DuckDB
releases must match, and the bindings and native module versions must be
equal. Both Go SBOMs must include the native module at the reported version.
The summary records the native module and DuckDB release version, using
the [upstream version scheme](https://github.com/duckdb/duckdb-go-bindings/tree/v0.10505.0#semantic-versioning).
This checks the bundled library inventory; it does not add a separate scan
of DuckDB's C++ code or bundled extensions.

JSON reports contain all severities. Reports and CycloneDX JSON SBOMs are in
`build/security`. CI retains them for 90 days, including when a scan fails.
`scan-summary.json` records the result, scanner and database versions, and
report checksums.

### Release gate

A release tag must point to a commit that passed the main-branch Tests
workflow. The release workflow then builds and scans a new candidate.

The publish job downloads that candidate and its scan records. It checks the
image, binary archive, dependency inputs, and report checksums before it can
publish. It copies the checked OCI archive without a rebuild and requires the
registry to preserve its manifest digest.

The GitHub release contains the checked binary archive, image identity
manifest, three SBOMs, and an archive of the JSON scan records. Release
candidates do not update the `latest` image or documentation aliases.

### Daily release scans

The scheduled workflow scans all published `vnext` releases, including release
candidates. Each scan uses current vulnerability data and retains new reports
and SBOMs for 90 days. A failure marks the workflow as failed; it does not
remove or replace a published release.

When a release has a security manifest, the scan uses its immutable image
digest and source commit and checks the binary and dependency input hashes.
Without a security manifest, it scans the release image tag, published binary,
and tagged dependency manifests. The reports record the source commit and
image identity. A missing release artifact fails the scan.

To run the same rescan locally, put the source checkout in
`build/release-source` and the published binary archive in
`build/release-metadata/linux-amd64-package-r.tar.gz`. Put its security manifest
in `build/release-metadata/security-manifest.json` when available, then run:

```bash
make security-rescan RELEASE_IMAGE='ghcr.io/versioneer-tech/package-r@sha256:<digest>'
```
