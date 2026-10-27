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
        self.migration_count = len(list(self.directory.glob("*.sql")))

    def apply(self):
        migrate(self.directory, DATABASE)

    def test_fresh_schema_and_idempotent_repeat(self):
        self.apply()
        self.apply()
        self.assertEqual(sql(DATABASE, "SELECT count(*) FROM public.cc4_schema_migrations"), str(self.migration_count))
        self.assertEqual(sql(DATABASE, "SELECT to_regclass('public.users') IS NOT NULL"), "t")

    def test_failed_increment_rolls_back_schema_and_ledger(self):
        self.apply()
        (self.directory / "999_deliberate_failure.sql").write_text("CREATE TABLE rollback_fixture(id integer); SELECT 1/0;")
        with self.assertRaises(RuntimeError):
            self.apply()
        self.assertEqual(sql(DATABASE, "SELECT to_regclass('public.rollback_fixture') IS NULL"), "t")
        self.assertEqual(sql(DATABASE, "SELECT count(*) FROM public.cc4_schema_migrations"), str(self.migration_count))

    def test_modified_applied_migration_is_rejected(self):
        self.apply()
        path = self.directory / "000_baseline_schema.sql"
        path.write_text(path.read_text() + "\n-- changed migration\n")
        with self.assertRaisesRegex(ValueError, "modified"):
            self.apply()

    def test_snapshot_adoption_aligns_sequences_without_rewinding_them(self):
        for name in ("000_baseline_schema.sql", "001_go_completion.sql"):
            sql(DATABASE, (self.directory / name).read_text())
        sql(DATABASE, """
            INSERT INTO products(id,name,code,price) VALUES(1,'Imported','IMPORTED',9);
            INSERT INTO users(id,phone,password_hash,role) VALUES(904,'00000000904','test','admin');
            SELECT setval('salary_records_id_seq',100,true);
        """)
        migrate(self.directory, DATABASE, adopt=True)
        self.assertEqual(sql(DATABASE, "SELECT nextval('products_id_seq')"), "2")
        self.assertEqual(sql(DATABASE, "SELECT nextval('users_id_seq')"), "905")
        self.assertEqual(sql(DATABASE, "SELECT nextval('salary_records_id_seq')"), "101")
        self.assertEqual(sql(DATABASE, "SELECT nextval('customers_id_seq')"), "1")
        self.apply()
        self.assertEqual(sql(DATABASE, "SELECT nextval('products_id_seq')"), "3")

    def test_monthly_salary_expense_is_split_per_payment(self):
        later = self.directory / "005_salary_expense_per_payment.sql"
        body = later.read_bytes()
        later.unlink()
        self.apply()
        sql(DATABASE, """
            INSERT INTO employees(id,employee_code,name,level,status,join_date) VALUES
              (9001,'MIG_A','迁移甲','trainee','active','2026-06-01'),(9002,'MIG_B','迁移乙','trainee','active','2026-06-01'),
              (9003,'MIG_C','迁移丙','trainee','active','2026-06-01');
            INSERT INTO salary_records(id,employee_id,period_year,period_month,net_salary,payment_status,payment_date) VALUES
              (8001,9001,2026,6,70,'paid','2026-06-22'),(8002,9002,2026,6,90,'paid','2026-06-26'),(8003,9003,2026,6,50,'pending',NULL);
            INSERT INTO finance_records(type,category,amount,transaction_date,description,status,reference_type,reference_id,source)
              VALUES('expense','salary',160,'2026-06-26','2026年6月已发薪资','confirmed','salary','salary-2026-6','auto'),
                    ('expense','office',12,'2026-06-23','unrelated','confirmed',NULL,NULL,'manual');
        """)
        later.write_bytes(body)
        self.apply()
        rows = sql(DATABASE, "SELECT reference_id||'|'||transaction_date||'|'||amount FROM finance_records WHERE reference_type='salary' ORDER BY reference_id")
        self.assertEqual(rows.splitlines(), ["salary-record-8001|2026-06-22|70.00", "salary-record-8002|2026-06-26|90.00"])
        self.assertEqual(sql(DATABASE, "SELECT count(*) FROM finance_records WHERE category='office'"), "1")
        with self.assertRaises(RuntimeError):
            sql(DATABASE, "INSERT INTO finance_records(type,category,amount,transaction_date,reference_type,reference_id) VALUES('expense','salary',1,'2026-06-30','salary','salary-record-8001')")


if __name__ == "__main__":
    unittest.main()
