"""Bound decompression before parsing tar metadata; never execute release code."""
import gzip
import hashlib
import pathlib
import re
import sys
import tarfile

MAX_BINARY = 32 * 1024 * 1024
MAX_TAR = MAX_BINARY + 2 * 1024 * 1024


def verify(root):
    root = pathlib.Path(root)
    archive = root / "bundle.tar.gz"
    if archive.stat().st_size > MAX_BINARY:
        raise ValueError("Compressed bundle exceeds limit")
    size = 0
    with gzip.open(archive, "rb") as source, (root / "bundle.tar").open("xb") as target:
        while chunk := source.read(64 * 1024):
            size += len(chunk)
            if size > MAX_TAR:
                raise ValueError("Decompressed bundle exceeds limit")
            target.write(chunk)
    names = set()
    with (root / "bundle.tar").open("rb") as source:
        for _ in range(2):
            # frombuf examines only one fixed header; it does not interpret or
            # read a potentially huge PAX/GNU extension payload.
            member = tarfile.TarInfo.frombuf(source.read(512), "utf-8", "strict")
            if member.type not in (tarfile.REGTYPE, tarfile.AREGTYPE):
                raise ValueError("Only plain regular-file headers are accepted")
            if member.name not in {"lead-source-mock", "SHA256SUMS"} or member.name in names:
                raise ValueError("Unexpected or duplicate release file")
            limit = MAX_BINARY if member.name == "lead-source-mock" else 1024
            if not 0 < member.size <= limit:
                raise ValueError("Release member exceeds limit")
            content = source.read(member.size)
            if len(content) != member.size:
                raise ValueError("Truncated release member")
            (root / member.name).write_bytes(content)
            source.read((-member.size) % 512)
            names.add(member.name)
        padding = 0
        while chunk := source.read(64 * 1024):
            if any(chunk):
                raise ValueError("Unexpected trailing release member")
            padding += len(chunk)
        if padding < 1024 or padding % 512:
            raise ValueError("Invalid end-of-archive padding")
    manifest = (root / "SHA256SUMS").read_text()
    match = re.fullmatch(r"([0-9a-f]{64})  lead-source-mock\n", manifest)
    binary = (root / "lead-source-mock").read_bytes()
    if not match or hashlib.sha256(binary).hexdigest() != match[1] or binary[:4] != b"\x7fELF":
        raise ValueError("Release checksum or binary format is invalid")


if __name__ == "__main__":
    try:
        verify(sys.argv[1])
    except (ValueError, OSError, tarfile.TarError, EOFError) as error:
        raise SystemExit("Release rejected: " + str(error))
