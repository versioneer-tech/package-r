"""Scan only the files selected by pre-commit."""

import shutil
import subprocess
import sys
import tempfile
from pathlib import Path


def main(filenames: list[str]) -> int:
    with tempfile.TemporaryDirectory(prefix="package-r-secrets-") as directory:
        destination = Path(directory)
        for filename in filenames:
            source = Path(filename)
            if source.is_absolute() or ".." in source.parts:
                raise ValueError("Expected a path inside the repository")
            if any(path.is_symlink() for path in (source, *source.parents)):
                continue
            if not source.is_file():
                continue
            target = destination / source
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(source, target)
        return subprocess.run(
            ["gitleaks", "dir", "--redact", "--no-banner", str(destination)],
            check=False,
        ).returncode


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
