"""The runner must pass an unrelated correct implementation and catch plausible wrong ones."""
import json
from pathlib import Path
import socket
import sys
import tempfile
import threading
import unittest

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
sys.path.insert(0, str(HERE / "examples"))

import accept  # noqa: E402
import toy_system  # noqa: E402

EXPECTED_RULE = {
    "commission_linear": "R01",
    "pending_counts": "R02",
    "c_monthly_reset": "R03",
    "b_monthly_reset": "R04",
    "history_current_level": "R06",
    "leave_overridden": "R07",
    "overpay_allowed": "R08",
    "salary_double_expense": "R09",
    "zero_entry_absent": "R05",
}


def free_port():
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


class AcceptanceRunnerTest(unittest.TestCase):
    def run_against(self, mutant="", decisions=None, extra=()):
        port = free_port()
        server = toy_system.serve(port, mutant)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            with tempfile.TemporaryDirectory() as tmp:
                manifest = Path(tmp) / "adapter.json"
                manifest.write_text(json.dumps({"decisions": decisions if decisions is not None else {"D1": "carry_over"}}))
                output = Path(tmp) / "out"
                code = accept.main(["--adapter", f"http://127.0.0.1:{port}/acceptance/act", "--manifest", str(manifest),
                                    "--output", str(output), *extra])
                summary = json.loads((output / "summary.json").read_text())
                self.assertTrue((output / "REPORT.md").read_text().startswith("# 学员验收报告"))
                return code, summary
        finally:
            server.shutdown()
            server.server_close()

    @staticmethod
    def failing_rules(summary):
        return {f["rule"] for s in summary["scenarios"] for f in s["failures"]}

    def test_independent_correct_implementation_passes(self):
        code, summary = self.run_against()
        self.assertEqual(code, 0, json.dumps(summary["scenarios"], ensure_ascii=False)[:2000])
        self.assertEqual(summary["counts"]["passed"], len(accept.load_scenarios(HERE / "scenarios")))
        self.assertEqual(summary["negative"]["status"], "passed")

    def test_each_plausible_mistake_fails_with_its_rule(self):
        for mutant, rule in EXPECTED_RULE.items():
            with self.subTest(mutant=mutant):
                code, summary = self.run_against(mutant)
                self.assertEqual(code, 1)
                self.assertIn(rule, self.failing_rules(summary))

    def test_decision_branch_follows_manifest(self):
        code, summary = self.run_against("b_monthly_reset", {"D1": "monthly_reset"})
        self.assertEqual(code, 0, self.failing_rules(summary))

    def test_missing_decision_is_reported_not_guessed(self):
        code, summary = self.run_against(decisions={})
        self.assertEqual(code, 1)
        messages = [f.get("message", "") for s in summary["scenarios"] for f in s["failures"]]
        self.assertTrue(any("D1" in m for m in messages))

    def test_state_held_only_in_the_adapter_process_is_caught_by_restart(self):
        port = free_port()
        with tempfile.TemporaryDirectory() as tmp:
            manifest = Path(tmp) / "adapter.json"
            manifest.write_text(json.dumps({"decisions": {"D1": "carry_over"}}))
            code = accept.main(["--adapter", f"http://127.0.0.1:{port}/acceptance/act", "--manifest", str(manifest),
                                "--adapter-cmd", f"{sys.executable} {HERE / 'examples' / 'toy_system.py'} --port {port}",
                                "--output", str(Path(tmp) / "out")])
            summary = json.loads((Path(tmp) / "out" / "summary.json").read_text())
        self.assertEqual(code, 1)
        self.assertIn("重启", summary["restart"])

    def test_unsupported_actions_are_skipped_not_failed(self):
        code, summary = self.run_against(decisions={"D1": "carry_over"}, extra=["--rule", "R08"])
        self.assertEqual(code, 0)
        with tempfile.TemporaryDirectory() as tmp:
            port = free_port()
            server = toy_system.serve(port)
            threading.Thread(target=server.serve_forever, daemon=True).start()
            try:
                manifest = Path(tmp) / "adapter.json"
                manifest.write_text(json.dumps({"decisions": {"D1": "carry_over"}, "unsupported": ["submit_order"]}))
                accept.main(["--adapter", f"http://127.0.0.1:{port}/acceptance/act", "--manifest", str(manifest),
                             "--output", str(Path(tmp) / "out")])
                summary = json.loads((Path(tmp) / "out" / "summary.json").read_text())
            finally:
                server.shutdown()
                server.server_close()
        using = [s for s in accept.load_scenarios(HERE / "scenarios")
                 if any(step.get("do") == "submit_order" for step in s["steps"])]
        self.assertEqual(summary["counts"]["unsupported"], len(using))
        self.assertEqual(summary["status"], "passed")

    def test_scenarios_reference_only_contract_actions_and_known_rules(self):
        contract = json.loads((HERE / "contract.json").read_text())
        rules = accept.rule_titles()
        for scenario in accept.load_scenarios(HERE / "scenarios"):
            for rule in scenario["rules"]:
                self.assertIn(rule, rules, scenario["id"])
            for step in scenario["steps"]:
                self.assertIn(step.get("do") or step.get("check"), contract["actions"], scenario["id"])
                self.assertIn(step.get("rule", scenario["rules"][0]), rules, scenario["id"])


    def test_hidden_variants_are_deterministic_secret_and_pass_a_correct_implementation(self):
        sys.path.insert(0, str(HERE / "tools"))
        import hidden_seeds
        seed = "unit-test-hidden-seed-0001"
        first, info = hidden_seeds.build_variant(seed, 0)
        again, _ = hidden_seeds.build_variant(seed, 0)
        other, other_info = hidden_seeds.build_variant("unit-test-hidden-seed-0002", 0)
        self.assertEqual(first, again)
        self.assertNotEqual(info["plan"], other_info["plan"])
        self.assertNotIn(seed, json.dumps(first, ensure_ascii=False))
        with tempfile.TemporaryDirectory() as tmp:
            hidden_seeds.main(["--seed", seed, "--count", "2", "--out", tmp])
            manifest = json.loads((Path(tmp) / "manifest.json").read_text())
            self.assertNotIn(seed, json.dumps(manifest))
            code, summary = self.run_against(extra=["--scenarios", tmp])
            self.assertEqual(code, 0, self.failing_rules(summary))
            self.assertEqual(summary["counts"]["passed"], 2)
            code, summary = self.run_against("commission_linear", extra=["--scenarios", tmp])
            self.assertEqual(code, 1)


if __name__ == "__main__":
    unittest.main()
