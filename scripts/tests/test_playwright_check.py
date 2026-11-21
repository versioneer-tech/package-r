"""Check missing browser failures before builds, services, or tests start."""

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

    def check(self):
        return subprocess.run(
            [NODE, str(self.script)], env=self.environment,
            capture_output=True, text=True, check=False,
        )

    def test_missing_package_reports_setup_commands(self):
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("pnpm install --frozen-lockfile", result.stderr)
        self.assertIn("pnpm exec playwright install chromium", result.stderr)

    def test_available_browser_passes(self):
        self.browser("export const chromium = {launch: async () => ({close: async () => {}})};")
        result = self.check()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Chromium is available", result.stdout)

    @unittest.skipUnless(shutil.which("make"), "make is required")
    def test_missing_browser_stops_make_before_go_commands(self):
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
            "cc": "exit 0",
            "cmp": "exit 0",
            "curl": "exit 0",
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

        for target in ("test-check", "test", "test-frontend"):
            with self.subTest(target=target):
                result = subprocess.run(
                    ["make", target], cwd=self.root, env=self.environment,
                    capture_output=True, text=True, check=False,
                )
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("Chromium executable is missing", result.stderr)
                self.assertIn("pnpm exec playwright install chromium", result.stderr)
                self.assertFalse((self.root / "go-started").exists(), result.stdout + result.stderr)
