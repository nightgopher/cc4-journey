import gzip
import hashlib
import io
import json
import pathlib
import tarfile
import tempfile
import unittest

from build_release import migration_sql
from verify_release import MAX_TAR, verify

REVISION = "a" * 40


class ReleaseTest(unittest.TestCase):
    def fixture(self):
        return {
            "bin/cc4-server": b"\x7fELFfixture",
            "web/index.html": b"<html></html>",
            "web/assets/index-123.js": b"console.log('fixture')",
            "web/version.json": json.dumps({"revision": REVISION}).encode(),
            "migrations/000_baseline_schema.sql": b"CREATE TABLE fixture(id integer);",
            "migrations/001_go_completion.sql": b"SELECT 1;",
        }

    def bundle(self, root, files, extra=None):
        entries = dict(files)
        entries["SHA256SUMS"] = "".join(hashlib.sha256(body).hexdigest() + "  " + name + "\n" for name, body in sorted(files.items())).encode()
        with tarfile.open(pathlib.Path(root) / "bundle.tar.gz", "w:gz", format=tarfile.USTAR_FORMAT) as archive:
            for name, body in entries.items():
                info = tarfile.TarInfo(name)
                info.size = len(body)
                archive.addfile(info, io.BytesIO(body))
            if extra:
                archive.addfile(extra)

    def test_accept_static_assets_binary_and_migrations(self):
        with tempfile.TemporaryDirectory() as root:
            self.bundle(root, self.fixture())
            verify(root, REVISION)

    def test_only_named_synthetic_receipts_are_allowed(self):
        files = self.fixture()
        files.update({"web/samples/invoice-demo.png": b"synthetic-image", "web/samples/invoice-demo.pdf": b"synthetic-pdf"})
        with tempfile.TemporaryDirectory() as root:
            self.bundle(root, files)
            verify(root, REVISION)
        files["web/samples/customer-upload.pdf"] = b"unexpected"
        with tempfile.TemporaryDirectory() as root:
            self.bundle(root, files)
            with self.assertRaisesRegex(ValueError, "Unexpected"):
                verify(root, REVISION)

    def test_reject_traversal_symlink_duplicate_and_wrong_revision(self):
        for case in ("traversal", "symlink", "duplicate", "revision"):
            files = self.fixture()
            extra = None
            if case == "traversal":
                files["../outside"] = b"bad"
            elif case in ("symlink", "duplicate"):
                extra = tarfile.TarInfo("web/index.html")
                if case == "symlink":
                    extra.type = tarfile.SYMTYPE
                    extra.linkname = "/etc/passwd"
            else:
                files["web/version.json"] = json.dumps({"revision": "b" * 40}).encode()
            with tempfile.TemporaryDirectory() as root:
                self.bundle(root, files, extra)
                with self.assertRaises(ValueError):
                    verify(root, REVISION)

    def test_reject_pax_header_before_claimed_payload(self):
        with tempfile.TemporaryDirectory() as root:
            info = tarfile.TarInfo("PaxHeader")
            info.type = tarfile.XHDTYPE
            info.size = 2**40
            with gzip.open(pathlib.Path(root) / "bundle.tar.gz", "wb") as output:
                output.write(info.tobuf(format=tarfile.GNU_FORMAT) + bytes(1024))
            with self.assertRaisesRegex(ValueError, "regular-file"):
                verify(root, REVISION)

    def test_reject_decompression_bomb(self):
        with tempfile.TemporaryDirectory() as root:
            with gzip.open(pathlib.Path(root) / "bundle.tar.gz", "wb") as output:
                for _ in range(MAX_TAR // 65536 + 2):
                    output.write(bytes(65536))
            with self.assertRaisesRegex(ValueError, "Decompressed"):
                verify(root, REVISION)

    def test_remove_local_ownership_and_outer_transactions(self):
        body = "BEGIN;\nCREATE TABLE root_table(id int);\nALTER TABLE root_table OWNER TO root;\nCOMMIT;\n"
        self.assertEqual(migration_sql(body).strip(), b"CREATE TABLE root_table(id int);")


if __name__ == "__main__":
    unittest.main()
