import React, { useCallback, useState } from "react";
import {
  Alert,
  Btn,
  Column,
  DataTablePage,
  Field,
  FormDialog,
  MoneyCell,
  SelectField,
  Status,
} from "../components/kit";
import { collection, download, queryString, request, Row } from "../lib/api";
import { LEVEL_ZH, money, STATUS_ZH } from "../lib/tokens";
import {
  ActionDialog,
  amountColumn,
  dateField,
  DetailTable,
  FormField,
  idField,
  number,
  ResourceDefinition,
  ResourcePage,
  roles,
  SimpleHeader,
  stateColumn,
  statusOptions,
  text,
  today,
  useRemote,
  useWorkspace,
  valuesBody,
} from "./shared";

const leadStates = ["new", "contacted", "qualified", "converted", "lost"];
const phoneField: FormField = {
  key: "phone",
  label: "手机号",
  type: "tel",
  required: true,
};
const notesField: FormField = { key: "notes", label: "备注", type: "textarea" };

export function LeadsPage() {
  const { user } = useWorkspace();
  const manage = roles(user, "admin", "manager", "workplace_manager");
  const d: ResourceDefinition = {
    title: "线索管理",
    description: "分配客户线索、记录跟进，直到成交或结束跟进。",
    path: "/leads",
    key: "leads",
    search: true,
    statuses: leadStates,
    canCreate: true,
    canEdit: true,
    canDelete: manage,
    fields: [
      { key: "customer_name", label: "客户姓名", required: true },
      phoneField,
      { key: "source", label: "线索来源" },
      {
        key: "status",
        label: "跟进状态",
        type: "select",
        options: statusOptions(leadStates),
        value: "new",
        required: true,
      },
      {
        key: "priority",
        label: "优先级",
        type: "select",
        options: statusOptions(["high", "medium", "low"]),
        value: "medium",
      },
      { key: "next_follow_up", label: "下次跟进", type: "date" },
      notesField,
    ],
    columns: [
      { key: "customer_name", title: "客户姓名" },
      { key: "phone", title: "手机号" },
      stateColumn(),
      {
        key: "assignee_name",
        title: "负责人",
        render: (r) => text(r, "assignee_name") || "未分配",
      },
      {
        key: "next_follow_up",
        title: "下次跟进",
        render: (r) => text(r, "next_follow_up") || "—",
      },
      { key: "source", title: "来源" },
    ],
    actions: [
      ...(manage
        ? [
            {
              label: "分配",
              fields: [{ ...idField, key: "assignedTo" }],
              run: (r: Row, v: Record<string, string | boolean>) =>
                request("POST", "/leads/assign-batch", {
                  leadIds: [number(r, "id")],
                  assignedTo: Number(v.assignedTo),
                }),
            },
          ]
        : []),
      {
        label: "标记成交",
        visible: (r) => !["converted", "lost"].includes(text(r, "status")),
        confirm: "确认该客户已经成交。",
        run: (r) => request("POST", `/leads/${text(r, "id")}/mark-paid`),
      },
      {
        label: "结束跟进",
        visible: (r) => !["converted", "lost"].includes(text(r, "status")),
        run: (r) => request("POST", `/leads/${text(r, "id")}/mark-invalid`),
      },
    ],
  };
  return <ResourcePage definition={d} />;
}

