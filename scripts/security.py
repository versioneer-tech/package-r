"""Prepare release artifacts and apply the same security policy in CI and locally."""

import argparse
import hashlib
import json
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
from pathlib import Path

import yaml

REPOSITORY = "ghcr.io/versioneer-tech/package-r"
RELEASE = re.compile(r"vnext\.\d+\.\d+\.\d+(?:-rc\.\d+)?")
DIGEST = re.compile(r"sha256:[0-9a-f]{64}")
SCOPES = ("image", "go", "frontend")
DUCKDB_MODULES = {
    "driver": "github.com/duckdb/duckdb-go/v2",
    "bindings": "github.com/duckdb/duckdb-go-bindings",
    "native": "github.com/duckdb/duckdb-go-bindings/lib/linux-amd64",
}


def sha256(path: Path) -> str:
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def write_json(path: Path, value: object) -> None:
    path.write_text(json.dumps(value, indent=2) + "\n")


def inputs(source: Path, destination: Path) -> None:
    (destination / "go").mkdir(parents=True, exist_ok=True)
    (destination / "frontend").mkdir(parents=True, exist_ok=True)
    for filename in ("go.mod", "go.sum"):
        shutil.copyfile(source / filename, destination / "go" / filename)
    shutil.copyfile(source / "frontend/package.json", destination / "frontend/package.json")
    lock = (source / "frontend/pnpm-lock.yaml").read_text()
    application_documents = []
    # pnpm 12 puts its own tools in a separate YAML document.
    for document in re.split(r"(?m)^---[ \t]*(?:\r?\n|$)", lock):
        value = yaml.safe_load(document)
        if isinstance(value, dict) and value.get("importers", {}).get(".", {}).get("dependencies"):
            application_documents.append(document)
    if len(application_documents) != 1:
        raise ValueError("Expected one pnpm application lockfile document")
    (destination / "frontend/pnpm-lock.yaml").write_text(application_documents[0].lstrip("\n"))


def input_hashes(directory: Path) -> dict[str, str]:
    return {
        str(path.relative_to(directory)): sha256(path)
        for path in sorted(directory.rglob("*")) if path.is_file()
    }


def image_digest(archive: Path) -> str:
    with tarfile.open(archive) as bundle:
        index_file = bundle.extractfile("index.json")
        if index_file is None:
            raise ValueError("The candidate is not an OCI image archive")
        manifests = json.load(index_file)["manifests"]
        if len(manifests) != 1 or not DIGEST.fullmatch(manifests[0]["digest"]):
            raise ValueError("Expected one OCI image manifest")
        digest = manifests[0]["digest"]
        manifest_file = bundle.extractfile("blobs/sha256/" + digest.removeprefix("sha256:"))
        if manifest_file is None or hashlib.sha256(manifest_file.read()).hexdigest() != digest[7:]:
            raise ValueError("OCI manifest checksum does not match")
        return digest


def unpack_oci(archive: Path, directory: Path) -> None:
    with tarfile.open(archive) as bundle:
        for member in bundle:
            if member.isdir() and member.name in ("blobs", "blobs/sha256"):
                continue
            if not member.isfile() or not re.fullmatch(r"index\.json|oci-layout|blobs/sha256/[0-9a-f]{64}", member.name):
                raise ValueError("Unexpected OCI archive member")
            stream = bundle.extractfile(member)
            if stream is None:
                raise ValueError("OCI archive member has no data")
            destination = directory / member.name
            destination.parent.mkdir(parents=True, exist_ok=True)
            with destination.open("wb") as output:
                shutil.copyfileobj(stream, output)


def unpack_binary(archive: Path, destination: Path) -> None:
    with tarfile.open(archive) as bundle:
        members = bundle.getmembers()
        if len(members) != 1 or members[0].name != "package-r" or not members[0].isfile():
            raise ValueError("Expected one package-r binary in the release archive")
        binary = bundle.extractfile(members[0])
        if binary is None:
            raise ValueError("Release archive has no binary")
        destination.write_bytes(binary.read())
        destination.chmod(0o755)


