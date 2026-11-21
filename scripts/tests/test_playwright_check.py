"""Check test prerequisites before builds, services, or tests start."""

import json
import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
NODE = shutil.which("node")


@unittest.skipUnless(NODE, "Node.js is required for the browser prerequisite check")
class PlaywrightCheckTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="test-playwright-check-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.frontend = self.root / "frontend"
        self.script = self.frontend / "scripts/check-playwright.mjs"
        self.script.parent.mkdir(parents=True)
        shutil.copyfile(ROOT / "frontend/scripts/check-playwright.mjs", self.script)
        self.environment = {"HOME": str(self.root), "PATH": os.defpath}

    def browser(self, implementation):
        package = self.frontend / "node_modules/@playwright/test"
        package.mkdir(parents=True)
        (package / "package.json").write_text(json.dumps({
            "type": "module", "exports": "./index.js",
        }))
        (package / "index.js").write_text(implementation)

    @unittest.skipUnless(shutil.which("make"), "make is required")
    def test_incompatible_libraries_or_missing_browser_stops_make_before_go_commands(self):
        self.browser(
            "export const chromium = {launch: async () => {"
            "throw new Error('Chromium executable is missing');}};"
        )
        for filename in ("Makefile", "common.mk", "tools.mk", "tests/check-tools.bash", "scripts/check-rclone.sh"):
            destination = self.root / filename
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy(ROOT / filename, destination)
        binaries = self.root / "bin"
        binaries.mkdir()
        (binaries / "node").symlink_to(NODE)
        # The clean PATH omits tools installed by CI setup actions.
        commands = {
            "c++": 'printf "%s\\n" "$HOME/libstdc++.so.6"',
            "cc": "exit 0",
            "cmp": "exit 0",
            "curl": "exit 0",
            "getconf": 'cat "$HOME/glibc-version"',
            "pnpm": "exit 0",
            "python3": "exit 0",
            "jq": "exit 0",
            "stac-check": "exit 0",
            "uv": "exit 0",
            "rclone": "printf 'rclone v1.76.0\\n'",
            "go": 'touch "$HOME/go-started"; exit 1',
        }
        for name, command in commands.items():
            executable = binaries / name
            executable.write_text("#!/bin/sh\n" + command + "\n")
            executable.chmod(0o755)
        self.environment["PATH"] = str(binaries) + os.pathsep + os.defpath

        glibc = self.root / "glibc-version"
        libstdcxx = self.root / "libstdc++.so.6"
        for glibc_version, libstdcxx_version, message in (
            ("2.35", "3.4.32", "requires glibc 2.38"),
            ("2.38", "3.4.30", "requires libstdc++ with GLIBCXX_3.4.32"),
            ("", "3.4.32", "requires glibc 2.38"),
            ("2.38", None, "requires libstdc++ with GLIBCXX_3.4.32"),
        ):
            glibc.write_text(f"glibc {glibc_version}\n" if glibc_version else "")
            if libstdcxx_version is None:
                libstdcxx.unlink()
            else:
                libstdcxx.write_text(f"GLIBCXX_3.4.9\nGLIBCXX_{libstdcxx_version}\n")
            for target in ("test-check", "test"):
                with self.subTest(glibc=glibc_version, libstdcxx=libstdcxx_version, target=target):
                    result = subprocess.run(
                        ["make", target], cwd=self.root, env=self.environment,
                        capture_output=True, text=True, check=False,
                    )
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn(message, result.stderr)
                    self.assertFalse((self.root / "go-started").exists())

        for glibc_version, libstdcxx_version in (("2.38", "3.4.32"), ("2.40", "3.4.34")):
            glibc.write_text(f"glibc {glibc_version}\n")
            libstdcxx.write_text(f"GLIBCXX_3.4.9\nGLIBCXX_{libstdcxx_version}\n")
            for target in ("test-check", "test", "test-frontend"):
                with self.subTest(glibc=glibc_version, libstdcxx=libstdcxx_version, target=target):
                    result = subprocess.run(
                        ["make", target], cwd=self.root, env=self.environment,
                        capture_output=True, text=True, check=False,
                    )
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn("Chromium executable is missing", result.stderr)
                    self.assertIn("pnpm exec playwright install chromium", result.stderr)
                    self.assertFalse((self.root / "go-started").exists(), result.stdout + result.stderr)