export function ProductsPage() {
  const { user } = useWorkspace();
  const manage = roles(user, "admin", "workplace_manager");
  return (
    <ResourcePage
      definition={{
        title: "产品管理",
        description: "维护体验课售价和按当日订单序号递增的提成档位。",
        path: "/products",
        key: "products",
        client: true,
        search: true,
        statuses: ["active", "inactive"],
        canCreate: manage,
        canEdit: manage,
        canDelete: manage,
        fields: [
          { key: "name", label: "产品名称", required: true },
          { key: "code", label: "产品编码", required: true },
          {
            key: "price",
            label: "售价（元）",
            type: "number",
            min: 0,
            required: true,
          },
          { key: "category", label: "产品分类" },
          {
            key: "status",
            label: "启用状态",
            type: "select",
            options: [
              { value: "active", label: "启用" },
              { value: "inactive", label: "停用" },
            ],
            value: "active",
            required: true,
          },
          { key: "description", label: "产品说明", type: "textarea" },
          {
            key: "commission_rules",
            label: "提成规则",
            type: "tiers",
            value: JSON.stringify({
              tiers: [
                { from: 1, to: 3, amount: 10 },
                { from: 4, to: 5, amount: 20 },
                { from: 6, to: null, amount: 30 },
              ],
            }),
          },
        ],
        columns: [
          { key: "name", title: "产品名称" },
          { key: "code", title: "编码" },
          amountColumn("price", "售价"),
          { key: "category", title: "分类" },
          stateColumn(),
          {
            key: "commission_rules",
            title: "提成档位",
            render: (r) => {
              try {
                const rules =
                  typeof r.commission_rules === "string"
                    ? JSON.parse(r.commission_rules)
                    : r.commission_rules;
                return `${(rules as { tiers?: unknown[] })?.tiers?.length ?? 0} 个档位`;
              } catch {
                return "待配置";
              }
            },
          },
        ],
      }}
    />
  );
}

function CustomerOrders({ row, onClose }: { row: Row; onClose: () => void }) {
  const { notify, user } = useWorkspace();
  const [open, setOpen] = useState(false),
    [version, setVersion] = useState(0);
  const path = `/customers/${text(row, "id")}/orders`;
  const fetchPage = useCallback(
    (page: number, limit: number) => collection(path, "orders", page, limit),
    [path],
  );
  const fields: FormField[] = [
    {
      key: "total_amount",
      label: "订单总额（元）",
      type: "number",
      min: 0.01,
      required: true,
    },
    {
      key: "order_date",
      label: "下单日期",
      type: "date",
      value: today(),
      required: true,
    },
    {
      key: "status",
      label: "订单状态",
      type: "select",
      options: statusOptions([
        "pending",
        "confirmed",
        "delivered",
        "cancelled",
      ]),
      value: "pending",
      required: true,
    },
  ];
  return (
    <>
      <DataTablePage
        title={`${text(row, "name")}的订单`}
        fetchPage={fetchPage}
        refreshKey={version}
        rowKey={(r) => text(r, "id")}
        columns={[
          { key: "order_number", title: "订单号" },
          { key: "order_date", title: "日期" },
          amountColumn("total_amount", "订单总额"),
          stateColumn(),
        ]}
        toolbar={
          <>
            <Btn kind="ghost" onClick={onClose}>
              收起详情
            </Btn>
            <Btn onClick={() => setOpen(true)}>新建订单</Btn>
          </>
        }
      />
      {open && (
        <ActionDialog
          title="新建客户订单"
          fields={fields}
          onClose={() => setOpen(false)}
          onSave={async (v) => {
            await request("POST", path, {
              ...valuesBody(fields, v),
              employee_id: user.employee_id,
            });
            setVersion((n) => n + 1);
            notify("客户订单已创建");
          }}
        />
      )}
    </>
  );
}
export function CustomersPage() {
  const { user } = useWorkspace();
  return (
    <ResourcePage
      definition={{
        title: "客户管理",
        description: "维护客户资料，查看客户订单与交易记录。",
        path: "/customers",
        key: "customers",
        search: true,
        canCreate: true,
        canEdit: true,
        canDelete: roles(user, "admin", "manager", "workplace_manager"),
        fields: [
          { key: "name", label: "客户姓名", required: true },
          phoneField,
          {
            key: "customer_type",
            label: "客户类型",
            type: "select",
            options: statusOptions(["individual", "business", "vip"]),
            value: "individual",
            required: true,
          },
          { key: "company", label: "公司" },
          { key: "email", label: "邮箱", type: "email" },
          { key: "address", label: "地址" },
          {
            key: "status",
            label: "客户状态",
            type: "select",
            options: [
              { value: "active", label: "正常" },
              { value: "inactive", label: "停用" },
              { value: "blacklisted", label: "黑名单" },
            ],
            value: "active",
          },
        ],
        columns: [
          { key: "name", title: "客户姓名" },
          { key: "phone", title: "手机号" },
          {
            key: "customer_type",
            title: "客户类型",
            render: (r) => STATUS_ZH[text(r, "customer_type")] ?? "—",
          },
          { key: "company", title: "公司" },
          stateColumn(),
          amountColumn("balance", "账户余额"),
        ],
      }}
      detail={(row, close) => <CustomerOrders row={row} onClose={close} />}
    />
  );
}

