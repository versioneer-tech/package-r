"""Check that documentation selection cannot skip runtime changes."""

import runpy
import subprocess
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

CHECKS = runpy.run_path(str(Path(__file__).resolve().parents[1] / "ci-checks.py"))


class CheckSelectionTests(unittest.TestCase):
    def test_guides_and_images_use_documentation_checks(self):
        self.assertTrue(CHECKS["documentation_only"]([
            "README.md", "docs/how-to-guides/configuration.md", "docs/images/example.png",
        ]))

    def test_dependencies_and_code_in_docs_require_full_checks(self):
        self.assertFalse(CHECKS["documentation_only"](["README.md", "docs/requirements.txt"]))
        self.assertFalse(CHECKS["documentation_only"](["docs/helper.py"]))

    def test_unknown_files_and_browser_fixtures_require_full_checks(self):
        self.assertFalse(CHECKS["documentation_only"](["unknown.txt"]))
        self.assertFalse(CHECKS["documentation_only"]([
            "docs/imgs/screenshots/authenticated-public-listing.png",
        ]))

    def test_local_changes_include_untracked_files(self):
        def git(*arguments):
            if arguments[0] == "rev-parse":
                return b"1234\n"
            return b"README.md\0" if arguments[0] == "diff" else b"scripts/new.py\0"

        with patch.dict(CHECKS["select_checks"].__globals__, {"git": git}):
            self.assertFalse(CHECKS["select_checks"]("HEAD"))
            self.assertTrue(CHECKS["select_checks"]("base", "head"))

    def test_staged_runtime_change_cannot_be_hidden_by_worktree_edits(self):
        def git(*arguments):
            if arguments[0] == "rev-parse":
                return b"1234\n"
            if "--cached" in arguments:
                return b"http/catalog.go\0"
            return b"README.md\0"

        with patch.dict(CHECKS["select_checks"].__globals__, {"git": git}):
            self.assertFalse(CHECKS["select_checks"]("HEAD"))

    def test_empty_or_unavailable_comparison_requires_full_checks(self):
        self.assertFalse(CHECKS["select_checks"](None))
        with patch.dict(CHECKS["select_checks"].__globals__, {"git": lambda *args: b""}):
            self.assertFalse(CHECKS["select_checks"]("HEAD"))
        with patch.dict(CHECKS["select_checks"].__globals__, {
            "git": Mock(side_effect=subprocess.CalledProcessError(128, "git")),
        }):
            self.assertFalse(CHECKS["select_checks"]("missing"))
