"""Run checksummed SQL migrations as the database owner, using peer auth."""
import argparse
import hashlib
import pathlib
import re
import subprocess


def sql(database, statement):
    result = subprocess.run(["psql", "-X", "-qAt", "-v", "ON_ERROR_STOP=1", "-d", database], input=statement, text=True, capture_output=True, timeout=120)
    if result.returncode:
        raise RuntimeError(result.stderr[-3000:])
    return result.stdout.strip()


def migrate(directory, database="cc4", adopt=False):
    paths = sorted(pathlib.Path(directory).glob("*.sql"))
    if not paths or any(not re.fullmatch(r"[0-9]{3}_[a-z0-9_]+\.sql", path.name) for path in paths):
        raise ValueError("Invalid migration filenames")
    sql(database, "CREATE TABLE IF NOT EXISTS public.cc4_schema_migrations (version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now());")
    applied = dict(line.split("|", 1) for line in sql(database, "SELECT version, checksum FROM public.cc4_schema_migrations ORDER BY version;").splitlines())
    available = {path.name: path for path in paths}
    if applied.keys() - available.keys():
        raise ValueError("Release is missing already applied migrations")
    snapshot_versions = {"000_baseline_schema.sql", "001_go_completion.sql"}
    if adopt:
        if applied or not snapshot_versions.issubset(available):
            raise ValueError("Adoption is only for the initial known schema snapshot")
        present = sql(database, "SELECT to_regclass('users') IS NOT NULL AND to_regclass('agent_tool_audit') IS NOT NULL AND to_regclass('order_edit_history') IS NOT NULL AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='attendance_records' AND column_name='source');")
        if present != "t":
            raise ValueError("Snapshot does not have the expected completed schema")
    for path in paths:
        body = path.read_text()
        checksum = hashlib.sha256(path.read_bytes()).hexdigest()
        if path.name in applied:
            if applied[path.name] != checksum:
                raise ValueError("Applied migration was modified: " + path.name)
            continue
        adopt_snapshot = adopt and path.name in snapshot_versions
        statement = "" if adopt_snapshot else body
        sql(database, "BEGIN;\nSET LOCAL lock_timeout='10s';\n" + statement + "\nINSERT INTO public.cc4_schema_migrations(version,checksum) VALUES ('" + path.name + "','" + checksum + "');\nCOMMIT;\n")
        print(("Adopted " if adopt_snapshot else "Applied ") + path.name)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("directory")
    parser.add_argument("--database", default="cc4")
    parser.add_argument("--adopt-existing", action="store_true")
    args = parser.parse_args()
    migrate(args.directory, args.database, args.adopt_existing)
