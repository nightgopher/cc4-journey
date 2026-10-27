"""Reference adapter: acceptance contract -> the instructor's CC4 HTTP API.

Translation only. No business calculation and no business state live here:
every number returned is read back from CC4, and IDs are CC4's own.
The only cached value is the admin session token (re-login on 401).

Environment:
  CC4_BASE_URL         e.g. http://127.0.0.1:3101/api/v1
  CC4_ADMIN_PHONE      administrator login
  CC4_ADMIN_PASSWORD
  CC4_ADAPTER_SECRET   derives per-employee test passwords (any long random string)
"""
import argparse
import calendar
from decimal import Decimal
import hashlib
import hmac
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
import urllib.error
import urllib.request

BASE = os.environ.get("CC4_BASE_URL", "http://127.0.0.1:3101/api/v1").rstrip("/")
SECRET = os.environ.get("CC4_ADAPTER_SECRET", "cc4-reference-adapter-secret").encode()
PRODUCT_CODE = "ACCEPT_TRIAL"
PNG = bytes.fromhex("89504e470d0a1a0a0000000d4948445200000001000000010804000000b51c0c020000000b4944415478da63fcff1f0003030200efa347db0000000049454e44ae426082")
ATTENDANCE = {"full": "work", "half": "partial", "absent": "absent"}


class Rejected(Exception):
    pass


class Unsupported(Exception):
    pass


class HTTPFailure(Exception):
    def __init__(self, status, method, path, body):
        self.status = status
        super().__init__(f"{method} {path.split('?')[0]} -> HTTP {status}: {body.get('error', body) if isinstance(body, dict) else body}")


def money(value):
    return float(Decimal(str(value or 0)))


class CC4:
    token = None

    def request(self, method, path, body=None, token=None, raw=None, content_type="application/json"):
        headers = {"Content-Type": content_type}
        use = token or self.token
        if use:
            headers["Authorization"] = "Bearer " + use
        data = raw if raw is not None else (json.dumps(body).encode() if body is not None else None)
        req = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
        try:
            with urllib.request.urlopen(req, timeout=30) as response:
                return json.loads(response.read() or b"{}")
        except urllib.error.HTTPError as error:
            payload = error.read()
            try:
                parsed = json.loads(payload)
            except json.JSONDecodeError:
                parsed = payload.decode(errors="replace")
            if error.code == 401 and token is None and path != "/auth/login":
                self.login()
                return self.request(method, path, body, None, raw, content_type)
            raise HTTPFailure(error.code, method, path, parsed)

    def login(self):
        result = self.request("POST", "/auth/login", {"phone": os.environ["CC4_ADMIN_PHONE"], "password": os.environ["CC4_ADMIN_PASSWORD"]})
        CC4.token = result["session"]["access_token"]

    def rows(self, path, key):
        rows, page = [], 1
        while True:
            result = self.request("GET", path + ("&" if "?" in path else "?") + f"limit=100&page={page}")
            rows.extend(result[key])
            if page >= (result.get("pagination") or {}).get("totalPages", 1):
                return rows
            page += 1


api = CC4()


def employee_secret(name):
    digest = hashlib.sha256(name.encode()).hexdigest()
    phone = "001" + str(int(digest[:12], 16) % 100000000).zfill(8)
    return phone, hmac.new(SECRET, phone.encode(), hashlib.sha256).hexdigest()[:32]


def product_id():
    # Configuration seeding, looked up each time (find-or-create) so the adapter keeps no state.
    for row in api.request("GET", "/products").get("products", []):
        if row.get("code") == PRODUCT_CODE:
            return row["id"]
    tiers = [{"from": 1, "to": 3, "amount": 10}, {"from": 4, "to": 5, "amount": 20}, {"from": 6, "to": None, "amount": 30}]
    return api.request("POST", "/products", {"name": "验收·体验课", "code": PRODUCT_CODE, "price": 99, "commissionRules": {"tiers": tiers}})["id"]


def month_parts(month):
    year, number = int(month[:4]), int(month[5:])
    return year, number, f"{month}-01", f"{month}-{calendar.monthrange(year, number)[1]:02d}"


