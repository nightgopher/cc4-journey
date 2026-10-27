"""Export the 08 continuous-operation drill (app/simulation/run.py) as a contract scenario.

The daily plan below mirrors run.py's run_day(); dates are pinned to the
acceptance calendar instead of "the last 7 days". Expected salaries are
literal numbers worked out by hand from RULES.md (derivation in L3.md), not
produced by any implementation. Order and money totals are cross-checked
against the 08 drill's recorded baseline before anything is written.

Usage: python3 course/acceptance/tools/build_l3.py
"""
import json
from pathlib import Path

HERE = Path(__file__).resolve().parents[1]
DATES = ["2026-07-15", "2026-07-16", "2026-07-17", "2026-07-18", "2026-07-19", "2026-07-20", "2026-07-21"]
WINDOW = {"from": DATES[0], "to": DATES[-1]}
MONTH = "2026-07"

# run.py run_day(): senior cycles 3/5/8 orders; growth records 1 then 5 from day 3;
# growth's self-submitted orders add one approved order per day.
SENIOR = [(3, 30), (5, 70), (8, 160), (3, 30), (5, 70), (8, 160), (3, 30)]
GROWTH_RECORDED = [1, 1, 1, 5, 5, 5, 5]
GROWTH_AFTER_REVIEW = [(2, 20), (2, 20), (2, 20), (6, 100), (6, 100), (6, 100), (6, 100)]
LEAVE_DAYS = {1, 5}          # index % 4 == 1: new hire's approved leave
HALF_DAYS = {2, 6}           # index % 4 == 2: senior half-day attendance
PROMOTIONS = {3: "b_level", 6: "a_level"}

# July salary as of the end of each day (see L3.md for the arithmetic).
SENIOR_SALARY = [  # A level: floor(2200 × units / 23) + commission
    {"base": 95, "bonus": 0, "commission": 30, "total": 125},
    {"base": 191, "bonus": 0, "commission": 100, "total": 291},
    {"base": 239, "bonus": 0, "commission": 260, "total": 499},
    {"base": 239, "bonus": 0, "commission": 290, "total": 529},
    {"base": 239, "bonus": 0, "commission": 360, "total": 599},
    {"base": 334, "bonus": 0, "commission": 520, "total": 854},
    {"base": 382, "bonus": 0, "commission": 550, "total": 932},
]
GROWTH_SALARY = [  # C (90) → B from 07-18 (6×88) → A from 07-21 (2200×5/23)
    {"base": 90, "commission": 20, "total": 110},
    {"base": 90, "commission": 40, "total": 130},
    {"base": 90, "commission": 60, "total": 150},
    {"base": 528, "commission": 160, "total": 688},
    {"base": 528, "commission": 260, "total": 788},
    {"base": 528, "commission": 360, "total": 888},
    {"base": 478, "bonus": 0, "commission": 460, "total": 938},
]
BASELINE = {"orders": 77, "sales": 7623, "inflow": 8323, "outflow": 528.5}  # journey/08 cloud & isolated runs


def emp(alias):
    return "$" + alias


def hire(alias, level, date, title):
    return {"do": "hire", "args": {"name": f"验收{{run}}-{alias}", "level": level, "date": date}, "as": alias, "event": title}


def record(alias, date, count, title):
    return {"do": "record_orders", "args": {"emp": emp(alias), "date": date, "count": count}, "event": title}


def daily(alias, date, orders, commission, rule="R01", hint=""):
    step = {"check": "get_daily", "args": {"emp": emp(alias), "date": date},
            "expect": {"orders": orders, "sales": orders * 99, "commission": commission}, "rule": rule}
    if hint:
        step["hint"] = hint
    return step


def salary(alias, expect, rule, hint=""):
    step = {"check": "get_salary", "args": {"emp": emp(alias), "month": MONTH}, "expect": expect, "rule": rule}
    if hint:
        step["hint"] = hint
    return step


