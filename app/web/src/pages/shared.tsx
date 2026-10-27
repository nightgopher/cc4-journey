import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import {
  Alert,
  Btn,
  Column,
  DataTablePage,
  Field,
  FormDialog,
  MoneyCell,
  RowMenu,
  SelectField,
  Status,
  TextAreaField,
} from "../components/kit";
import { collection, queryString, request, Row, User } from "../lib/api";
import { STATUS_ZH } from "../lib/tokens";
import { businessDate } from "../lib/business-date";

export const text = (row: Row, key: string) =>
  row[key] == null ? "" : String(row[key]);
export const number = (row: Row, key: string) => Number(row[key] ?? 0);
export const today = () => businessDate();
export const roles = (user: User, ...allowed: string[]) =>
  allowed.includes(user.role);
export type Notice = (message: string, error?: boolean) => void;
export const WorkspaceContext = createContext<{
  user: User;
  notify: Notice;
  employees: Row[];
  products: Row[];
  teams: Row[];
  approvers: Row[];
  refresh: () => void;
}>({
  user: {
    id: 0,
    name: null,
    role: "employee",
    employee_id: null,
    phone: null,
    email: null,
    level: null,
    position: null,
    team_id: null,
  },
  notify: () => {},
  employees: [],
  products: [],
  teams: [],
  approvers: [],
  refresh: () => {},
});
export const useWorkspace = () => useContext(WorkspaceContext);

export function useRemote<T>(
  load: () => Promise<T>,
  deps: React.DependencyList,
) {
  const [version, setVersion] = useState(0);
  const key = useMemo(() => ({}), [...deps, version]);
  const [state, setState] = useState<{
    key: object;
    data: T | null;
    error: string;
    loading: boolean;
  }>({ key, data: null, error: "", loading: true });
  useEffect(() => {
    let live = true;
    setState({ key, data: null, error: "", loading: true });
    load()
      .then((v) => {
        if (live) setState({ key, data: v, error: "", loading: false });
      })
      .catch((e: Error) => {
        if (live)
          setState({
            key,
            data: null,
            error:
              e instanceof TypeError
                ? "网络连接失败，请检查连接后重试"
                : e.message,
            loading: false,
          });
      });
    return () => {
      live = false;
    };
  }, [key]);
  // Hide previous results in the render that changes parameters, before the
  // next effect runs; a failed request must not relabel an older result.
  const current =
    state.key === key ? state : { data: null, error: "", loading: true };
  return { ...current, reload: () => setVersion((v) => v + 1) };
}
export const statusOptions = (values: string[]) =>
  values.map((value) => ({ value, label: STATUS_ZH[value] ?? value }));
export interface FormField {
  key: string;
  label: string;
  type?:
    | "text"
    | "tel"
    | "email"
    | "date"
    | "number"
    | "password"
    | "textarea"
    | "select"
    | "employee"
    | "approver"
    | "product"
    | "team"
    | "checkbox"
    | "tiers";
  required?: boolean;
  options?: { value: string; label: string }[];
  value?: string | boolean;
  min?: number;
  step?: number;
  hint?: string;
  disabled?: boolean;
  createOnly?: boolean;
}
export type Values = Record<string, string | boolean>;
export function formDefaults(fields: FormField[], row?: Row): Values {
  const out: Values = {};
  for (const field of fields) {
    const value = row?.[field.key];
    out[field.key] =
      value == null
        ? (field.value ?? (field.type === "checkbox" ? false : ""))
        : field.type === "checkbox"
          ? Boolean(value)
          : field.type === "tiers"
            ? typeof value === "string"
              ? value
              : JSON.stringify(value)
            : String(value).slice(0, field.type === "date" ? 10 : undefined);
  }
  return out;
}
export function valuesBody(fields: FormField[], values: Values): Row {
  const out: Row = {};
  for (const f of fields) {
    const v = values[f.key];
    out[f.key] =
      f.type === "number" ||
      f.type === "employee" ||
      f.type === "approver" ||
      f.type === "product" ||
      f.type === "team"
        ? v === ""
          ? null
          : Number(v)
        : f.type === "tiers"
          ? v
            ? JSON.parse(String(v))
            : null
          : v;
  }
  return out;
}

