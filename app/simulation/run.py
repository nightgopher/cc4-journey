"""Continuous demo operations through the same HTTP API used by the frontend.

No SQL writes and no automatic POST retries. A durable pending-event checkpoint
stops an uncertain write from being replayed. Credentials never enter the report.
"""
import argparse
import base64
import calendar
import datetime as dt
from decimal import Decimal, ROUND_FLOOR
import hashlib
import hmac
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid
from zoneinfo import ZoneInfo

STATE_KEY = "business_simulation_state_v1"
REPORT_KEY = "business_simulation_report_v1"
STATE_VERSION = 2
BUSINESS_ZONE = ZoneInfo("Asia/Shanghai")
PNG = base64.b64decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jR9sAAAAASUVORK5CYII=")


class ScenarioFailure(Exception):
    pass


class APIError(ScenarioFailure):
    def __init__(self, status, message):
        self.status = status
        super().__init__(message)


def cents(value):
    return int(Decimal(str(value or 0)) * 100)


class API:
    def __init__(self, base, token=""):
        self.base, self.token = base.rstrip("/"), token
        self.before_write = None
        self.simulation_owner = ""

    def request(self, method, path, body=None, expected=200, raw=None, content_type=None):
        if method != "GET" and path != "/simulation/lease" and self.before_write:
            self.before_write()
        headers = {"Content-Type": content_type or "application/json"}
        if self.token:
            headers["Authorization"] = "Bearer " + self.token
        if self.simulation_owner:
            headers["X-CC4-Simulation-Owner"] = self.simulation_owner
        data = raw if raw is not None else (json.dumps(body).encode() if body is not None else None)
        request = urllib.request.Request(self.base + path, data=data, headers=headers, method=method)
        try:
            with urllib.request.urlopen(request, timeout=45) as response:
                status, payload = response.status, response.read()
        except urllib.error.HTTPError as error:
            status, payload = error.code, error.read()
        result = json.loads(payload)
        if status != expected:
            # Do not include request bodies, auth headers, or login responses.
            raise APIError(status, f"{method} {path.split('?')[0]}: HTTP {status}, expected {expected}; {result.get('error', 'unexpected response')}")
        return result

    def get(self, path):
        return self.request("GET", path)

    def post(self, path, body, expected=200):
        return self.request("POST", path, body, expected)

    def rows(self, path, key):
        rows, page = [], 1
        while True:
            result = self.get(path + ("&" if "?" in path else "?") + f"limit=100&page={page}")
            rows.extend(result[key])
            pagination = result.get("pagination")
            if not pagination or page >= pagination.get("totalPages", 1):
                return rows
            page += 1


