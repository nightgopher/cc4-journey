import datetime as dt
import json
import unittest
from unittest.mock import Mock, patch

from run import API, REPORT_KEY, STATE_KEY, ScenarioFailure, Simulation, cents


class MemoryAPI:
    def __init__(self):
        self.values = {}
        self.business_writes = 0

    def get(self, path):
        if path == "/settings":
            return {"settings": [{"key": key, "value": value} for key, value in self.values.items()]}
        if path == "/simulation/report":
            return {"enabled": True}
        raise AssertionError(path)

    def post(self, path, body, expected=200):
        if path == "/settings":
            self.values[body["key"]] = body["value"]
            return {}
        self.business_writes += 1
        raise ScenarioFailure("Connection lost after write; result uncertain")


class SimulationSafetyTest(unittest.TestCase):
    def simulation(self, api=None):
        return Simulation(api or MemoryAPI(), "test-seed-that-is-longer-than-32-characters", dt.date(2026, 10, 27))

    def test_money_difference_fails_and_is_recorded(self):
        simulation = self.simulation()
        simulation.money_check("收入", 9900, "99.00")
        with self.assertRaises(ScenarioFailure):
            simulation.money_check("收入", 9900, "100.00")
        self.assertEqual(len(simulation.state["checks"]), 1)
        self.assertFalse(simulation.state["checks"][0]["passed"])

    def test_pending_write_is_not_replayed(self):
        api = MemoryAPI()
        simulation = self.simulation(api)
        with self.assertRaises(ScenarioFailure):
            simulation.event("2026-10-27", "录入订单", lambda: api.post("/daily-performance", {}))
        self.assertEqual(api.business_writes, 1)
        self.assertIsNotNone(json.loads(api.values[STATE_KEY])["pending"])
        resumed = self.simulation(api)
        with self.assertRaisesRegex(ScenarioFailure, "reconciliation"):
            resumed.run()
        self.assertEqual(api.business_writes, 1)
        self.assertEqual(json.loads(api.values[REPORT_KEY])["status"], "failed")

    def test_passwords_are_not_persisted_in_state_or_report(self):
        simulation = self.simulation()
        password = simulation.password("employee")
        simulation.save()
        stored = json.dumps(simulation.api.values)
        self.assertNotIn(password, stored)
        self.assertNotIn(simulation.seed.decode(), stored)
        self.assertTrue(simulation.phone("employee").startswith("000"))

    def test_rechecking_an_assertion_updates_its_result(self):
        simulation = self.simulation()
        simulation.check("订单数", 3, 3)
        simulation.check("订单数", 3, 3)
        self.assertEqual(len(simulation.state["checks"]), 1)
        self.assertEqual(cents("65.50"), 6550)

    def test_short_seed_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "32"):
            Simulation(MemoryAPI(), "short", dt.date(2026, 10, 27))

    def test_completed_event_is_not_replayed_after_midday_restart(self):
        api = MemoryAPI()
        simulation = self.simulation(api)
        write = Mock()
        simulation.event("2026-10-27", "已完成的订单", write)
        self.assertIsNone(simulation.state["lastDay"])
        resumed = self.simulation(api)
        resumed.event("2026-10-27", "已完成的订单", write)
        self.assertEqual(write.call_count, 1)

    def test_existing_empty_work_calendar_is_preserved(self):
        class EmptyCalendarAPI(MemoryAPI):
            def get(self, path):
                return {"workdays": []} if path.startswith("/workdays/") else super().get(path)
        api = EmptyCalendarAPI()
        simulation = self.simulation(api)
        simulation.configure_workdays("2026-10-27")
        self.assertEqual(api.business_writes, 0)
        self.assertEqual(simulation.state["workdays"]["2026-10"], [])

    def test_partial_setup_continues_after_product_checkpoint(self):
        api = MemoryAPI()
        simulation = self.simulation(api)
        simulation.state["product"] = 42
        original_get, original_post = api.get, api.post
        api.get = lambda path: {"user": {"employee_id": 1}} if path == "/auth/session" else original_get(path)
        api.post = lambda path, body, expected=200: {"id": 7} if path == "/teams" else original_post(path, body, expected)
        simulation.hire = Mock()
        simulation.setup()
        self.assertEqual(simulation.state["team"], 7)
        self.assertEqual(simulation.hire.call_count, 5)

    def test_completed_day_detects_corrupted_sales_and_commission(self):
        simulation = self.simulation()
        day = "2026-10-27"
        simulation.state.update({"product": 9, "actors": {"sales": {"id": 2, "name": "模拟销售"}},
                                 "daily": {day: {"sales": {"orders": 5, "commission": 70}}}})
        def rows(path, key):
            if key == "income":
                return [{"reference_type": "daily_performance", "reference_id": "daily-2-2026-10-27-9", "amount": 495}]
            return [{"work_date": day, "product_id": 9, "orders_count": 5, "sales_amount": 0, "commission_amount": 0}]
        simulation.api.rows = rows
        with self.assertRaisesRegex(ScenarioFailure, "回读业绩销售额"):
            simulation.reconcile(day)

    def test_lost_lease_blocks_http_mutation(self):
        api = API("http://127.0.0.1:1/api/v1")
        api.before_write = Mock(side_effect=ScenarioFailure("lease lost"))
        with patch("urllib.request.urlopen") as send:
            with self.assertRaisesRegex(ScenarioFailure, "lease lost"):
                api.post("/daily-performance", {"orders": 5})
            send.assert_not_called()


if __name__ == "__main__":
    unittest.main()
