"""Optional real-PostgreSQL tests, always restricted to a disposable test DB."""
import os
import pathlib
import re
import tempfile
import unittest

from build_release import migration_sql
from migrate import migrate, sql

DATABASE = os.environ.get("CC4_MIGRATION_TEST_DATABASE", "")


@unittest.skipUnless(DATABASE, "Set CC4_MIGRATION_TEST_DATABASE to an isolated database")
class MigrationTest(unittest.TestCase):
    def setUp(self):
        if not re.fullmatch(r"cc4_test_[a-z0-9_]+", DATABASE):
            raise ValueError("Refusing to modify a non-test database")
        sql(DATABASE, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = pathlib.Path(self.temporary.name)
        for path in (pathlib.Path(__file__).resolve().parents[1] / "migrations").glob("*.sql"):
            (self.directory / path.name).write_bytes(migration_sql(path.read_text()))

    def apply(self):
        migrate(self.directory, DATABASE)

    def test_fresh_schema_and_idempotent_repeat(self):
        self.apply()
        self.apply()
        self.assertEqual(sql(DATABASE, "SELECT count(*) FROM public.cc4_schema_migrations"), "2")
        self.assertEqual(sql(DATABASE, "SELECT to_regclass('public.users') IS NOT NULL"), "t")

    def test_failed_increment_rolls_back_schema_and_ledger(self):
        self.apply()
        (self.directory / "002_deliberate_failure.sql").write_text("CREATE TABLE rollback_fixture(id integer); SELECT 1/0;")
        with self.assertRaises(RuntimeError):
            self.apply()
        self.assertEqual(sql(DATABASE, "SELECT to_regclass('public.rollback_fixture') IS NULL"), "t")
        self.assertEqual(sql(DATABASE, "SELECT count(*) FROM public.cc4_schema_migrations"), "2")

    def test_modified_applied_migration_is_rejected(self):
        self.apply()
        path = self.directory / "000_baseline_schema.sql"
        path.write_text(path.read_text() + "\n-- changed migration\n")
        with self.assertRaisesRegex(ValueError, "modified"):
            self.apply()


if __name__ == "__main__":
    unittest.main()
