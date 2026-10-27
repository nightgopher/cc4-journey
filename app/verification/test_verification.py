import copy
import datetime as dt
import unittest

from check_report import FAMILIES, ZONE, validate_negative, validate_report
from verify import isolated_environment
from unittest.mock import patch


def fixture():
    today = dt.datetime.now(ZONE).date()
    day = today.isoformat()
    checks = [{"scope": "账务复核：测试", "date": day, "title": title+"（元）", "expected": value, "actual": value, "passed": True}
              for title, value in (("回读业绩销售额", "99.00"), ("回读回款入账合计", "100.00"),
                                   ("回读运营费用", "65.50"), ("回读离职结算工资", "70.00"))]
    events = [{"date": day, "title": family, "status": "passed"} for family in FAMILIES]
    return {"version": 2, "status": "passed", "origin": (today-dt.timedelta(days=6)).isoformat(),
            "through": day, "updatedAt": dt.datetime.now(dt.timezone.utc).isoformat(), "error": None,
            "checks": checks, "events": events,
            "summary": {"days": 7, "employees": 1, "checks": len(checks), "events": len(events)},
            "money": {"sales": 99, "inflow": 199, "outflow": 135.5, "net": 63.5}}


class VerificationTest(unittest.TestCase):
    def test_consistent_report_is_accepted(self):
        self.assertEqual(validate_report(fixture())["status"], "passed")

    def test_passed_flag_cannot_hide_one_yuan_difference(self):
        report = fixture()
        report["checks"][0]["actual"] = "100.00"
        with self.assertRaisesRegex(ValueError, "预期/实际"):
            validate_report(report)

    def test_stage_snapshots_are_not_counted_twice_as_final_cash(self):
        report = fixture()
        duplicate = dict(report["checks"][0], scope="财务：同步收入时的中间核对")
        report["checks"].append(duplicate)
        report["summary"]["checks"] += 1
        validate_report(report)

    def test_boolean_is_not_interchangeable_with_number(self):
        report = fixture()
        report["checks"][0].update(expected=True, actual=1)
        with self.assertRaisesRegex(ValueError, "预期/实际"):
            validate_report(report)

    def test_summary_cannot_claim_checks_that_did_not_run(self):
        report = fixture()
        report["summary"]["checks"] += 1
        with self.assertRaisesRegex(ValueError, "计数"):
            validate_report(report)

    def test_missing_business_family_is_not_a_full_pass(self):
        report = fixture()
        report["events"][3]["title"] = "无关事件"
        with self.assertRaisesRegex(ValueError, "B→A"):
            validate_report(report)

    def test_stale_or_unfinished_report_is_rejected(self):
        for key, value in (("through", "2020-01-01"), ("status", "running"), ("status", "failed")):
            report = fixture()
            report[key] = value
            with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                validate_report(report)

    def test_null_and_nonfinite_amounts_are_rejected(self):
        for value in (None, "NaN", "Infinity", "65.501"):
            report = fixture()
            report["money"]["outflow"] = value
            with self.subTest(value=value), self.assertRaises((ValueError, ArithmeticError)):
                validate_report(report)

    def test_correct_net_does_not_hide_wrong_source_amounts(self):
        report = fixture()
        report["money"].update(inflow=200, outflow=136.5)
        with self.assertRaisesRegex(ValueError, "流入"):
            validate_report(report)

    def test_fault_must_fail_for_the_injected_one_yuan(self):
        report = {"status": "failed", "error": "accounting difference", "checks": [
            {"title": "已确认订单与收入一致（元）", "expected": "99.00", "actual": "100.00", "passed": False}]}
        validate_negative(report)
        wrong = copy.deepcopy(report)
        wrong["checks"][0]["actual"] = "101.00"
        with self.assertRaises(ValueError):
            validate_negative(wrong)
        with self.assertRaises(ValueError):
            validate_negative({"status": "failed", "error": "connection refused", "checks": []})

    def test_inherited_business_connections_and_credentials_are_removed(self):
        with patch.dict("os.environ", {"DATABASE_URL": "production", "PGSERVICE": "production", "PGPASSFILE": "production",
                                      "CC4_KEEP_TEST_DB": "1", "CC4_VERIFY_TOKEN": "secret", "GOOKI_HOST": "remote",
                                      "MCP_TOKEN": "secret", "PADDLEOCR_API_KEY": "secret", "PATH": "/bin"}, clear=True):
            env = isolated_environment()
        for key in ("DATABASE_URL", "PGSERVICE", "PGPASSFILE", "CC4_VERIFY_TOKEN", "GOOKI_HOST", "MCP_TOKEN", "PADDLEOCR_API_KEY"):
            self.assertNotIn(key, env)
        self.assertEqual(env["CC4_KEEP_TEST_DB"], "0")
        self.assertEqual(env["PATH"], "/bin")


if __name__ == "__main__":
    unittest.main()
