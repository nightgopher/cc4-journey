import hashlib
import http.client
import hmac
import json
import pathlib
import tempfile
import threading
import time
import unittest

from deploy_http import DeploymentServer, authenticate, signature

SECRET = "fixture-only-signing-key-32-characters"


def headers(body, nonce="a" * 32, stamp=None):
    revision, stamp = "b" * 40, str(stamp or int(time.time()))
    digest = hashlib.sha256(body).hexdigest()
    return {"X-CC4-Revision": revision, "X-CC4-Timestamp": stamp, "X-CC4-Nonce": nonce, "X-CC4-SHA256": digest, "X-CC4-Signature": signature(SECRET, revision, stamp, nonce, digest)}


class DeployHTTPTest(unittest.TestCase):
    def test_authentication_binds_every_field_and_expiry(self):
        fields = headers(b"release")
        authenticate(fields, SECRET, int(time.time()))
        for name in fields:
            altered = dict(fields)
            altered[name] = "0" * len(altered[name])
            with self.assertRaises(ValueError):
                authenticate(altered, SECRET, int(time.time()))
        with self.assertRaises(ValueError):
            authenticate(headers(b"release", stamp=int(time.time()) - 301), SECRET, int(time.time()))

    def test_http_rejects_unsigned_tampered_and_replayed_requests(self):
        calls = []
        with tempfile.TemporaryDirectory() as root:
            server = DeploymentServer(("127.0.0.1", 0), SECRET, root, lambda file, rev: calls.append((file.read(), rev)))
            thread = threading.Thread(target=server.serve_forever)
            thread.start()
            def post(body, fields):
                connection = http.client.HTTPConnection(*server.server_address, timeout=5)
                connection.request("POST", "/deploy", body, fields)
                response = connection.getresponse()
                result = response.status
                body = json.loads(response.read())
                connection.close()
                return result, body
            try:
                self.assertEqual(post(b"release", {})[0], 401)
                fields = headers(b"release")
                self.assertEqual(post(b"tampered", fields)[0], 400)
                code, receipt = post(b"release", fields)
                self.assertEqual(code, 200)
                expected = hmac.new(SECRET.encode(), ("deployed\n" + "b" * 40 + "\n" + "a" * 32 + "\n").encode(), hashlib.sha256).hexdigest()
                self.assertEqual(receipt["signature"], expected)
                self.assertEqual(post(b"release", fields)[0], 409)
                self.assertEqual(calls, [(b"release", "b" * 40)])
                self.assertTrue((pathlib.Path(root) / ".http-deploy-state.json").exists())
            finally:
                server.shutdown()
                thread.join()
                server.server_close()


class UploadDeadlineTest(unittest.TestCase):
    def exercise(self, expired):
        calls, initial = [], int(time.time())
        current = [initial]
        checked = threading.Event()
        def clock():
            checked.set()
            return current[0]
        with tempfile.TemporaryDirectory() as root:
            server = DeploymentServer(("127.0.0.1", 0), SECRET, root, lambda file, rev: calls.append(rev), upload_timeout=0.03 if not expired else 2, clock=clock)
            thread = threading.Thread(target=server.serve_forever)
            thread.start()
            connection = http.client.HTTPConnection(*server.server_address, timeout=5)
            body = b"release"
            try:
                connection.putrequest("POST", "/deploy")
                for key, value in headers(body, stamp=initial).items():
                    connection.putheader(key, value)
                connection.putheader("Content-Length", str(len(body)))
                connection.endheaders()
                connection.send(body[:1])
                self.assertTrue(checked.wait(1))
                if expired:
                    current[0] += 301
                    connection.send(body[1:])
                else:
                    time.sleep(0.06)
                response = connection.getresponse()
                self.assertEqual(response.status, 401 if expired else 408)
                response.read()
                self.assertEqual(calls, [])
            finally:
                connection.close()
                server.shutdown()
                thread.join()
                server.server_close()

    def test_total_upload_deadline(self):
        self.exercise(False)

    def test_revalidate_signature_after_upload(self):
        self.exercise(True)