def prepare(source: Path, candidate: Path, version: str, commit: str) -> None:
    if not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("Expected a full source commit SHA")
    candidate.mkdir(parents=True, exist_ok=True)
    inputs(source, candidate / "inputs")
    with tarfile.open(candidate / "linux-amd64-package-r.tar.gz", "w:gz") as bundle:
        bundle.add(source / "package-r", arcname="package-r")
    write_json(candidate / "security-manifest.json", {
        "version": version,
        "commit": commit,
        "image": REPOSITORY,
        "image_digest": image_digest(candidate / "image.tar"),
        "image_archive_sha256": sha256(candidate / "image.tar"),
        "binary_archive_sha256": sha256(candidate / "linux-amd64-package-r.tar.gz"),
        "inputs": input_hashes(candidate / "inputs"),
    })


def candidate_manifest(candidate: Path) -> dict:
    manifest = json.loads((candidate / "security-manifest.json").read_text())
    if manifest["image_archive_sha256"] != sha256(candidate / "image.tar"):
        raise ValueError("Candidate image archive changed")
    if manifest["binary_archive_sha256"] != sha256(candidate / "linux-amd64-package-r.tar.gz"):
        raise ValueError("Candidate binary archive changed")
    if manifest["image_digest"] != image_digest(candidate / "image.tar"):
        raise ValueError("Candidate image digest changed")
    if manifest["inputs"] != input_hashes(candidate / "inputs"):
        raise ValueError("Candidate dependency inputs changed")
    return manifest


def duckdb_inventory(results: list[dict]) -> dict:
    packages = [package for result in results if result.get("Type") == "gobinary"
                for package in result.get("Packages") or []]
    versions = {}
    releases = {}
    for component, module in DUCKDB_MODULES.items():
        found = {package.get("Version") for package in packages if package.get("Name") == module}
        if len(found) != 1 or not all(found):
            raise ValueError("missing or ambiguous DuckDB inventory: " + module)
        version = found.pop()
        major = "2" if component == "driver" else "0"
        match = re.fullmatch(r"v" + major + r"\.([1-9]\d{4,})\.\d+", version)
        if match is None:
            raise ValueError("unsupported DuckDB module version: " + module + "@" + version)
        versions[component] = version
        releases[component] = int(match[1])
    if len(set(releases.values())) != 1 or versions["bindings"] != versions["native"]:
        raise ValueError("DuckDB driver, bindings, and native library versions do not match")
    # Upstream encodes major, minor, and patch in the module's minor version.
    release = releases["native"]
    return {
        "version": f"{release // 10000}.{release // 100 % 100}.{release % 100}",
        "native_module": DUCKDB_MODULES["native"],
        "native_module_version": versions["native"],
        "bindings_version": versions["bindings"],
        "driver_version": versions["driver"],
    }


def check_sbom(scope: str, sbom: dict, assessment: dict) -> None:
    if sbom.get("bomFormat") != "CycloneDX" or not sbom.get("components"):
        raise ValueError(scope + ": empty SBOM")
    if "duckdb" in assessment:
        native = assessment["duckdb"]
        if not any(component.get("name") == native["native_module"] and
                   component.get("version") == native["native_module_version"]
                   for component in sbom["components"]):
            raise ValueError(scope + ": missing DuckDB native library in SBOM")


def assess(scope: str, report: dict, frontend: Path) -> dict:
    results = report.get("Results") or []
    packages = [package for result in results for package in result.get("Packages") or []]
    expected = {"image": "gobinary", "go": "gobinary", "frontend": "pnpm"}[scope]
    if not any(result.get("Type") == expected and result.get("Packages") for result in results):
        raise ValueError(f"{scope}: missing {expected} package inventory")
    if scope in ("image", "go") and not any(
        package.get("Name") == "stdlib" and package.get("Version")
        for result in results if result.get("Type") == "gobinary"
        for package in result.get("Packages") or []
    ):
        raise ValueError(scope + ": missing Go standard library version")
    if scope == "image":
        os_info = report.get("Metadata", {}).get("OS") or {}
        if not os_info.get("Family") or not os_info.get("Name"):
            raise ValueError("image: missing OS identification")
        if not any(result.get("Class") == "os-pkgs" and result.get("Packages") for result in results):
            raise ValueError("image: missing OS package inventory")
        if os_info.get("EOSL"):
            raise ValueError("image: OS has reached end of support")
    if scope == "frontend":
        production = set(json.loads(frontend.read_text())["dependencies"])
        missing = production - {package["Name"] for package in packages}
        if missing:
            raise ValueError("frontend: production dependencies missing: " + ", ".join(sorted(missing)))
    findings = [finding for result in results for finding in result.get("Vulnerabilities") or []]
    blocking = [finding for finding in findings if finding["Severity"] == "CRITICAL" or
                (finding["Severity"] == "HIGH" and (finding.get("FixedVersion") or "").strip())]
    unfixed_high = [finding for finding in findings if finding["Severity"] == "HIGH" and
                    not (finding.get("FixedVersion") or "").strip()]
    assessment = {
        "packages": len(packages),
        "blocking": len(blocking),
        "ids": sorted({finding["VulnerabilityID"] for finding in blocking}),
        "unfixed_high": len(unfixed_high),
        "unfixed_high_ids": sorted({finding["VulnerabilityID"] for finding in unfixed_high}),
    }
    if scope in ("image", "go"):
        assessment["duckdb"] = duckdb_inventory(results)
    return assessment


