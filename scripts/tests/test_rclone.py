"""Check the rclone requirement shared by the local harnesses."""

import shlex
import subprocess
import tempfile
import unittest
from pathlib import Path

CHECK = Path(__file__).resolve().parents[1] / "check-rclone.sh"


class RcloneVersionTests(unittest.TestCase):
    def check_version(self, version, exit_code=0):
        with tempfile.TemporaryDirectory(prefix="test-rclone-version-") as directory:
            binary = Path(directory) / "rclone"
            binary.write_text(
                "#!/bin/sh\nprintf '%s\\n' " + shlex.quote(version) + f"\nexit {exit_code}\n"
            )
            binary.chmod(0o755)
            return subprocess.run([str(CHECK), str(binary)], capture_output=True, text=True)

    def test_supported_versions(self):
        for version in ("1.76.0", "1.76.1", "1.77.0", "2.0.0"):
            with self.subTest(version=version):
                result = self.check_version("rclone v" + version + "\n- os/type: linux")
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout, "rclone v" + version + "\n")

    def test_old_or_unrecognized_versions_fail(self):
        for version in ("rclone v1.74.0", "rclone v1.75.0", "rclone v0.99.0", "", "unknown"):
            with self.subTest(version=version):
                result = self.check_version(version)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("rclone 1.76 or newer is required", result.stderr)

    def test_failed_version_command_fails(self):
        result = self.check_version("rclone v1.76.0", exit_code=1)
        self.assertNotEqual(result.returncode, 0)