def build():
    steps = [{"check": "get_ledger", "args": WINDOW, "as": "L0", "event": "演练开始前的资金快照"},
             hire("senior", "a_level", DATES[0], "入职：资深销售（A）"),
             hire("growth", "c_level", DATES[0], "入职：成长销售（C）"),
             hire("settle", "trainee", DATES[0], "入职：离职结算员工（实习生）")]
    totals = {"orders": 0, "sales": 0, "inflow": 0, "outflow": 0}
    for i, day in enumerate(DATES):
        hired = f"hire{i}"
        steps.append(hire(hired, "c_level", day, f"D{i} 入职：当日新销售（C）"))
        steps.append(record(hired, day, 1, f"D{i} 录单：新销售 1 单"))
        steps.append(daily(hired, day, 1, 10))
        if i in LEAVE_DAYS:
            steps.append({"do": "approve_leave", "args": {"emp": emp(hired), "date": day}, "event": f"D{i} 请假：批准后录 0 单"})
            steps.append(record(hired, day, 0, f"D{i} 请假日补录 0 单"))
            steps.append(daily(hired, day, 1, 10, "R07", "请假不改变已录订单"))
        orders, commission = SENIOR[i]
        steps.append(record("senior", day, orders, f"D{i} 录单：资深销售 {orders} 单"))
        steps.append(daily("senior", day, orders, commission, hint=f"{orders} 单的独立金标准提成 {commission}"))
        if i in PROMOTIONS:
            steps.append({"do": "promote", "args": {"emp": emp("growth"), "level": PROMOTIONS[i], "date": day},
                          "event": f"D{i} 职级流转：成长销售 → {PROMOTIONS[i]}"})
        recorded = GROWTH_RECORDED[i]
        steps.append(record("growth", day, recorded, f"D{i} 录单：成长销售 {recorded} 单"))
        steps.append(daily("growth", day, recorded, {1: 10, 5: 70}[recorded]))
        # Self-submitted orders: three pending, approve, duplicate approve, reject, delete, resubmit.
        final, final_commission = GROWTH_AFTER_REVIEW[i]
        o = [f"o{i}_{n}" for n in range(4)]
        steps += [{"do": "submit_order", "args": {"emp": emp("growth"), "date": day}, "as": o[n], "event": f"D{i} 员工自录第 {n + 1} 笔"} for n in range(3)]
        steps += [daily("growth", day, recorded, {1: 10, 5: 70}[recorded], "R02", "待审订单不计入"),
                  {"do": "approve_order", "args": {"order": "$" + o[0]}, "event": f"D{i} 审核通过一笔"},
                  daily("growth", day, final, final_commission, "R02"),
                  {"do": "approve_order", "args": {"order": "$" + o[0]}, "expect_error": "REJECTED", "rule": "R02", "hint": "重复审核必须被拒绝"},
                  {"do": "reject_order", "args": {"order": "$" + o[1]}, "event": f"D{i} 驳回一笔"},
                  daily("growth", day, final, final_commission, "R02", "驳回不计入"),
                  {"do": "delete_order", "args": {"order": "$" + o[0]}, "event": f"D{i} 撤销已通过订单"},
                  daily("growth", day, recorded, {1: 10, 5: 70}[recorded], "R02", "删除已通过订单后业绩回退"),
                  {"do": "submit_order", "args": {"emp": emp("growth"), "date": day}, "as": o[3], "event": f"D{i} 重新提交"},
                  {"do": "approve_order", "args": {"order": "$" + o[3]}, "event": f"D{i} 新订单审核通过"},
                  daily("growth", day, final, final_commission, "R02", "一笔通过、一笔驳回、一笔待审后只增加 1 单")]
        if i in HALF_DAYS:
            steps.append({"do": "record_attendance", "args": {"emp": emp("senior"), "date": day, "status": "half"},
                          "event": f"D{i} 考勤：资深销售半天"})
        day_orders = 1 + orders + final
        if i == 0:
            steps += [record("settle", day, 5, "D0 录单：结算员工 5 单"), daily("settle", day, 5, 70),
                      salary("settle", {"commission": 70, "total": 70}, "R06", "实习生 5 单离职工资 70 元")]
            for n, (action, outflow, hint) in enumerate([("pay_salary", 70, "发放 70 元工资"), ("pay_salary", 70, "重复发放不增加流出"),
                                                         ("revoke_salary", 0, "撤销后流出回退"), ("pay_salary", 70, "再次发放")]):
                args = {"emp": emp("settle"), "month": MONTH}
                if action == "pay_salary":
                    args["date"] = day
                steps.append({"do": action, "args": args, "event": f"D0 工资：{hint}"})
                steps.append({"check": "get_ledger", "args": WINDOW, "rule": "R09", "hint": hint,
                              "expect_delta": {"since": "L0", "outflow": outflow}})
            day_orders += 5
            totals["outflow"] += 70
        receivable = f"recv{i}"
        steps += [{"do": "create_receivable", "args": {"amount": 100, "date": day}, "as": receivable, "event": f"D{i} 财务：应收 100"},
                  {"do": "receive_payment", "args": {"receivable": "$" + receivable, "amount": 30, "date": day}, "event": f"D{i} 首款 30"},
                  {"check": "get_receivable", "args": {"receivable": "$" + receivable}, "expect": {"remaining": 70}, "rule": "R08"},
                  {"do": "receive_payment", "args": {"receivable": "$" + receivable, "amount": 71, "date": day}, "expect_error": "REJECTED",
                   "rule": "R08", "hint": "超收必须被拒绝"},
                  {"check": "get_receivable", "args": {"receivable": "$" + receivable}, "expect": {"remaining": 70}, "rule": "R08", "hint": "超收被拒后余额不变"},
                  {"do": "receive_payment", "args": {"receivable": "$" + receivable, "amount": 70, "date": day}, "event": f"D{i} 尾款 70"},
                  {"check": "get_receivable", "args": {"receivable": "$" + receivable}, "expect": {"remaining": 0}, "rule": "R08"},
                  {"do": "record_expense", "args": {"amount": 65.5, "date": day}, "event": f"D{i} 财务：运营支出 65.50"}]
        totals["orders"] += day_orders
        totals["sales"] += day_orders * 99
        totals["inflow"] += day_orders * 99 + 100
        totals["outflow"] += 65.5
        steps += [salary("senior", SENIOR_SALARY[i], "R05", "A级：底薪按工作日有效出勤折算（半天 0.5），加逐日提成"),
                  salary("growth", GROWTH_SALARY[i], {0: "R03", 3: "R04", 6: "R05"}.get(i, "R03" if i < 3 else "R04"),
                         "按月末等级：C 前3个工作日×30 / B 6×88 / A 底薪折算"),
                  {"check": "get_ledger", "args": WINDOW, "rule": "R10", "hint": f"D{i} 累计：流入 = 订单销售 + 回款；流出 = 费用 + 已发工资",
                   "expect_delta": {"since": "L0", "inflow": totals["inflow"], "outflow": totals["outflow"]}}]
    # Final read-back of every day, like run.py's reconcile(): numbers must not drift.
    for i, day in enumerate(DATES):
        steps += [daily(f"hire{i}", day, 1, 10, hint="账务复核"), daily("senior", day, *SENIOR[i], hint="账务复核"),
                  daily("growth", day, *GROWTH_AFTER_REVIEW[i], hint="账务复核")]
        if i not in LEAVE_DAYS:
            steps.append(salary(f"hire{i}", {"base": 90, "commission": 10, "total": 100}, "R03", "当月入职C级：前3个工作日×30 + 1单提成"))
    steps.append(daily("settle", DATES[0], 5, 70, hint="账务复核"))
    steps.append(salary("settle", {"commission": 70, "total": 70}, "R09", "已发工资保持 70 元"))
    steps.append({"check": "get_ledger", "args": WINDOW, "rule": "R10", "hint": "七日累计资金与 08 演练基线一致",
                  "expect_delta": {"since": "L0", "inflow": BASELINE["inflow"], "outflow": BASELINE["outflow"]}})
    for key, value in BASELINE.items():
        if totals[key] != value:
            raise SystemExit(f"plan drifted from the 08 baseline: {key} {totals[key]} != {value}")
    return {"id": "L3-SIM-01", "layer": "L3", "title": "08 连续经营七日演练（2026-07-15 至 07-21）",
            "rules": ["R01", "R02", "R03", "R04", "R05", "R06", "R07", "R08", "R09", "R10"],
            "source": "app/simulation/run.py run_day()；映射与推导见 L3.md", "baseline": BASELINE, "steps": steps}


if __name__ == "__main__":
    scenario = build()
    out = HERE / "scenarios" / "L3-SIM-01.json"
    out.write_text(json.dumps(scenario, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
    events = sum("do" in s for s in scenario["steps"])
    checks = sum("expect" in s or "expect_delta" in s or "expect_error" in s for s in scenario["steps"])
    print(f"{out.relative_to(HERE)}: {events} contract events, {checks} checks")
