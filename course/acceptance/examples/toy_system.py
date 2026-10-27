"""A deliberately different implementation: in-memory, single file, no database.

It plays the part of "another student's system" so the runner can prove that
an unrelated implementation passes, and that plausible-but-wrong variants
(TOY_MUTANT / mutant=...) fail with the right rule id. It speaks the contract
directly, so system and adapter are the same thing here.
"""
import argparse
import calendar as cal
import datetime as dt
from decimal import Decimal
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import itertools
import json
import os
import threading

MUTANTS = {
    "commission_linear": "提成按每单10元线性计算",
    "pending_counts": "待审订单也计入业绩",
    "c_monthly_reset": "C级3天额度每月重置",
    "b_monthly_reset": "B级6天额度每月重置",
    "history_current_level": "历史月份按当前等级计算",
    "leave_overridden": "请假日录单后改回出勤",
    "overpay_allowed": "允许超收",
    "salary_double_expense": "重复发放工资重复计流出",
    "zero_entry_absent": "录0单不算出勤",
}


class Rejected(Exception):
    pass


def commission(n, mutant):
    if mutant == "commission_linear":
        return n * 10
    if n <= 0:
        return 0
    if n <= 3:
        return n * 10
    if n <= 5:
        return 30 + (n - 3) * 20
    return 70 + (n - 5) * 30