export function TeamsPage() {
  const { user } = useWorkspace();
  const manage = roles(user, "admin", "workplace_manager");
  return (
    <ResourcePage
      definition={{
        title: "团队管理",
        description: "维护团队负责人、组织归属与成员。",
        path: "/teams",
        key: "teams",
        client: true,
        search: true,
        canCreate: manage,
        canEdit: manage,
        canDelete: manage,
        fields: [
          { key: "name", label: "团队名称", required: true },
          { key: "code", label: "团队编码", required: true },
          { ...idField, key: "manager_id", label: "团队负责人" },
          { key: "description", label: "团队说明", type: "textarea" },
        ],
        columns: [
          { key: "name", title: "团队名称" },
          { key: "code", title: "编码" },
          { key: "manager_name", title: "负责人" },
          { key: "member_count", title: "成员人数", align: "right" },
          stateColumn(),
        ],
      }}
      detail={(row, close) => (
        <DetailTable
          path={`/teams/${text(row, "id")}/members`}
          itemKey="members"
          title={`${text(row, "name")}成员`}
          onClose={close}
          columns={[
            { key: "name", title: "姓名" },
            { key: "employee_code", title: "工号" },
            { key: "position", title: "职位" },
            {
              key: "level",
              title: "等级",
              render: (r) => LEVEL_ZH[text(r, "level")],
            },
            stateColumn(),
          ]}
        />
      )}
    />
  );
}

export function AttendancePage() {
  const { user, notify } = useWorkspace();
  const [tab, setTab] = useState("daily"),
    [month, setMonth] = useState(today().slice(0, 7)),
    [version, setVersion] = useState(0);
  const [year, m] = month.split("-").map(Number);
  const manage = roles(user, "admin", "hr", "manager", "workplace_manager");
  const fetchSummary = useCallback(
    (page: number, limit: number) =>
      collection("/attendance/monthly-summary", "summary", page, limit, {
        year,
        month: m,
      }),
    [year, m],
  );
  return (
    <>
      <div className="tabs" aria-label="考勤视图">
        <button aria-pressed={tab === "daily"} onClick={() => setTab("daily")}>
          每日考勤
        </button>
        <button
          aria-pressed={tab === "monthly"}
          onClick={() => setTab("monthly")}
        >
          月度汇总
        </button>
        <button aria-pressed={tab === "leave"} onClick={() => setTab("leave")}>
          请假申请
        </button>
      </div>
      {tab === "daily" && (
        <ResourcePage
          definition={{
            title: "考勤管理",
            description: "按工作日记录出勤，半天出勤按 0.5 天统计。",
            path: "/attendance",
            key: "attendance",
            dateFilter: true,
            canCreate: true,
            fields: [
              idField,
              dateField,
              {
                key: "status",
                label: "出勤状态",
                type: "select",
                options: [
                  { value: "work", label: "出勤（1 天）" },
                  { value: "partial", label: "半天出勤（0.5 天）" },
                  { value: "absent", label: "缺勤（0 天）" },
                ],
                value: "work",
                required: true,
              },
              notesField,
            ],
            prepare: (b) => ({ ...b, employeeId: b.employee_id }),
            columns: [
              { key: "date", title: "日期" },
              { key: "employee_name", title: "员工" },
              stateColumn(),
              { key: "attendance_days", title: "出勤天数", align: "right" },
              { key: "notes", title: "备注" },
            ],
          }}
        />
      )}{" "}
      {tab === "monthly" && (
        <DataTablePage
          key={month}
          title="月度考勤"
          description="按所选月份的实际考勤与工作日配置汇总。"
          fetchPage={fetchSummary}
          refreshKey={version}
          rowKey={(r) => text(r, "employee_id")}
          filters={
            <Field
              label="考勤月份"
              type="month"
              value={month}
              onChange={(e) => setMonth(e.target.value)}
            />
          }
          columns={[
            { key: "employee_name", title: "员工" },
            { key: "total_workdays", title: "应出勤天数", align: "right" },
            { key: "actual_work_days", title: "实际出勤天数", align: "right" },
            {
              key: "attendance_rate",
              title: "出勤率",
              align: "right",
              render: (r) =>
                `${Math.round(number(r, "attendance_rate") * 100)}%`,
            },
          ]}
          toolbar={
            manage && (
              <Btn
                onClick={async () => {
                  try {
                    await request("POST", "/attendance/monthly-summary/sync", {
                      year,
                      month: m,
                    });
                    setVersion((v) => v + 1);
                    notify("月度考勤已同步");
                  } catch (e) {
                    notify((e as Error).message, true);
                  }
                }}
              >
                同步月度考勤
              </Btn>
            )
          }
        />
      )}{" "}
      {tab === "leave" && (
        <ResourcePage
          definition={{
            title: "请假申请",
            description: "提交请假日期与原因，通过后同步考勤。",
            path: "/attendance/leave-requests",
            key: "leaveRequests",
            canCreate: true,
            fields: [
              {
                key: "startDate",
                label: "开始日期",
                type: "date",
                required: true,
                value: today(),
              },
              {
                key: "endDate",
                label: "结束日期",
                type: "date",
                required: true,
                value: today(),
              },
              {
                key: "leaveType",
                label: "请假类型",
                type: "select",
                options: [
                  { value: "personal", label: "事假" },
                  { value: "sick", label: "病假" },
                  { value: "annual", label: "年假" },
                ],
                required: true,
                value: "personal",
              },
              {
                key: "reason",
                label: "请假原因",
                type: "textarea",
                required: true,
              },
            ],
            columns: [
              { key: "requester_name", title: "申请人" },
              { key: "title", title: "请假内容" },
              stateColumn(),
              {
                key: "requested_at",
                title: "申请日期",
                render: (r) => text(r, "requested_at").slice(0, 10),
              },
            ],
          }}
        />
      )}
    </>
  );
}

