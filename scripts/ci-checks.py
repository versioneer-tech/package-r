#!/usr/bin/env python3
# Copyright 2026, Versioneer (https://versioneer.at)
# SPDX-License-Identifier: Apache-2.0

"""Select documentation checks or the full test suite."""

import argparse
import subprocess
from pathlib import PurePosixPath


def documentation_only(paths):
    # Screenshot baselines are also browser test inputs.
    return bool(paths) and all(
        (path.startswith("docs/") and not path.startswith("docs/imgs/screenshots/")
         and PurePosixPath(path).suffix.lower() in {
             ".md", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".ico",
         })
        or path in {"docs/reference-guides/openapi.yaml", "tests/README.md"}
        or (path.endswith(".md") and ("/" not in path or path.startswith(".github/")))
        for path in paths
    )


def git(*arguments):
    return subprocess.run(["git", *arguments], check=True, capture_output=True).stdout


def select_checks(base, head=None):
    if not base:
        return False
    try:
        refs = [base] if head is None else [base, head]
        revisions = [
            git("rev-parse", "--verify", "--end-of-options", ref + "^{commit}").decode().strip()
            for ref in refs
        ]
        # Include both rename paths so moved runtime files still require tests.
        options = ("--name-only", "--no-renames", "-z")
        changed = git("diff", *options, *revisions, "--")
        if head is None:
            # Staged and unstaged edits can cancel in the worktree comparison.
            changed += git("diff", *options, "--cached", "--")
            changed += git("diff", *options, "--")
            changed += git("ls-files", "--others", "--exclude-standard", "-z")
    except (OSError, subprocess.CalledProcessError):
        return False
    paths = changed.decode("utf-8", "surrogateescape").split("\0")
    return documentation_only([path for path in paths if path])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", help="Base commit or target branch; missing refs select all checks.")
    parser.add_argument("--head", help="Head commit; omit to include staged, unstaged, and untracked files.")
    args = parser.parse_args()
    print("docs_only=" + str(select_checks(args.base, args.head)).lower())


if __name__ == "__main__":
    main()