def scan(image: list[str], directory: Path, reports: Path, trivy: str, go_directory: Path, binding: str = "") -> int:
    reports.mkdir(parents=True, exist_ok=True)
    (reports / "scan-summary.json").unlink(missing_ok=True)
    summary = {"passed": False, "candidate_manifest_sha256": binding, "scopes": {}, "errors": []}
    targets = {"image": ["image", *image], "go": ["rootfs", str(go_directory / "package-r")], "frontend": ["fs", str(directory / "frontend")]}
    for scope, target in targets.items():
        report_path = reports / f"{scope}.json"
        sbom_path = reports / f"{scope}.cdx.json"
        report_path.unlink(missing_ok=True)
        sbom_path.unlink(missing_ok=True)
        try:
            subprocess.run([
                trivy, *target, "--config", "", "--cache-dir", "build/trivy-cache",
                "--scanners", "vuln", "--list-all-pkgs", "--ignore-unfixed=false",
                "--ignorefile", "/dev/null", "--exit-code", "0", "--timeout", "10m",
                "--severity", "UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL",
                *(["--include-dev-deps=false"] if target[0] == "fs" else []),
                "--format", "json", "--output", str(report_path),
            ], check=True)
            report = json.loads(report_path.read_text())
            # Save the inventory before applying the blocking policy.
            subprocess.run([trivy, "convert", "--format", "cyclonedx", "--output", str(sbom_path), str(report_path)], check=True)
            sbom = json.loads(sbom_path.read_text())
            assessment = assess(scope, report, directory / "frontend/package.json")
            check_sbom(scope, sbom, assessment)
            summary["scopes"][scope] = assessment
            print(scope + ": " + json.dumps(summary["scopes"][scope]), flush=True)
        except (subprocess.CalledProcessError, ValueError, KeyError, TypeError, OSError) as error:
            summary["errors"].append(f"{scope}: {error}")
    try:
        version = subprocess.run(
            [trivy, "version", "--format", "json", "--cache-dir", "build/trivy-cache"],
            capture_output=True, text=True, check=True,
        )
        summary["scanner"] = json.loads(version.stdout)
    except (subprocess.CalledProcessError, ValueError, OSError) as error:
        summary["errors"].append("scanner metadata: " + str(error))
    summary["passed"] = not summary["errors"] and all(
        scope in summary["scopes"] and summary["scopes"][scope]["blocking"] == 0 for scope in SCOPES
    )
    summary["reports"] = {
        name: sha256(reports / name) for name in
        (f"{scope}{suffix}" for scope in SCOPES for suffix in (".json", ".cdx.json"))
        if (reports / name).is_file()
    }
    write_json(reports / "scan-summary.json", summary)
    for error in summary["errors"]:
        print(error, file=sys.stderr)
    return 0 if summary["passed"] else 1


def verify(candidate: Path, reports: Path) -> None:
    candidate_manifest(candidate)
    summary = json.loads((reports / "scan-summary.json").read_text())
    if summary.get("passed") is not True or summary.get("candidate_manifest_sha256") != sha256(candidate / "security-manifest.json"):
        raise ValueError("The candidate has no matching successful scan")
    expected = {f"{scope}{suffix}" for scope in SCOPES for suffix in (".json", ".cdx.json")}
    if set(summary["reports"]) != expected:
        raise ValueError("The scan does not contain all reports and SBOMs")
    for name, digest in summary["reports"].items():
        if sha256(reports / name) != digest:
            raise ValueError("Security report changed: " + name)
    for scope in SCOPES:
        sbom = json.loads((reports / f"{scope}.cdx.json").read_text())
        report = json.loads((reports / f"{scope}.json").read_text())
        result = assess(scope, report, candidate / "inputs/frontend/package.json")
        check_sbom(scope, sbom, result)
        if result["blocking"]:
            raise ValueError("The security reports contain blocking findings")


