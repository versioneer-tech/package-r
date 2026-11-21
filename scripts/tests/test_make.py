"""Check test selection and safe cleanup."""

import os
import re
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


@unittest.skipUnless(shutil.which("make"), "make is required")
class TestCommandTests(unittest.TestCase):
    def test_clean_removes_outputs_and_preserves_data_and_dependencies(self):
        with tempfile.TemporaryDirectory(prefix="test-clean-") as directory:
            root = Path(directory)
            for filename in ("Makefile", "common.mk", "tools.mk"):
                shutil.copy(ROOT / filename, root / filename)
            outputs = (
                "package-r", "bin/helper", "tools/bin/linter", "build/security/report.json",
                "dist/candidate/image.tar", "site/index.html", "docs/generated/api.md",
                "frontend/dist/assets/app.js", "frontend/test-results/output.txt",
                "frontend/playwright-report/index.html", "frontend/playwright/.cache/output",
            )
            retained = (
                "frontend/dist/.gitkeep", "frontend/node_modules/package/index.js",
                ".venv/bin/python", ".uv-cache/entry", ".vscode/package-r.db", "local.db",
                "tests/data/vienna-s2l2a-26/vienna-s2l2a-26.parquet",
            )
            for filename in (*outputs, *retained):
                path = root / filename
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("test content\n")
            result = subprocess.run(
                ["make", "clean"], cwd=root, env={"HOME": str(root), "PATH": os.defpath},
                capture_output=True, text=True, check=False,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            for filename in outputs:
                self.assertFalse((root / filename).exists(), filename)
            for filename in retained:
                self.assertEqual((root / filename).read_text(), "test content\n", filename)

    def run_tests(self):
        with tempfile.TemporaryDirectory(prefix="test-make-") as directory:
            root = Path(directory)
            for filename in ("Makefile", "common.mk", "tools.mk"):
                shutil.copy(ROOT / filename, root / filename)
            targets = re.findall(r"^(test-[\w-]+):", (root / "Makefile").read_text(), re.MULTILINE)
            overrides = []
            for target in [*targets, "performance"]:
                overrides.append(f"{target}:\n\t@echo {target} >> executed\n")
            (root / "overrides.mk").write_text("".join(overrides))
            result = subprocess.run(
                ["make", "-j4", "-f", "Makefile", "-f", "overrides.mk", "test",
                 "MAKE=make -f Makefile -f overrides.mk"],
                cwd=root, env={"HOME": str(root), "PATH": os.defpath},
                capture_output=True, text=True, check=False,
            )
            executed = (root / "executed").read_text().splitlines()
            return result, targets, executed

    def test_all_test_targets_run_once_without_performance(self):
        result, targets, executed = self.run_tests()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertCountEqual(executed, targets)
        self.assertNotIn("performance", executed)
        self.assertEqual(executed[0], "test-check")
        self.assertLess(executed.index("test-unit"), executed.index("test-integration"))
        self.assertLess(executed.index("test-integration"), executed.index("test-frontend"))
