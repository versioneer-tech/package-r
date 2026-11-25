"""Check the saved production binary and image against local S3 storage."""

import argparse
import contextlib
import gzip
import json
import re
import shutil
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
from pathlib import Path

from security import candidate_manifest, unpack_binary


def run(command, **options):
    return subprocess.run(command, check=True, timeout=120, **options)


def free_port():
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        return listener.getsockname()[1]


@contextlib.contextmanager
def service(command, log, **options):
    with log.open("w+") as output:
        process = subprocess.Popen(command, stdout=output, stderr=output, **options)
        try:
            yield process
        except Exception:
            output.seek(0)
            print(output.read())
            raise
        finally:
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=10)


HTTP = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def fetch(url):
    with HTTP.open(url, timeout=5) as response:
        body = response.read()
        if response.headers.get("Content-Encoding") == "gzip":
            body = gzip.decompress(body)
        return response.headers.get_content_type(), body


def wait_for_service(url, process, expected=200):
    deadline = time.monotonic() + 30
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise RuntimeError("Service exited before it was ready")
        try:
            fetch(url)
            return
        except urllib.error.HTTPError as error:
            if error.code == expected:
                return
        except (OSError, TimeoutError):
            pass
        time.sleep(0.25)
    raise RuntimeError("Service did not become ready within 30 seconds")


def check_application(url, storage_url, expected):
    content_type, page = fetch(url + "/")
    if content_type != "text/html":
        raise ValueError("Production frontend did not return HTML")
    scripts = re.findall(r'<script\b[^>]*\bsrc="(/static/[^"?]+\.js)"', page.decode())
    if not scripts:
        raise ValueError("Production frontend has no embedded JavaScript bundle")
    for script in scripts:
        content_type, bundle = fetch(url + script)
        if "javascript" not in content_type or not bundle:
            raise ValueError("Embedded JavaScript bundle is missing or has the wrong content type")
    _, response = fetch(url + "/api/public/share/test-share/smoke.txt?presign=true")
    signed_url = json.loads(response)["presignedURL"]
    # Presigned reads must stay on this test's local S3 service.
    if urllib.parse.urlsplit(signed_url)[:2] != urllib.parse.urlsplit(storage_url)[:2]:
        raise ValueError("Presigned read did not target local S3 storage")
    if fetch(signed_url)[1] != expected:
        raise ValueError("Published share returned different object contents")


def smoke(candidate: Path):
    candidate = candidate.resolve()
    manifest = candidate_manifest(candidate)
    with tempfile.TemporaryDirectory(prefix="test-package-r-candidate-") as directory:
        root = Path(directory)
        binary = root / "package-r"
        unpack_binary(candidate / "linux-amd64-package-r.tar.gz", binary)
        storage = root / "storage"
        (storage / "test-data").mkdir(parents=True)
        expected = b"packageR release smoke test\n"
        (storage / "test-data/smoke.txt").write_bytes(expected)
        storage_url = f"http://127.0.0.1:{free_port()}"
        environment = {
            "AWS_ACCESS_KEY_ID": "test-access-key", "AWS_SECRET_ACCESS_KEY": "test-secret-key",
            "AWS_REGION": "us-east-1", "AWS_ENDPOINT_URL": storage_url,
            "RCLONE_CONFIG": "/dev/null", "PACKAGE_R_ROOT": "test-data",
        }
        docker_config = root / "docker"
        docker_config.mkdir()
        authfile = docker_config / "config.json"
        authfile.write_text('{"auths": {}}\n')
        docker = ["docker", "--config", str(docker_config), "--host", "unix:///var/run/docker.sock"]
        image = "test-package-r-smoke:" + uuid.uuid4().hex
        container = "test-package-r-smoke-" + uuid.uuid4().hex
        volume = "test-package-r-smoke-" + uuid.uuid4().hex
        run([*docker, "version", "--format", "{{.Server.Version}}"], stdout=subprocess.DEVNULL)
        try:
            run(["skopeo", "--insecure-policy", "copy", "--authfile", str(authfile),
                 "oci-archive:" + str(candidate / "image.tar"), "docker-daemon:" + image])
            with service([shutil.which("rclone") or "rclone", "serve", "s3", str(storage), "--addr", storage_url.removeprefix("http://"),
                          "--auth-key", "test-access-key,test-secret-key", "--config", "/dev/null"],
                         root / "s3.log", env={}) as s3:
                wait_for_service(storage_url, s3, expected=403)
                for mode in ("binary", "image"):
                    state = root / mode
                    state.mkdir()
                    port = free_port()
                    server_environment = {**environment, "PACKAGE_R_ADDRESS": "127.0.0.1", "PACKAGE_R_PORT": str(port)}
                    if mode == "binary":
                        command = [str(binary), "--config", str(state / "config.yaml"),
                                   "--database", str(state / "package-r.db")]
                        options = {"env": {**server_environment, "XDG_CACHE_HOME": str(state / "cache")}, "cwd": state}
                    else:
                        command = [*docker, "run", "--rm", "--name", container, "--network", "host",
                                   "--mount", f"type=volume,source={volume},target=/home/package-r"]
                        for key, value in {**server_environment, "XDG_CACHE_HOME": "/home/package-r/cache"}.items():
                            command.extend(["--env", key + "=" + value])
                        command.extend([image, "--config", "/home/package-r/config.yaml",
                                        "--database", "/home/package-r/package-r.db"])
                        options = {}
                    with (state / "setup.log").open("w+") as output:
                        try:
                            version = run([*command, "version"], capture_output=True, text=True, **options).stdout
                            if not version.strip().startswith("packageR " + manifest["version"] + "/"):
                                raise ValueError("Candidate version does not match its manifest")
                            for arguments in (
                                ["config", "init", "--root=test-data", "--address=127.0.0.1", f"--port={port}"],
                                ["users", "add", "test-user", "test-password"],
                                ["shares", "add", "test-user", "test-share", "/"],
                            ):
                                run([*command, *arguments], stdout=output, stderr=output, **options)
                        except Exception:
                            output.seek(0)
                            print(output.read())
                            raise
                    with service(command, state / "server.log", **options) as server:
                        url = f"http://127.0.0.1:{port}"
                        wait_for_service(url + "/health", server)
                        probe = [*command, "healthcheck"] if mode == "binary" else [
                            *docker, "exec", container, "/package-r", "healthcheck",
                        ]
                        run([*probe, "--address=127.0.0.1", f"--port={port}"], **options)
                        check_application(url, storage_url, expected)
                    print(mode + ": startup, health, embedded frontend, and S3 read passed", flush=True)
        finally:
            subprocess.run([*docker, "rm", "--force", container], stdout=subprocess.DEVNULL,
                           stderr=subprocess.DEVNULL, timeout=30, check=False)
            subprocess.run([*docker, "volume", "rm", volume], stdout=subprocess.DEVNULL,
                           stderr=subprocess.DEVNULL, timeout=30, check=False)
            subprocess.run([*docker, "image", "rm", image], stdout=subprocess.DEVNULL,
                           stderr=subprocess.DEVNULL, timeout=30, check=False)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--candidate", type=Path, default=Path("dist/candidate"))
    smoke(parser.parse_args().candidate)
