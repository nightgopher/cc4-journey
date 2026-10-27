"""Hidden L3 variants for instructor grading.

Each variant replays the shape of the 08 drill (L3-SIM-01) with different
order counts, amounts, start date (half of them straddle a month end),
promotion timing, half days and leave days, all derived from a secret seed.

Expected values come from the rule model in examples/toy_system.py, run once
per D1 decision; fields where the two decisions differ become by_decision
branches. Before grading, prove the expectations against the instructor's CC4
(two independent engines must agree, as in GOLDEN.md's differential protocol):

  CC4_HIDDEN_SEED=... python3 course/acceptance/tools/hidden_seeds.py --count 6
  bash course/acceptance/adapters/cc4-reference/run-reference.sh --scenarios course/acceptance/hidden

Keep the seed out of the repository; hidden/ is git-ignored and manifest.json
records only a fingerprint.
"""
import argparse
import calendar
import copy
import datetime as dt
from decimal import Decimal
import hashlib
import hmac
import json
import os
from pathlib import Path
import random
import sys

HERE = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(HERE / "examples"))
import toy_system  # noqa: E402

VERSION = 2
FIRST, LAST = dt.date(2026, 1, 1), dt.date(2026, 8, 31)  # covered by calendar.json


def fingerprint(seed):
    return hashlib.sha256(("cc4-hidden-v%d:" % VERSION + seed).encode()).hexdigest()[:12]


def plan(seed, index):
    rng = random.Random(hmac.new(seed.encode(), f"variant-{index}".encode(), hashlib.sha256).digest())
    if index % 2 == 0:
        # Straddle a month end so carry-over and history rules are exercised.
        month = rng.randint(1, 7)
        end = dt.date(2026, month, calendar.monthrange(2026, month)[1])
        start = end - dt.timedelta(days=rng.randint(1, 5))
    else:
        start = FIRST + dt.timedelta(days=rng.randint(0, (LAST - FIRST).days - 6))
    promote_b = rng.randint(1, 4)
    return {
        "dates": [(start + dt.timedelta(days=i)).isoformat() for i in range(7)],
        # 0-order days included on purpose: entering performance counts as attendance (R05).
        "senior": [rng.choice([0, 0] + list(range(1, 13))) for _ in range(7)],
        "growth": [rng.randint(1, 8) for _ in range(7)],
        "hire_orders": [rng.randint(1, 4) for _ in range(7)],
        "settle_orders": rng.randint(1, 8),
        "promote_b": promote_b,
        "promote_a": rng.choice([None, rng.randint(promote_b + 1, 6)]) if promote_b < 6 else None,
        "half_days": sorted(rng.sample(range(7), rng.randint(0, 2))),
        "senior_leave": rng.choice([None, rng.randint(0, 6), rng.randint(0, 6)]),
        "leave_days": sorted(rng.sample(range(7), rng.randint(1, 2))),
        "receivables": [rng.randint(5, 50) * 10 for _ in range(7)],
        "first_share": [rng.randint(10, 90) for _ in range(7)],
        "overpay_by": [rng.randint(1, 30) for _ in range(7)],
        "expenses": [rng.randint(1000, 20000) for _ in range(7)],  # cents
    }


def month_of(date):
    return date[:7]


