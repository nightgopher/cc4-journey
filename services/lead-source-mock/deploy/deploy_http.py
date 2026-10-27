"""Signed release transport for runners whose outbound SSH is blocked.

The shared secret is never transmitted. A signature binds the body digest,
revision, timestamp and one-use nonce. Only the fixed release installer runs.
"""
import hashlib
import hmac
import http.server
import json
import logging
import os
import pathlib
import re
import signal
import subprocess
import tempfile
import threading
import time

MAX_BODY = 32 * 1024 * 1024


def signature(secret, revision, stamp, nonce, digest):
    message = f"{revision}\n{stamp}\n{nonce}\n{digest}\n".encode()
    return hmac.new(secret.encode(), message, hashlib.sha256).hexdigest()


def authenticate(headers, secret, now):
    values = [headers.get("X-CC4-" + key, "") for key in ("Revision", "Timestamp", "Nonce", "SHA256", "Signature")]
    revision, stamp, nonce, digest, supplied = values
    patterns = [r"[0-9a-f]{40}", r"[0-9]{10}", r"[0-9a-f]{32}", r"[0-9a-f]{64}", r"[0-9a-f]{64}"]
    if not all(re.fullmatch(pattern, value) for pattern, value in zip(patterns, values)):
        raise ValueError("Invalid release authentication")
    if abs(now - int(stamp)) > 300 or not hmac.compare_digest(supplied, signature(secret, revision, stamp, nonce, digest)):
        raise ValueError("Expired or invalid release signature")
    return revision, int(stamp), nonce, digest


def install(file, revision):
    env = os.environ.copy()
    env["SSH_ORIGINAL_COMMAND"] = "deploy " + revision
    # The installer bounds its own waits and owns rollback. Do not hard-kill its
    # shell with subprocess.run(timeout=...), which would bypass the EXIT trap.
    result = subprocess.run(["/usr/local/sbin/cc4-lead-source-deploy"], stdin=file, capture_output=True, env=env)
    if result.returncode:
        logging.error("Release installer failed: %s", result.stderr.decode(errors="replace")[-4000:])
        raise RuntimeError("Release installer rejected the deployment")


class DeploymentServer(http.server.HTTPServer):
    def __init__(self, address, secret, state_dir, installer=install, upload_timeout=60, clock=time.time):
        self.secret = secret
        self.state_dir = pathlib.Path(state_dir)
        self.installer = installer
        self.upload_timeout = upload_timeout
        self.clock = clock
        super().__init__(address, Handler)


class Handler(http.server.BaseHTTPRequestHandler):
    def setup(self):
        super().setup()
        self.connection.settimeout(45)

    def reply(self, code, body):
        data = json.dumps(body).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if self.path == "/health":
            self.reply(200, {"status": "ok", "transport": "HMAC-SHA256"})
        else:
            self.reply(404, {"error": "Not found"})

    def do_POST(self):
        if self.path != "/deploy":
            self.reply(404, {"error": "Not found"})
            return
        try:
            revision, stamp, nonce, expected = authenticate(self.headers, self.server.secret, int(self.server.clock()))
        except ValueError:
            self.reply(401, {"error": "Invalid release authentication"})
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            length = 0
        if not 0 < length <= MAX_BODY:
            self.reply(413, {"error": "Invalid release size"})
            return
        state_file = self.server.state_dir / ".http-deploy-state.json"
        state = json.loads(state_file.read_text()) if state_file.exists() else {"timestamp": 0, "nonces": {}}
        if stamp < state["timestamp"] or nonce in state["nonces"]:
            self.reply(409, {"error": "Release request already used or superseded"})
            return
        with tempfile.TemporaryFile(dir=self.server.state_dir) as bundle:
            remaining, digest = length, hashlib.sha256()
            deadline = time.monotonic() + self.server.upload_timeout
            while remaining:
                budget = deadline - time.monotonic()
                if budget <= 0:
                    self.reply(408, {"error": "Release upload deadline exceeded"})
                    return
                self.connection.settimeout(min(45, budget))
                try:
                    # read1 returns after at most one socket read, so a trickle
                    # cannot keep a buffered read alive beyond the total budget.
                    chunk = self.rfile.read1(min(64 * 1024, remaining))
                except TimeoutError:
                    self.reply(408, {"error": "Release upload deadline exceeded"})
                    return
                if not chunk:
                    self.reply(400, {"error": "Incomplete release body"})
                    return
                bundle.write(chunk)
                digest.update(chunk)
                remaining -= len(chunk)
            if not hmac.compare_digest(digest.hexdigest(), expected):
                self.reply(400, {"error": "Release body checksum mismatch"})
                return
            try:
                authenticate(self.headers, self.server.secret, int(self.server.clock()))
            except ValueError:
                self.reply(401, {"error": "Release signature expired during upload"})
                return
            state["timestamp"] = stamp
            state["nonces"] = {key: value for key, value in state["nonces"].items() if value >= int(self.server.clock()) - 600}
            state["nonces"][nonce] = stamp
            pending = state_file.with_suffix(".next")
            pending.write_text(json.dumps(state))
            pending.chmod(0o600)
            pending.replace(state_file)
            bundle.seek(0)
            try:
                self.server.installer(bundle, revision)
            except RuntimeError:
                self.reply(500, {"error": "Deployment failed; check the service journal"})
                return
        receipt = hmac.new(self.server.secret.encode(), f"deployed\n{revision}\n{nonce}\n".encode(), hashlib.sha256).hexdigest()
        self.reply(200, {"status": "deployed", "revision": revision, "nonce": nonce, "signature": receipt})

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    secret = os.environ.get("CC4_MOCK_DEPLOY_SECRET", "")
    if len(secret) < 32:
        raise SystemExit("CC4_MOCK_DEPLOY_SECRET must have at least 32 characters")
    server = DeploymentServer(("127.0.0.1", 18082), secret, "/opt/cc4-lead-source-mock")
    def stop(*args):
        threading.Thread(target=server.shutdown, daemon=True).start()
    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    try:
        server.serve_forever()
    finally:
        server.server_close()
