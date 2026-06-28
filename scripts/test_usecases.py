from __future__ import annotations

import argparse
import os
import socket
import subprocess
import sys
import tempfile
from pathlib import Path

from render_usecases import DEFAULT_SOURCE_DIR, DEFAULT_SOURCE_PATTERN, parse_usecase_shell

REPO_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_TIMEOUT_SECONDS = 90


def free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return int(sock.getsockname()[1])


def usecase_sources(source_dir: Path, source_pattern: str) -> list[Path]:
    sources = sorted(source_dir.glob(source_pattern))
    return sorted(sources, key=lambda path: (parse_usecase_shell(path).order, path.name))


def build_backend() -> None:
    env = os.environ.copy()
    env.setdefault("GOCACHE", "/tmp/package-r-go-build")
    subprocess.run(["make", "build-backend"], cwd=REPO_ROOT, env=env, check=True)


def run_usecase(source: Path, *, timeout: int) -> None:
    slug = source.stem
    port = free_port()
    with tempfile.TemporaryDirectory(prefix=f"package-r-{slug}.") as tmp:
        tmp_path = Path(tmp)
        env = os.environ.copy()
        env.update(
            {
                "PACKAGE_R_USECASE_TEST": "true",
                "PACKAGE_R_USECASE_SKIP_BUILD": "true",
                "FB_ROOT": str(tmp_path / "root"),
                "FB_DATABASE": str(tmp_path / "filebrowser.db"),
                "FB_FILEBROWSER_BIN": str(REPO_ROOT / "filebrowser"),
                "FB_SERVER_PORT": str(port),
                "BASE_URL": f"http://127.0.0.1:{port}",
                "PACKAGE_R_USECASE_SERVER_LOG": str(tmp_path / "filebrowser.log"),
            }
        )
        env.setdefault("GOCACHE", "/tmp/package-r-go-build")

        print(f"::group::{source.relative_to(REPO_ROOT)}")
        result = subprocess.run(
            ["bash", str(source)],
            cwd=REPO_ROOT,
            env=env,
            text=True,
            capture_output=True,
            timeout=timeout,
        )
        if result.stdout:
            print(result.stdout, end="")
        if result.stderr:
            print(result.stderr, end="", file=sys.stderr)
        if result.returncode != 0:
            server_log = tmp_path / "filebrowser.log"
            if server_log.exists():
                print("--- filebrowser.log ---", file=sys.stderr)
                print(server_log.read_text(encoding="utf-8", errors="replace"), file=sys.stderr)
            print("::endgroup::")
            raise SystemExit(result.returncode)
        print("::endgroup::")


def main() -> None:
    parser = argparse.ArgumentParser(description="Run executable packageR use cases.")
    parser.add_argument("--source-dir", type=Path, default=DEFAULT_SOURCE_DIR)
    parser.add_argument("--source-pattern", default=DEFAULT_SOURCE_PATTERN)
    parser.add_argument("--skip-build", action="store_true")
    parser.add_argument("--timeout", type=int, default=DEFAULT_TIMEOUT_SECONDS)
    args = parser.parse_args()

    if not args.skip_build:
        build_backend()

    sources = usecase_sources(args.source_dir, args.source_pattern)
    if not sources:
        raise SystemExit(f"No use cases matched {args.source_dir}/{args.source_pattern}")

    for source in sources:
        run_usecase(source, timeout=args.timeout)


if __name__ == "__main__":
    main()
