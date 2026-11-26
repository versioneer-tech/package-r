# Contributing

Use GitHub issues for bug reports and feature requests. For a bug, include the
affected version, steps to reproduce it, and the expected and actual results.
Use synthetic data and remove credentials, tokens, and presigned URLs.

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).

## Pull requests

Start from the latest `main` and keep each pull request focused on one change.
Explain the problem, how the change fixes it, and which checks you ran.
Use a Conventional Commit title, such as `fix: handle empty catalogs`.

Follow the [development guide](README.md#development) and run the applicable
[local CI checks](SECURITY.md#run-checks-locally). State any checks you could
not run and the reason.

Add tests for new behavior, bug fixes, and failures that could affect users or
releases. Use clear names and small examples. Check the result and avoid tests
that just repeat the code.

Update the documentation when public behavior changes. Preserve the
Apache-2.0 license and required upstream notices.