def variant_steps(p):
    """Contract steps for one variant; observation expectations are placeholders (None)."""
    dates = p["dates"]
    window = {"from": dates[0], "to": dates[-1]}
    salary_fields = ("base", "bonus", "commission", "total")

    def daily(alias, date, rule="R01", hint=""):
        return {"check": "get_daily", "args": {"emp": "$" + alias, "date": date}, "expect": dict.fromkeys(("orders", "sales", "commission")),
                "rule": rule, **({"hint": hint} if hint else {})}

    def salary(alias, month, rule, hint=""):
        return {"check": "get_salary", "args": {"emp": "$" + alias, "month": month}, "expect": dict.fromkeys(salary_fields),
                "rule": rule, **({"hint": hint} if hint else {})}

    def ledger(rule, hint, **fields):
        return {"check": "get_ledger", "args": window, "rule": rule, "hint": hint, "expect_delta": dict({"since": "L0"}, **dict.fromkeys(fields))}

    steps = [{"check": "get_ledger", "args": window, "as": "L0", "event": "开始前资金快照"}]
    for alias, level, title in (("senior", "a_level", "资深销售（A）"), ("growth", "c_level", "成长销售（C）"), ("settle", "trainee", "结算员工（实习生）")):
        steps.append({"do": "hire", "args": {"name": f"验收{{run}}-{alias}", "level": level, "date": dates[0]}, "as": alias, "event": "入职：" + title})
    growth_level = "c_level"
    for i, day in enumerate(dates):
        hired = f"hire{i}"
        steps += [{"do": "hire", "args": {"name": f"验收{{run}}-{hired}", "level": "c_level", "date": day}, "as": hired, "event": f"D{i} 入职新销售"},
                  {"do": "record_orders", "args": {"emp": "$" + hired, "date": day, "count": p["hire_orders"][i]}, "event": f"D{i} 新销售录单"},
                  daily(hired, day)]
        if i in p["leave_days"]:
            steps += [{"do": "approve_leave", "args": {"emp": "$" + hired, "date": day}, "event": f"D{i} 新销售请假"},
                      {"do": "record_orders", "args": {"emp": "$" + hired, "date": day, "count": 0}, "event": f"D{i} 请假日补录 0 单"},
                      daily(hired, day, "R07", "请假不改变已录订单")]
        if i == p["senior_leave"] and i not in p["half_days"]:
            # A-level leave changes base pay, so leave priority (R07) shows up in money.
            steps.append({"do": "approve_leave", "args": {"emp": "$senior", "date": day}, "event": f"D{i} 资深销售请假后仍录单"})
        steps += [{"do": "record_orders", "args": {"emp": "$senior", "date": day, "count": p["senior"][i]}, "event": f"D{i} 资深销售录单"},
                  daily("senior", day)]
        if i == p["promote_b"] or i == p["promote_a"]:
            growth_level = "b_level" if i == p["promote_b"] else "a_level"
            steps.append({"do": "promote", "args": {"emp": "$growth", "level": growth_level, "date": day}, "event": f"D{i} 成长销售晋升 {growth_level}"})
        steps += [{"do": "record_orders", "args": {"emp": "$growth", "date": day, "count": p["growth"][i]}, "event": f"D{i} 成长销售录单"},
                  daily("growth", day)]
        o = [f"o{i}_{n}" for n in range(4)]
        steps += [{"do": "submit_order", "args": {"emp": "$growth", "date": day}, "as": o[n], "event": f"D{i} 员工自录第 {n + 1} 笔"} for n in range(3)]
        steps += [daily("growth", day, "R02", "待审订单不计入"),
                  {"do": "approve_order", "args": {"order": "$" + o[0]}, "event": f"D{i} 审核通过"},
                  daily("growth", day, "R02"),
                  {"do": "approve_order", "args": {"order": "$" + o[0]}, "expect_error": "REJECTED", "rule": "R02", "hint": "重复审核必须被拒绝"},
                  {"do": "reject_order", "args": {"order": "$" + o[1]}, "event": f"D{i} 驳回"},
                  daily("growth", day, "R02", "驳回不计入"),
                  {"do": "delete_order", "args": {"order": "$" + o[0]}, "event": f"D{i} 撤销已通过订单"},
                  daily("growth", day, "R02", "删除后业绩回退"),
                  {"do": "submit_order", "args": {"emp": "$growth", "date": day}, "as": o[3], "event": f"D{i} 重新提交"},
                  {"do": "approve_order", "args": {"order": "$" + o[3]}, "event": f"D{i} 新订单通过"},
                  daily("growth", day, "R02")]
        if i in p["half_days"]:
            steps.append({"do": "record_attendance", "args": {"emp": "$senior", "date": day, "status": "half"}, "event": f"D{i} 资深销售半天"})
        if i == 0:
            month = month_of(day)
            steps += [{"do": "record_orders", "args": {"emp": "$settle", "date": day, "count": p["settle_orders"]}, "event": "D0 结算员工录单"},
                      daily("settle", day), salary("settle", month, "R06", "实习生工资 = 当月提成")]
            for action, hint in (("pay_salary", "发放工资"), ("pay_salary", "重复发放不增加流出"), ("revoke_salary", "撤销后流出回退"), ("pay_salary", "再次发放")):
                args = {"emp": "$settle", "month": month, **({"date": day} if action == "pay_salary" else {})}
                steps += [{"do": action, "args": args, "event": "D0 工资：" + hint}, ledger("R09", hint, outflow=None)]
        amount = p["receivables"][i]
        first = max(1, amount * p["first_share"][i] // 100)
        recv = f"recv{i}"
        steps += [{"do": "create_receivable", "args": {"amount": amount, "date": day}, "as": recv, "event": f"D{i} 应收 {amount}"},
                  {"do": "receive_payment", "args": {"receivable": "$" + recv, "amount": first, "date": day}, "event": f"D{i} 首款 {first}"},
                  {"check": "get_receivable", "args": {"receivable": "$" + recv}, "expect": {"remaining": None}, "rule": "R08"},
                  {"do": "receive_payment", "args": {"receivable": "$" + recv, "amount": amount - first + p["overpay_by"][i], "date": day},
                   "expect_error": "REJECTED", "rule": "R08", "hint": "超收必须被拒绝"},
                  {"check": "get_receivable", "args": {"receivable": "$" + recv}, "expect": {"remaining": None}, "rule": "R08", "hint": "超收被拒后余额不变"},
                  {"do": "receive_payment", "args": {"receivable": "$" + recv, "amount": amount - first, "date": day}, "event": f"D{i} 尾款"},
                  {"check": "get_receivable", "args": {"receivable": "$" + recv}, "expect": {"remaining": None}, "rule": "R08"},
                  {"do": "record_expense", "args": {"amount": float(Decimal(p["expenses"][i]) / 100), "date": day}, "event": f"D{i} 运营支出"}]
        growth_rule = {"c_level": "R03", "b_level": "R04", "a_level": "R05"}[growth_level]
        steps += [salary("senior", month_of(day), "R05", "A级：底薪按工作日有效出勤折算，加提成"),
                  salary("growth", month_of(day), growth_rule, "按月末等级计算"),
                  ledger("R10", f"D{i} 累计资金", inflow=None, outflow=None)]
    for i, day in enumerate(dates):
        steps += [daily(f"hire{i}", day, hint="账务复核"), daily("senior", day, hint="账务复核"), daily("growth", day, hint="账务复核")]
        if i not in p["leave_days"]:
            steps.append(salary(f"hire{i}", month_of(day), "R03", "当月入职C级"))
    for month in sorted({month_of(d) for d in dates}):
        # After every promotion has happened: earlier months must not be rewritten.
        steps += [salary("senior", month, "R06", "历史月工资复核"), salary("growth", month, "R06", "晋升后历史月工资不变")]
    steps += [salary("settle", month_of(dates[0]), "R09", "已发工资保持不变"), ledger("R10", "七日累计资金", inflow=None, outflow=None)]
    return steps


def evaluate(steps, decision):
    """Run the steps on the rule model; return observed values per step index."""
    toy = toy_system.Toy("b_monthly_reset" if decision == "monthly_reset" else "")
    for month, days in json.loads((HERE / "calendar.json").read_text(encoding="utf-8"))["months"].items():
        toy.act("set_workdays", {"month": month, "days": days})
    saved, observed = {}, {}

    def resolve(value):
        if isinstance(value, str) and value.startswith("$"):
            return saved[value[1:]]
        if isinstance(value, dict):
            return {k: resolve(v) for k, v in value.items()}
        return value

    for index, step in enumerate(steps):
        action = step.get("do") or step.get("check")
        try:
            reply = toy.act(action, resolve(step.get("args", {})))
        except toy_system.Rejected:
            if step.get("expect_error") != "REJECTED":
                raise SystemExit(f"rule model rejected step {index} ({action}) that the plan expects to succeed")
            continue
        if step.get("expect_error"):
            raise SystemExit(f"rule model accepted step {index} ({action}) that must be rejected")
        if "as" in step:
            saved[step["as"]] = reply.get("id") if "do" in step else reply["result"]
        if "expect" in step:
            observed[index] = {k: reply["result"][k] for k in step["expect"]}
        elif "expect_delta" in step:
            base = saved[step["expect_delta"]["since"]]
            observed[index] = {k: float(Decimal(str(reply["result"][k])) - Decimal(str(base[k])))
                               for k in step["expect_delta"] if k != "since"}
    return observed


def tidy(value):
    value = Decimal(str(value))
    return int(value) if value == value.to_integral_value() else float(value)


def build_variant(seed, index):
    p = plan(seed, index)
    steps = variant_steps(p)
    carry, reset = evaluate(steps, "carry_over"), evaluate(steps, "monthly_reset")
    branched = 0
    for i, step in enumerate(steps):
        key = "expect" if "expect" in step else "expect_delta" if "expect_delta" in step else None
        if key is None:
            continue
        for field in carry[i]:
            a, b = tidy(carry[i][field]), tidy(reset[i][field])
            step[key][field] = a if a == b else {"by_decision": {"D1": {"carry_over": a, "monthly_reset": b}}}
            branched += a != b
    vid = f"H-{fingerprint(seed)}-{index + 1:02d}"
    crosses = month_of(p["dates"][0]) != month_of(p["dates"][-1])
    return {"id": vid, "layer": "H", "title": f"隐藏变体 {index + 1}：{p['dates'][0]} 起七日" + ("（跨月）" if crosses else ""),
            "rules": ["R01", "R02", "R03", "R04", "R05", "R06", "R07", "R08", "R09", "R10"],
            "source": f"tools/hidden_seeds.py v{VERSION}，结构同 L3-SIM-01", "steps": steps}, {"id": vid, "plan": copy.deepcopy(p), "branchedFields": branched}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--seed", default=os.environ.get("CC4_HIDDEN_SEED"), help="秘密种子；默认读取 CC4_HIDDEN_SEED")
    parser.add_argument("--count", type=int, default=6)
    parser.add_argument("--out", type=Path, default=HERE / "hidden")
    args = parser.parse_args(argv)
    if not args.seed or len(args.seed) < 16:
        parser.error("需要至少 16 个字符的秘密种子（--seed 或 CC4_HIDDEN_SEED）")
    args.out.mkdir(parents=True, exist_ok=True)
    for old in args.out.glob("H-*.json"):
        old.unlink()
    plans = []
    for index in range(args.count):
        scenario, info = build_variant(args.seed, index)
        (args.out / (scenario["id"] + ".json")).write_text(json.dumps(scenario, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
        plans.append(info)
        print(f"{scenario['id']}: {scenario['title']}，{sum('do' in s for s in scenario['steps'])} 个动作，D1 分支字段 {info['branchedFields']}")
    manifest = {"generator": f"hidden_seeds.py v{VERSION}", "seedFingerprint": fingerprint(args.seed), "count": args.count,
                "generatedAt": dt.datetime.now().isoformat(timespec="seconds"), "variants": plans}
    (args.out / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main())