class Simulation:
    def __init__(self, api, seed, today, days=7, lease_owner=""):
        if len(seed) < 32:
            raise ValueError("Simulation seed must have at least 32 characters")
        self.api, self.seed, self.today = api, seed.encode(), today
        self.lease_owner = lease_owner
        if lease_owner:
            self.api.before_write = self.touch_lease
            self.api.simulation_owner = lease_owner
        settings = {row["key"]: row["value"] for row in api.get("/settings?includeSimulation=true")["settings"]}
        self.state = json.loads(settings[STATE_KEY]) if STATE_KEY in settings else {
            "version": STATE_VERSION, "origin": (today - dt.timedelta(days=days - 1)).isoformat(),
            "lastDay": None, "actors": {}, "hires": [], "daily": {}, "events": [], "checks": [], "pending": None,
            "receivables": {}, "expenses": {}, "settlements": {}, "contacts": {}, "approvedOrders": {}, "completedEvents": {},
        }
        self.tokens = {}
        self.report = {"version": STATE_VERSION, "status": "running", "origin": self.state["origin"], "through": self.state["lastDay"],
                       "businessTimezone": "Asia/Shanghai", "events": self.state["events"], "checks": self.state["checks"], "error": None}

    def setting(self, key, value):
        self.api.post("/settings", {"key": key, "value": json.dumps(value, ensure_ascii=False), "category": "simulation", "is_public": False})

    def touch_lease(self):
        self.api.post("/simulation/lease", {"owner": self.lease_owner, "action": "renew"})

    def save(self):
        self.setting(STATE_KEY, self.state)
        self.report["through"] = self.state["lastDay"]
        self.report["updatedAt"] = dt.datetime.now(dt.timezone.utc).isoformat()
        self.setting(REPORT_KEY, self.report)

    def check(self, title, expected, actual, day=None):
        scope = (self.state.get("pending") or {}).get("title", getattr(self, "check_scope", "账务复核"))
        entry = {"title": title, "scope": scope, "date": day or self.today.isoformat(), "expected": expected, "actual": actual, "passed": expected == actual}
        # Rechecking an existing invariant updates its result; it does not grow
        # the persisted report quadratically on every daily/repeated run.
        for index, old in enumerate(self.state["checks"]):
            if (old["date"], old["title"], old.get("scope")) == (entry["date"], entry["title"], scope):
                self.state["checks"][index] = entry
                break
        else:
            self.state["checks"].append(entry)
        if not entry["passed"]:
            raise ScenarioFailure(f"{title}: expected {expected}, actual {actual}")

    def money_check(self, title, expected_cents, actual, day=None):
        displayed = "未返回金额" if actual is None else f"{Decimal(str(actual)):.2f}"
        self.check(title + "（元）", f"{Decimal(expected_cents)/100:.2f}", displayed, day)

    def event(self, day, title, run):
        event_key = hashlib.sha256((day + "\n" + title).encode()).hexdigest()
        if self.state.setdefault("completedEvents", {}).get(event_key):
            return None
        if self.state["pending"]:
            raise ScenarioFailure("Previous write needs reconciliation: " + self.state["pending"]["title"])
        self.state["pending"] = {"date": day, "title": title}
        self.save()
        result = run()
        self.state["events"].append({"date": day, "title": title, "status": "passed"})
        self.state["completedEvents"][event_key] = True
        self.state["pending"] = None
        self.save()
        return result

    def password(self, key):
        return hmac.new(self.seed, (self.state["origin"] + ":" + key).encode(), hashlib.sha256).hexdigest()[:40]

    def phone(self, key):
        digest = hashlib.sha256((self.state["origin"] + ":" + key).encode()).hexdigest()
        return "000" + str(int(digest[:12], 16) % 100000000).zfill(8)

    def actor(self, key):
        if key not in self.tokens:
            account = self.state["actors"][key]
            result = self.api.post("/auth/login", {"phone": account["phone"], "password": self.password(key)})
            self.tokens[key] = API(self.api.base, result["session"]["access_token"])
            if self.lease_owner:
                self.tokens[key].before_write = self.touch_lease
                self.tokens[key].simulation_owner = self.lease_owner
        return self.tokens[key]

    def hire(self, key, name, level, day, role="employee"):
        def create():
            body = {"name": "模拟·" + name, "phone": self.phone(key), "password": self.password(key),
                    "employeeCode": "SIM" + self.state["origin"].replace("-", "") + hashlib.sha256(key.encode()).hexdigest()[:8],
                    "level": level, "joinDate": day, "role": role, "position": "模拟" + role,
                    "department": "模拟业务演练", "teamId": self.state.get("team")}
            result = self.api.post("/employees", body, 201)
            employee = result["employee"]
            self.state["actors"][key] = {"id": result["id"], "phone": body["phone"], "userId": employee["user_id"], "name": body["name"], "level": level}
            self.check("入职员工状态", "active", employee["status"], day)
            self.check("入职等级", level, employee["level"], day)
            self.check("入职账号可以登录", True, bool(self.actor(key).get("/auth/session")["user"]), day)
        self.event(day, "入职：模拟·" + name, create)

    def setup(self):
        origin = self.state["origin"]
        session = self.api.get("/auth/session")
        self.state["adminEmployee"] = session["user"]["employee_id"]
        def product():
            self.state["product"] = self.api.post("/products", {
                "name": "模拟·99元体验课", "code": "SIM_" + origin.replace("-", ""), "price": 99,
                "commissionRules": {"tiers": [{"from": 1, "to": 3, "amount": 10}, {"from": 4, "to": 5, "amount": 20}, {"from": 6, "to": None, "amount": 30}]},
            }, 201)["id"]
        if not self.state.get("product"):
            self.event(origin, "配置模拟产品：99元体验课", product)
        def team():
            self.state["team"] = self.api.post("/teams", {"name": "模拟·经营演练团队", "code": "SIM_" + origin.replace("-", ""), "managerId": self.state["adminEmployee"]}, 201)["id"]
        if not self.state.get("team"):
            self.event(origin, "建立模拟团队", team)
        for key, name, level, role in [("senior", "资深销售", "a_level", "employee"), ("growth", "成长销售", "c_level", "employee"),
                                        ("settlement", "离职结算员工", "trainee", "employee"), ("manager", "演练主管", "trainee", "manager"), ("hr", "演练人事", "trainee", "hr")]:
            if key not in self.state["actors"]:
                self.hire(key, name, level, origin, role)

    def configure_workdays(self, day):
        date = dt.date.fromisoformat(day)
        period = f"{date.year:04}-{date.month:02}"
        if period in self.state.get("workdays", {}):
            return
        try:
            result = self.api.get(f"/workdays/{date.year}/{date.month}")
            missing = False
        except APIError as error:
            if error.status != 404:
                raise
            result = {}
            missing = True
        days = result.get("workdays", [])
        if missing:
            days = [n for n in range(1, calendar.monthrange(date.year, date.month)[1] + 1) if dt.date(date.year, date.month, n).weekday() < 5]
            self.event(day, "配置模拟经营工作日", lambda: self.api.post("/workdays", {"year": date.year, "month": date.month, "workdays": days}))
        self.state.setdefault("workdays", {})[period] = days
        self.save()

    def record(self, key, day, orders, commission):
        employee = self.state["actors"][key]["id"]
        def write():
            result = self.api.post("/daily-performance", {"employeeId": employee, "workDate": day, "productId": self.state["product"], "orders": orders}, 201)
            rows = self.api.rows(f"/daily-performance?employee_id={employee}&start_date={day}&end_date={day}", "performances")
            row = next(r for r in rows if r["work_date"] == day and r.get("product_id") == self.state["product"])
            self.check("当日订单汇总", orders, row["orders_count"], day)
            self.money_check("99元产品销售额", orders * 9900, row["sales_amount"], day)
            self.money_check("独立金标准：日提成", commission * 100, row["commission_amount"], day)
            self.state["daily"].setdefault(day, {})[key] = {"orders": orders, "commission": commission, "attendance": 1, "id": row["id"]}
            return result
        self.event(day, f"录单：{self.state['actors'][key]['name']} {orders}单", write)

    def promotion(self, day, before, after):
        actor = self.state["actors"]["growth"]
        def run():
            manager_id = self.state["actors"]["manager"]["id"]
            hr_id = self.state["actors"]["hr"]["id"]
            request_id = self.api.post("/approvals", {"requestType": "promotion", "title": "模拟·职级晋升", "requestedBy": actor["id"], "currentApprover": manager_id,
                "metadata": {"from_level": before, "to_level": after, "condition_met_date": day, "reason": "模拟经营：达到演练晋升条件"}}, 201)["id"]
            self.actor("manager").post(f"/approvals/{request_id}/action", {"action": "forwarded", "nextApproverId": hr_id, "comments": "模拟·主管核对业绩后转交"})
            self.check("主管转交时尚未变更职级", before, self.api.get(f"/employees/{actor['id']}")["level"], day)
            self.actor("hr").post(f"/approvals/{request_id}/action", {"action": "approved", "comments": "模拟·人事批准"})
            self.check("批准后的员工职级", after, self.api.get(f"/employees/{actor['id']}")["level"], day)
            history = self.api.get(f"/employees/{actor['id']}/level-history")["history"]
            self.check("晋升历史已保存", True, any(row["from_level"] == before and row["to_level"] == after for row in history), day)
            self.actor("hr").post(f"/approvals/{request_id}/action", {"action": "approved"}, 403)
            self.check("重复审批被拒绝", after, self.api.get(f"/employees/{actor['id']}")["level"], day)
            actor["level"] = after
        self.event(day, f"职级流转：C→B" if after == "b_level" else "职级流转：B→A", run)

    def leave(self, key, day):
        actor = self.state["actors"][key]
        def run():
            old_token = self.actor(key)
            before = self.api.get(f"/employees/{actor['id']}/performance-stats")["total_orders"]
            self.api.request("PUT", "/employees/batch-status", {"employeeIds": [actor["id"]], "status": "inactive"})
            self.api.post("/auth/login", {"phone": actor["phone"], "password": self.password(key)}, 401)
            old_token.request("GET", "/auth/session", expected=401)
            self.check("离职后历史业绩保留", before, self.api.get(f"/employees/{actor['id']}/performance-stats")["total_orders"], day)
            self.check("离职状态", "inactive", self.api.get(f"/employees/{actor['id']}")["status"], day)
        self.event(day, "离职：" + actor["name"], run)

    def upstream(self, day):
        def run():
            result = self.api.post("/wechat-leads/sync", {"startTime": day + " 00:00:00", "endTime": day + " 23:59:59"})["data"]
            self.check("外呼来源为模拟服务", True, result["mock"], day)
            self.check("上游当天通话数", 120, result["processedCount"], day)
            self.check("筛选后微信线索数（新增+重复）", 30, result["savedCount"] + result["duplicateCount"], day)
            rows = self.api.rows(f"/wechat-leads?startDate={day}&endDate={day}", "leads")
            mock = [row for row in rows if row["data_source"] == "mock"]
            self.check("模拟线索已落库", 30, len(mock), day)
            for row in mock:
                if row.get("assigned_to") is None:
                    self.api.request("PUT", f"/wechat-leads/{row['id']}/assign", {"employeeId": self.state["actors"]["growth"]["id"]})
            available = [row for row in mock if row.get("assigned_to") in (None, self.state["actors"]["growth"]["id"])]
            if not available:
                raise ScenarioFailure("当日模拟线索已分配给其他员工，没有演练销售可跟进的线索；未覆盖已有分配")
            if available:
                first = available[0]
                self.actor("growth").post(f"/wechat-leads/{first['id']}/follow-up", {"content": "模拟·已联系家长，介绍99元体验课"})
                self.actor("growth").request("PUT", f"/wechat-leads/{first['id']}", {"status": "contacted"})
                self.state.setdefault("contacts", {})[day] = {"id": first["id"], "phone": first["real_phone"], "name": first["customer_name"]}
        self.event(day, "外呼：同步、分配微信线索并由员工跟进", run)

    def self_orders(self, day):
        actor_id = self.state["actors"]["growth"]["id"]
        expected = self.state["daily"][day]["growth"]
        def income_check(stage, orders):
            rows = self.api.rows("/finance/income", "income")
            own = [row for row in rows if row.get("recorded_by") == actor_id and str(row.get("transaction_date", ""))[:10] == day]
            self.check(stage + "：收入流水数", 1, len(own), day)
            self.money_check(stage + "：已确认收入", orders * 9900, sum(Decimal(str(row["amount"])) for row in own), day)
        def current():
            rows = self.api.rows(f"/daily-performance?employee_id={actor_id}&start_date={day}&end_date={day}", "performances")
            return next(row for row in rows if row["work_date"] == day and row.get("product_id") == self.state["product"])
        def submit(index):
            boundary = "cc4simulation" + day.replace("-", "") + str(index)
            chunks = []
            contact_phone = self.state["contacts"][day]["phone"]
            fields = {"employeeId": actor_id, "workDate": day, "productId": self.state["product"], "orders": 1,
                      "parentPhone": contact_phone if index in (0,3) else self.phone("self_" + day + str(index)), "orderId": "SIM_" + day + "_" + str(index), "notes": "模拟业务审核样例"}
            for key, value in fields.items():
                chunks.append(f'--{boundary}\r\nContent-Disposition: form-data; name="{key}"\r\n\r\n{value}\r\n'.encode())
            for key in ("wechatPayScreenshot", "wechatChatScreenshot", "wechatContactScreenshot"):
                chunks.append(f'--{boundary}\r\nContent-Disposition: form-data; name="{key}"; filename="simulation.png"\r\nContent-Type: image/png\r\n\r\n'.encode() + PNG + b"\r\n")
            chunks.append(f"--{boundary}--\r\n".encode())
            return self.actor("growth").request("POST", "/daily-performance", expected=201, raw=b"".join(chunks), content_type="multipart/form-data; boundary=" + boundary)["orderDetail"]["id"]
        def run():
            approved, rejected, pending = [submit(index) for index in range(3)]
            self.check("待审订单不计入业绩", expected["orders"], current()["orders_count"], day)
            income_check("三笔待审", expected["orders"])
            self.api.post(f"/daily-performance/order-details/{approved}/approve", {"approved": True})
            income_check("一笔审核通过", expected["orders"] + 1)
            self.api.post(f"/daily-performance/order-details/{approved}/approve", {"approved": True}, 400)
            income_check("重复审核被拒绝", expected["orders"] + 1)
            self.api.post(f"/daily-performance/order-details/{rejected}/approve", {"approved": False, "rejectionReason": "模拟·截图资料不完整"})
            income_check("一笔驳回", expected["orders"] + 1)
            self.api.request("DELETE", f"/daily-performance/order-details/{approved}")
            self.check("撤销订单后业绩回退", expected["orders"], current()["orders_count"], day)
            income_check("撤销已审核订单", expected["orders"])
            replacement = submit(3)
            self.api.post(f"/daily-performance/order-details/{replacement}/approve", {"approved": True})
            income_check("新订单重新审核", expected["orders"] + 1)
            self.state.setdefault("approvedOrders", {})[day] = replacement
            row = current()
            self.check("一笔通过、一笔驳回后只增加1单", expected["orders"] + 1, row["orders_count"], day)
            commission = {1: 20, 5: 100}[expected["orders"]]
            self.money_check("审核后的提成档位", commission * 100, row["commission_amount"], day)
            expected["orders"] += 1
            expected["commission"] = commission
            self.state.setdefault("pendingOrders", []).append(pending)
        self.event(day, "员工自录：三张截图，审核通过、驳回和待处理", run)

    def attendance(self, day, half_day):
        actor_id = self.state["actors"]["senior"]["id"]
        if not half_day:
            return
        def run():
            self.api.post("/attendance", {"employeeId": actor_id, "date": day, "status": "partial", "notes": "模拟·半天出勤"}, 201)
            rows = self.api.rows(f"/attendance?employee_id={actor_id}&start_date={day}&end_date={day}", "attendance")
            row = next(row for row in rows if row["date"] == day)
            self.check("半天出勤折算", 0.5, row["attendance_days"], day)
            self.state["daily"][day]["senior"]["attendance"] = 0.5
        self.event(day, "考勤：模拟半天出勤", run)

    def approved_leave(self, key, day):
        actor_id = self.state["actors"][key]["id"]
        def run():
            request_id = self.actor(key).post("/approvals", {"requestType": "leave", "title": "模拟·请假优先级核对", "currentApprover": self.state["adminEmployee"],
                "metadata": {"start_date": day, "end_date": day, "leave_type": "personal"}}, 201)["id"]
            self.api.post(f"/approvals/{request_id}/action", {"action": "approved", "comments": "模拟·批准请假，考勤优先于业绩兜底"})
            self.actor(key).post("/daily-performance", {"employeeId": actor_id, "workDate": day, "orders": 0}, 201)
            rows = self.api.rows(f"/attendance?employee_id={actor_id}&start_date={day}&end_date={day}", "attendance")
            row = next(row for row in rows if row["date"] == day)
            self.check("请假批准后保持缺勤", "absent", row["status"], day)
            self.check("后续业绩录入不得覆盖请假", 0, row["attendance_days"], day)
            self.state["daily"][day][key]["attendance"] = 0
        self.event(day, "请假：批准后考勤优先于业绩自动同步", run)

    def wages(self, day):
        date = dt.date.fromisoformat(day)
        def run():
            for key in ("senior", "growth"):
                actor_id = self.state["actors"][key]["id"]
                result = self.api.post("/salary/calculate", {"employeeId": actor_id, "periodYear": date.year, "periodMonth": date.month})
                records = {d: values[key] for d, values in self.state["daily"].items() if d.startswith(day[:7]) and key in values}
                # Independent expected totals come from the scenario plan's fixed
                # daily examples, never by calling the production calculator.
                expected_commission = sum(row["commission"] for row in records.values())
                self.money_check(self.state["actors"][key]["name"] + "：工资提成等于逐日金标准合计", expected_commission * 100, result["commissionAmount"], day)
                if key == "senior":
                    workdays = self.state["workdays"][day[:7]]
                    units = sum(Decimal(str(row["attendance"])) for d, row in records.items() if int(d[-2:]) in workdays)
                    base = int((Decimal(2200) * units / len(workdays)).to_integral_value(rounding=ROUND_FLOOR)) if workdays else 0
                    orders = sum(row["orders"] for row in records.values())
                    bonus = (400 if units >= 25 else 0) + (1000 if orders >= 125 else 600 if orders >= 100 else 300 if orders >= 75 else 0)
                    self.money_check("A级底薪按有效出勤折算", base * 100, result["baseSalary"], day)
                    self.money_check("A级工资总额", (base + expected_commission + bonus) * 100, result["netSalary"], day)
        self.event(day, "工资：核算当月工资与逐日提成", run)

    def receivable(self, day):
        def run():
            row_id = self.api.post("/finance/receivable", {"customer": "模拟·分期回款家长 " + day, "amount": 100, "dueDate": day, "note": "模拟经营回款核对"}, 201)["id"]
            path = f"/finance/receivable/{row_id}/payments"
            self.api.post(path, {"amount": 30, "paymentDate": day, "paymentMethod": "bank", "note": "模拟首款"}, 201)
            row = self.api.get(f"/finance/receivable/{row_id}")["receivable"]
            self.money_check("首款后应收余额", 7000, row["remaining_amount"], day)
            self.api.post(path, {"amount": 71, "paymentDate": day}, 400)
            self.money_check("超收拒绝后余额不变", 7000, self.api.get(f"/finance/receivable/{row_id}")["receivable"]["remaining_amount"], day)
            self.api.post(path, {"amount": 70, "paymentDate": day, "paymentMethod": "bank", "note": "模拟尾款"}, 201)
            row = self.api.get(f"/finance/receivable/{row_id}")["receivable"]
            self.money_check("结清后的应收余额", 0, row["remaining_amount"], day)
            payments = self.api.get(path)["payments"]
            self.check("逐笔回款金额", [3000, 7000], sorted(cents(p["amount"]) for p in payments), day)
            income = self.api.rows("/finance/income", "income")
            refs = {"payment-" + str(p["id"]) for p in payments}
            self.money_check("逐笔回款对应收入", 10000, sum(Decimal(str(r["amount"])) for r in income if r.get("reference_id") in refs), day)
            self.state.setdefault("receivables", {})[day] = {"id": row_id, "paymentIds": [p["id"] for p in payments], "paymentAmounts": {str(p["id"]): cents(p["amount"]) for p in payments}}
        self.event(day, "财务：100元应收，30元首款与70元尾款", run)

    def settlement(self, day):
        actor_id = self.state["actors"]["settlement"]["id"]
        date = dt.date.fromisoformat(day)
        body = {"employeeId": actor_id, "periodYear": date.year, "periodMonth": date.month}
        def run():
            result = self.api.post("/salary/calculate", body)
            self.money_check("实习生5单离职工资", 7000, result["netSalary"], day)
            rows = self.api.rows(f"/salary?employee_id={actor_id}&year={date.year}&month={date.month}", "salaries")
            salary_id = next(row["id"] for row in rows if row["employee_id"] == actor_id)
            path = f"/salary/{salary_id}/payment-status"
            def ledger():
                salary_rows = self.api.rows(f"/salary?year={date.year}&month={date.month}", "salaries")
                paid = sum(cents(row["net_salary"]) for row in salary_rows if row["payment_status"] == "paid")
                expense = self.api.rows("/finance/expense", "expense")
                matching = [row for row in expense if row.get("reference_type") == "salary" and row.get("reference_id") == f"salary-{date.year}-{date.month}"]
                self.check("整月工资流水唯一", 1 if paid > 0 else 0, len(matching), day)
                self.money_check("工资支出等于已发工资合计", paid, sum(Decimal(str(row["amount"])) for row in matching), day)
            self.api.request("PUT", path, {"paymentStatus": "paid", "paymentDate": day})
            ledger()
            self.api.request("PUT", path, {"paymentStatus": "paid", "paymentDate": day})
            ledger()
            self.api.post("/salary/calculate", body, 400)
            self.api.request("PUT", path, {"paymentStatus": "pending"})
            ledger()
            self.api.request("PUT", path, {"paymentStatus": "paid", "paymentDate": day})
            ledger()
            self.state.setdefault("settlements", {})[day] = {"id": salary_id, "year": date.year, "month": date.month, "amount": 70}
        self.event(day, "工资：70元离职结算、重复发放、撤销和再发放", run)

    def validate_state(self):
        required = ("receivables", "expenses", "settlements", "contacts", "approvedOrders", "completedEvents")
        if self.state.get("version") != STATE_VERSION or any(key not in self.state for key in required):
            raise ScenarioFailure("旧版演练状态缺少完整业务索引，请先备份核查并迁移，未自动重放业务")
        through = self.state.get("lastDay")
        if through:
            for date in self.state["daily"]:
                if date <= through and any(date not in self.state[key] for key in ("receivables", "expenses", "contacts", "approvedOrders")):
                    raise ScenarioFailure("已完成日期缺少业务索引：" + date)
            if not self.state["settlements"]:
                raise ScenarioFailure("已完成演练缺少工资结算索引")

    def reconcile(self, day):
        self.validate_state()
        income = self.api.rows("/finance/income", "income")
        expected_sales = actual_sales = 0
        for date, actors in self.state["daily"].items():
            for key, expected in actors.items():
                self.check_scope = "账务复核：" + self.state["actors"][key]["name"]
                actor_id = self.state["actors"][key]["id"]
                rows = self.api.rows(f"/daily-performance?employee_id={actor_id}&start_date={date}&end_date={date}", "performances")
                own = [row for row in rows if row["work_date"] == date and row.get("product_id") == self.state["product"]]
                self.check("录单汇总记录唯一", 1, len(own), date)
                self.check("回读订单数", expected["orders"], own[0]["orders_count"], date)
                self.money_check("回读业绩销售额", expected["orders"] * 9900, own[0]["sales_amount"], date)
                self.money_check("回读业绩提成", expected["commission"] * 100, own[0]["commission_amount"], date)
                ref = f"daily-{actor_id}-{date}-{self.state['product']}"
                ledger = [row for row in income if row.get("reference_type") == "daily_performance" and row.get("reference_id") == ref]
                self.check("销售收入流水唯一", 1, len(ledger), date)
                self.money_check("已确认订单与收入一致", expected["orders"] * 9900, sum(Decimal(str(row["amount"])) for row in ledger), date)
                expected_sales += expected["orders"] * 9900
                actual_sales += sum(cents(row["amount"]) for row in ledger)
        actual_receipts = 0
        for date, expected in self.state.get("receivables", {}).items():
            self.check_scope = "账务复核：应收与回款"
            row = self.api.get(f"/finance/receivable/{expected['id']}")["receivable"]
            self.money_check("回读应收总额", 10000, row["amount"], date)
            self.money_check("回读累计收款", 10000, row["received_amount"], date)
            self.money_check("回读应收余额", 0, row["remaining_amount"], date)
            payments = self.api.get(f"/finance/receivable/{expected['id']}/payments")["payments"]
            self.check("回款记录保持原两笔", sorted(expected["paymentIds"]), sorted(row["id"] for row in payments), date)
            self.check("回款分笔金额（分）", [3000, 7000], sorted(cents(row["amount"]) for row in payments), date)
            for payment in payments:
                planned = expected["paymentAmounts"][str(payment["id"])]
                self.money_check("回款记录" + str(payment["id"]) + "金额", planned, payment["amount"], date)
                corresponding = [row for row in income if row.get("reference_type") == "receivable_payment" and row.get("reference_id") == "payment-" + str(payment["id"])]
                self.check("回款记录" + str(payment["id"]) + "收入唯一", 1, len(corresponding), date)
                self.money_check("逐笔回款收入" + str(payment["id"]), planned, corresponding[0]["amount"], date)
            refs = {"payment-" + str(pid) for pid in expected["paymentIds"]}
            ledger = [row for row in income if row.get("reference_type") == "receivable_payment" and row.get("reference_id") in refs]
            self.check("回款收入保持两笔", 2, len(ledger), date)
            amount = sum(cents(row["amount"]) for row in ledger)
            self.money_check("回读回款入账合计", 10000, Decimal(amount)/100, date)
            actual_receipts += amount
        expense = self.api.rows("/finance/expense", "expense")
        actual_costs = 0
        for date, expected in self.state.get("expenses", {}).items():
            self.check_scope = "账务复核：运营费用"
            own = [row for row in expense if row["id"] == expected["id"]]
            self.check("费用记录存在且唯一", 1, len(own), date)
            self.check("运营费用仍为已确认", "confirmed", own[0]["status"], date)
            self.money_check("回读运营费用", 6550, own[0]["amount"], date)
            actual_costs += cents(own[0]["amount"])
        actual_wages = 0
        for date, expected in self.state.get("settlements", {}).items():
            self.check_scope = "账务复核：已发工资"
            salaries = self.api.rows(f"/salary?year={expected['year']}&month={expected['month']}", "salaries")
            own = [row for row in salaries if row["id"] == expected["id"]]
            self.check("工资结算记录存在", 1, len(own), date)
            self.check("离职工资仍为已发放", "paid", own[0]["payment_status"], date)
            self.money_check("回读离职结算工资", 7000, own[0]["net_salary"], date)
            paid = sum(cents(row["net_salary"]) for row in salaries if row["payment_status"] == "paid")
            ledger = [row for row in expense if row.get("reference_type") == "salary" and row.get("reference_id") == f"salary-{expected['year']}-{expected['month']}"]
            self.check("回读整月工资流水唯一", 1, len(ledger), date)
            self.money_check("回读工资支出与已发工资一致", paid, ledger[0]["amount"], date)
            actual_wages += cents(own[0]["net_salary"])
        self.check_scope = "演练累计资金核对"
        expected_inflow = expected_sales + len(self.state.get("receivables", {})) * 10000
        expected_outflow = len(self.state.get("expenses", {})) * 6550 + len(self.state.get("settlements", {})) * 7000
        self.money_check("累计流入", expected_inflow, Decimal(actual_sales + actual_receipts)/100, day)
        self.money_check("累计流出", expected_outflow, Decimal(actual_costs + actual_wages)/100, day)
        self.money_check("净流入等于流入减流出", expected_inflow - expected_outflow, Decimal(actual_sales + actual_receipts - actual_costs - actual_wages)/100, day)
        self.report["money"] = {"sales": actual_sales/100, "inflow": (actual_sales+actual_receipts)/100, "outflow": (actual_costs+actual_wages)/100, "net": (actual_sales+actual_receipts-actual_costs-actual_wages)/100}

    def run_day(self, date):
        day = date.isoformat()
        index = (date - dt.date.fromisoformat(self.state["origin"])).days
        self.configure_workdays(day)
        self.upstream(day)
        key = "hire_" + day.replace("-", "")
        self.hire(key, date.strftime("%m月%d日入职销售"), "c_level", day)
        if not any(hire["key"] == key for hire in self.state["hires"]):
            self.state["hires"].append({"key": key, "date": day})
        self.record(key, day, 1, 10)
        if index % 4 == 1:
            self.approved_leave(key, day)
        self.actor(key).request("GET", f"/employees/{self.state['actors']['senior']['id']}", expected=403)
        self.check("新员工不能读取他人员工档案", True, True, day)
        senior_orders, senior_commission = [(3, 30), (5, 70), (8, 160)][index % 3]
        self.record("senior", day, senior_orders, senior_commission)
        growth_orders, growth_commission = (1, 10) if index < 3 else (5, 70)
        if index == 3:
            self.promotion(day, "c_level", "b_level")
        if index == 6:
            self.promotion(day, "b_level", "a_level")
        self.record("growth", day, growth_orders, growth_commission)
        self.self_orders(day)
        self.attendance(day, index % 4 == 2)
        if index == 0:
            self.record("settlement", day, 5, 70)
            self.leave("settlement", day)
            self.settlement(day)
        if index >= 3:
            self.leave(self.state["hires"][index - 3]["key"], day)
        def customer():
            contact = self.state["contacts"][day]
            phone = contact["phone"]
            row_id = self.api.post("/customers", {"name": "模拟·体验课家长 " + day, "phone": phone, "status": "active", "customerType": "individual"}, 201)["id"]
            employee_id = self.state["actors"]["growth"]["id"]
            self.api.post(f"/customers/{row_id}/orders", {"totalAmount": 99, "orderDate": day, "status":"confirmed", "employeeId":employee_id}, 201)
            order = self.api.get(f"/customers/{row_id}/orders")["orders"][0]
            self.check("客户订单金额", 9900, cents(order["total_amount"]), day)
            self.check("客户订单归属销售", employee_id, order["employee_id"], day)
            stored_customers = self.api.rows("/customers?search=" + urllib.parse.quote(phone), "customers")
            stored = next(row for row in stored_customers if row["id"] == row_id)
            self.check("客户档案保存外呼线索手机号", phone, stored["phone"], day)
            self.check("客户订单关联已保存客户", row_id, order["customer_id"], day)
            details = self.api.rows(f"/daily-performance/order-details?employee_id={employee_id}","orderDetails")
            detail = next(row for row in details if row["id"] == self.state["approvedOrders"][day])
            self.check("外呼线索、审核订单与客户使用同一手机号",phone,detail["parent_phone"],day)
            lead_id = self.api.post("/leads", {"customerName":"模拟·体验课家长 " + day,"phone":phone,"assignedTo":employee_id,"notes":"模拟·外呼跟进后成交"},201)["id"]
            self.api.post(f"/leads/{lead_id}/mark-paid",{})
            if contact:
                self.actor("growth").request("PUT",f"/wechat-leads/{contact['id']}",{"is_converted":True})
                self.check("成交后微信线索转化状态",True,self.api.get(f"/wechat-leads/{contact['id']}")["data"]["is_converted"],day)
        self.event(day, "客户：建立体验课客户与订单档案", customer)
        self.receivable(day)
        def operating_cost():
            result = self.api.post("/finance/expense", {"category": "模拟运营工具", "amount": 65.5, "transactionDate": day, "description": "模拟经营工具费 " + day}, 201)
            self.state.setdefault("expenses", {})[day] = {"id": result["id"]}
        self.event(day, "财务：65.50元模拟运营支出", operating_cost)
        self.event(day, "汇总模拟团队当月业绩", lambda: self.actor("manager").post("/performance/aggregate-to-monthly", {"year": date.year, "month": date.month}))
        def sync_income():
            for _ in range(2):
                self.actor("manager").post("/finance/sync-income", {"year": date.year, "month": date.month})
            self.reconcile(day)
        self.event(day, "财务：重复同步模拟团队收入，核对没有重复入账", sync_income)
        self.wages(day)
        self.reconcile(day)
        self.state["lastDay"] = day
        self.save()

    def run(self):
        try:
            if self.state["pending"]:
                raise ScenarioFailure("Uncertain prior event requires operator reconciliation: " + self.state["pending"]["title"])
            self.validate_state()
            if not self.api.get("/simulation/report")["enabled"]:
                raise ScenarioFailure("Business simulation is not enabled on this server")
            if self.state["daily"]:
                self.reconcile(self.today.isoformat())
            self.save()
            self.setup()
            start = dt.date.fromisoformat(self.state["lastDay"]) + dt.timedelta(days=1) if self.state["lastDay"] else dt.date.fromisoformat(self.state["origin"])
            if (self.today - start).days > 30:
                raise ScenarioFailure("More than 31 dates pending; reconcile the long pause before continuing")
            date = start
            while date <= self.today:
                self.run_day(date)
                date += dt.timedelta(days=1)
            self.reconcile(self.today.isoformat())
            self.report["status"] = "passed"
            self.report["summary"] = {"days": len(self.state["daily"]), "employees": len(self.state["actors"]), "checks": len(self.state["checks"]), "events": len(self.state["events"])}
            self.save()
            return self.report
        except Exception as error:
            title = (self.state.get("pending") or {}).get("title", "经营核对")
            message = (title + ": " + str(error))[:1000]
            self.report["status"], self.report["error"] = "failed", message
            self.report.pop("money", None)
            self.save()
            raise ScenarioFailure(message) from error


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default=os.environ.get("CC4_SIMULATION_BASE_URL", "http://127.0.0.1:3101/api/v1"))
    parser.add_argument("--days", type=int, default=7)
    parser.add_argument("--date", default=dt.datetime.now(BUSINESS_ZONE).date().isoformat())
    args = parser.parse_args()
    if os.environ.get("CC4_SIMULATION_ALLOW_WRITE") != "true":
        raise SystemExit("Set CC4_SIMULATION_ALLOW_WRITE=true for this dedicated demo environment")
    if not 1 <= args.days <= 31:
        raise SystemExit("days must be between 1 and 31")
    api = API(args.base_url)
    result = api.post("/auth/login", {"phone": os.environ["CC4_SIMULATION_ADMIN_PHONE"], "password": os.environ["CC4_SIMULATION_ADMIN_PASSWORD"]})
    if result["user"]["role"] != "admin":
        raise SystemExit("An administrator is required for business simulation")
    api.token = result["session"]["access_token"]
    owner = uuid.uuid4().hex
    api.post("/simulation/lease", {"owner": owner, "action": "acquire"})
    try:
        report = Simulation(api, os.environ["CC4_SIMULATION_SEED"], dt.date.fromisoformat(args.date), args.days, owner).run()
    finally:
        api.post("/simulation/lease", {"owner": owner, "action": "release"})
    print(json.dumps({"status": report["status"], "through": report["through"], "summary": report["summary"]}, ensure_ascii=False))


if __name__ == "__main__":
    try:
        main()
    except (ScenarioFailure, ValueError, urllib.error.URLError) as error:
        raise SystemExit(str(error))
