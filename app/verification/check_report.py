"""Validate a saved business report or fetch one without changing demo business."""
import argparse
import datetime as dt
from decimal import Decimal, InvalidOperation
import json
import os
from pathlib import Path
import urllib.error
import urllib.parse
import urllib.request
from zoneinfo import ZoneInfo

ZONE = ZoneInfo("Asia/Shanghai")
FAMILIES = ("入职：", "离职：", "职级流转：C→B", "职级流转：B→A", "外呼：", "员工自录：",
            "请假：", "财务：100元应收", "工资：70元离职结算", "财务：重复同步")


def require(condition, message):
    if not condition:
        raise ValueError(message)


def amount(value):
    require(value is not None and not isinstance(value, bool), "金额缺失或类型错误")
    number = Decimal(str(value))
    require(number.is_finite() and number == number.quantize(Decimal("0.01")), "金额必须是有限的两位小数")
    return number


def validate_report(payload, expected_date=None):
    expected_date = expected_date or dt.datetime.now(ZONE).date().isoformat()
    report = payload.get("report", payload)
    require(isinstance(report, dict), "尚无演练报告")
    require(report.get("version") == 2, "报告格式不是version=2，请核对版本")
    require(report.get("status") == "passed" and not report.get("error"), "演练未通过或仍在执行")
    require(report.get("through") == expected_date, "演练尚未推进至指定业务日期")
    if "report" in payload:
        require(payload.get("enabled") is True, "服务端未启用演练")
        require(payload.get("businessDate") == expected_date, "服务端业务日期与验收日期不一致")
    origin, through = dt.date.fromisoformat(report["origin"]), dt.date.fromisoformat(report["through"])
    updated = dt.datetime.fromisoformat(report["updatedAt"].replace("Z", "+00:00"))
    require(updated.tzinfo is not None and updated.astimezone(ZONE).date() == through, "报告更新时间不属于验收业务日")
    checks, events, summary = report["checks"], report["events"], report["summary"]
    require(isinstance(checks, list) and checks and isinstance(events, list) and events, "缺少核对或事件明细")
    require(summary["checks"] == len(checks) and summary["events"] == len(events), "摘要计数与明细不一致")
    require(summary["days"] >= 7 and summary["days"] == (through-origin).days+1, "必须覆盖连续至少7个日期")
    require(all(e["status"] == "passed" for e in events), "存在未完成业务事件")
    require(len({(e["date"], e["title"]) for e in events}) == len(events), "业务事件重复")
    require(summary["employees"] == sum(e["title"].startswith("入职：") for e in events), "入职人数与摘要不一致")
    for family in FAMILIES:
        require(any(e["title"].startswith(family) for e in events), "缺少场景：" + family)
    for row in [*events, *checks]:
        require(origin <= dt.date.fromisoformat(row["date"]) <= through, "明细日期不在演练范围内")
    for row in checks:
        require(row.get("passed") is True, "有业务断言未通过：" + row["title"])
        require("expected" in row and "actual" in row, "断言缺少预期或实际值")
        # Canonical JSON also distinguishes true from 1 and null from zero.
        require(json.dumps(row["expected"], sort_keys=True) == json.dumps(row["actual"], sort_keys=True),
                "通过标记与预期/实际不符：" + row["title"])
    money = {key: amount(report["money"][key]) for key in ("sales", "inflow", "outflow", "net")}
    def total(title):
        rows = [r for r in checks if r["title"] == title + "（元）" and r.get("scope", "").startswith("账务复核：")]
        require(bool(rows), "缺少资金明细：" + title)
        return sum((amount(r["actual"]) for r in rows), Decimal(0))
    require(money["sales"] == total("回读业绩销售额"), "销售额与逐项核对不一致")
    require(money["inflow"] == money["sales"] + total("回读回款入账合计"), "流入与订单/回款不一致")
    require(money["outflow"] == total("回读运营费用") + total("回读离职结算工资"), "流出与费用/工资不一致")
    require(money["net"] == money["inflow"] - money["outflow"], "净流入不等于流入减流出")
    return {"status": "passed", "through": expected_date, "summary": summary, "money": report["money"]}


def validate_negative(report):
    require(report.get("status") == "failed" and report.get("error"), "故障注入没有保存失败报告")
    differences = [r for r in report.get("checks", []) if r.get("passed") is False
                   and r.get("title") == "已确认订单与收入一致（元）"]
    require(any(amount(r["actual"])-amount(r["expected"]) == Decimal(1) for r in differences),
            "失败原因不是被注入的1元收入差异")


def fetch_report(base):
    url = urllib.parse.urlsplit(base)
    require(url.scheme in ("http", "https") and url.hostname and url.username is None and url.password is None and not url.query
            and not url.fragment and url.path.rstrip("/").endswith("/api/v1"), "--base-url 必须为不含凭据的 /api/v1 地址")
    token = os.environ.get("CC4_VERIFY_TOKEN", "")
    def request(path, body=None):
        headers = {"Content-Type": "application/json"}
        if token:
            headers["Authorization"] = "Bearer " + token
        req = urllib.request.Request(base.rstrip("/")+path, headers=headers,
                                     data=json.dumps(body).encode() if body is not None else None)
        # Credentials must never follow an unexpected redirect to another host.
        class NoRedirect(urllib.request.HTTPRedirectHandler):
            def redirect_request(self, *args, **kwargs):
                return None
        with urllib.request.build_opener(NoRedirect).open(req, timeout=30) as response:
            return json.load(response)
    if not token:
        phone, password = os.environ.get("CC4_VERIFY_ADMIN_PHONE"), os.environ.get("CC4_VERIFY_ADMIN_PASSWORD")
        require(phone and password, "通过环境变量提供 CC4_VERIFY_TOKEN 或管理员手机号/密码")
        token = request("/auth/login", {"phone": phone, "password": password})["session"]["access_token"]
    return request("/simulation/report")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--input", type=Path, help="本地 report JSON，不访问网络")
    source.add_argument("--base-url", help="自己的演示环境 API 地址；不推进或重置业务")
    parser.add_argument("--date", default=dt.datetime.now(ZONE).date().isoformat(), help="验收业务日期，默认北京时间今天")
    parser.add_argument("--output", type=Path, help="保存不含令牌的报告与验证结论；拒绝覆盖")
    args = parser.parse_args()
    try:
        payload = json.loads(args.input.read_text()) if args.input else fetch_report(args.base_url)
        result = validate_report(payload, args.date)
        if args.output:
            args.output.parent.mkdir(parents=True, exist_ok=True)
            with args.output.open("x") as output:
                json.dump({"validation": result, "evidence": payload}, output, ensure_ascii=False, indent=2)
        print(json.dumps(result, ensure_ascii=False))
    except urllib.error.HTTPError as error:
        parser.exit(1, f"报告接口返回HTTP {error.code}，检查角色、凭据和URL；业务未推进。\n")
    except (ValueError, KeyError, TypeError, InvalidOperation, OSError, urllib.error.URLError) as error:
        parser.exit(1, f"验收未通过：{error}\n")


if __name__ == "__main__":
    main()
