import React, { useCallback, useMemo, useState } from "react";
import {
  Alert,
  Btn,
  Column,
  DataTablePage,
  Field,
  FormDialog,
  LevelBadge,
  MoneyCell,
  SelectField,
  StatCard,
  Status,
} from "../components/kit";
import { collection, download, queryString, request, Row } from "../lib/api";
import { LEVEL_ZH, money } from "../lib/tokens";
import { MCPAccess } from "../components/mcp-access";
import {
  ActionDialog,
  amountColumn,
  DetailTable,
  FormField,
  FormFields,
  number,
  ResourcePage,
  roles,
  SimpleHeader,
  stateColumn,
  text,
  today,
  useRemote,
  useWorkspace,
  Values,
  valuesBody,
  formDefaults,
} from "./shared";

interface TodayData {
  date: string;
  totals: { orders: number; revenue: number };
  attendance: { present: number; expected: number; rate: number };
  products: Row[];
  attendanceByTeam: Row[];
  funnel: Row;
}
interface Series {
  date: string;
  orders: number;
  revenue: number;
  wechat_added: number;
  answered: number;
  attendance_rate: number;
}
export function DashboardPage({
  navigate,
}: {
  navigate: (key: string) => void;
}) {
  const [date, setDate] = useState(
    () => new URLSearchParams(location.search).get("date") ?? today(),
  );
  const [weekEnd, setWeekEnd] = useState(
    () => new URLSearchParams(location.search).get("weekEnd") ?? date,
  );
  const start = useMemo(() => {
    const d = new Date(`${weekEnd}T12:00:00`);
    d.setDate(d.getDate() - 6);
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
  }, [weekEnd]);
  const overview = useRemote(
    () => request<{ data: TodayData }>("GET", `/dashboard/today?date=${date}`),
    [date],
  );
  const trend = useRemote(
    () =>
      request<{ data: { series: Series[] } }>(
        "GET",
        `/dashboard/trend?${queryString({ startDate: start, endDate: weekEnd })}`,
      ),
    [weekEnd, start],
  );
  const pending = useRemote(
    () =>
      request<{ pagination: { total: number } }>(
        "GET",
        "/daily-performance/pending-approval?limit=1",
      ),
    [],
  );
  const d = overview.data?.data,
    series = trend.data?.data.series ?? [];
  const choose = (value: string, changeWeek = false) => {
    if (!value) return;
    setDate(value);
    if (changeWeek) setWeekEnd(value);
    const u = new URL(location.href);
    u.searchParams.set("date", value);
    u.searchParams.set("weekEnd", changeWeek ? value : weekEnd);
    history.replaceState(null, "", u);
  };
  const max = Math.max(1, ...series.map((s) => s.revenue));
  return (
    <>
      <SimpleHeader
        title="经营概览"
        description="从今天的跟进与审核开始，核对团队的每一笔成果。"
      >
        <Field
          label="查看日期"
          type="date"
          value={date}
          onChange={(e) => choose(e.target.value, true)}
        />
      </SimpleHeader>
      <div className="day-strip" aria-label="最近七日业绩">
        {series.map((s) => (
          <button
            key={s.date}
            aria-pressed={s.date === date}
            onClick={() => choose(s.date)}
          >
            <span>
              {new Intl.DateTimeFormat("zh-CN", { weekday: "short" }).format(
                new Date(`${s.date}T12:00:00`),
              )}
            </span>
            <strong>
              {new Intl.DateTimeFormat("zh-CN", {
                month: "numeric",
                day: "numeric",
              }).format(new Date(`${s.date}T12:00:00`))}
            </strong>
            <small>{s.orders} 单</small>
          </button>
        ))}
      </div>
      {overview.error && (
        <Alert retry={overview.reload}>{overview.error}</Alert>
      )}
      {overview.loading && !d && (
        <div className="table-skeleton" role="status">
          正在载入经营数据…
          <div />
          <div />
        </div>
      )}
      {d && (
        <>
          <div className="summary-strip">
            <StatCard
              label="当日成交订单"
              value={
                <>
                  {d.totals.orders}
                  <small> 单</small>
                </>
              }
            />
            <StatCard label="当日销售额" value={money(d.totals.revenue)} />
            <StatCard
              label="已出勤 / 应出勤"
              value={
                <>
                  {d.attendance.present}
                  <small> / {d.attendance.expected} 人</small>
                </>
              }
            />
            <div className="stat stat-action">
              <div className="muted">待审核订单</div>
              <button onClick={() => navigate("approvals")}>
                {pending.data?.pagination.total ?? "—"}
                <span>去处理</span>
              </button>
            </div>
          </div>
          <div className="dashboard-grid">
            <section className="panel">
              <div className="section-heading">
                <h2>近 7 日销售额</h2>
                <span className="muted">
                  {start} 至 {weekEnd}
                </span>
              </div>
              {trend.loading ? (
                <div className="table-skeleton" role="status">
                  正在载入趋势…
                </div>
              ) : trend.error ? (
                <Alert retry={trend.reload}>{trend.error}</Alert>
              ) : (
                <>
                  <svg
                    className="chart"
                    viewBox="0 0 660 230"
                    role="img"
                    aria-label="最近七日销售额，详细数据见图表下方"
                  >
                    <line
                      x1="40"
                      y1="190"
                      x2="650"
                      y2="190"
                      stroke="var(--line)"
                    />
                    <line
                      x1="40"
                      y1="105"
                      x2="650"
                      y2="105"
                      stroke="var(--line)"
                      strokeDasharray="3 6"
                    />
                    <text x="0" y="20" className="chart-axis">
                      {money(max)}
                    </text>
                    <text x="18" y="193" className="chart-axis">
                      0
                    </text>
                    {series.map((s, i) => {
                      const x = 61 + i * 84;
                      const height = (s.revenue / max) * 150;
                      return (
                        <g key={s.date}>
                          <rect
                            x={x}
                            y={190 - height}
                            width="38"
                            height={height}
                            rx="3"
                            fill={s.date === date ? "var(--accent)" : "#adbee9"}
                          >
                            <title>
                              {s.date}：{money(s.revenue)}，{s.orders} 单
                            </title>
                          </rect>
                          <text
                            x={x + 19}
                            y="216"
                            textAnchor="middle"
                            className="chart-axis"
                          >
                            {s.date.slice(5)}
                          </text>
                        </g>
                      );
                    })}
                  </svg>
                  <details className="chart-data">
                    <summary>查看图表数据</summary>
                    <div className="table-scroll">
                      <table>
                        <thead>
                          <tr>
                            <th>日期</th>
                            <th>成交订单</th>
                            <th>销售额</th>
                            <th>出勤率</th>
                          </tr>
                        </thead>
                        <tbody>
                          {series.map((s) => (
                            <tr key={s.date}>
                              <td>{s.date}</td>
                              <td>{s.orders}</td>
                              <td>{money(s.revenue)}</td>
                              <td>{Math.round(s.attendance_rate * 100)}%</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </details>
                </>
              )}
            </section>
            <section className="panel">
              <h2>产品成交构成</h2>
              {d.products.length ? (
                <div className="table-scroll">
                  <table>
                    <thead>
                      <tr>
                        <th>产品</th>
                        <th className="number">订单</th>
                        <th className="number">销售额</th>
                      </tr>
                    </thead>
                    <tbody>
                      {d.products.map((p, i) => (
                        <tr key={text(p, "product_id") || i}>
                          <td>{text(p, "product_name")}</td>
                          <td className="number">{number(p, "orders")}</td>
                          <td className="number">
                            <MoneyCell v={number(p, "revenue")} />
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className="empty">
                  <p>这一天还没有成交记录。</p>
                  <Btn kind="ghost" onClick={() => navigate("performance")}>
                    录入业绩
                  </Btn>
                </div>
              )}
              <div className="panel-footer">
                <button
                  className="text-button"
                  onClick={() => navigate("products")}
                >
                  管理产品
                </button>
                <button
                  className="text-button"
                  onClick={() => navigate("performance")}
                >
                  查看每日业绩
                </button>
              </div>
            </section>
          </div>
          <section className="panel" style={{ marginTop: 24 }}>
            <div className="section-heading">
              <h2>团队出勤</h2>
              <button
                className="text-button"
                onClick={() => navigate("attendance")}
              >
                查看考勤
              </button>
            </div>
            <div className="attendance-teams">
              {d.attendanceByTeam.length ? (
                d.attendanceByTeam.map((team, i) => (
                  <div key={text(team, "team_id") || i}>
                    <strong>{text(team, "team_name")}</strong>
                    <span>
                      {number(team, "present")} / {number(team, "expected")}{" "}
                      人出勤
                    </span>
                    <b>{Math.round(number(team, "rate") * 100)}%</b>
                  </div>
                ))
              ) : (
                <p className="muted">还没有可统计的团队成员。</p>
              )}
            </div>
          </section>
        </>
      )}
    </>
  );
}

export function EmployeesPage() {
  const { user, notify } = useWorkspace();
  const manage = roles(user, "admin", "hr", "manager", "workplace_manager"),
    levelManage = roles(user, "admin", "hr", "workplace_manager");
  const fields: FormField[] = [
    { key: "name", label: "员工姓名", required: true },
    { key: "employee_code", label: "工号", required: true, createOnly: true },
    { key: "phone", label: "手机号", type: "tel", required: true },
    {
      key: "password",
      label: "初始密码",
      type: "password",
      required: true,
      createOnly: true,
    },
    { key: "department", label: "部门" },
    { key: "position", label: "职位", value: "员工" },
    { key: "team_id", label: "所属团队", type: "team" },
    {
      key: "join_date",
      label: "入职日期",
      type: "date",
      value: today(),
      required: true,
    },
    {
      key: "level",
      label: "入职等级",
      type: "select",
      value: "trainee",
      options: Object.entries(LEVEL_ZH)
        .filter(([k]) => !k.includes("elimin"))
        .map(([value, label]) => ({ value, label })),
      required: true,
      createOnly: true,
    },
    ...(user.role === "admin"
      ? [
          {
            key: "role",
            label: "账号角色",
            type: "select" as const,
            options: [
              { value: "employee", label: "销售" },
              { value: "manager", label: "团队经理" },
              { value: "workplace_manager", label: "职场经理" },
              { value: "hr", label: "人力" },
              { value: "finance", label: "财务" },
              { value: "admin", label: "管理员" },
            ],
            value: "employee",
            required: true,
            createOnly: true,
          },
        ]
      : []),
  ];
  return (
    <ResourcePage
      definition={{
        title: "员工管理",
        description: "维护员工档案、团队归属与等级变更。",
        path: "/employees",
        key: "employees",
        search: true,
        statuses: ["active", "inactive", "suspended"],
        canCreate: manage,
        canEdit: manage,
        fields,
        columns: [
          { key: "name", title: "姓名" },
          { key: "employee_code", title: "工号" },
          {
            key: "level",
            title: "等级",
            render: (r) => <LevelBadge level={text(r, "level")} />,
          },
          { key: "position", title: "职位" },
          { key: "department", title: "部门" },
          {
            key: "join_date",
            title: "入职日期",
            render: (r) => text(r, "join_date").slice(0, 10),
          },
          stateColumn(),
        ],
        actions: levelManage
          ? [
              {
                label: "调整等级",
                fields: [
                  {
                    key: "toLevel",
                    label: "变更为",
                    type: "select",
                    options: Object.entries(LEVEL_ZH)
                      .filter(([k]) => !k.includes("suggested"))
                      .map(([value, label]) => ({ value, label })),
                    required: true,
                  },
                  {
                    key: "changeDate",
                    label: "生效日期",
                    type: "date",
                    value: today(),
                    required: true,
                  },
                  {
                    key: "reason",
                    label: "变更原因",
                    type: "textarea",
                    required: true,
                  },
                ],
                run: (r, v) =>
                  request(
                    "POST",
                    `/employees/${text(r, "id")}/level-change`,
                    v,
                  ),
              },
              {
                label: "停用",
                visible: (r) =>
                  r.status === "active" && number(r, "user_id") !== user.id,
                confirm: "停用后，该员工将无法登录。",
                run: (r) =>
                  request("PUT", "/employees/batch-status", {
                    employeeIds: [number(r, "id")],
                    status: "inactive",
                  }),
              },
              {
                label: "恢复在职",
                visible: (r) => r.status !== "active",
                run: (r) =>
                  request("PUT", "/employees/batch-status", {
                    employeeIds: [number(r, "id")],
                    status: "active",
                  }),
              },
            ]
          : [],
      }}
      detail={(row, close) => (
        <DetailTable
          title={`${text(row, "name")}的等级记录`}
          path={`/employees/${text(row, "id")}/level-history`}
          itemKey="history"
          onClose={close}
          columns={[
            {
              key: "change_date",
              title: "生效日期",
              render: (r) => text(r, "change_date").slice(0, 10),
            },
            {
              key: "from_level",
              title: "原等级",
              render: (r) => LEVEL_ZH[text(r, "from_level")],
            },
            {
              key: "to_level",
              title: "新等级",
              render: (r) => LEVEL_ZH[text(r, "to_level")],
            },
            { key: "reason", title: "原因" },
          ]}
        />
      )}
    />
  );
}

export function PerformancePage() {
  const { user, notify } = useWorkspace();
  const [open, setOpen] = useState(false),
    [tab, setTab] = useState("daily");
  const employee = user.role === "employee";
  const fields: FormField[] = [
    ...(!employee
      ? [
          {
            key: "employeeId",
            label: "员工",
            type: "employee" as const,
            required: true,
          },
        ]
      : []),
    {
      key: "workDate",
      label: "工作日期",
      type: "date",
      value: today(),
      required: true,
    },
    {
      key: "productId",
      label: "成交产品",
      type: "product",
      required: employee,
    },
    {
      key: "orders",
      label: "订单数量",
      type: "number",
      min: 0,
      step: 1,
      value: "1",
      required: true,
    },
    {
      key: "workPeriod",
      label: "工作时段",
      type: "select",
      options: [
        { value: "full_day", label: "全天" },
        { value: "morning", label: "上午" },
        { value: "afternoon", label: "下午" },
      ],
      value: "full_day",
      required: true,
    },
    ...(employee
      ? [
          {
            key: "parentPhone",
            label: "家长手机号",
            type: "tel" as const,
            required: true,
          },
          { key: "orderId", label: "订单编号" },
          {
            key: "parentAddedTeacherWechat",
            label: "家长已添加老师微信",
            type: "checkbox" as const,
          },
        ]
      : []),
  ];
  const [values, setValues] = useState<Values>(() => formDefaults(fields)),
    [files, setFiles] = useState<Record<string, File>>({});
  const submit = async () => {
    const body = valuesBody(fields, values);
    if (employee) {
      body.employeeId = user.employee_id;
      if (Number(values.orders) > 1)
        throw new Error("自录每次提交 1 单，多笔订单请逐笔录入");
      const form = new FormData();
      Object.entries(body).forEach(([key, v]) => {
        if (v != null) form.set(key, String(v));
      });
      for (const [key, file] of Object.entries(files)) form.set(key, file);
      await request("POST", "/daily-performance", form);
      notify(
        Number(values.orders) === 0
          ? "零业绩与出勤已记录"
          : "业绩已提交，等待审核",
      );
    } else {
      await request("POST", "/daily-performance", body);
      notify("业绩已录入，提成已自动计算");
    }
    window.dispatchEvent(new Event("cc4:refresh"));
  };
  return (
    <>
      <div className="tabs" aria-label="业绩视图">
        <button aria-pressed={tab === "daily"} onClick={() => setTab("daily")}>
          每日汇总
        </button>
        <button
          aria-pressed={tab === "details"}
          onClick={() => setTab("details")}
        >
          订单明细与审核状态
        </button>
      </div>
      {tab === "daily" ? (
        <ResourcePage
          definition={{
            title: "每日业绩",
            description:
              "按员工、日期与产品汇总；新增订单自动接续当日提成档位。",
            path: "/daily-performance",
            key: "performances",
            dateFilter: true,
            fields: [],
            columns: [
              { key: "work_date", title: "工作日期" },
              { key: "employee_name", title: "员工" },
              {
                key: "product_name",
                title: "产品",
                render: (r) => text(r, "product_name") || "日汇总",
              },
              { key: "orders_count", title: "订单数", align: "right" },
              amountColumn("sales_amount", "销售额"),
              amountColumn("commission_amount", "提成", "in"),
            ],
            actions: !employee
              ? [
                  {
                    label: "调整单量",
                    fields: [
                      {
                        key: "ordersCount",
                        label: "调整后的总订单数",
                        type: "number",
                        min: 0,
                        step: 1,
                        required: true,
                      },
                    ],
                    run: (r, v) =>
                      request("PUT", `/daily-performance/${text(r, "id")}`, {
                        ordersCount: Number(v.ordersCount),
                      }),
                  },
                ]
              : [],
            canDelete: roles(user, "admin", "hr"),
          }}
          extraToolbar={
            <Btn
              onClick={() => {
                setValues(formDefaults(fields));
                setFiles({});
                setOpen(true);
              }}
            >
              {employee ? "录入我的业绩" : "新增业绩"}
            </Btn>
          }
        />
      ) : (
        <OrderDetailsPage />
      )}
      {open && (
        <FormDialog
          open
          title={employee ? "录入我的业绩" : "新增业绩"}
          onClose={() => setOpen(false)}
          onSubmit={submit}
          submitText={employee ? "提交审核" : "录入业绩"}
          dirty={
            JSON.stringify(values) !== JSON.stringify(formDefaults(fields)) ||
            Object.keys(files).length > 0
          }
        >
          <FormFields
            fields={fields.map((f) =>
              employee &&
              Number(values.orders) === 0 &&
              ["productId", "parentPhone"].includes(f.key)
                ? { ...f, required: false }
                : f,
            )}
            values={values}
            setValues={setValues}
          />
          {employee && Number(values.orders) > 0 && (
            <>
              <p className="muted">
                每次提交一笔订单，上传三张截图供审核。支持
                PNG、JPEG、WebP，每张不超过 4 MB。
              </p>
              {[
                ["wechatPayScreenshot", "微信支付截图"],
                ["wechatChatScreenshot", "微信聊天记录截图"],
                ["wechatContactScreenshot", "添加微信联系页截图"],
              ].map(([key, label]) => (
                <Field
                  key={key}
                  label={label}
                  name={key}
                  type="file"
                  accept="image/png,image/jpeg,image/webp"
                  required
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    if (file) setFiles((v) => ({ ...v, [key]: file }));
                  }}
                />
              ))}
            </>
          )}
          {!employee && (
            <p className="muted">
              选择产品时，销售额和提成按产品规则计算；未选产品时，按日汇总规则计算提成。
            </p>
          )}
        </FormDialog>
      )}
    </>
  );
}

function ScreenshotButtons({ row }: { row: Row }) {
  const { notify } = useWorkspace();
  const urls = [
    ["wechat_pay_screenshot_url", "支付截图"],
    ["wechat_chat_screenshot_url", "聊天截图"],
    ["wechat_contact_screenshot_url", "联系截图"],
  ];
  return (
    <div className="row-actions">
      {urls
        .filter(([key]) => row[key])
        .map(([key, label]) =>
          /^https?:\/\//i.test(text(row, key)) ? (
            <a
              className="btn btn--ghost"
              key={key}
              href={text(row, key)}
              target="_blank"
              rel="noreferrer"
            >
              {label}
            </a>
          ) : (
            <Btn
              kind="ghost"
              key={key}
              onClick={async () => {
                try {
                  await download(text(row, key));
                } catch (e) {
                  notify((e as Error).message, true);
                }
              }}
            >
              {label}
            </Btn>
          ),
        )}
    </div>
  );
}
export function OrderDetailsPage({ pending = false }: { pending?: boolean }) {
  const { user } = useWorkspace();
  const manage = roles(user, "admin", "hr", "manager", "workplace_manager");
  return (
    <ResourcePage
      definition={{
        title: pending ? "订单审核" : "订单明细",
        description: pending
          ? "核对订单资料和三张截图，通过后计入员工的当日业绩。"
          : "查看每笔订单的提交资料与审核结果。",
        path: pending
          ? "/daily-performance/pending-approval"
          : "/daily-performance/order-details",
        key: "orderDetails",
        fields: [],
        columns: [
          { key: "work_date", title: "工作日期" },
          { key: "employee_name", title: "员工" },
          { key: "product_name", title: "产品" },
          { key: "parent_phone", title: "家长手机号" },
          stateColumn("approval_status", "审核状态"),
          {
            key: "screenshots",
            title: "订单资料",
            render: (r) => <ScreenshotButtons row={r} />,
          },
          { key: "rejection_reason", title: "驳回原因" },
        ],
        actions: manage
          ? [
              {
                label: "审核通过",
                visible: (r) => r.approval_status === "pending_approval",
                confirm: "通过后，此订单计入员工当日业绩与提成。",
                run: (r) =>
                  request(
                    "POST",
                    `/daily-performance/order-details/${text(r, "id")}/approve`,
                    { approved: true },
                  ),
              },
              {
                label: "驳回",
                visible: (r) => r.approval_status === "pending_approval",
                fields: [
                  {
                    key: "rejectionReason",
                    label: "驳回原因",
                    type: "textarea",
                    required: true,
                  },
                ],
                run: (r, v) =>
                  request(
                    "POST",
                    `/daily-performance/order-details/${text(r, "id")}/approve`,
                    { approved: false, rejectionReason: v.rejectionReason },
                  ),
              },
            ]
          : [],
      }}
    />
  );
}

export function SalaryPage() {
  const { user, notify } = useWorkspace();
  const [month, setMonth] = useState(today().slice(0, 7)),
    [calc, setCalc] = useState(false),
    [batch, setBatch] = useState(false),
    [version, setVersion] = useState(0),
    [detail, setDetail] = useState<Row | null>(null),
    [action, setAction] = useState<Row | null>(null);
  const [year, m] = month.split("-").map(Number);
  const manage = roles(user, "admin", "hr", "manager", "finance");
  const load = useCallback(
    (page: number, limit: number) =>
      collection("/salary", "salaries", page, limit, { year, month: m }),
    [year, m],
  );
  const fields: FormField[] = [
    { key: "employeeId", label: "员工", type: "employee", required: true },
  ];
  const changed = () => {
    setVersion((v) => v + 1);
    notify("薪资记录已更新");
  };
  const { employees } = useWorkspace();
  const cols: Column<Row>[] = [
    {
      key: "employee_id",
      title: "员工",
      render: (r) =>
        text(employees.find((e) => e.id === r.employee_id) ?? {}, "name") ||
        `员工 ${text(r, "employee_id")}`,
    },
    {
      key: "period",
      title: "账期",
      render: (r) => `${r.period_year} 年 ${r.period_month} 月`,
    },
    amountColumn("base_salary", "底薪"),
    amountColumn("commission_amount", "提成", "in"),
    amountColumn("bonus_amount", "奖金"),
    amountColumn("net_salary", "应发金额"),
    {
      key: "payment_status",
      title: "发放状态",
      render: (r: Row) => (
        <Status
          value={text(r, "payment_status")}
          label={text(r, "payment_status") === "paid" ? "已发放" : "待发放"}
        />
      ),
    },
    {
      key: "actions",
      title: "操作",
      render: (r) => (
        <div className="row-actions">
          <Btn
            kind="ghost"
            onClick={async () => {
              try {
                const result = await request<{ salary: Row }>(
                  "GET",
                  `/salary/${text(r, "id")}/detail`,
                );
                setDetail(result.salary);
              } catch (e) {
                notify((e as Error).message, true);
              }
            }}
          >
            计算明细
          </Btn>
          {manage && (
            <Btn kind="ghost" onClick={() => setAction(r)}>
              {r.payment_status === "paid" ? "撤销发放" : "登记发放"}
            </Btn>
          )}
        </div>
      ),
    },
  ];
  return (
    <>
      <DataTablePage
        key={month}
        title="薪资管理"
        description="核对每月工资与计算依据，登记发放后同步财务支出。"
        columns={cols}
        fetchPage={load}
        refreshKey={version}
        rowKey={(r) => text(r, "id")}
        filters={
          <Field
            label="薪资月份"
            type="month"
            value={month}
            onChange={(e) => setMonth(e.target.value)}
          />
        }
        toolbar={
          manage && (
            <>
              <Btn kind="ghost" onClick={() => setBatch(true)}>
                计算全员薪资
              </Btn>
              <Btn onClick={() => setCalc(true)}>计算单人薪资</Btn>
            </>
          )
        }
      />
      {calc && (
        <ActionDialog
          title={`计算 ${year} 年 ${m} 月薪资`}
          fields={fields}
          onClose={() => setCalc(false)}
          onSave={async (v) => {
            await request("POST", "/salary/calculate", {
              employeeId: Number(v.employeeId),
              periodYear: year,
              periodMonth: m,
            });
            changed();
          }}
          submitText="计算薪资"
        />
      )}
      {batch && (
        <FormDialog
          open
          title="计算全员薪资"
          onClose={() => setBatch(false)}
          onSubmit={async () => {
            const result = await request<{ message: string; failed: Row[] }>(
              "POST",
              "/salary/calculate-batch",
              { periodYear: year, periodMonth: m },
            );
            setVersion((v) => v + 1);
            notify(result.message, !!result.failed?.length);
          }}
          submitText="开始计算"
        >
          <p>
            按 {year} 年 {m}{" "}
            月的业绩与工作日配置，计算当前可管理员工的薪资。已发放的记录会跳过并列出原因。
          </p>
        </FormDialog>
      )}
      {action && (
        <ActionDialog
          title={
            action.payment_status === "paid" ? "撤销薪资发放" : "登记薪资发放"
          }
          description={`${text(employees.find((e) => e.id === action.employee_id) ?? {}, "name") || "该员工"}，应发 ${money(number(action, "net_salary"))}。`}
          fields={
            action.payment_status === "paid"
              ? []
              : [
                  {
                    key: "paymentDate",
                    label: "发放日期",
                    type: "date",
                    value: today(),
                    required: true,
                  },
                ]
          }
          onClose={() => setAction(null)}
          onSave={async (v) => {
            await request(
              "PUT",
              `/salary/${text(action, "id")}/payment-status`,
              {
                paymentStatus:
                  action.payment_status === "paid" ? "pending" : "paid",
                paymentDate: v.paymentDate,
              },
            );
            changed();
          }}
        />
      )}
      {detail && (
        <FormDialog
          open
          title="薪资计算明细"
          onClose={() => setDetail(null)}
          onSubmit={async () => setDetail(null)}
          submitText="关闭"
        >
          <dl className="detail-list">
            {[
              ["employee_name", "员工"],
              ["period_year", "年份"],
              ["period_month", "月份"],
              ["base_salary", "底薪"],
              ["commission_amount", "提成"],
              ["bonus_amount", "奖金"],
              ["deductions", "扣款"],
              ["net_salary", "应发金额"],
            ].map(([key, label]) => (
              <div key={key}>
                <dt>{label}</dt>
                <dd>
                  {[
                    "base_salary",
                    "commission_amount",
                    "bonus_amount",
                    "deductions",
                    "net_salary",
                  ].includes(key)
                    ? money(number(detail, key))
                    : text(detail, key)}
                </dd>
              </div>
            ))}
          </dl>
        </FormDialog>
      )}
    </>
  );
}

export function WorkdaysPage() {
  const { user, notify } = useWorkspace();
  const [month, setMonth] = useState(today().slice(0, 7)),
    [days, setDays] = useState<number[]>([]),
    [base, setBase] = useState<number[]>([]),
    [busy, setBusy] = useState(false);
  const [year, m] = month.split("-").map(Number);
  const manage = roles(user, "admin", "workplace_manager");
  const dirty = JSON.stringify(days) !== JSON.stringify(base);
  const config = useRemote(async () => {
    try {
      const row = await request<{ workdays: number[] }>(
        "GET",
        `/workdays/${year}/${m}`,
      );
      setDays(row.workdays);
      setBase(row.workdays);
      return true;
    } catch (e) {
      if ((e as { status?: number }).status === 404) {
        setDays([]);
        setBase([]);
        return false;
      }
      throw e;
    }
  }, [month]);
  const max = new Date(year, m, 0).getDate(),
    offset = (new Date(year, m - 1, 1).getDay() + 6) % 7;
  const save = async (next: number[]) => {
    setBusy(true);
    try {
      await request("POST", "/workdays", { year, month: m, workdays: next });
      setBase([...next]);
      setDays([...next]);
      notify("工作日配置已保存");
    } catch (e) {
      notify((e as Error).message, true);
    } finally {
      setBusy(false);
    }
  };
  return (
    <>
      <SimpleHeader
        title="工作日配置"
        description="出勤、考核窗口与薪资计算使用同一份工作日历。"
      >
        <Field
          label="配置月份"
          type="month"
          disabled={dirty || busy}
          value={month}
          onChange={(e) => setMonth(e.target.value)}
        />
      </SimpleHeader>
      {config.error && <Alert retry={config.reload}>{config.error}</Alert>}
      {config.loading ? (
        <p role="status">正在读取工作日…</p>
      ) : (
        <>
          <div className="actions" style={{ marginBottom: 20 }}>
            {manage && (
              <>
                <Btn
                  kind="ghost"
                  disabled={busy}
                  onClick={() => {
                    const next = [];
                    for (let day = 1; day <= max; day++) {
                      const weekday = new Date(year, m - 1, day).getDay();
                      if (weekday !== 0 && weekday !== 6) next.push(day);
                    }
                    setDays(next);
                  }}
                >
                  填入周一至周五
                </Btn>
                <Btn disabled={busy} onClick={() => save(days)}>
                  {busy ? "保存中…" : `保存 ${days.length} 个工作日`}
                </Btn>
                {dirty && (
                  <Btn kind="ghost" onClick={() => setDays([...base])}>
                    放弃更改
                  </Btn>
                )}
              </>
            )}
            <span className="muted">
              {config.data === false
                ? "该月尚未配置，保存后生效。"
                : dirty
                  ? "有未保存的更改"
                  : "已保存"}
            </span>
          </div>
          <div
            className="calendar-grid"
            aria-label={`${year} 年 ${m} 月工作日`}
          >
            {["一", "二", "三", "四", "五", "六", "日"].map((day) => (
              <span key={day}>周{day}</span>
            ))}
            {Array.from({ length: offset }, (_, i) => (
              <span key={`blank-${i}`} />
            ))}
            {Array.from({ length: max }, (_, i) => i + 1).map((day) => (
              <button
                key={day}
                disabled={!manage || busy}
                aria-label={`${m} 月 ${day} 日${days.includes(day) ? "，工作日" : "，休息日"}`}
                aria-pressed={days.includes(day)}
                onClick={() =>
                  setDays((prev) =>
                    prev.includes(day)
                      ? prev.filter((d) => d !== day)
                      : [...prev, day].sort((a, b) => a - b),
                  )
                }
              >
                {day}
              </button>
            ))}
          </div>
          <p className="calendar-note">
            蓝色为工作日。点击日期可以切换，保存后用于本月的所有工作日计算。
          </p>
        </>
      )}
    </>
  );
}

export function ApprovalsPage() {
  const { user } = useWorkspace();
  const [tab, setTab] = useState(
    roles(user, "admin", "hr", "manager", "workplace_manager")
      ? "orders"
      : "mine",
  );
  const manage = roles(user, "admin", "hr", "manager", "workplace_manager");
  return (
    <>
      <div className="tabs" aria-label="审批分类">
        {manage && (
          <button
            aria-pressed={tab === "orders"}
            onClick={() => setTab("orders")}
          >
            订单审核
          </button>
        )}
        {manage && (
          <button
            aria-pressed={tab === "general"}
            onClick={() => setTab("general")}
          >
            等级与请假审批
          </button>
        )}
        <button aria-pressed={tab === "mine"} onClick={() => setTab("mine")}>
          我的申请
        </button>
      </div>
      {tab === "orders" ? (
        <OrderDetailsPage pending />
      ) : (
        <ResourcePage
          key={tab}
          definition={{
            title: tab === "mine" ? "我的审批申请" : "审批管理",
            description: "核对申请内容，按实际情况通过、驳回或转交。",
            path: tab === "mine" ? "/approvals/my-requests" : "/approvals",
            key: "approvals",
            fields: [],
            statuses: ["pending", "approved", "rejected", "cancelled"],
            columns: [
              { key: "title", title: "申请事项" },
              { key: "requester_name", title: "申请人" },
              stateColumn("request_type", "类型"),
              stateColumn(),
              {
                key: "requested_at",
                title: "申请日期",
                render: (r) => text(r, "requested_at").slice(0, 10),
              },
            ],
            actions:
              manage && tab !== "mine"
                ? [
                    {
                      label: "通过申请",
                      visible: (r) => r.status === "pending",
                      fields: [
                        {
                          key: "comments",
                          label: "审批意见",
                          type: "textarea",
                        },
                      ],
                      run: (r, v) =>
                        request("POST", `/approvals/${text(r, "id")}/action`, {
                          action: "approved",
                          comments: v.comments,
                        }),
                    },
                    {
                      label: "驳回申请",
                      visible: (r) => r.status === "pending",
                      fields: [
                        {
                          key: "comments",
                          label: "驳回原因",
                          type: "textarea",
                          required: true,
                        },
                      ],
                      run: (r, v) =>
                        request("POST", `/approvals/${text(r, "id")}/action`, {
                          action: "rejected",
                          comments: v.comments,
                        }),
                    },
                    {
                      label: "转交",
                      visible: (r) => r.status === "pending",
                      fields: [
                        {
                          key: "nextApproverId",
                          label: "转交给",
                          type: "approver",
                          required: true,
                        },
                      ],
                      run: (r, v) =>
                        request("POST", `/approvals/${text(r, "id")}/action`, {
                          action: "forwarded",
                          nextApproverId: Number(v.nextApproverId),
                        }),
                    },
                  ]
                : [],
          }}
        />
      )}
    </>
  );
}

export function MonthlyPerformancePage() {
  const { user, notify } = useWorkspace();
  const [month, setMonth] = useState(today().slice(0, 7)),
    [version, setVersion] = useState(0);
  const [year, m] = month.split("-").map(Number);
  const load = useCallback(
    (page: number, limit: number) =>
      collection("/performance", "performances", page, limit, {
        year,
        month: m,
      }),
    [year, m],
  );
  return (
    <DataTablePage
      key={month}
      title="月度业绩"
      description="从每日业绩按员工和产品汇总，保留一致的销售额与提成。"
      fetchPage={load}
      refreshKey={version}
      rowKey={(r) => text(r, "id")}
      filters={
        <Field
          label="业绩月份"
          type="month"
          value={month}
          onChange={(e) => setMonth(e.target.value)}
        />
      }
      columns={[
        { key: "employee_name", title: "员工" },
        { key: "product_name", title: "产品" },
        { key: "orders_count", title: "总订单数", align: "right" },
        amountColumn("sales_amount", "总销售额"),
        amountColumn("commission_amount", "总提成", "in"),
      ]}
      toolbar={
        roles(user, "admin", "hr", "manager", "workplace_manager") && (
          <Btn
            onClick={async () => {
              try {
                const result = await request<{ message: string }>(
                  "POST",
                  "/performance/aggregate-to-monthly",
                  { year, month: m },
                );
                setVersion((v) => v + 1);
                notify(result.message);
              } catch (e) {
                notify((e as Error).message, true);
              }
            }}
          >
            汇总本月业绩
          </Btn>
        )
      }
    />
  );
}

export function ProfilePage() {
  const { user, notify } = useWorkspace();
  const [password, setPassword] = useState(false);
  return (
    <>
      <SimpleHeader title="个人中心" description="查看当前工作账号与个人资料。">
        <Btn onClick={() => setPassword(true)}>修改密码</Btn>
      </SimpleHeader>
      <section className="panel">
        <dl className="detail-list">
          {[
            ["姓名", user.name],
            ["登录手机号", user.phone],
            ["职位", user.position],
            ["员工等级", LEVEL_ZH[user.level ?? ""] ?? "—"],
          ].map(([label, value]) => (
            <div key={label}>
              <dt>{label}</dt>
              <dd>{value || "—"}</dd>
            </div>
          ))}
        </dl>
      </section>
      <MCPAccess />
      {password && (
        <ActionDialog
          title="修改登录密码"
          fields={[
            {
              key: "currentPassword",
              label: "当前密码",
              type: "password",
              required: true,
            },
            {
              key: "newPassword",
              label: "新密码（至少 6 位）",
              type: "password",
              required: true,
            },
            {
              key: "confirmPassword",
              label: "再次输入新密码",
              type: "password",
              required: true,
            },
          ]}
          onClose={() => setPassword(false)}
          onSave={async (v) => {
            if (v.newPassword !== v.confirmPassword)
              throw new Error("两次输入的新密码不一致");
            await request("PUT", "/auth/change-password", {
              currentPassword: v.currentPassword,
              newPassword: v.newPassword,
            });
            notify("密码已修改");
          }}
        />
      )}
    </>
  );
}
