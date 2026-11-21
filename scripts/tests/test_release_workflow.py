"""Check release approval against synthetic workflow and job results."""

import json
import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[2]
COMMIT = "a" * 40


@unittest.skipUnless(shutil.which("jq"), "jq is required for the release gate")
class ReleaseWorkflowTests(unittest.TestCase):
    def test_release_requires_full_tests_on_the_same_main_commit(self):
        workflow = yaml.safe_load((ROOT / ".github/workflows/tag.yaml").read_text())
        script = next(
            step["run"] for step in workflow["jobs"]["verify_release_commit"]["steps"]
            if "run" in step
        )
        cases = (
            ("full push", {}, "success", True),
            ("full manual run", {"event": "workflow_dispatch"}, "success", True),
            ("failed tests", {}, "failure", False),
            ("docs only", {}, "skipped", False),
            ("missing tests", {}, None, False),
            ("other commit", {"head_sha": "b" * 40}, "success", False),
            ("other branch", {"head_branch": "feature"}, "success", False),
            ("PR run", {"event": "pull_request"}, "success", False),
        )
        with tempfile.TemporaryDirectory(prefix="test-release-gate-") as directory:
            root = Path(directory)
            for name, body in {
                "git": f"printf '%s\\n' {COMMIT}",
                "curl": 'for url do :; done\ncase "$url" in\n'
                        '  */jobs?*) cat "$HOME/jobs.json" ;;\n'
                        '  *) cat "$HOME/runs.json" ;;\nesac',
            }.items():
                executable = root / name
                executable.write_text("#!/bin/sh\n" + body + "\n")
                executable.chmod(0o755)
            environment = {
                "HOME": str(root), "PATH": str(root) + os.pathsep + os.defpath,
                "GH_TOKEN": "test-token", "GITHUB_API_URL": "https://api.invalid",
                "GITHUB_REPOSITORY": "test/project",
            }
            for name, changes, test_result, allowed in cases:
                with self.subTest(case=name):
                    run = {
                        "id": 1, "head_sha": COMMIT, "head_branch": "main",
                        "conclusion": "success", "event": "push", **changes,
                    }
                    (root / "runs.json").write_text(json.dumps({"workflow_runs": [run]}))
                    jobs = [
                        {"name": job, "conclusion": "success"}
                        for job in ("lint_backend", "lint_frontend", "build", "required_checks")
                    ]
                    if test_result is not None:
                        jobs.append({"name": "tests", "conclusion": test_result})
                    (root / "jobs.json").write_text(json.dumps({"jobs": jobs}))
                    result = subprocess.run(
                        ["bash", "-c", script], env=environment,
                        capture_output=True, text=True, check=False,
                    )
                    self.assertEqual(result.returncode == 0, allowed, result.stdout + result.stderr)