export function NotificationsPage() {
  const { user, employees } = useWorkspace();
  return (
    <ResourcePage
      definition={{
        title: "通知中心",
        description: "查看业绩审核、审批结果与团队通知。",
        path: "/notifications",
        key: "notifications",
        canCreate: roles(user, "admin", "hr", "manager", "workplace_manager"),
        fields: [
          { ...idField, key: "employee_id", label: "接收员工" },
          { key: "title", label: "通知标题", required: true },
          {
            key: "message",
            label: "通知内容",
            type: "textarea",
            required: true,
          },
        ],
        prepare: (b) => ({
          ...b,
          userId: employees.find(
            (e) => number(e, "id") === Number(b.employee_id),
          )?.user_id,
        }),
        columns: [
          { key: "title", title: "标题" },
          {
            key: "message",
            title: "内容",
            render: (r) => (
              <span className="wrap-text">{text(r, "message")}</span>
            ),
          },
          {
            key: "is_read",
            title: "阅读状态",
            render: (r) => (r.is_read ? "已读" : "未读"),
          },
          {
            key: "created_at",
            title: "时间",
            render: (r) =>
              new Intl.DateTimeFormat("zh-CN", {
                dateStyle: "short",
                timeStyle: "short",
              }).format(new Date(text(r, "created_at"))),
          },
        ],
        actions: [
          {
            label: "标为已读",
            visible: (r) => !r.is_read,
            run: (r) => request("PUT", `/notifications/${text(r, "id")}/read`),
          },
        ],
        canDelete: true,
      }}
      extraToolbar={<MarkAllRead />}
    />
  );
}
function MarkAllRead() {
  const { notify } = useWorkspace();
  return (
    <Btn
      kind="ghost"
      onClick={async () => {
        try {
          await request("PUT", "/notifications/mark-all-read");
          notify("通知已全部标为已读");
          window.dispatchEvent(new Event("cc4:refresh"));
        } catch (e) {
          notify((e as Error).message, true);
        }
      }}
    >
      全部已读
    </Btn>
  );
}
export function SettingsPage() {
  const { user } = useWorkspace();
  const manage = roles(user, "admin", "workplace_manager");
  return (
    <ResourcePage
      definition={{
        title: "系统设置",
        description: "维护系统配置与对登录用户公开的展示项。",
        path: "/settings",
        key: "settings",
        client: true,
        search: true,
        canCreate: manage,
        canEdit: manage,
        canDelete: manage,
        updatePath: () => "/settings",
        deletePath: (r) => `/settings/${encodeURIComponent(text(r, "key"))}`,
        fields: [
          { key: "key", label: "配置名称", required: true },
          { key: "value", label: "配置值", type: "textarea", required: true },
          { key: "category", label: "分类", value: "general", required: true },
          { key: "description", label: "说明" },
          { key: "is_public", label: "允许所有用户查看", type: "checkbox" },
        ],
        columns: [
          { key: "key", title: "配置名称" },
          {
            key: "value",
            title: "配置值",
            render: (r) => (
              <span className="wrap-text">{text(r, "value")}</span>
            ),
          },
          { key: "category", title: "分类" },
          { key: "description", title: "说明" },
          {
            key: "is_public",
            title: "可见范围",
            render: (r) => (r.is_public ? "所有用户" : "管理人员"),
          },
        ],
      }}
    />
  );
}

