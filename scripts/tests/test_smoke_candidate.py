"""Check that the production smoke gate rejects bad HTTP responses."""

import json
import sys
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import smoke_candidate


class CandidateResponseTests(unittest.TestCase):
    def test_missing_assets_wrong_objects_and_other_storage_origins_fail(self):
        responses = {}

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                content_type, body = responses[self.path]
                self.send_response(200)
                self.send_header("Content-Type", content_type)
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, *_arguments):
                pass

        with ThreadingHTTPServer(("127.0.0.1", 0), Handler) as server:
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            url = f"http://127.0.0.1:{server.server_address[1]}"
            expected = b"test object\n"
            valid = {
                "/": ("text/html", b'<script src="/static/app.js"></script>'),
                "/static/app.js": ("application/javascript", b"console.log('test');"),
                "/api/public/share/test-share/smoke.txt?presign=true": (
                    "application/json", json.dumps({"presignedURL": url + "/object"}).encode(),
                ),
                "/object": ("text/plain", expected),
            }
            faults = (
                ("missing bundle", "/", ("text/html", b"<html></html>")),
                ("HTML fallback", "/static/app.js", ("text/html", b"<html></html>")),
                ("wrong object", "/object", ("text/plain", b"different object")),
                ("other storage", "/api/public/share/test-share/smoke.txt?presign=true", (
                    "application/json", b'{"presignedURL":"http://127.0.0.1:1/object"}',
                )),
            )
            try:
                for name, path, response in faults:
                    with self.subTest(name=name):
                        responses.clear()
                        responses.update(valid)
                        responses[path] = response
                        with self.assertRaises(ValueError):
                            smoke_candidate.check_application(url, url, expected)
            finally:
                server.shutdown()
                thread.join(timeout=5)


if __name__ == "__main__":
    unittest.main()
