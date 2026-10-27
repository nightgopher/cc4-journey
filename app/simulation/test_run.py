import datetime as dt
import json
import unittest

from run import REPORT_KEY, STATE_KEY, ScenarioFailure, Simulation, cents


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


if __name__ == "__main__":
    unittest.main()
