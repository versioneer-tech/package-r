"""Check that cleanup preserves local data and installed dependencies."""

import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


@unittest.skipUnless(shutil.which("make"), "make is required")
class CleanupTests(unittest.TestCase):
    def test_clean_removes_outputs_and_preserves_data_and_dependencies(self):
        with tempfile.TemporaryDirectory(prefix="test-clean-") as directory:
            root = Path(directory)
            for filename in ("Makefile", "common.mk", "tools.mk"):
                shutil.copy(ROOT / filename, root / filename)
            outputs = (
                "package-r", "build/security/report.json", "frontend/dist/assets/app.js",
            )
            retained = (
                "frontend/dist/.gitkeep", "frontend/node_modules/package/index.js",
                ".venv/bin/python", "local.db",
                "tests/data/vienna-s2l2a-26/vienna-s2l2a-26.parquet",
            )
            for filename in (*outputs, *retained):
                path = root / filename
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("test content\n")
            result = subprocess.run(
                ["make", "clean"], cwd=root, env={"PATH": os.defpath},
                capture_output=True, text=True, check=False,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            for filename in outputs:
                self.assertFalse((root / filename).exists(), filename)
            for filename in retained:
                self.assertEqual((root / filename).read_text(), "test content\n", filename)
