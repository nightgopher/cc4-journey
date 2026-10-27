"""Implementation-neutral acceptance runner.

Drives any system through the acceptance contract (POST /acceptance/act),
compares business results with the rule card, and reports by rule id.
Standard library only; the student's system can be written in anything.
"""
import argparse
import copy
import datetime as dt
from decimal import Decimal, InvalidOperation
import json
from pathlib import Path
import re
import secrets
import shlex
import subprocess
import sys
import time
import urllib.error
import urllib.request

HERE = Path(__file__).resolve().parent
# Rule constants that belong in the system, not in a translation layer.
ADAPTER_SMELLS = re.compile(r"(?<![\w.])(88|2200|264|1000)(?![\w.])")


class ContractError(Exception):
    """The adapter broke the contract: unreachable, malformed reply, unknown error."""


def cents(value):
    try:
        return int((Decimal(str(value)) * 100).to_integral_value())
    except (InvalidOperation, TypeError, ValueError):
        raise ContractError(f"不是金额/数字：{value!r}")


def rule_titles(path=HERE / "RULES.md"):
    titles = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        match = re.match(r"## (R\d+) (.+)", line)
        if match:
            titles[match.group(1)] = match.group(2).strip()
    return titles


def load_scenarios(directory):
    scenarios = []
    for file in sorted(Path(directory).glob("*.json")):
        scenario = json.loads(file.read_text(encoding="utf-8"))
        if not isinstance(scenario, dict) or "steps" not in scenario:
            continue  # e.g. hidden/manifest.json
        scenario["file"] = file.name
        scenarios.append(scenario)
    return scenarios


class Adapter:
    def __init__(self, url, contract, command=None, timeout=60):
        self.url, self.contract, self.command, self.timeout = url, contract, command, timeout
        self.process = None
        self.calls = 0

    def start(self):
        if self.command:
            self.process = subprocess.Popen(shlex.split(self.command))
        for _ in range(100):
            try:
                result = self.raw("ping", {})
                if result.get("ok"):
                    return result.get("result", {})
            except (urllib.error.URLError, ConnectionError, OSError, ContractError):
                pass
            if self.process and self.process.poll() is not None:
                raise ContractError("适配进程启动后立即退出，退出码 " + str(self.process.returncode))
            time.sleep(0.2)
        raise ContractError("适配层无法连通：" + self.url)

    def stop(self):
        if self.process and self.process.poll() is None:
            self.process.terminate()
            try:
                self.process.wait(10)
            except subprocess.TimeoutExpired:
                self.process.kill()
        self.process = None

    def restart(self):
        self.stop()
        return self.start()

    def raw(self, action, args):
        body = json.dumps({"action": action, "args": args}, ensure_ascii=False).encode()
        request = urllib.request.Request(self.url, data=body, headers={"Content-Type": "application/json"}, method="POST")
        try:
            with urllib.request.urlopen(request, timeout=self.timeout) as response:
                payload = response.read()
        except urllib.error.HTTPError as error:
            payload = error.read()
        try:
            reply = json.loads(payload)
        except json.JSONDecodeError:
            raise ContractError(f"{action} 返回的不是 JSON")
        if not isinstance(reply, dict) or "ok" not in reply:
            raise ContractError(f"{action} 返回缺少 ok 字段")
        return reply

    def act(self, action, args):
        spec = self.contract["actions"].get(action)
        if spec is None:
            raise ContractError("剧本使用了契约外的动作：" + action)
        missing = [name for name in spec["args"] if name not in args]
        if missing:
            raise ContractError(f"剧本缺少 {action} 参数：{missing}")
        self.calls += 1
        reply = self.raw(action, args)
        if reply["ok"]:
            if spec["returns"] == "id" and reply.get("id") in (None, ""):
                raise ContractError(f"{action} 成功但没有返回 id")
            if isinstance(spec["returns"], dict) and spec["kind"] == "observe":
                result = reply.get("result")
                if not isinstance(result, dict):
                    raise ContractError(f"{action} 成功但没有返回 result 对象")
                absent = [key for key in spec["returns"] if key not in result]
                if absent:
                    raise ContractError(f"{action} 的 result 缺少字段：{absent}")
            return "ok", reply
        error = reply.get("error")
        if error in ("REJECTED", "UNSUPPORTED"):
            return error, reply
        raise ContractError(f"{action} 返回未定义的错误：{error!r} {reply.get('message', '')}".strip())


