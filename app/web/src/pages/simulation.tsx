import React, { useCallback, useState } from "react";
import {
  Alert,
  Btn,
  Column,
  DataTablePage,
  StatCard,
  Status,
} from "../components/kit";
import { request } from "../lib/api";
import { money } from "../lib/tokens";
import { SimpleHeader, useRemote } from "./shared";

interface Check {
  scope?: string;
  date: string;
  title: string;
  expected: unknown;
  actual: unknown;
  passed: boolean;
}
interface Event {
  date: string;
  title: string;
  status: string;
}
interface Report {
  money?: { sales: number; inflow: number; outflow: number; net: number };
  status: "running" | "passed" | "failed";
  origin: string;
  through: string | null;
  updatedAt: string;
  error: string | null;
  events: Event[];
  checks: Check[];
  summary?: { days: number; employees: number; events: number; checks: number };
}
const display = (value: unknown) =>
  typeof value === "object" ? JSON.stringify(value) : String(value ?? "—");

export function SimulationPage() {
  const remote = useRemote(
    () =>
      request<{
        enabled: boolean;
        businessDate: string;
        report: Report | null;
      }>("GET", "/simulation/report"),
    [],
  );
  const [failedOnly, setFailedOnly] = useState(false);
  const [tab, setTab] = useState("checks");
  const report = remote.data?.report;
  const checks = report?.checks ?? [];
  const failed = checks.filter((row) => !row.passed).length;
  const selected = failedOnly ? checks.filter((row) => !row.passed) : checks;
  const fetchChecks = useCallback(
    async (page: number, limit: number) => ({
      rows: selected.slice((page - 1) * limit, page * limit),
      total: selected.length,
    }),
    [selected],
  );
  const events = [...(report?.events ?? [])].reverse();
  const fetchEvents = useCallback(
    async (page: number, limit: number) => ({
      rows: events.slice((page - 1) * limit, page * limit),
      total: events.length,
    }),
    [events],
  );
  const columns: Column<Check>[] = [
    {
      key: "scope",
      title: "业务场景",
      render: (row) => (
        <span className="wrap-text">{row.scope ?? "业务核对"}</span>
      ),
    },
    { key: "date", title: "业务日期" },
    {
      key: "title",
      title: "检查项目",
      render: (row) => <span className="wrap-text">{row.title}</span>,
    },
    {
      key: "expected",
      title: "预期结果",
      render: (row) => (
        <span className="wrap-text">{display(row.expected)}</span>
      ),
    },
    {
      key: "actual",
      title: "实际结果",
      render: (row) => <span className="wrap-text">{display(row.actual)}</span>,
    },
    {
      key: "passed",
      title: "核对结果",
      render: (row) => <Status value={row.passed ? "approved" : "rejected"} />,
    },
  ];
  return (
    <>
      <SimpleHeader
        title="业务演练"
        description="按天推进员工、线索、订单、审批与资金流，逐项核对预期和实际结果。"
      >
        <Btn kind="ghost" onClick={remote.reload}>
          刷新报告
        </Btn>
      </SimpleHeader>
      {remote.error && <Alert retry={remote.reload}>{remote.error}</Alert>}
      {remote.loading && <p role="status">正在载入演练报告…</p>}
      {!remote.loading && !remote.error && !report && (
        <section className="panel">
          <p>
            尚未运行经营演练。管理员可运行仓库的 Business simulation
            工作流，随后在这里查看结果。
          </p>
        </section>
      )}
      {report && (
        <>
          <section className="panel">
            <div className="section-heading">
              <h2>
                {report.status === "passed"
                  ? "本轮核对通过"
                  : report.status === "failed"
                    ? "本轮发现差异"
                    : "经营演练进行中"}
              </h2>
              <Status
                value={
                  report.status === "passed"
                    ? "approved"
                    : report.status === "failed"
                      ? "rejected"
                      : "pending"
                }
              />
            </div>
            <p>
              {report.origin} 至 {report.through ?? "首日执行中"}，当前业务日期{" "}
              {remote.data?.businessDate}。
            </p>
            <p className="muted">
              通过系统实际接口操作并回读核对；覆盖入离职、职级审批、订单收入、分期回款和工资发放。结果仅代表列出的场景。
            </p>
            {report.error && <Alert>{report.error}</Alert>}
          </section>
          <div className="summary-strip">
            <StatCard label="演练天数" value={report.summary?.days ?? "—"} />
            <StatCard label="业务事件" value={events.length} />
            <StatCard label="核对通过" value={checks.length - failed} />
            <StatCard label="发现差异" value={failed} />
          </div>
          {report.status === "passed" && report.money && (
            <div className="summary-strip">
              <StatCard
                label="演练订单销售"
                value={money(report.money.sales)}
              />
              <StatCard
                label="演练累计流入"
                value={money(report.money.inflow)}
              />
              <StatCard
                label="演练累计流出"
                value={money(report.money.outflow)}
              />
              <StatCard label="演练净流入" value={money(report.money.net)} />
            </div>
          )}
          <nav className="tabs" aria-label="演练报告分类">
            <Btn
              kind={tab === "checks" ? "primary" : "ghost"}
              onClick={() => setTab("checks")}
            >
              核对明细
            </Btn>
            <Btn
              kind={tab === "events" ? "primary" : "ghost"}
              onClick={() => setTab("events")}
            >
              每日业务流转
            </Btn>
          </nav>
          {tab === "checks" ? (
            <DataTablePage<Check>
              key={`checks-${report.updatedAt}-${failedOnly}`}
              title="业务断言"
              description="金额检查以元展示，差异会使本轮演练失败。"
              fetchPage={fetchChecks}
              columns={columns}
              rowKey={(row) =>
                `${row.date}-${row.title}-${checks.indexOf(row)}`
              }
              filters={
                <label>
                  <input
                    type="checkbox"
                    checked={failedOnly}
                    onChange={(event) => setFailedOnly(event.target.checked)}
                  />{" "}
                  仅看差异
                </label>
              }
              emptyText={failedOnly ? "没有发现差异。" : "等待首项核对。"}
            />
          ) : (
            <DataTablePage<Event>
              key={`events-${report.updatedAt}`}
              title="每日业务流转"
              description="每条记录对应一次实际业务操作，不包含账号密码或访问令牌。"
              fetchPage={fetchEvents}
              columns={[
                { key: "date", title: "业务日期" },
                {
                  key: "title",
                  title: "业务动作",
                  render: (row) => (
                    <span className="wrap-text">{row.title}</span>
                  ),
                },
                {
                  key: "status",
                  title: "状态",
                  render: (row) => (
                    <Status
                      value={row.status === "passed" ? "approved" : "rejected"}
                    />
                  ),
                },
              ]}
              rowKey={(row) =>
                `${row.date}-${row.title}-${events.indexOf(row)}`
              }
            />
          )}
        </>
      )}
    </>
  );
}