def salary_row(emp, month):
    year, number, _, _ = month_parts(month)
    rows = api.rows(f"/salary?employee_id={emp}&year={year}&month={number}", "salaries")
    return next((row for row in rows if row["employee_id"] == emp), None)


# ---- contract actions ----

def ping(a):
    return {"result": {"contract": "0.1", "system": "cc4-reference"}}


def set_workdays(a):
    year, number, _, _ = month_parts(a["month"])
    api.request("POST", "/workdays", {"year": year, "month": number, "workdays": a["days"]})
    return {}


def hire(a):
    phone, password = employee_secret(a["name"])
    body = {"name": a["name"], "phone": phone, "password": password, "employeeCode": "ACC" + phone[-8:], "level": a["level"],
            "joinDate": a["date"], "role": "employee", "position": "销售", "department": "验收"}
    return {"id": api.request("POST", "/employees", body)["id"]}


def promote(a):
    api.request("POST", f"/employees/{a['emp']}/level-change", {"toLevel": a["level"], "changeDate": a["date"], "reason": "验收剧本"})
    return {}


def record_orders(a):
    api.request("POST", "/daily-performance", {"employeeId": a["emp"], "workDate": a["date"], "productId": product_id(), "orders": a["count"]})
    return {}


def submit_order(a):
    employee = api.request("GET", f"/employees/{a['emp']}")
    phone = employee.get("phone") or (employee.get("user") or {}).get("phone")
    _, password = employee_secret(employee["name"])
    token = api.request("POST", "/auth/login", {"phone": phone, "password": password})["session"]["access_token"]
    boundary = "cc4acceptance" + hashlib.sha256(os.urandom(8)).hexdigest()[:12]
    fields = {"employeeId": a["emp"], "workDate": a["date"], "productId": product_id(), "orders": 1,
              "parentPhone": "002" + str(int.from_bytes(os.urandom(4), "big") % 100000000).zfill(8),
              "orderId": "ACC_" + hashlib.sha256(os.urandom(8)).hexdigest()[:12]}
    chunks = [f'--{boundary}\r\nContent-Disposition: form-data; name="{k}"\r\n\r\n{v}\r\n'.encode() for k, v in fields.items()]
    for key in ("wechatPayScreenshot", "wechatChatScreenshot", "wechatContactScreenshot"):
        chunks.append(f'--{boundary}\r\nContent-Disposition: form-data; name="{key}"; filename="a.png"\r\nContent-Type: image/png\r\n\r\n'.encode() + PNG + b"\r\n")
    chunks.append(f"--{boundary}--\r\n".encode())
    result = api.request("POST", "/daily-performance", token=token, raw=b"".join(chunks), content_type="multipart/form-data; boundary=" + boundary)
    return {"id": result["orderDetail"]["id"]}


def approve_order(a):
    return review(a, {"approved": True})


def reject_order(a):
    return review(a, {"approved": False, "rejectionReason": "验收剧本驳回"})


def review(a, body):
    try:
        api.request("POST", f"/daily-performance/order-details/{a['order']}/approve", body)
    except HTTPFailure as error:
        if error.status == 400:
            raise Rejected()
        raise
    return {}


def delete_order(a):
    api.request("DELETE", f"/daily-performance/order-details/{a['order']}")
    return {}


def record_attendance(a):
    api.request("POST", "/attendance", {"employeeId": a["emp"], "date": a["date"], "status": ATTENDANCE[a["status"]], "notes": "验收剧本"})
    return {}


def approve_leave(a):
    admin = api.request("GET", "/auth/session")["user"]["employee_id"]
    request_id = api.request("POST", "/approvals", {"requestType": "leave", "title": "验收·请假", "requestedBy": a["emp"], "currentApprover": admin,
                                                    "metadata": {"start_date": a["date"], "end_date": a["date"], "leave_type": "personal"}})["id"]
    api.request("POST", f"/approvals/{request_id}/action", {"action": "approved", "comments": "验收剧本"})
    return {}


def create_receivable(a):
    return {"id": api.request("POST", "/finance/receivable", {"customer": "验收·分期家长", "amount": a["amount"], "dueDate": a["date"]})["id"]}