class Runner:
    def __init__(self, adapter, manifest, run_id):
        self.adapter, self.manifest, self.run_id = adapter, manifest, run_id
        self.decisions = manifest.get("decisions", {})
        self.unsupported = set(manifest.get("unsupported", []))

    def substitute(self, value, saved):
        if isinstance(value, str):
            if value.startswith("$"):
                if value[1:] not in saved:
                    raise ContractError("剧本引用了未保存的对象：" + value)
                return saved[value[1:]]
            return value.replace("{run}", self.run_id + "-" + self.scope)
        if isinstance(value, list):
            return [self.substitute(item, saved) for item in value]
        if isinstance(value, dict):
            return {key: self.substitute(item, saved) for key, item in value.items()}
        return value

    def expected_value(self, spec):
        if isinstance(spec, dict) and "by_decision" in spec:
            (decision, branches), = spec["by_decision"].items()
            choice = self.decisions.get(decision)
            if choice not in branches:
                return None, f"缺少或无效的裁决 {decision}（可选 {list(branches)}），请在 adapter.json 的 decisions 中声明"
            return branches[choice], None
        return spec, None

    def compare(self, step, actual, base=None):
        failures = []
        expectations = step.get("expect_delta") if base is not None else step["expect"]
        for field, spec in expectations.items():
            if field == "since":
                continue
            expected, problem = self.expected_value(spec)
            if problem:
                failures.append({"field": field, "message": problem})
                continue
            got = cents(actual[field]) - (cents(base[field]) if base is not None else 0)
            if got != cents(expected):
                shown = Decimal(got) / 100
                failures.append({"field": field, "expected": expected, "actual": float(shown) if shown % 1 else int(shown),
                                 "delta": base is not None})
        return failures

    def run(self, scenario):
        result = {"id": scenario["id"], "title": scenario["title"], "layer": scenario["layer"], "rules": scenario["rules"],
                  "status": "passed", "checks": 0, "failures": []}
        used = {step.get("do") or step.get("check") for step in scenario["steps"]}
        skipped = sorted(used & self.unsupported)
        if skipped:
            result.update(status="unsupported", reason="adapter.json 声明不支持：" + ", ".join(skipped))
            return result
        saved = {}
        self.scope = scenario["id"]
        event = ""
        try:
            for index, step in enumerate(scenario["steps"], 1):
                action = step.get("do") or step.get("check")
                args = self.substitute(step.get("args", {}), saved)
                status, reply = self.adapter.act(action, args)
                event = step.get("event", event)
                where = {"step": index, "action": action, "rule": step.get("rule", scenario["rules"][0]), "hint": step.get("hint", ""), "event": event}
                if status == "UNSUPPORTED":
                    result.update(status="unsupported", reason=f"第 {index} 步 {action} 返回 UNSUPPORTED")
                    return result
                wanted = step.get("expect_error")
                if wanted or status != "ok":
                    result["checks"] += 1
                    if status != (wanted or "ok"):
                        result["failures"].append(dict(where, field="响应", expected=wanted or "成功", actual=status))
                        if not wanted:
                            # The write did not happen; later checks would only cascade.
                            break
                    continue
                if "as" in step:
                    saved[step["as"]] = reply.get("id") if "do" in step else reply["result"]
                if "expect" in step or "expect_delta" in step:
                    result["checks"] += 1
                    base = saved[step["expect_delta"]["since"]] if "expect_delta" in step else None
                    for failure in self.compare(step, reply["result"], base):
                        result["failures"].append(dict(where, **failure))
        except ContractError as error:
            result.update(status="error", reason=str(error))
            return result
        if result["failures"]:
            result["status"] = "failed"
        return result


def mutate_first_money_expectation(scenario):
    """Return a copy whose first plain observation expectation is off by 1 yuan."""
    mutated = copy.deepcopy(scenario)
    for step in mutated["steps"]:
        for key in ("expect", "expect_delta"):
            for field, value in step.get(key, {}).items():
                if field not in ("since", "orders") and isinstance(value, (int, float)) and not isinstance(value, bool):
                    step[key][field] = value + 1
                    return mutated, step.get("check"), field
    return None, None, None


