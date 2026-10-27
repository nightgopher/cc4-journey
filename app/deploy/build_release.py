"""Package the CI-built binary, static frontend and owner-independent migrations."""
import argparse
import hashlib
import io
import json
import pathlib
import re
import tarfile


def migration_sql(source):
    # The historical baseline was dumped from a local role named root. Cloud
    # objects belong to the unprivileged deployment role instead.
    source = re.sub(r"^ALTER .+ OWNER TO root;\n", "", source, flags=re.MULTILINE)
    source = re.sub(r"^(?:BEGIN|COMMIT);\s*$", "", source, flags=re.MULTILINE)
    return source.encode()


def build(app, revision, output):
    if not re.fullmatch(r"[0-9a-f]{40}", revision):
        raise ValueError("Expected a full Git revision")
    app = pathlib.Path(app)
    files = {"bin/cc4-server": (app / "bin/cc4-server").read_bytes()}
    for path in sorted((app / "web/dist").rglob("*")):
        if path.is_file():
            files["web/" + path.relative_to(app / "web/dist").as_posix()] = path.read_bytes()
    files["web/version.json"] = (json.dumps({"revision": revision}) + "\n").encode()
    for path in sorted((app / "migrations").glob("*.sql")):
        files["migrations/" + path.name] = migration_sql(path.read_text())
    files["SHA256SUMS"] = "".join(hashlib.sha256(body).hexdigest() + "  " + name + "\n" for name, body in sorted(files.items())).encode()
    with tarfile.open(output, "w:gz", format=tarfile.USTAR_FORMAT) as archive:
        for name, body in sorted(files.items()):
            info = tarfile.TarInfo(name)
            info.size = len(body)
            info.mode = 0o755 if name.startswith("bin/") else 0o644
            archive.addfile(info, io.BytesIO(body))


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--app", default="app")
    parser.add_argument("--revision", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    build(args.app, args.revision, args.output)
