"""Verify bounded, regular-file-only release archives before root installs them."""
import gzip
import hashlib
import json
import pathlib
import re
import sys
import tarfile

MAX_ARCHIVE = 32 * 1024 * 1024
MAX_TAR = 34 * 1024 * 1024
MAX_FILES = 128


def allowed(name):
    return name in {"bin/cc4-server", "web/index.html", "web/version.json", "SHA256SUMS"} or bool(
        re.fullmatch(r"web/assets/[A-Za-z0-9_.-]+\.(?:js|css)", name)
        or re.fullmatch(r"migrations/[0-9]{3}_[a-z0-9_]+\.sql", name)
    )


def verify(root, revision):
    root = pathlib.Path(root)
    archive = root / "bundle.tar.gz"
    if archive.stat().st_size > MAX_ARCHIVE:
        raise ValueError("Compressed bundle exceeds limit")
    total = 0
    with gzip.open(archive, "rb") as source, (root / "bundle.tar").open("xb") as target:
        while chunk := source.read(64 * 1024):
            total += len(chunk)
            if total > MAX_TAR:
                raise ValueError("Decompressed bundle exceeds limit")
            target.write(chunk)
    files = {}
    destination = root / "verified"
    destination.mkdir()
    with (root / "bundle.tar").open("rb") as source:
        while True:
            header = source.read(512)
            if header == bytes(512):
                padding = source.read()
                if len(padding) < 512 or len(padding) % 512 or any(padding):
                    raise ValueError("Invalid end-of-archive padding")
                break
            member = tarfile.TarInfo.frombuf(header, "utf-8", "strict")
            if member.type not in (tarfile.REGTYPE, tarfile.AREGTYPE):
                raise ValueError("Only plain regular-file headers are accepted")
            if not allowed(member.name) or member.name in files or len(files) >= MAX_FILES:
                raise ValueError("Unexpected or duplicate release file")
            limit = 24 * 1024 * 1024 if member.name == "bin/cc4-server" else 8 * 1024 * 1024
            if not 0 < member.size <= limit:
                raise ValueError("Invalid release member size")
            body = source.read(member.size)
            if len(body) != member.size:
                raise ValueError("Truncated release member")
            source.read((-member.size) % 512)
            files[member.name] = hashlib.sha256(body).hexdigest()
            path = destination / member.name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(body)
    required = {"bin/cc4-server", "web/index.html", "web/version.json", "migrations/000_baseline_schema.sql", "migrations/001_go_completion.sql", "SHA256SUMS"}
    if not required <= files.keys():
        raise ValueError("Required release files are missing")
    manifest = (destination / "SHA256SUMS").read_text()
    expected = "".join(digest + "  " + name + "\n" for name, digest in sorted(files.items()) if name != "SHA256SUMS")
    if manifest != expected or (destination / "bin/cc4-server").read_bytes()[:4] != b"\x7fELF":
        raise ValueError("Release checksum or binary format is invalid")
    if json.loads((destination / "web/version.json").read_text()) != {"revision": revision}:
        raise ValueError("Release revision does not match request")


if __name__ == "__main__":
    try:
        verify(sys.argv[1], sys.argv[2])
    except (ValueError, OSError, tarfile.TarError, EOFError) as error:
        raise SystemExit("Release rejected: " + str(error))