export function WechatPage() {
  const { user, notify } = useWorkspace();
  const manage = roles(user, "admin", "manager", "workplace_manager");
  const [sync, setSync] = useState(false);
  const path = "/wechat-leads";
  const d: ResourceDefinition = {
    title: "微信线索",
    description: "查看外呼沉淀的微信客户，分配负责人并记录跟进。",
    path,
    key: "leads",
    canEdit: true,
    fields: [
      { key: "customer_name", label: "客户姓名" },
      {
        key: "status",
        label: "跟进状态",
        type: "select",
        options: statusOptions([...leadStates, "duplicate"]),
        required: true,
      },
      notesField,
      { key: "is_converted", label: "已转化为客户", type: "checkbox" },
    ],
    columns: [
      { key: "customer_name", title: "客户姓名" },
      { key: "real_phone", title: "手机号" },
      {
        key: "data_source",
        title: "来源",
        render: (row) =>
          row.data_source === "mock"
            ? "模拟外呼"
            : row.data_source === "import"
              ? "人工导入"
              : "外呼同步",
      },
      stateColumn(),
      { key: "assigned_employee_name", title: "负责人" },
      { key: "city", title: "城市" },
      { key: "follow_up_count", title: "跟进次数", align: "right" },
      {
        key: "last_follow_up_content",
        title: "最近跟进",
        render: (r) => (
          <span className="wrap-text">
            {text(r, "last_follow_up_content") || "—"}
          </span>
        ),
      },
    ],
    actions: [
      {
        label: "记录跟进",
        fields: [
          {
            key: "content",
            label: "本次跟进内容",
            type: "textarea",
            required: true,
          },
        ],
        run: (r, v) =>
          request("POST", `${path}/${text(r, "id")}/follow-up`, {
            content: v.content,
          }),
      },
      ...(manage
        ? [
            {
              label: "分配",
              fields: [{ ...idField, key: "employeeId" }],
              run: (r: Row, v: Record<string, string | boolean>) =>
                request("PUT", `${path}/${text(r, "id")}/assign`, {
                  employeeId: Number(v.employeeId),
                }),
            },
          ]
        : []),
    ],
  };
  return (
    <>
      <ResourcePage
        definition={d}
        extraToolbar={
          <>
            <Btn
              kind="ghost"
              onClick={async () => {
                try {
                  await download("/wechat-leads/export/full");
                  notify("线索文件已导出");
                } catch (e) {
                  notify((e as Error).message, true);
                }
              }}
            >
              导出线索
            </Btn>
            {manage && <Btn onClick={() => setSync(true)}>同步外呼数据</Btn>}
          </>
        }
      />
      {sync && (
        <ActionDialog
          title="同步外呼数据"
          fields={[
            {
              key: "startDate",
              label: "开始日期",
              type: "date",
              value: today(),
              required: true,
            },
            {
              key: "endDate",
              label: "结束日期",
              type: "date",
              value: today(),
              required: true,
            },
          ]}
          onClose={() => setSync(false)}
          onSave={async (v) => {
            const result = await request<{
              data: {
                processedCount: number;
                savedCount: number;
                duplicateCount: number;
                mock: boolean;
              };
            }>("POST", "/wechat-leads/sync", {
              startTime: `${v.startDate} 00:00:00`,
              endTime: `${v.endDate} 23:59:59`,
            });
            const { processedCount, savedCount, duplicateCount, mock } =
              result.data;
            notify(
              `${mock ? "模拟外呼" : "外呼"}处理 ${processedCount} 通，新增 ${savedCount} 条，重复 ${duplicateCount} 条`,
            );
            window.dispatchEvent(new Event("cc4:refresh"));
          }}
          submitText="同步数据"
        />
      )}
    </>
  );
}

