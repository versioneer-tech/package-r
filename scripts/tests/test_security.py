import io
import json
import sys
import tarfile
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import security


class ReleaseSelectionTests(unittest.TestCase):
    @staticmethod
    def release(tag, **values):
        return {"tag_name": tag, "draft": False, "prerelease": "-rc." in tag,
                "assets": [{"name": "security-manifest.json"}], **values}

    def test_latest_stable_and_newest_active_candidate_are_selected(self):
        releases = [self.release(tag) for tag in (
            "vnext.1.9.0", "vnext.1.10.0-rc.1", "vnext.1.10.0",
            "vnext.1.11.0-rc.2", "vnext.1.11.0-rc.10", "vnext.1.11.0-rc.3",
        )]
        releases.append(self.release("vnext.2.0.0", draft=True))
        releases.append(self.release("vnext.3.0.0", prerelease=True))
        self.assertEqual(security.select_releases(releases), {"include": [
            {"tag": "vnext.1.10.0", "manifest": True},
            {"tag": "vnext.1.11.0-rc.10", "manifest": True},
        ]})

    def test_stable_publication_retires_its_candidates(self):
        releases = [self.release(tag) for tag in (
            "vnext.1.1.0-rc.1", "vnext.1.1.0", "vnext.1.2.0-rc.1", "vnext.1.2.0",
        )]
        self.assertEqual(security.select_releases(releases), {"include": [
            {"tag": "vnext.1.2.0", "manifest": True},
        ]})

    def test_first_release_candidate_and_empty_repository(self):
        self.assertEqual(security.select_releases([]), {"include": []})
        self.assertEqual(security.select_releases([self.release("vnext.1.0.0-rc.1")]), {"include": [
            {"tag": "vnext.1.0.0-rc.1", "manifest": True},
        ]})

    def test_manual_selection_can_scan_an_old_release_and_rejects_unknown_tags(self):
        releases = [self.release("vnext.1.0.0", assets=[]), self.release("vnext.1.2.0")]
        self.assertEqual(security.select_releases(releases, "vnext.1.0.0, vnext.1.0.0"), {"include": [
            {"tag": "vnext.1.0.0", "manifest": False},
        ]})
        with self.assertRaisesRegex(ValueError, "Unknown published release tags"):
            security.select_releases(releases, "vnext.9.0.0")


class SecurityGateTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="test-security-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.source = self.root / "source"
        self.candidate = self.root / "candidate"
        self.reports = self.root / "reports"
        (self.source / "frontend").mkdir(parents=True)
        self.candidate.mkdir()
        self.reports.mkdir()
        (self.source / "go.mod").write_text("module example.com/test-app\n\ngo 1.27.0\n")
        (self.source / "go.sum").write_text("")
        (self.source / "package-r").write_text("test binary")
        security.write_json(self.source / "frontend/package.json", {"dependencies": {"test-runtime": "1.0.0"}})
        (self.source / "frontend/pnpm-lock.yaml").write_text(
            "---\nlockfileVersion: '9.0'\nimporters:\n  .:\n    packageManagerDependencies:\n      pnpm: {version: 12.6.0}\n"
            "---\nlockfileVersion: '9.0'\nimporters:\n  .:\n    dependencies:\n      test-runtime: {version: 1.0.0}\n"
        )
        manifest = b'{"schemaVersion":2,"layers":[]}'
        digest = security.hashlib.sha256(manifest).hexdigest()
        index = json.dumps({"manifests": [{"digest": "sha256:" + digest}]}).encode()
        with tarfile.open(self.candidate / "image.tar", "w") as archive:
            for name, content in (("index.json", index), ("blobs/sha256/" + digest, manifest)):
                member = tarfile.TarInfo(name)
                member.size = len(content)
                archive.addfile(member, io.BytesIO(content))
        security.prepare(self.source, self.candidate, "vnext.1.2.3", "a" * 40)
        self.image = {
            "Metadata": {"OS": {"Family": "debian", "Name": "12"}},
            "Results": [
                {"Type": "gobinary", "Packages": [{"Name": "test-go", "Version": "1.0.0"}, {"Name": "stdlib", "Version": "v1.27.1"}]},
                {"Class": "os-pkgs", "Type": "debian", "Packages": [{"Name": "test-os"}]},
            ],
        }
        self.go = {"Results": [
            {"Type": "gomod", "Packages": [{"Name": "test-go"}]},
            {"Type": "gobinary", "Packages": [{"Name": "test-go"}, {"Name": "stdlib", "Version": "v1.27.1"}]},
        ]}
        self.duckdb_packages = [
            {"Name": module, "Version": "v2.20000.0" if component == "driver" else "v0.20000.0"}
            for component, module in security.DUCKDB_MODULES.items()
        ]
        for report in (self.image, self.go):
            next(result for result in report["Results"] if result["Type"] == "gobinary")["Packages"].extend(self.duckdb_packages)
        self.frontend = {"Results": [{"Type": "pnpm", "Packages": [{"Name": "test-runtime", "Version": "1.0.0"}]}]}
        for name, report in (("image", self.image), ("go", self.go), ("frontend", self.frontend)):
            security.write_json(self.reports / f"{name}.json", report)
            security.write_json(self.reports / f"{name}.cdx.json", self.sbom())
        self.summary = {
            "passed": True,
            "candidate_manifest_sha256": security.sha256(self.candidate / "security-manifest.json"),
            "reports": {path.name: security.sha256(path) for path in self.reports.iterdir()},
        }
        security.write_json(self.reports / "scan-summary.json", self.summary)

    def sbom(self):
        return {"bomFormat": "CycloneDX", "components": [
            {"name": package["Name"], "version": package["Version"]} for package in self.duckdb_packages
        ]}

    def test_pnpm_application_document_is_selected_without_changing_source(self):
        original = (self.source / "frontend/pnpm-lock.yaml").read_bytes()
        staged = (self.candidate / "inputs/frontend/pnpm-lock.yaml").read_text()
        self.assertIn("test-runtime", staged)
        self.assertNotIn("packageManagerDependencies", staged)
        self.assertEqual((self.source / "frontend/pnpm-lock.yaml").read_bytes(), original)

    def test_severity_and_fix_policy(self):
        for severity, fix, blocking, unfixed_high in (
            ("CRITICAL", None, 1, 0), ("CRITICAL", "1.1.0", 1, 0),
            ("HIGH", "1.1.0", 1, 0), ("HIGH", None, 0, 1),
            ("HIGH", "", 0, 1), ("HIGH", " ", 0, 1),
            ("MEDIUM", "1.1.0", 0, 0), ("LOW", None, 0, 0),
        ):
            with self.subTest(severity=severity, fix=fix):
                self.go["Results"][0]["Vulnerabilities"] = [
                    {"Severity": severity, "VulnerabilityID": "TEST-1", "FixedVersion": fix}
                ]
                assessment = security.assess("go", self.go, self.source / "frontend/package.json")
                self.assertEqual(assessment["blocking"], blocking)
                self.assertEqual(assessment["ids"], ["TEST-1"] if blocking else [])
                self.assertEqual(assessment["unfixed_high"], unfixed_high)
                self.assertEqual(assessment["unfixed_high_ids"], ["TEST-1"] if unfixed_high else [])

    def test_unfixed_high_findings_allow_publication(self):
        self.go["Results"][0]["Vulnerabilities"] = [{"Severity": "HIGH", "VulnerabilityID": "TEST-1"}]
        security.write_json(self.reports / "go.json", self.go)
        self.summary["reports"]["go.json"] = security.sha256(self.reports / "go.json")
        security.write_json(self.reports / "scan-summary.json", self.summary)
        security.verify(self.candidate, self.reports)

    def test_fixable_high_findings_prevent_publication(self):
        self.go["Results"][0]["Vulnerabilities"] = [
            {"Severity": "HIGH", "VulnerabilityID": "TEST-1", "FixedVersion": "1.1.0"}
        ]
        security.write_json(self.reports / "go.json", self.go)
        self.summary["reports"]["go.json"] = security.sha256(self.reports / "go.json")
        security.write_json(self.reports / "scan-summary.json", self.summary)
        with self.assertRaisesRegex(ValueError, "blocking findings"):
            security.verify(self.candidate, self.reports)

    def test_other_reports_are_retained_when_one_scanner_fails(self):
        def trivy(arguments, check, **options):
            if arguments[1] == "version":
                return security.subprocess.CompletedProcess(arguments, 0, stdout='{"Version":"test"}')
            if arguments[1] == "image":
                raise security.subprocess.CalledProcessError(2, arguments)
            output = Path(arguments[arguments.index("--output") + 1])
            if arguments[1] == "convert":
                security.write_json(output, self.sbom())
            else:
                security.write_json(output, self.go if arguments[1] == "rootfs" else self.frontend)

        reports = self.root / "failed-scan"
        with patch.object(security.subprocess, "run", side_effect=trivy):
            result = security.scan(["test-image"], self.candidate / "inputs", reports, "test-trivy", self.candidate / "inputs/go")
        self.assertEqual(result, 1)
        summary = json.loads((reports / "scan-summary.json").read_text())
        self.assertFalse(summary["passed"])
        for scope in ("go", "frontend"):
            self.assertTrue((reports / f"{scope}.json").is_file())
            self.assertTrue((reports / f"{scope}.cdx.json").is_file())

    def test_oci_archive_cannot_write_outside_its_directory(self):
        archive_path = self.root / "test-unsafe.tar"
        with tarfile.open(archive_path, "w") as archive:
            member = tarfile.TarInfo("../test-outside")
            member.size = 1
            archive.addfile(member, io.BytesIO(b"x"))
        with self.assertRaisesRegex(ValueError, "Unexpected OCI archive member"):
            security.unpack_oci(archive_path, self.root / "unpacked")
        self.assertFalse((self.root / "test-outside").exists())

    def test_empty_inventory_and_unsupported_os_are_rejected(self):
        with self.assertRaisesRegex(ValueError, "missing pnpm"):
            security.assess("frontend", {"Results": []}, self.source / "frontend/package.json")
        self.image["Metadata"]["OS"]["EOSL"] = True
        with self.assertRaisesRegex(ValueError, "end of support"):
            security.assess("image", self.image, self.source / "frontend/package.json")

    def test_missing_production_dependency_is_rejected(self):
        self.frontend["Results"][0]["Packages"] = [{"Name": "test-dev-tool"}]
        with self.assertRaisesRegex(ValueError, "test-runtime"):
            security.assess("frontend", self.frontend, self.source / "frontend/package.json")

    def test_missing_go_standard_library_is_rejected(self):
        self.go["Results"][1]["Packages"] = [{"Name": "test-go"}]
        with self.assertRaisesRegex(ValueError, "standard library version"):
            security.assess("go", self.go, self.source / "frontend/package.json")

    def test_native_duckdb_release_is_recorded(self):
        for encoded, release, native_module in (
            ("20000", "2.0.0", "go.duckdb.org/libduckdb/linux-amd64"),
            ("10505", "1.5.5", "github.com/duckdb/duckdb-go-bindings/lib/linux-amd64"),
        ):
            for package, component in zip(self.duckdb_packages, security.DUCKDB_MODULES):
                package["Version"] = ("v2." if component == "driver" else "v0.") + encoded + ".0"
            self.duckdb_packages[-1]["Name"] = native_module
            for scope, report in (("image", self.image), ("go", self.go)):
                with self.subTest(scope=scope, release=release):
                    inventory = security.assess(scope, report, self.source / "frontend/package.json")["duckdb"]
                    self.assertEqual(inventory["version"], release)
                    self.assertEqual(inventory["native_module"], native_module)
                    self.assertEqual(inventory["native_module_version"], "v0." + encoded + ".0")

    def test_duckdb_native_module_must_match_release(self):
        self.duckdb_packages[-1]["Name"] = "github.com/duckdb/duckdb-go-bindings/lib/linux-amd64"
        with self.assertRaisesRegex(ValueError, "DuckDB inventory"):
            security.assess("go", self.go, self.source / "frontend/package.json")

    def test_missing_or_unversioned_duckdb_native_library_is_rejected(self):
        for scope, report in (("image", self.image), ("go", self.go)):
            for version in (None, ""):
                with self.subTest(scope=scope, version=version):
                    packages = next(result for result in report["Results"] if result["Type"] == "gobinary")["Packages"]
                    packages[:] = [package for package in packages if package["Name"] != security.DUCKDB_MODULES["native"]]
                    if version is not None:
                        packages.append({"Name": security.DUCKDB_MODULES["native"], "Version": version})
                    with self.assertRaisesRegex(ValueError, "DuckDB inventory"):
                        security.assess(scope, report, self.source / "frontend/package.json")

    def test_inconsistent_or_unknown_duckdb_versions_are_rejected(self):
        for version in ("v0.20001.0", "v0.20000.1", "v0.3.5"):
            with self.subTest(version=version):
                native = self.duckdb_packages[-1]
                native["Version"] = version
                with self.assertRaisesRegex(ValueError, "DuckDB"):
                    security.assess("go", self.go, self.source / "frontend/package.json")

    def test_missing_native_library_in_sbom_prevents_publication(self):
        sbom = self.sbom()
        sbom["components"].pop()
        security.write_json(self.reports / "go.cdx.json", sbom)
        self.summary["reports"]["go.cdx.json"] = security.sha256(self.reports / "go.cdx.json")
        security.write_json(self.reports / "scan-summary.json", self.summary)
        with self.assertRaisesRegex(ValueError, "missing DuckDB native library in SBOM"):
            security.verify(self.candidate, self.reports)

    def test_matching_checked_artifacts_pass(self):
        security.verify(self.candidate, self.reports)

    def test_binary_substitution_is_rejected(self):
        (self.candidate / "linux-amd64-package-r.tar.gz").write_bytes(b"changed binary")
        with self.assertRaisesRegex(ValueError, "binary archive changed"):
            security.verify(self.candidate, self.reports)

    def test_image_substitution_is_rejected(self):
        with (self.candidate / "image.tar").open("ab") as stream:
            stream.write(b"changed image")
        with self.assertRaisesRegex(ValueError, "image archive changed"):
            security.verify(self.candidate, self.reports)

    def test_dependency_substitution_is_rejected(self):
        (self.candidate / "inputs/frontend/package.json").write_text("{}")
        with self.assertRaisesRegex(ValueError, "dependency inputs changed"):
            security.verify(self.candidate, self.reports)

    def test_release_manifest_cannot_select_another_repository(self):
        manifest_path = self.candidate / "security-manifest.json"
        manifest = json.loads(manifest_path.read_text())
        manifest["image"] = "registry.example.com/test-other"
        security.write_json(manifest_path, manifest)
        with self.assertRaisesRegex(ValueError, "does not match"):
            security.release_ref("vnext.1.2.3", manifest_path)

    def test_failed_scan_cannot_authorize_publication(self):
        self.summary["passed"] = False
        security.write_json(self.reports / "scan-summary.json", self.summary)
        with self.assertRaisesRegex(ValueError, "no matching successful scan"):
            security.verify(self.candidate, self.reports)

    def test_missing_sbom_and_changed_reports_are_rejected(self):
        (self.reports / "image.cdx.json").unlink()
        with self.assertRaises(OSError):
            security.verify(self.candidate, self.reports)
        security.write_json(self.reports / "image.cdx.json", {"components": []})
        with self.assertRaisesRegex(ValueError, "report changed"):
            security.verify(self.candidate, self.reports)


if __name__ == "__main__":
    unittest.main()
