# Local development

Run commands from the repository root. Install the
[required tools](../tests/README.md#requirements) and
[download the Sentinel-2 fixture](../tests/README.md#test-data) first.
The local harness uses rclone S3 and synthetic credentials.

## Use VS Code

Build the frontend before the first launch and after frontend changes:

```bash
make build-frontend
```

Open **Run and Debug** and start **PackageR**. The launch task starts local S3,
builds the backend, prepares `.vscode/package-r.db`, and shares
`vienna-s2l2a-26`. It then starts the Go debugger and opens
`http://localhost:8888`.
Sign in with the local account `admin` and password `my-password`.

The launch configuration permits all CORS origins for local development.
Production CORS is disabled unless configured.

Stop the debug session to stop local S3. Rclone output is in
`.vscode/local-s3.log`; cache data is in `.vscode/cache`. If startup fails,
check the task output, log, and ports `19100` (S3), `19101` (rclone control),
and `8888` (packageR).

To control local S3 without VS Code:

```bash
./scripts/local_s3.sh serve
./scripts/local_s3.sh stop
```

`serve` stays in the foreground. Set `RCLONE_BIN` to an absolute path to use
another rclone binary.

## Use a local STAC Browser

Make a STAC Browser checkout available at `~/stac-browser`. This path can be
a symbolic link. Install its dependencies:

```bash
pnpm --prefix ~/stac-browser install
```

Start **PackageR & STAC Browser** in VS Code to run both services. This launch
sets the browser URL to `http://localhost:8080/external/`.
See the [catalog guide](../docs/how-to-guides/share-and-preview-data.md#publish-a-stac-compatible-parquet-catalog)
for public catalog endpoints and preview links. Automated browser tests do not
need a separate STAC Browser checkout.