export function FinancePage() {
  const { user, notify } = useWorkspace();
  const [tab, setTab] = useState(
    () => new URLSearchParams(location.search).get("tab") ?? "income",
  );
  const [recurring, setRecurring] = useState(false);
  const tabs = [
    ["income", "收入"],
    ["expense", "支出"],
    ["receivable", "应收与回款"],
    ["cost-types", "成本类型"],
  ];
  const switchTab = (key: string) => {
    setTab(key);
    const url = new URL(location.href);
    url.searchParams.set("tab", key);
    url.searchParams.delete("page");
    history.replaceState(null, "", url);
  };
  const financeFields: FormField[] = [
    {
      key: "category",
      label: "记账分类",
      value: tab === "income" ? "销售收入" : "其他支出",
      required: true,
    },
    {
      key: "amount",
      label: "金额（元）",
      type: "number",
      min: 0.01,
      required: true,
    },
    {
      key: "transactionDate",
      label: "交易日期",
      type: "date",
      value: today(),
      required: true,
    },
    { key: "description", label: "说明", type: "textarea" },
  ];
  const fields: FormField[] =
    tab === "receivable"
      ? [
          { key: "customer", label: "客户名称", required: true },
          {
            key: "amount",
            label: "应收总额（元）",
            type: "number",
            min: 0.01,
            required: true,
          },
          {
            key: "dueDate",
            label: "到期日期",
            type: "date",
            required: true,
            value: today(),
          },
          { key: "accountTitle", label: "款项名称" },
          { key: "invoiceNumber", label: "单据编号" },
          { key: "note", label: "备注", type: "textarea" },
        ]
      : tab === "cost-types"
        ? [
            { key: "name", label: "成本类型名称", required: true },
            {
              key: "category",
              label: "成本分类",
              type: "select",
              required: true,
              value: "other",
              options: [
                { value: "salary", label: "薪资" },
                { value: "operation", label: "运营" },
                { value: "product", label: "产品" },
                { value: "marketing", label: "营销" },
                { value: "other", label: "其他" },
              ],
            },
            { key: "description", label: "说明", type: "textarea" },
          ]
        : financeFields;
  const d: ResourceDefinition = {
    title:
      tab === "receivable"
        ? "应收与回款"
        : tab === "cost-types"
          ? "成本类型"
          : tab === "expense"
            ? "支出管理"
            : "收入管理",
    description:
      tab === "receivable"
        ? "登记应收款，逐笔记录回款并自动生成收入流水。"
        : "核对实际收入与支出，自动流水可回到来源记录调整。",
    path: `/finance/${tab}`,
    key:
      tab === "receivable"
        ? "receivables"
        : tab === "cost-types"
          ? "costTypes"
          : tab,
    dateFilter: ["income", "expense"].includes(tab),
    client: tab === "cost-types",
    canCreate:
      tab === "cost-types" ? roles(user, "admin", "workplace_manager") : true,
    canEdit: ["income", "expense"].includes(tab)
      ? (row) => row.source !== "auto"
      : false,
    canDelete:
      tab === "receivable"
        ? false
        : tab === "cost-types"
          ? roles(user, "admin", "workplace_manager")
          : (row) => row.source !== "auto",
    fields,
    prepare: (body, row) =>
      row && ["income", "expense"].includes(tab)
        ? {
            ...body,
            transactionDate: body.transactionDate || row.transaction_date,
          }
        : body,
    columns:
      tab === "cost-types"
        ? [
            { key: "name", title: "成本类型" },
            {
              key: "category",
              title: "分类",
              render: (r) =>
                STATUS_ZH[text(r, "category")] ?? text(r, "category"),
            },
            { key: "description", title: "说明" },
          ]
        : tab === "receivable"
          ? [
              { key: "customer", title: "客户" },
              amountColumn("amount", "应收总额"),
              amountColumn("paid_amount", "已回款", "in"),
              amountColumn("outstanding_amount", "剩余应收"),
              { key: "due_date", title: "到期日期" },
              stateColumn("payment_status", "回款状态"),
            ]
          : [
              { key: "transaction_date", title: "日期" },
              {
                key: "category",
                title: "分类",
                render: (r) =>
                  STATUS_ZH[text(r, "category")] ?? text(r, "category"),
              },
              amountColumn("amount", "金额", tab === "expense" ? "out" : "in"),
              {
                key: "description",
                title: "说明",
                render: (r) => (
                  <span className="wrap-text">{text(r, "description")}</span>
                ),
              },
              { key: "recorder", title: "记录人" },
              stateColumn(),
            ],
    actions:
      tab === "receivable"
        ? [
            {
              label: "录入回款",
              visible: (r) => number(r, "outstanding_amount") > 0,
              fields: [
                {
                  key: "amount",
                  label: "本次回款（元）",
                  type: "number",
                  min: 0.01,
                  required: true,
                },
                {
                  key: "paymentDate",
                  label: "回款日期",
                  type: "date",
                  value: today(),
                  required: true,
                },
                {
                  key: "paymentMethod",
                  label: "收款方式",
                  type: "select",
                  value: "bank",
                  options: [
                    { value: "bank", label: "银行转账" },
                    { value: "wechat", label: "微信" },
                    { value: "cash", label: "现金" },
                    { value: "other", label: "其他" },
                  ],
                },
                { key: "note", label: "备注", type: "textarea" },
              ],
              run: (r, v) =>
                request(
                  "POST",
                  `/finance/receivable/${text(r, "id")}/payments`,
                  { ...v, amount: Number(v.amount) },
                ),
            },
          ]
        : [],
  };
  return (
    <>
      <div className="tabs" aria-label="财务分类">
        {tabs.map(([key, label]) => (
          <button
            key={key}
            aria-pressed={tab === key}
            onClick={() => switchTab(key)}
          >
            {label}
          </button>
        ))}
      </div>
      <ResourcePage
        key={tab}
        definition={d}
        extraToolbar={
          tab === "expense" && (
            <Btn kind="ghost" onClick={() => setRecurring(true)}>
              新增周期支出
            </Btn>
          )
        }
        detail={
          tab === "receivable"
            ? (row, close) => (
                <DetailTable
                  path={`/finance/receivable/${text(row, "id")}/payments`}
                  itemKey="payments"
                  title={`${text(row, "customer")}的回款记录`}
                  onClose={close}
                  columns={[
                    { key: "payment_date", title: "回款日期" },
                    amountColumn("amount", "回款金额", "in"),
                    { key: "payment_method", title: "收款方式" },
                    { key: "note", title: "备注" },
                  ]}
                />
              )
            : undefined
        }
      />
      {recurring && (
        <ActionDialog
          title="新增周期支出"
          fields={[
            {
              key: "category",
              label: "记账分类",
              value: "operation",
              required: true,
            },
            {
              key: "totalAmount",
              label: "周期总金额（元）",
              type: "number",
              min: 0.01,
              required: true,
            },
            {
              key: "frequency",
              label: "均摊周期",
              type: "select",
              options: [
                { value: "daily", label: "按起止日期均摊" },
                { value: "monthly", label: "按月（30 天）均摊" },
                { value: "yearly", label: "按年（365 天）均摊" },
              ],
              value: "monthly",
              required: true,
            },
            {
              key: "startDate",
              label: "开始日期",
              type: "date",
              value: today(),
              required: true,
            },
            { key: "endDate", label: "结束日期", type: "date" },
            { key: "description", label: "说明", type: "textarea" },
          ]}
          onClose={() => setRecurring(false)}
          onSave={async (v) => {
            await request("POST", "/finance/expense/recurring", {
              ...v,
              totalAmount: Number(v.totalAmount),
            });
            notify("周期支出已创建");
            window.dispatchEvent(new Event("cc4:refresh"));
          }}
        />
      )}
    </>
  );
}