def receive_payment(a):
    try:
        api.request("POST", f"/finance/receivable/{a['receivable']}/payments", {"amount": a["amount"], "paymentDate": a["date"], "paymentMethod": "bank"})
    except HTTPFailure as error:
        if error.status == 400:
            raise Rejected()
        raise
    return {}


def pay_salary(a):
    year, number, _, _ = month_parts(a["month"])
    row = salary_row(a["emp"], a["month"])
    if row is None or row.get("payment_status") != "paid":
        api.request("POST", "/salary/calculate", {"employeeId": a["emp"], "periodYear": year, "periodMonth": number})
        row = salary_row(a["emp"], a["month"])
    api.request("PUT", f"/salary/{row['id']}/payment-status", {"paymentStatus": "paid", "paymentDate": a["date"]})
    return {}


def revoke_salary(a):
    row = salary_row(a["emp"], a["month"])
    if row is not None:
        api.request("PUT", f"/salary/{row['id']}/payment-status", {"paymentStatus": "pending"})
    return {}


def record_expense(a):
    api.request("POST", "/finance/expense", {"category": "验收运营费用", "amount": a["amount"], "transactionDate": a["date"], "description": "验收剧本"})
    return {}


def get_daily(a):
    rows = api.rows(f"/daily-performance?employee_id={a['emp']}&start_date={a['date']}&end_date={a['date']}", "performances")
    rows = [row for row in rows if str(row["work_date"])[:10] == a["date"]]
    return {"result": {"orders": sum(int(row["orders_count"] or 0) for row in rows),
                       "sales": sum(money(row["sales_amount"]) for row in rows),
                       "commission": sum(money(row["commission_amount"]) for row in rows)}}


def get_salary(a):
    year, number, _, _ = month_parts(a["month"])
    row = salary_row(a["emp"], a["month"])
    if row is not None and row.get("payment_status") == "paid":
        # CC4 freezes paid salaries; read the stored record instead of recalculating.
        return {"result": {"base": money(row["base_salary"]), "bonus": money(row.get("bonus_amount")),
                           "commission": money(row["commission_amount"]), "total": money(row["net_salary"])}}
    result = api.request("POST", "/salary/calculate", {"employeeId": a["emp"], "periodYear": year, "periodMonth": number})
    return {"result": {"base": money(result["baseSalary"]), "bonus": money(result["bonusAmount"]),
                       "commission": money(result["commissionAmount"]), "total": money(result["netSalary"])}}


def get_receivable(a):
    return {"result": {"remaining": money(api.request("GET", f"/finance/receivable/{a['receivable']}")["receivable"]["remaining_amount"])}}


def get_ledger(a):
    window = f"start_date={a['from']}&end_date={a['to']}"
    income = api.rows("/finance/income?" + window, "income")
    expense = api.rows("/finance/expense?" + window, "expense")
    confirmed = lambda rows: sum(Decimal(str(row["amount"])) for row in rows if row.get("status", "confirmed") == "confirmed")
    return {"result": {"inflow": float(confirmed(income)), "outflow": float(confirmed(expense))}}


CONTRACT = json.loads(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "contract.json"), encoding="utf-8").read())
ACTIONS = {name: globals()[name] for name in CONTRACT["actions"] if callable(globals().get(name))}


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        try:
            body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", 0))))
            action = ACTIONS.get(body.get("action"))
            if action is None:
                reply = {"ok": False, "error": "UNSUPPORTED"}
            else:
                if CC4.token is None:
                    api.login()
                reply = dict({"ok": True}, **action(body.get("args", {})))
        except Rejected:
            reply = {"ok": False, "error": "REJECTED"}
        except Unsupported:
            reply = {"ok": False, "error": "UNSUPPORTED"}
        except Exception as error:  # surfaced to the runner as a contract error, with the CC4 reason
            reply = {"ok": False, "error": "ADAPTER_ERROR", "message": str(error)[:500]}
        data = json.dumps(reply, ensure_ascii=False).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--port", type=int, default=9000)
    args = parser.parse_args()
    ThreadingHTTPServer(("127.0.0.1", args.port), Handler).serve_forever()
