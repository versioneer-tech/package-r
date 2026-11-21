"""Install the pinned Trivy release after its checksum check."""

import hashlib
import platform
import sys
import tarfile
import tempfile
import urllib.request
from pathlib import Path


def main(version: str, output: str) -> None:
    system = {"Linux": "Linux", "Darwin": "macOS"}[platform.system()]
    architecture = {"x86_64": "64bit", "aarch64": "ARM64", "arm64": "ARM64"}[platform.machine()]
    asset = f"trivy_{version}_{system}-{architecture}.tar.gz"
    base = f"https://github.com/aquasecurity/trivy/releases/download/v{version}/"
    with urllib.request.urlopen(base + f"trivy_{version}_checksums.txt", timeout=60) as response:
        checksums = response.read().decode()
    expected = next(line.split()[0] for line in checksums.splitlines() if line.split()[-1] == asset)
    destination = Path(output)
    destination.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="package-r-trivy-") as directory:
        archive = Path(directory) / asset
        urllib.request.urlretrieve(base + asset, archive)
        with archive.open("rb") as stream:
            if hashlib.file_digest(stream, "sha256").hexdigest() != expected:
                raise ValueError("Trivy archive checksum does not match")
        with tarfile.open(archive) as bundle:
            binary = bundle.extractfile("trivy")
            if binary is None:
                raise ValueError("Trivy archive has no binary")
            destination.write_bytes(binary.read())
        destination.chmod(0o755)


if __name__ == "__main__":
    main(*sys.argv[1:])