export function PermissionsPage() {
  const { notify } = useWorkspace();
  const [selected, setSelected] = useState(""),
    [allowed, setAllowed] = useState<string[]>([]),
    [busy, setBusy] = useState(false),
    [create, setCreate] = useState(false);
  const positions = useRemote(
      () => request<{ data: Row[] }>("GET", "/permissions/positions"),
      [],
    ),
    modules = useRemote(
      () => request<{ data: Row[] }>("GET", "/permissions/modules"),
      [],
    );
  const config = useRemote(async () => {
    if (!selected) return null;
    const r = await request<{
      data: { allowedModules: string[]; configured: boolean };
    }>("GET", `/permissions/position/${encodeURIComponent(selected)}`);
    setAllowed(r.data.allowedModules);
    return r.data;
  }, [selected]);
  return (
    <>
      <SimpleHeader
        title="职位权限"
        description="按职位配置可见模块；个人中心始终保留。"
      >
        <Btn onClick={() => setCreate(true)}>新增职位</Btn>
      </SimpleHeader>
      {positions.error && (
        <Alert retry={positions.reload}>{positions.error}</Alert>
      )}
      <div className="filters">
        <SelectField
          label="选择职位"
          value={selected}
          onChange={(e) => setSelected(e.target.value)}
        >
          <option value="">请选择职位…</option>
          {positions.data?.data.map((p) => (
            <option key={text(p, "id")} value={text(p, "name")}>
              {text(p, "name")}（{number(p, "employee_count")} 人）
            </option>
          ))}
        </SelectField>
      </div>
      {config.error && <Alert retry={config.reload}>{config.error}</Alert>}
      {selected && modules.data && (
        <section className="panel">
          <h2>{selected}的可见模块</h2>
          <p className="muted">
            {config.data?.configured
              ? "正在使用该职位的自定义配置。"
              : "目前使用角色默认配置，保存后按此处的选择生效。"}
          </p>
          <div className="permission-grid">
            {modules.data.data.map((m) => (
              <label className="checkbox-line" key={text(m, "key")}>
                <input
                  type="checkbox"
                  checked={allowed.includes(text(m, "key"))}
                  disabled={Boolean(m.mandatory) || busy || config.loading}
                  onChange={(e) =>
                    setAllowed((v) =>
                      e.target.checked
                        ? [...v, text(m, "key")]
                        : v.filter((key) => key !== text(m, "key")),
                    )
                  }
                />
                {text(m, "label")}
              </label>
            ))}
          </div>
          <div className="actions" style={{ marginTop: 24 }}>
            <Btn
              disabled={busy || config.loading}
              onClick={async () => {
                setBusy(true);
                try {
                  await request(
                    "PUT",
                    `/permissions/position/${encodeURIComponent(selected)}`,
                    { allowedModules: allowed },
                  );
                  notify("职位权限已保存");
                  config.reload();
                  positions.reload();
                } catch (e) {
                  notify((e as Error).message, true);
                } finally {
                  setBusy(false);
                }
              }}
            >
              {busy ? "保存中…" : "保存权限"}
            </Btn>
            <Btn
              kind="ghost"
              disabled={busy}
              onClick={async () => {
                setBusy(true);
                try {
                  await request(
                    "DELETE",
                    `/permissions/position/${encodeURIComponent(selected)}`,
                  );
                  config.reload();
                  notify("已恢复角色默认权限");
                } catch (e) {
                  notify((e as Error).message, true);
                } finally {
                  setBusy(false);
                }
              }}
            >
              恢复默认
            </Btn>
          </div>
        </section>
      )}
      {create && (
        <ActionDialog
          title="新增职位"
          fields={[
            { key: "name", label: "职位名称", required: true },
            {
              key: "role_hint",
              label: "默认角色",
              type: "select",
              value: "employee",
              options: [
                { value: "employee", label: "销售" },
                { value: "manager", label: "团队经理" },
                { value: "workplace_manager", label: "职场经理" },
                { value: "hr", label: "人力" },
                { value: "finance", label: "财务" },
              ],
              required: true,
            },
          ]}
          onClose={() => setCreate(false)}
          onSave={async (v) => {
            await request("POST", "/permissions/positions", v);
            positions.reload();
            setSelected(String(v.name));
            notify("职位已创建");
          }}
        />
      )}
    </>
  );
}
