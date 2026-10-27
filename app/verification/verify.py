"""One-command student acceptance: disposable PostgreSQL, real APIs, retained evidence."""
import argparse
import datetime as dt
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import sys
import tempfile
import time
import uuid

from check_report import require, validate_negative, validate_report

APP = Path(__file__).resolve().parents[1]


def isolated_environment():
    prefixes = ("DB_", "PG", "GOOKI_", "MOCK_", "MCP_", "PADDLEOCR_", "CC4_", "VITE_")
    env = {k: v for k, v in os.environ.items() if not k.startswith(prefixes)
           and k not in {"DATABASE_URL", "JWT_SECRET", "HOST", "PORT", "ENABLE_CRON", "BUSINESS_TIMEZONE"}}
    env.update(DB_HOST="127.0.0.1", DB_USER="root", CC4_KEEP_TEST_DB="0", ENABLE_CRON="false",
               BUSINESS_TIMEZONE="Asia/Shanghai", VITE_API_BASE_URL="/cc4/api/v1")
    return env


def validate_evidence(output):
    simulation = output / "simulation"
    first = json.loads((simulation / "first-report.json").read_text())
    repeated = json.loads((simulation / "repeat-report.json").read_text())
    result = validate_report(first)
    validate_report(repeated)
    require(first["summary"] == repeated["summary"] and first["money"] == repeated["money"], "重跑改变了业务摘要")
    before = json.loads((simulation / "counts-before.json").read_text())
    after = json.loads((simulation / "counts-after.json").read_text())
    require(before == after and len(before) == 7, "重跑改变了业务记录数量")
    require((simulation / "unrelated-month-before.json").read_text() ==
            (simulation / "unrelated-month-after.json").read_text(), "演练改变了其他团队月记录")
    validate_negative(json.loads((simulation / "negative-report.json").read_text()))
    messages = [json.loads(line) for line in (output / "api-integration.jsonl").read_text().splitlines() if line.strip()]
    passed = {row.get("Test", "") for row in messages if row.get("Action") == "pass"}
    require({"TestBusinessIntegration", "TestMCPBusinessIntegration", "TestReimbursementIntegration"}.issubset(passed), "HTTP/MCP/报销集成被跳过或未通过")
    reimbursement = json.loads((output / "reimbursement-report.json").read_text())
    require(reimbursement.get("status") == "passed" and reimbursement.get("expenseCount") == 1 and reimbursement.get("expenseAmount") == 125.5, "报销付款证据缺失或金额错误")
    groups = [test for test in passed if test.startswith("TestBusinessIntegration/")]
    require(len(groups) >= 19, "实际运行的业务集成不足19组")
    require(sum(test.startswith("TestReimbursementIntegration/") for test in passed) >= 10, "实际运行的报销集成不足10组")
    return {"business": result, "httpIntegrationGroups": len(groups), "mcpBusinessIntegration": "passed",
            "idempotency": "passed", "unrelatedData": "preserved", "oneYuanFaultDetected": True, "ocrReimbursement": reimbursement}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="新的证据目录；默认 app/verification-results/时间-编号")
    parser.add_argument("--postgres-image", default="postgres:14", help="临时数据库镜像，默认与CI一致的PostgreSQL14")
    args = parser.parse_args()
    run_id = uuid.uuid4().hex[:12]
    output = (args.output or APP / "verification-results" / (dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")+"-"+run_id)).resolve()
    try:
        output.mkdir(parents=True, exist_ok=False)
    except FileExistsError:
        parser.exit(2, "证据目录已经存在，请指定新目录；不会覆盖旧验收记录。\n")
    os.chmod(output, 0o700)
    env = isolated_environment()
    name = "cc4-verify-" + run_id
    container = None
    report = {"version": 1, "status": "running", "startedAt": dt.datetime.now(dt.timezone.utc).isoformat(),
              "stages": [], "databaseImage": args.postgres_image, "scope": "isolated-local-acceptance"}
    def run(label, command, cwd=APP):
        stage = {"name": label, "command": command, "status": "running", "log": label+".log"}
        report["stages"].append(stage)
        print("[RUN] " + label, flush=True)
        start = time.monotonic()
        with (output / stage["log"]).open("w") as log:
            code = subprocess.call(command, cwd=cwd, env=env, stdout=log, stderr=subprocess.STDOUT)
        stage.update(status="passed" if code == 0 else "failed", exitCode=code, seconds=round(time.monotonic()-start, 2))
        require(code == 0, label+"失败，查看 "+str(output / stage["log"]))
        print("[PASS] " + label, flush=True)
    try:
        for program in ("go", "node", "npm", "docker", "bash", "curl", "git", "pdfinfo"):
            require(shutil.which(program), "缺少依赖："+program+"；见 verification/README.md")
        revision = subprocess.run(["git", "rev-parse", "HEAD"], cwd=APP, capture_output=True, text=True)
        dirty = subprocess.run(["git", "status", "--porcelain"], cwd=APP, capture_output=True, text=True)
        report["source"] = {"revision": revision.stdout.strip(), "dirty": bool(dirty.stdout.strip())}
        run("tool-versions", ["bash", "-c", "go version && node --version && npm --version && docker version --format '{{.Server.Version}}'"])
        env["POSTGRES_PASSWORD"] = secrets.token_hex(24)
        env["DB_PASSWORD"] = env["POSTGRES_PASSWORD"]
        run("database-create", ["docker", "create", "--name", name, "--label", "cc4.verification="+run_id,
                                "-e", "POSTGRES_USER=root", "-e", "POSTGRES_PASSWORD", "-p", "127.0.0.1::5432", args.postgres_image])
        container = name
        run("database-start", ["docker", "start", name])
        for attempt in range(60):
            ready = subprocess.run(["docker", "exec", name, "pg_isready", "-h", "127.0.0.1", "-U", "root"], capture_output=True)
            if ready.returncode == 0:
                break
            time.sleep(1)
        else:
            raise ValueError("临时PostgreSQL未就绪")
        binding = subprocess.check_output(["docker", "port", name, "5432/tcp"], text=True).strip()
        env.update(DB_PORT=binding.rsplit(":", 1)[1], CC4_TEST_CONTAINER=name)
        with tempfile.TemporaryDirectory(prefix="cc4-verification-") as temporary:
            # Migration tests use psql, provided by the isolated container.
            shim = Path(temporary) / "psql"
            shim.write_text('#!/usr/bin/env bash\nexec docker exec -i -e PGUSER=root "$CC4_TEST_CONTAINER" psql "$@"\n')
            shim.chmod(0o700)
            env["PATH"] = str(shim.parent)+os.pathsep+env["PATH"]
            run("go-tests", ["go", "test", "-race", "./..."])
            run("go-vet", ["go", "vet", "./..."])
            run("release-tests", [sys.executable, "-m", "unittest", "discover", "-s", "deploy", "-p", "test_release.py"])
            run("simulation-safety", [sys.executable, "-m", "unittest", "discover", "-s", "simulation", "-p", "test_*.py"])
            run("verification-tests", [sys.executable, "-m", "unittest", "discover", "-s", "verification", "-p", "test_*.py"])
            database = "cc4_test_migrations_"+run_id
            run("migration-database", ["docker", "exec", name, "createdb", "-U", "root", database])
            env["CC4_MIGRATION_TEST_DATABASE"] = database
            run("migration-tests", [sys.executable, "-m", "unittest", "discover", "-s", "deploy", "-p", "test_migrations.py"])
            del env["CC4_MIGRATION_TEST_DATABASE"]
            env["CC4_TEST_JSON"] = str(output / "api-integration.jsonl")
            env["CC4_REIMBURSEMENT_REPORT"] = str(output / "reimbursement-report.json")
            run("api-integration", ["bash", "scripts/test-integration.sh"])
            env["CC4_VERIFICATION_DIR"] = str(output / "simulation")
            run("business-simulation", ["bash", "scripts/test-simulation.sh"])
            report["evidence"] = validate_evidence(output)
            for label, command in (("frontend-install", ["npm", "ci"]),
                                   ("frontend-format", ["npm", "run", "format:check"]),
                                   ("frontend-dates", ["npm", "run", "test:dates"]),
                                   ("frontend-build", ["npm", "run", "build", "--", "--base=/cc4/"])):
                run(label, command, APP / "web")
        report["status"] = "passed"
    except (OSError, ValueError, KeyError, TypeError, ArithmeticError, subprocess.SubprocessError, KeyboardInterrupt) as error:
        report.update(status="failed", error=str(error) or "验收被中断")
    finally:
        if container:
            cleanup = subprocess.run(["docker", "rm", "-f", "-v", container], capture_output=True, text=True)
            report["cleanup"] = "passed" if cleanup.returncode == 0 else "failed"
            if cleanup.returncode:
                report.update(status="failed", error="临时容器清理失败，请检查："+container)
        report["finishedAt"] = dt.datetime.now(dt.timezone.utc).isoformat()
        (output / "summary.json").write_text(json.dumps(report, ensure_ascii=False, indent=2)+"\n")
        lines = ["# CC4 学员验收报告", "", "结果：**"+report["status"]+"**", "",
                 "源版本：`"+report.get("source", {}).get("revision", "unknown")+"`", "",
                 "| 阶段 | 结果 | 日志 |", "|---|---|---|"]
        lines.extend("| "+s["name"]+" | "+s["status"]+" | [日志]("+s["log"]+") |" for s in report["stages"])
        if report.get("error"):
            lines.extend(["", "失败原因："+report["error"]])
        if report.get("evidence"):
            lines.extend(["", "业务证据：", "```json", json.dumps(report["evidence"], ensure_ascii=False, indent=2), "```"])
        lines.extend(["", "通过仅覆盖 CASES.md 所列规则；没有验证真实银行/ERP到账或穷尽全部组合。"])
        (output / "REPORT.md").write_text("\n".join(lines)+"\n")
        print("["+report["status"].upper()+"] "+str(output / "REPORT.md"), flush=True)
    return 0 if report["status"] == "passed" else 1


if __name__ == "__main__":
    sys.exit(main())