function TierEditor({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) {
  type Tier = { from: number; to: number | null; amount: number };
  let tiers: Tier[] = [];
  try {
    tiers = JSON.parse(value || "{}").tiers ?? [];
  } catch {
    /* invalid persisted rules are replaced through the form */
  }
  const save = (next: Tier[]) => onChange(JSON.stringify({ tiers: next }));
  return (
    <fieldset className="tier-editor">
      <legend>订单提成档位</legend>
      <p className="muted">
        按员工当日的产品订单序号匹配；结束序号留空表示无上限。
      </p>
      {tiers.map((tier, i) => (
        <div className="tier-row" key={i}>
          <Field
            label="开始序号"
            type="number"
            min={1}
            required
            value={tier.from}
            onChange={(e) =>
              save(
                tiers.map((v, n) =>
                  n === i ? { ...v, from: Number(e.target.value) } : v,
                ),
              )
            }
          />
          <Field
            label="结束序号"
            type="number"
            min={tier.from}
            value={tier.to ?? ""}
            onChange={(e) =>
              save(
                tiers.map((v, n) =>
                  n === i
                    ? {
                        ...v,
                        to:
                          e.target.value === "" ? null : Number(e.target.value),
                      }
                    : v,
                ),
              )
            }
          />
          <Field
            label="每单提成（元）"
            type="number"
            min={0}
            required
            value={tier.amount}
            onChange={(e) =>
              save(
                tiers.map((v, n) =>
                  n === i ? { ...v, amount: Number(e.target.value) } : v,
                ),
              )
            }
          />
          <Btn
            kind="ghost"
            aria-label={`移除第 ${i + 1} 个档位`}
            onClick={() => save(tiers.filter((_, n) => n !== i))}
          >
            移除
          </Btn>
        </div>
      ))}
      <Btn
        kind="ghost"
        onClick={() =>
          save([
            ...tiers,
            {
              from: tiers.length
                ? (tiers.at(-1)?.to ?? tiers.at(-1)!.from) + 1
                : 1,
              to: null,
              amount: 0,
            },
          ])
        }
      >
        添加档位
      </Btn>
    </fieldset>
  );
}
export function FormFields({
  fields,
  values,
  setValues,
}: {
  fields: FormField[];
  values: Values;
  setValues: React.Dispatch<React.SetStateAction<Values>>;
}) {
  const { employees, products, teams, approvers } = useWorkspace();
  const set = (key: string, v: string | boolean) =>
    setValues((prev) => ({ ...prev, [key]: v }));
  return (
    <>
      {fields.map((f) => {
        const value = values[f.key] ?? "";
        if (f.type === "tiers")
          return (
            <TierEditor
              key={f.key}
              value={String(value)}
              onChange={(v) => set(f.key, v)}
            />
          );
        if (f.type === "checkbox")
          return (
            <label className="checkbox-line" key={f.key}>
              <input
                type="checkbox"
                name={f.key}
                checked={Boolean(value)}
                onChange={(e) => set(f.key, e.target.checked)}
                disabled={f.disabled}
              />
              {f.label}
            </label>
          );
        if (
          ["select", "employee", "approver", "product", "team"].includes(
            f.type ?? "",
          )
        ) {
          const rows =
            f.type === "employee"
              ? employees
              : f.type === "approver"
                ? approvers
                : f.type === "product"
                  ? products
                  : teams;
          const options =
            f.type === "select"
              ? (f.options ?? [])
              : rows.map((r) => ({
                  value: text(r, "id"),
                  label: `${text(r, "name")}${f.type === "employee" && r.employee_code ? `（${text(r, "employee_code")}）` : ""}`,
                }));
          return (
            <SelectField
              key={f.key}
              label={f.label}
              name={f.key}
              required={f.required}
              disabled={f.disabled}
              value={String(value)}
              onChange={(e) => set(f.key, e.target.value)}
            >
              <option value="">{f.required ? "请选择…" : "不指定"}</option>
              {options.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </SelectField>
          );
        }
        if (f.type === "textarea")
          return (
            <TextAreaField
              key={f.key}
              label={f.label}
              name={f.key}
              required={f.required}
              value={String(value)}
              onChange={(e) => set(f.key, e.target.value)}
            />
          );
        return (
          <Field
            key={f.key}
            label={f.label}
            name={f.key}
            type={f.type ?? "text"}
            required={f.required}
            value={String(value)}
            disabled={f.disabled}
            min={f.min}
            minLength={f.type === "password" ? 6 : undefined}
            step={f.step ?? (f.type === "number" ? "0.01" : undefined)}
            hint={f.hint}
            autoComplete={f.type === "password" ? "new-password" : "off"}
            onChange={(e) => set(f.key, e.target.value)}
          />
        );
      })}
    </>
  );
}
export function ActionDialog({
  title,
  fields,
  initial,
  onClose,
  onSave,
  submitText,
  description,
}: {
  title: string;
  fields: FormField[];
  initial?: Row;
  onClose: () => void;
  onSave: (values: Values) => Promise<void>;
  submitText?: string;
  description?: string;
}) {
  const defaults = useMemo(() => formDefaults(fields, initial), []);
  const [values, setValues] = useState<Values>(defaults);
  return (
    <FormDialog
      open
      title={title}
      onClose={onClose}
      onSubmit={() => onSave(values)}
      submitText={submitText}
      dirty={JSON.stringify(values) !== JSON.stringify(defaults)}
    >
      {description && <p>{description}</p>}
      <FormFields fields={fields} values={values} setValues={setValues} />
    </FormDialog>
  );
}
export interface RowAction {
  label: string;
  kind?: "ghost" | "primary" | "ghost-danger";
  fields?: FormField[];
  visible?: (row: Row) => boolean;
  confirm?: string;
  run: (row: Row, values: Values) => Promise<unknown>;
}
export interface ResourceDefinition {
  title: string;
  description: string;
  path: string;
  key: string;
  fields: FormField[];
  columns: Column<Row>[];
  client?: boolean;
  search?: boolean;
  searchPlaceholder?: string;
  statuses?: string[];
  dateFilter?: boolean;
  canCreate?: boolean;
  canEdit?: boolean | ((row: Row) => boolean);
  canDelete?: boolean | ((row: Row) => boolean);
  actions?: RowAction[];
  prepare?: (body: Row, row?: Row) => Row;
  createPath?: string;
  updatePath?: (row: Row) => string;
  deletePath?: (row: Row) => string;
  createLabel?: string;
}
export function ResourcePage({
  definition: d,
  extraToolbar,
  detail,
}: {
  definition: ResourceDefinition;
  extraToolbar?: React.ReactNode;
  detail?: (row: Row, close: () => void) => React.ReactNode;
}) {
  const { notify, refresh } = useWorkspace();
  const query = new URLSearchParams(location.search);
  const [search, setSearch] = useState(query.get("q") ?? ""),
    [status, setStatus] = useState(query.get("status") ?? ""),
    [start, setStart] = useState(query.get("start") ?? ""),
    [end, setEnd] = useState(query.get("end") ?? "");
  const [filters, setFilters] = useState({ search, status, start, end }),
    [version, setVersion] = useState(0);
  const [edit, setEdit] = useState<Row | null | undefined>(),
    [remove, setRemove] = useState<Row | null>(null),
    [action, setAction] = useState<{ row: Row; action: RowAction } | null>(
      null,
    ),
    [detailRow, setDetailRow] = useState<Row | null>(null);
  useEffect(() => {
    const refresh = () => setVersion((v) => v + 1);
    addEventListener("cc4:refresh", refresh);
    return () => removeEventListener("cc4:refresh", refresh);
  }, []);
  const editingFields = d.fields.filter((f) => !edit || !f.createOnly);
  const updated = (message: string) => {
    setVersion((v) => v + 1);
    refresh();
    notify(message);
  };
  const fetchPage = useCallback(
    async (page: number, limit: number) => {
      const params = {
        search: filters.search,
        status: filters.status,
        start_date: filters.start,
        end_date: filters.end,
      };
      if (!d.client) return collection(d.path, d.key, page, limit, params);
      const result = await request<Record<string, Row[]>>("GET", d.path);
      let rows = result[d.key] ?? [];
      if (filters.search)
        rows = rows.filter((row) =>
          Object.values(row).some((v) =>
            String(v ?? "")
              .toLocaleLowerCase()
              .includes(filters.search.toLocaleLowerCase()),
          ),
        );
      if (filters.status)
        rows = rows.filter((row) => row.status === filters.status);
      return {
        rows: rows.slice((page - 1) * limit, page * limit),
        total: rows.length,
      };
    },
    [d.path, d.key, d.client, filters],
  );
  const columns: Column<Row>[] = [...d.columns];
  if (d.canEdit || d.canDelete || d.actions?.length || detail)
    columns.push({
      key: "actions",
      title: "操作",
      render: (row) => {
        const canEditRow =
          typeof d.canEdit === "function" ? d.canEdit(row) : d.canEdit;
        const canDeleteRow =
          typeof d.canDelete === "function" ? d.canDelete(row) : d.canDelete;
        const items: {
          key: string;
          label: string;
          kind?: "ghost" | "primary" | "ghost-danger";
          run: () => void;
        }[] = [
          ...(canEditRow
            ? [{ key: "edit", label: "编辑", kind: "ghost" as const, run: () => setEdit(row) }]
            : []),
          ...(detail
            ? [
                {
                  key: "detail",
                  label: "查看详情",
                  kind: "ghost" as const,
                  run: () => setDetailRow(row),
                },
              ]
            : []),
          ...(d.actions ?? [])
            .filter((a) => !a.visible || a.visible(row))
            .map((a) => ({
              key: a.label,
              label: a.label,
              kind: a.kind,
              run: () => setAction({ row, action: a }),
            })),
          ...(canDeleteRow
            ? [
                {
                  key: "delete",
                  label: "删除",
                  kind: "ghost-danger" as const,
                  run: () => setRemove(row),
                },
              ]
            : []),
        ];
        // 超过 3 个操作时收敛：主操作 + 首个常规操作保留在行内，其余进「更多」
        const overflow = items.length > 3;
        const primaries = items.filter((i) => i.kind === "primary");
        const ghostFirst = items.find(
          (i) => i.kind !== "primary" && i.kind !== "ghost-danger",
        );
        const inline = overflow
          ? [...primaries, ...(ghostFirst ? [ghostFirst] : [])]
          : items;
        const rest = overflow ? items.filter((i) => !inline.includes(i)) : [];
        return (
          <div className="row-actions">
            {inline.map((i) => (
              <Btn key={i.key} kind={i.kind ?? "ghost"} onClick={i.run}>
                {i.label}
              </Btn>
            ))}
            {rest.length > 0 && (
              <RowMenu
                items={rest.map((i) => ({
                  label: i.label,
                  danger: i.kind === "ghost-danger",
                  onSelect: i.run,
                }))}
              />
            )}
          </div>
        );
      },
    });
  const filterForm =
    d.search || d.statuses || d.dateFilter ? (
      <form
        className="filter-form"
        onSubmit={(e) => {
          e.preventDefault();
          setFilters({ search, status, start, end });
          const url = new URL(location.href);
          for (const [k, v] of Object.entries({
            q: search,
            status,
            start,
            end,
          })) {
            v ? url.searchParams.set(k, v) : url.searchParams.delete(k);
          }
          url.searchParams.delete("page");
          history.replaceState(null, "", url);
        }}
      >
        {d.search && (
          <Field
            label="搜索"
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={d.searchPlaceholder ?? "输入姓名、手机号或关键词…"}
          />
        )}
        {d.statuses && (
          <SelectField
            label="状态"
            value={status}
            onChange={(e) => setStatus(e.target.value)}
          >
            <option value="">全部状态</option>
            {statusOptions(d.statuses).map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </SelectField>
        )}
        {d.dateFilter && (
          <>
            <Field
              label="开始日期"
              type="date"
              value={start}
              onChange={(e) => setStart(e.target.value)}
            />
            <Field
              label="结束日期"
              type="date"
              min={start}
              value={end}
              onChange={(e) => setEnd(e.target.value)}
            />
          </>
        )}
        <Btn type="submit" kind="ghost">
          筛选
        </Btn>
      </form>
    ) : undefined;
  return (
    <>
      <DataTablePage
        key={`${d.path}:${JSON.stringify(filters)}`}
        title={d.title}
        description={d.description}
        fetchPage={fetchPage}
        columns={columns}
        rowKey={(row) => text(row, "id") || text(row, "key")}
        refreshKey={version}
        emptyText={
          Object.values(filters).some(Boolean)
            ? "未找到符合条件的记录，请调整筛选。"
            : d.canCreate
              ? "暂无记录，点击右上角按钮创建第一条。"
              : "暂无记录。"
        }
        filters={filterForm}
        toolbar={
          <>
            {extraToolbar}
            {d.canCreate && (
              <Btn onClick={() => setEdit(null)}>
                {d.createLabel ?? "新建记录"}
              </Btn>
            )}
          </>
        }
      />
      {edit !== undefined && (
        <ActionDialog
          key={edit ? text(edit, "id") || text(edit, "key") : "new"}
          title={`${edit ? "编辑" : "新建"}${d.title.replace("管理", "")}`}
          fields={editingFields.map((f) => ({
            ...f,
            disabled: f.disabled || (!!edit && f.key === "key"),
          }))}
          initial={edit ?? undefined}
          onClose={() => setEdit(undefined)}
          onSave={async (values) => {
            let body = valuesBody(editingFields, values);
            if (d.prepare) body = d.prepare(body, edit ?? undefined);
            await request(
              edit ? "PUT" : "POST",
              edit
                ? (d.updatePath?.(edit) ?? `${d.path}/${text(edit, "id")}`)
                : (d.createPath ?? d.path),
              body,
            );
            updated("记录已保存");
          }}
        />
      )}
      {remove && (
        <FormDialog
          open
          title="删除记录"
          onClose={() => setRemove(null)}
          onSubmit={async () => {
            await request(
              "DELETE",
              d.deletePath?.(remove) ?? `${d.path}/${text(remove, "id")}`,
            );
            updated("记录已删除");
            setRemove(null);
          }}
          submitText="确认删除"
        >
          <p>
            将删除“{text(remove, d.columns[0].key) || text(remove, "id")}
            ”。关联记录可能阻止删除，请核对后继续。
          </p>
        </FormDialog>
      )}
      {action && (
        <ActionDialog
          title={action.action.label}
          description={action.action.confirm}
          fields={action.action.fields ?? []}
          onClose={() => setAction(null)}
          onSave={async (values) => {
            await action.action.run(action.row, values);
            updated(`${action.action.label}已完成`);
          }}
          submitText={action.action.label}
        />
      )}
      {detailRow && detail && (
        <section className="detail-panel">
          {detail(detailRow, () => setDetailRow(null))}
        </section>
      )}
    </>
  );
}
export const idField: FormField = {
  key: "employee_id",
  label: "员工",
  type: "employee",
  required: true,
};
export const dateField: FormField = {
  key: "date",
  label: "日期",
  type: "date",
  required: true,
  value: today(),
};
export const amountColumn = (
  key: string,
  title: string,
  tone?: "in" | "out",
): Column<Row> => ({
  key,
  title,
  align: "right",
  render: (r) => <MoneyCell v={number(r, key)} tone={tone} />,
});
export const stateColumn = (
  key = "status",
  title = "状态",
  labels?: Record<string, string>,
): Column<Row> => ({
  key,
  title,
  render: (r) => <Status value={text(r, key)} label={labels?.[text(r, key)]} />,
});
export function DetailTable({
  path,
  itemKey,
  columns,
  title,
  onClose,
}: {
  path: string;
  itemKey: string;
  columns: Column<Row>[];
  title: string;
  onClose: () => void;
}) {
  const fetchPage = useCallback(
    (page: number, limit: number) => collection(path, itemKey, page, limit),
    [path, itemKey],
  );
  return (
    <DataTablePage
      title={title}
      fetchPage={fetchPage}
      columns={columns}
      rowKey={(r) => text(r, "id")}
      toolbar={
        <Btn kind="ghost" onClick={onClose}>
          收起详情
        </Btn>
      }
      emptyText="还没有相关记录。"
    />
  );
}
export function SimpleHeader({
  title,
  description,
  children,
}: {
  title: string;
  description?: string;
  children?: React.ReactNode;
}) {
  return (
    <header className="page-heading">
      <div>
        <h1>{title}</h1>
        {description && <p>{description}</p>}
      </div>
      <div className="actions">{children}</div>
    </header>
  );
}
