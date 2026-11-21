import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPTS = Path(__file__).resolve().parents[1]


class SecretCheckTests(unittest.TestCase):
    def test_only_selected_regular_files_reach_the_scanner(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "selected file.txt").write_text("test content")
            (root / "ignored.txt").write_text("excluded content")
            (root / "linked.txt").symlink_to(root / "ignored.txt")
            (root / "linked-directory").symlink_to(root, target_is_directory=True)
            executable = root / "gitleaks"
            executable.write_text(
                f"#!{sys.executable}\n"
                "import json, sys\n"
                "from pathlib import Path\n"
                "root = Path(sys.argv[-1])\n"
                "print(json.dumps({str(p.relative_to(root)): p.read_text() "
                "for p in root.rglob('*') if p.is_file()}))\n"
            )
            executable.chmod(0o755)
            result = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPTS / "check_secrets.py"),
                    "selected file.txt",
                    "linked.txt",
                    "linked-directory/ignored.txt",
                ],
                cwd=root,
                # CI's Python needs its library path in child processes.
                env={**os.environ, "PATH": str(root)},
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(json.loads(result.stdout), {"selected file.txt": "test content"})

    def test_paths_outside_the_repository_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            for path in ("../outside.txt", str(Path(directory) / "outside.txt")):
                with self.subTest(path=path):
                    result = subprocess.run(
                        [sys.executable, str(SCRIPTS / "check_secrets.py"), path],
                        cwd=directory,
                        capture_output=True,
                        text=True,
                        check=False,
                    )
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn("Expected a path inside the repository", result.stderr)


if __name__ == "__main__":
    unittest.main()