def lint_adapter(paths):
    warnings = []
    for root in paths:
        root = Path(root)
        files = [root] if root.is_file() else [p for p in root.rglob("*") if p.is_file() and p.suffix in
                                                (".py", ".js", ".ts", ".go", ".java", ".kt", ".rb", ".php", ".cs")]
        for file in files:
            for number, line in enumerate(file.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
                if ADAPTER_SMELLS.search(line.split("#")[0].split("//")[0]):
                    warnings.append(f"{file}:{number}: {line.strip()[:100]}")
    return warnings


def write_report(output, summary, titles):
    lines = ["# 学员验收报告（契约 " + summary["contract"] + "）", "",
             f"结果：**{summary['status']}**　运行编号 `{summary['runId']}`　{summary['finishedAt']}", "",
             f"场景：通过 {summary['counts']['passed']} / 失败 {summary['counts']['failed']} / "
             f"错误 {summary['counts']['error']} / 不支持 {summary['counts']['unsupported']}　检查项 {summary['checks']}", "",
             *(["**契约错误**：" + summary["error"], ""] if summary.get("error") else []),
             "## 按规则", "", "| 规则 | 名称 | 场景通过 | 结论 |", "|---|---|---|---|"]
    for rule, row in summary["rules"].items():
        verdict = "✓" if row["failed"] == 0 and row["passed"] > 0 else ("–" if row["failed"] == 0 else "✗")
        lines.append(f"| {rule} | {titles.get(rule, '')} | {row['passed']}/{row['total']} | {verdict} |")
    problems = [s for s in summary["scenarios"] if s["status"] in ("failed", "error", "unsupported")]
    if problems:
        lines += ["", "## 需要处理", ""]
    for scenario in problems:
        lines.append(f"### {scenario['status'].upper()} {scenario['id']} {scenario['title']}")
        if scenario.get("reason"):
            lines.append("- " + scenario["reason"])
        for failure in scenario["failures"]:
            if "message" in failure:
                detail = failure["message"]
            else:
                detail = f"期望 {failure['expected']}，实际 {failure['actual']}" + ("（增量）" if failure.get("delta") else "")
            lines.append(f"- 第 {failure['step']} 步 `{failure['action']}`.{failure['field']}：{detail}　"
                         f"→ {failure['rule']} {titles.get(failure['rule'], '')}" + (f"；提示：{failure['hint']}" if failure["hint"] else "")
                         + (f"（事件：{failure['event']}）" if failure.get("event") else ""))
        lines.append("")
    lines += ["", "## 验收器自检", ""]
    lines += [f"- 适配重启：{summary['restart']}", f"- 1 元反例：{summary['negative']['status']}（{summary['negative'].get('detail', '')}）"]
    if summary["adapterWarnings"]:
        lines += ["- 适配层出现规则常量，请确认业务计算不在适配层：", *["  - `" + w + "`" for w in summary["adapterWarnings"][:20]]]
    (output / "REPORT.md").write_text("\n".join(lines) + "\n", encoding="utf-8")
    (output / "summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--adapter", default="http://127.0.0.1:9000/acceptance/act", help="适配入口 URL")
    parser.add_argument("--adapter-cmd", help="由验收器启动适配进程（启用中途重启检查），例如 'python3 adapter.py --port 9000'")
    parser.add_argument("--manifest", type=Path, help="adapter.json：裁决与不支持的动作")
    parser.add_argument("--adapter-src", action="append", default=[], help="适配层源码目录，用于扫描规则常量（可多次）")
    parser.add_argument("--scenarios", type=Path, default=HERE / "scenarios")
    parser.add_argument("--layer", action="append", help="只跑某层，如 L1（可多次）")
    parser.add_argument("--rule", action="append", help="只跑涉及某规则的场景，如 R04（可多次）")
    parser.add_argument("--only", action="append", help="只跑某个场景 id（可多次）")
    parser.add_argument("--output", type=Path, help="报告目录；默认 results/时间-编号")
    args = parser.parse_args(argv)

    contract = json.loads((HERE / "contract.json").read_text(encoding="utf-8"))
    calendar = json.loads((HERE / "calendar.json").read_text(encoding="utf-8"))["months"]
    manifest = json.loads(args.manifest.read_text(encoding="utf-8")) if args.manifest else {}
    titles = rule_titles()
    run_id = secrets.token_hex(3)
    output = args.output or HERE / "results" / (dt.datetime.now().strftime("%Y%m%dT%H%M%S") + "-" + run_id)
    output.mkdir(parents=True, exist_ok=False)

    scenarios = load_scenarios(args.scenarios)
    if args.layer:
        scenarios = [s for s in scenarios if s["layer"] in args.layer]
    if args.rule:
        scenarios = [s for s in scenarios if set(s["rules"]) & set(args.rule)]
    if args.only:
        scenarios = [s for s in scenarios if s["id"] in args.only]

    adapter = Adapter(args.adapter, contract, args.adapter_cmd)
    runner = Runner(adapter, manifest, run_id)
    summary = {"contract": contract["contract"], "runId": run_id, "adapter": args.adapter, "decisions": runner.decisions,
               "startedAt": dt.datetime.now().isoformat(timespec="seconds"), "scenarios": [], "status": "running",
               "restart": "未启用（未提供 --adapter-cmd）", "negative": {"status": "未执行"},
               "adapterWarnings": lint_adapter(args.adapter_src)}
    try:
        info = adapter.start()
        print(f"[契约] 适配已连通，声明契约 {info.get('contract', '?')}", flush=True)
        for month, days in calendar.items():
            status, _ = adapter.act("set_workdays", {"month": month, "days": days})
            if status != "ok":
                raise ContractError("set_workdays 未成功：" + status)
        restart_at = len(scenarios) // 2 if args.adapter_cmd else -1
        for index, scenario in enumerate(scenarios):
            if index == restart_at and index > 0:
                adapter.restart()
                summary["restart"] = f"已在第 {index + 1} 个场景前重启适配进程"
                print("[重启] " + summary["restart"], flush=True)
            result = runner.run(scenario)
            summary["scenarios"].append(result)
            mark = {"passed": "PASS", "failed": "FAIL", "error": "ERR ", "unsupported": "SKIP"}[result["status"]]
            print(f"[{mark}] {scenario['id']} {scenario['title']}", flush=True)
        # A checker that cannot fail proves nothing: rerun one passed scenario
        # with an expectation off by 1 yuan; it must fail at exactly that field.
        candidate = next((s for s, r in zip(scenarios, summary["scenarios"]) if r["status"] == "passed"), None)
        mutated, action, field = mutate_first_money_expectation(candidate) if candidate else (None, None, None)
        if mutated:
            negative = Runner(adapter, manifest, run_id + "n").run(mutated)
            caught = negative["status"] == "failed" and any(f["action"] == action and f["field"] == field for f in negative["failures"])
            summary["negative"] = {"status": "passed" if caught else "failed", "scenario": candidate["id"],
                                   "detail": f"{candidate['id']} 的 {action}.{field} 期望值 +1 元后" + ("被正确判为失败" if caught else "未被识别")}
    except ContractError as error:
        summary["error"] = str(error)
        print("[契约错误] " + str(error), file=sys.stderr)
    finally:
        adapter.stop()

    counts = {key: sum(r["status"] == key for r in summary["scenarios"]) for key in ("passed", "failed", "error", "unsupported")}
    rules = {}
    for result in summary["scenarios"]:
        for rule in result["rules"]:
            row = rules.setdefault(rule, {"total": 0, "passed": 0, "failed": 0})
            row["total"] += 1
            row["passed"] += result["status"] == "passed"
            row["failed"] += result["status"] in ("failed", "error")
    ok = (not summary.get("error") and counts["failed"] == 0 and counts["error"] == 0 and counts["passed"] > 0
          and summary["negative"]["status"] == "passed")
    summary.update(status="passed" if ok else "failed", counts=counts, rules=dict(sorted(rules.items())),
                   checks=sum(r["checks"] for r in summary["scenarios"]), calls=adapter.calls,
                   finishedAt=dt.datetime.now().isoformat(timespec="seconds"))
    write_report(output, summary, titles)
    print(f"[{summary['status'].upper()}] {output / 'REPORT.md'}", flush=True)
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