def release_ref(tag: str, manifest_path: Path) -> None:
    if not RELEASE.fullmatch(tag):
        raise ValueError("Invalid release tag")
    if manifest_path.is_file():
        manifest = json.loads(manifest_path.read_text())
        if manifest["version"] != tag or manifest["image"] != REPOSITORY:
            raise ValueError("Release manifest does not match the requested release")
        if not DIGEST.fullmatch(manifest["image_digest"]) or not re.fullmatch(r"[0-9a-f]{40}", manifest["commit"]):
            raise ValueError("Invalid release manifest identity")
        print("commit=" + manifest["commit"])
        print("image=" + REPOSITORY + "@" + manifest["image_digest"])
    else:
        print("commit=" + tag)
        print("image=" + REPOSITORY + ":" + tag)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    prepare_parser = commands.add_parser("prepare")
    prepare_parser.add_argument("--source", type=Path, required=True)
    prepare_parser.add_argument("--candidate", type=Path, required=True)
    prepare_parser.add_argument("--version", required=True)
    prepare_parser.add_argument("--commit", required=True)
    for name in ("scan", "verify"):
        command = commands.add_parser(name)
        command.add_argument("--candidate", type=Path, required=True)
        command.add_argument("--reports", type=Path, required=True)
        if name == "scan":
            command.add_argument("--trivy", required=True)
    ref_parser = commands.add_parser("release-ref")
    ref_parser.add_argument("--tag", required=True)
    ref_parser.add_argument("--manifest", type=Path, required=True)
    commands.add_parser("releases")
    rescan = commands.add_parser("rescan")
    rescan.add_argument("--source", type=Path, required=True)
    rescan.add_argument("--binary", type=Path, required=True)
    rescan.add_argument("--image", required=True)
    rescan.add_argument("--manifest", type=Path, required=True)
    rescan.add_argument("--reports", type=Path, required=True)
    rescan.add_argument("--trivy", required=True)
    args = parser.parse_args()
    if args.command == "prepare":
        prepare(args.source, args.candidate, args.version, args.commit)
    elif args.command == "scan":
        candidate_manifest(args.candidate)
        with tempfile.TemporaryDirectory(prefix="package-r-binary-scan-") as directory:
            go_directory = Path(directory) / "go"
            image_directory = Path(directory) / "image"
            unpack_oci(args.candidate / "image.tar", image_directory)
            shutil.copytree(args.candidate / "inputs/go", go_directory, dirs_exist_ok=True)
            unpack_binary(args.candidate / "linux-amd64-package-r.tar.gz", go_directory / "package-r")
            return scan(["--input", str(image_directory)], args.candidate / "inputs", args.reports, args.trivy, go_directory, sha256(args.candidate / "security-manifest.json"))
    elif args.command == "verify":
        verify(args.candidate, args.reports)
    elif args.command == "release-ref":
        release_ref(args.tag, args.manifest)
    elif args.command == "releases":
        releases = [release for page in json.load(sys.stdin) for release in page]
        include = [
            {"tag": release["tag_name"], "manifest": any(asset["name"] == "security-manifest.json" for asset in release["assets"])}
            for release in releases if not release["draft"] and RELEASE.fullmatch(release["tag_name"])
        ]
        print(json.dumps({"include": include}))
    elif args.command == "rescan":
        with tempfile.TemporaryDirectory(prefix="package-r-rescan-") as directory:
            staged = Path(directory)
            inputs(args.source, staged)
            unpack_binary(args.binary, staged / "go/package-r")
            if args.manifest.is_file():
                manifest = json.loads(args.manifest.read_text())
                actual_inputs = input_hashes(staged)
                actual_inputs.pop("go/package-r")
                if manifest["inputs"] != actual_inputs or args.image != REPOSITORY + "@" + manifest["image_digest"] or manifest["binary_archive_sha256"] != sha256(args.binary):
                    raise ValueError("Published release artifact or dependency inputs changed")
            args.reports.mkdir(parents=True, exist_ok=True)
            commit = subprocess.check_output(["git", "-C", str(args.source), "rev-parse", "HEAD"], text=True).strip()
            write_json(args.reports / "release.json", {"commit": commit, "image": args.image, "inputs": input_hashes(staged)})
            return scan(["--image-src", "remote", "--platform", "linux/amd64", args.image], staged, args.reports, args.trivy, go_directory=staged / "go")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (ValueError, KeyError, OSError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