class Toy:
    def __init__(self, mutant=""):
        self.mutant = mutant
        self.lock = threading.Lock()
        self.ids = itertools.count(1)
        self.workdays = {}                   # "YYYY-MM" -> [day, ...]
        self.employees = {}                  # id -> {"history": [(date, level)]}
        self.confirmed = {}                  # (emp, date) -> orders; a key means performance was entered
        self.details = {}                    # id -> {"emp", "date", "status"}
        self.attendance = {}                 # (emp, date) -> (units, source)
        self.receivables = {}                # id -> {"amount", "received"}
        self.payments = []                   # (date, amount)
        self.expenses = []                   # (date, amount)
        self.salary_payments = {}            # (emp, month) -> (date, amount) | None
        self.salary_expense_rows = []        # mutant only: every payment appended

    # ---- helpers ----
    def level_on(self, emp, date):
        level = None
        for when, to in sorted(self.employees[emp]["history"]):
            if when <= date:
                level = to
        return level

    def orders(self, emp, date):
        approved = sum(1 for d in self.details.values() if d["emp"] == emp and d["date"] == date
                       and (d["status"] == "approved" or (self.mutant == "pending_counts" and d["status"] == "pending")))
        return self.confirmed.get((emp, date), 0) + approved

    def workday_dates(self, start, end):
        dates = []
        for month, days in self.workdays.items():
            for day in days:
                date = f"{month}-{day:02d}"
                if start <= date <= end:
                    dates.append(date)
        return sorted(dates)

    def allowance(self, emp, level, month, per_day, quota, reset_mutant):
        month_start = month + "-01"
        month_end = f"{month}-{cal.monthrange(int(month[:4]), int(month[5:]))[1]:02d}"
        starts = [when for when, to in self.employees[emp]["history"] if to == level and when <= month_end]
        if not starts:
            return 0
        days = self.workday_dates(max(starts), month_end)
        before = 0 if self.mutant == reset_mutant else sum(d < month_start for d in days)
        in_month = sum(d >= month_start for d in days)
        return max(0, min(in_month, quota - before)) * per_day

    def salary(self, emp, month):
        y, m = int(month[:4]), int(month[5:])
        month_end = f"{month}-{cal.monthrange(y, m)[1]:02d}"
        when = "9999-12-31" if self.mutant == "history_current_level" else month_end
        level = self.level_on(emp, when)
        dates = [f"{month}-{d:02d}" for d in range(1, cal.monthrange(y, m)[1] + 1)]
        comm = sum(commission(self.orders(emp, d), self.mutant) for d in dates)
        base = bonus = 0
        if level == "c_level":
            base = self.allowance(emp, "c_level", month, 30, 3, "c_monthly_reset")
        elif level == "b_level":
            base = self.allowance(emp, "b_level", month, 88, 6, "b_monthly_reset")
        elif level == "a_level":
            workdays = [f"{month}-{d:02d}" for d in self.workdays.get(month, [])]
            units = Decimal(0)
            for d in workdays:
                record = self.attendance.get((emp, d))
                if record is not None:
                    units += record[0]
                elif ((emp, d) in self.confirmed and self.mutant != "zero_entry_absent") or self.orders(emp, d) > 0:
                    # Entering performance counts as attendance, even with 0 orders.
                    units += 1
            base = int(Decimal(2200) * units / len(workdays)) if workdays else 0
            total_orders = sum(self.orders(emp, d) for d in dates)
            bonus = (400 if units >= 25 else 0) + (1000 if total_orders >= 125 else 600 if total_orders >= 100 else 300 if total_orders >= 75 else 0)
        elif level is None:
            comm = 0
        return {"base": base, "bonus": bonus, "commission": comm, "total": base + bonus + comm}

    def ledger(self, start, end):
        inflow = Decimal(0)
        for (emp, date) in {(e, d) for (e, d) in self.confirmed} | {(d["emp"], d["date"]) for d in self.details.values()}:
            if start <= date <= end:
                inflow += self.orders(emp, date) * 99
        inflow += sum(Decimal(str(a)) for d, a in self.payments if start <= d <= end)
        outflow = sum(Decimal(str(a)) for d, a in self.expenses if start <= d <= end)
        rows = self.salary_expense_rows if self.mutant == "salary_double_expense" else [v for v in self.salary_payments.values() if v]
        outflow += sum(Decimal(str(a)) for d, a in rows if start <= d <= end)
        return {"inflow": float(inflow), "outflow": float(outflow)}

    # ---- contract ----
    def act(self, action, a):
        with self.lock:
            return getattr(self, "do_" + action)(a)

    def do_ping(self, a):
        return {"result": {"contract": "0.1", "system": "toy", "mutant": self.mutant}}

    def do_set_workdays(self, a):
        self.workdays[a["month"]] = list(a["days"])
        return {}

    def do_hire(self, a):
        emp = next(self.ids)
        self.employees[emp] = {"history": [(a["date"], a["level"])]}
        return {"id": emp}

    def do_promote(self, a):
        self.employees[a["emp"]]["history"].append((a["date"], a["level"]))
        return {}

    def do_record_orders(self, a):
        key = (a["emp"], a["date"])
        self.confirmed[key] = self.confirmed.get(key, 0) + a["count"]
        if self.mutant == "leave_overridden" and a["count"] > 0 and key in self.attendance:
            self.attendance[key] = (Decimal(1), "orders")
        return {}

    def do_submit_order(self, a):
        order = next(self.ids)
        self.details[order] = {"emp": a["emp"], "date": a["date"], "status": "pending"}
        return {"id": order}

    def do_approve_order(self, a):
        detail = self.details[a["order"]]
        if detail["status"] != "pending":
            raise Rejected()
        detail["status"] = "approved"
        return {}

    def do_reject_order(self, a):
        detail = self.details[a["order"]]
        if detail["status"] != "pending":
            raise Rejected()
        detail["status"] = "rejected"
        return {}

    def do_delete_order(self, a):
        self.details.pop(a["order"])
        return {}

    def do_record_attendance(self, a):
        units = {"full": Decimal(1), "half": Decimal("0.5"), "absent": Decimal(0)}[a["status"]]
        self.attendance[(a["emp"], a["date"])] = (units, "manual")
        return {}

    def do_approve_leave(self, a):
        self.attendance[(a["emp"], a["date"])] = (Decimal(0), "leave")
        return {}

    def do_create_receivable(self, a):
        rid = next(self.ids)
        self.receivables[rid] = {"amount": Decimal(str(a["amount"])), "received": Decimal(0)}
        return {"id": rid}

    def do_receive_payment(self, a):
        row = self.receivables[a["receivable"]]
        amount = Decimal(str(a["amount"]))
        if amount <= 0 or (amount > row["amount"] - row["received"] and self.mutant != "overpay_allowed"):
            raise Rejected()
        row["received"] += amount
        self.payments.append((a["date"], amount))
        return {}

    def do_pay_salary(self, a):
        key = (a["emp"], a["month"])
        amount = self.salary(a["emp"], a["month"])["total"]
        if not self.salary_payments.get(key):
            self.salary_payments[key] = (a["date"], amount)
        self.salary_expense_rows.append((a["date"], amount))
        return {}

    def do_revoke_salary(self, a):
        key = (a["emp"], a["month"])
        if self.salary_payments.get(key):
            paid = self.salary_payments[key]
            self.salary_expense_rows.append((paid[0], -paid[1]))
        self.salary_payments[key] = None
        return {}

    def do_record_expense(self, a):
        self.expenses.append((a["date"], a["amount"]))
        return {}

    def do_get_daily(self, a):
        n = self.orders(a["emp"], a["date"])
        return {"result": {"orders": n, "sales": n * 99, "commission": commission(n, self.mutant)}}

    def do_get_salary(self, a):
        return {"result": self.salary(a["emp"], a["month"])}

    def do_get_receivable(self, a):
        row = self.receivables[a["receivable"]]
        return {"result": {"remaining": float(row["amount"] - row["received"])}}

    def do_get_ledger(self, a):
        return {"result": self.ledger(a["from"], a["to"])}


def serve(port, mutant=""):
    toy = Toy(mutant)

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", 0))))
            try:
                if not hasattr(toy, "do_" + body["action"]):
                    reply = {"ok": False, "error": "UNSUPPORTED"}
                else:
                    reply = dict({"ok": True}, **toy.act(body["action"], body.get("args", {})))
            except Rejected:
                reply = {"ok": False, "error": "REJECTED"}
            data = json.dumps(reply, ensure_ascii=False).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", port), Handler)
    return server


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--port", type=int, default=9000)
    parser.add_argument("--mutant", default=os.environ.get("TOY_MUTANT", ""), choices=[""] + list(MUTANTS))
    args = parser.parse_args()
    print(f"toy system on http://127.0.0.1:{args.port}/acceptance/act mutant={args.mutant or '-'}", flush=True)
    serve(args.port, args.mutant).serve_forever()
