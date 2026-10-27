import gzip
import hashlib
import io
import pathlib
import tarfile
import tempfile
import unittest

from verify_release import MAX_TAR, verify


class ReleaseTest(unittest.TestCase):
    def bundle(self, root, entries):
        with tarfile.open(pathlib.Path(root) / "bundle.tar.gz", "w:gz", format=tarfile.USTAR_FORMAT) as archive:
            for name, body in entries:
                info = tarfile.TarInfo(name)
                info.size = len(body)
                archive.addfile(info, io.BytesIO(body))

    def test_valid_bundle(self):
        binary = b"\x7fELFfixture"
        manifest = (hashlib.sha256(binary).hexdigest() + "  lead-source-mock\n").encode()
        with tempfile.TemporaryDirectory() as root:
            self.bundle(root, [("lead-source-mock", binary), ("SHA256SUMS", manifest)])
            verify(root)

    def test_reject_traversal_duplicate_and_bad_checksum(self):
        for entries in [[("../outside", b"a"), ("SHA256SUMS", b"b")], [("lead-source-mock", b"a"), ("lead-source-mock", b"b")], [("lead-source-mock", b"\x7fELF"), ("SHA256SUMS", b"bad")]]:
            with tempfile.TemporaryDirectory() as root:
                self.bundle(root, entries)
                with self.assertRaises(ValueError):
                    verify(root)

    def test_reject_pax_header_before_reading_its_claimed_payload(self):
        with tempfile.TemporaryDirectory() as root:
            header = tarfile.TarInfo("PaxHeader")
            header.type = tarfile.XHDTYPE
            header.size = 2**40
            with gzip.open(pathlib.Path(root) / "bundle.tar.gz", "wb") as output:
                output.write(header.tobuf(format=tarfile.GNU_FORMAT) + b"\0" * 1024)
            with self.assertRaisesRegex(ValueError, "regular-file"):
                verify(root)

    def test_reject_decompression_bomb(self):
        with tempfile.TemporaryDirectory() as root:
            with gzip.open(pathlib.Path(root) / "bundle.tar.gz", "wb") as output:
                for _ in range(MAX_TAR // (64 * 1024) + 2):
                    output.write(b"\0" * (64 * 1024))
            with self.assertRaisesRegex(ValueError, "Decompressed"):
                verify(root)


if __name__ == "__main__":
    unittest.main()
