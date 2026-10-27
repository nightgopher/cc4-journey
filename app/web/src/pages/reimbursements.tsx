import React, { useCallback, useEffect, useRef, useState } from "react";
import {
  Alert,
  Btn,
  Column,
  DataTablePage,
  Field,
  FormDialog,
  SelectField,
  Status,
  TextAreaField,
} from "../components/kit";
import { BASE, getToken, request } from "../lib/api";
import { money } from "../lib/tokens";
import { SimpleHeader, today, useRemote, useWorkspace } from "./shared";

type Fields = {
  version: number;
  title: string;
  category: string;
  merchant: string;
  invoiceNumber: string;
  invoiceCode: string;
  invoiceDate: string;
  currency: string;
  invoiceTotal: string;
  amount: string;
  reason: string;
};
type Recognition = {
  provider: string;
  model: string;
  text: string;
  warnings: string[];
  fields: {
    merchant: string;
    invoiceNumber: string;
    invoiceCode: string;
    invoiceDate: string;
    currency: string;
    category: string;
    total: number | null;
  };
};
type Claim = {
  version: number;
  id: number;
  employee_id: number;
  employee_name: string;
  title: string;
  status: string;
  category: string;
  merchant: string;
  invoice_number: string;
  invoice_code: string;
  invoice_date: string | null;
  currency: string;
  invoice_total: number | null;
  amount: number | null;
  reason: string;
  review_note: string;
  receipt_url: string;
  receipt_mime: string;
  ocr_status: string;
  ocr_error: string | null;
  ocr_result: Recognition | null;
  ocr_progress: { totalPages?: number; extractedPages?: number };
  finance_record_id: number | null;
  payment_date: string | null;
  updated_at: string;
};
const labels: Record<string, string> = {
  draft: "草稿",
  submitted: "待审批",
  approved: "待付款",
  rejected: "已驳回",
  paid: "已付款",
  cancelled: "已撤销",
};
const categories: Record<string, string> = {
  office: "办公用品",
  transport: "交通出行",
  travel: "差旅住宿",
  meals: "餐饮",
  other: "其他费用",
};
const ocrLabels: Record<string, string> = {
  queued: "等待识别",
  submitting: "正在提交票据",
  running: "正在识别票据",
  ready: "识别已完成",
  failed: "识别未完成",
};
const activeOCR = (claim: Claim) =>
  ["queued", "submitting", "running"].includes(claim.ocr_status);
const claimStatus = (status: string) => (
  <Status
    value={
      (
        {
          submitted: "pending",
          approved: "approved",
          paid: "confirmed",
          rejected: "rejected",
          cancelled: "inactive",
        } as Record<string, string>
      )[status] ?? "pending"
    }
    label={labels[status] ?? status}
  />
);
const fieldsFor = (r: Claim): Fields => ({
  version: r.version,
  title: r.title,
  category: r.category,
  merchant: r.merchant,
  invoiceNumber: r.invoice_number,
  invoiceCode: r.invoice_code,
  invoiceDate: r.invoice_date ?? "",
  currency: r.currency,
  invoiceTotal: r.invoice_total == null ? "" : String(r.invoice_total),
  amount: r.amount == null ? "" : String(r.amount),
  reason: r.reason,
});

export function ReimbursementsPage() {
  const { user, notify } = useWorkspace();
  const [selected, setSelected] = useState<number | null>(
    () => Number(new URLSearchParams(location.search).get("id")) || null,
  );
  const [scope, setScope] = useState("mine");
  const [status, setStatus] = useState("");
  const [upload, setUpload] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const reviewer = [
    "admin",
    "finance",
    "manager",
    "workplace_manager",
  ].includes(user.role);
  const config = useRemote(
    () => request<{ configured: boolean }>("GET", "/reimbursements/config"),
    [],
  );
  const open = (id: number | null) => {
    setSelected(id);
    const u = new URL(location.href);
    if (id) u.searchParams.set("id", String(id));
    else u.searchParams.delete("id");
    u.searchParams.delete("page");
    history.replaceState(null, "", u);
    window.scrollTo(0, 0);
  };
  const fetchPage = useCallback(
    async (page: number, limit: number) => {
      const r = await request<{
        claims: Claim[];
        pagination: { total: number };
      }>(
        "GET",
        `/reimbursements?scope=${scope}&status=${status}&page=${page}&limit=${limit}`,
      );
      return { rows: r.claims, total: r.pagination.total };
    },
    [scope, status],
  );
  const columns: Column<Claim>[] = [
    {
      key: "id",
      title: "报销单",
      render: (r) => (
        <Btn kind="ghost" onClick={() => open(r.id)}>
          BX-{String(r.id).padStart(6, "0")}
        </Btn>
      ),
    },
    { key: "employee_name", title: "申请人" },
    { key: "merchant", title: "商家", render: (r) => r.merchant || "待核对" },
    {
      key: "category",
      title: "费用类别",
      render: (r) => categories[r.category],
    },
    {
      key: "amount",
      title: "申请金额",
      align: "right",
      render: (r) => (r.amount == null ? "待填写" : money(r.amount)),
    },
    {
      key: "invoice_date",
      title: "票据日期",
      render: (r) => r.invoice_date || "待填写",
    },
    {
      key: "ocr_status",
      title: "识别进度",
      render: (r) => ocrLabels[r.ocr_status],
    },
    { key: "status", title: "报销状态", render: (r) => claimStatus(r.status) },
  ];
  if (selected)
    return (
      <ClaimDetail key={selected} id={selected} onBack={() => open(null)} />
    );
  const sampleBase =
    (import.meta as ImportMeta & { env?: Record<string, string> }).env
      ?.BASE_URL ?? "/";
  return (
    <>
      <SimpleHeader
        title="智能报销"
        description="上传票据、核对识别结果，审批通过后由财务登记付款。"
      >
        <a
          className="btn btn--ghost"
          href={`${sampleBase}samples/invoice-demo.png`}
          download
        >
          下载合成示例票据
        </a>
        <Btn onClick={() => setUpload(true)}>上传票据</Btn>
      </SimpleHeader>
      {config.data?.configured === false && (
        <Alert tone="info">
          识别服务暂未就绪，可先保存票据并人工填写，或稍后重新识别。
        </Alert>
      )}
      {config.error && <Alert retry={config.reload}>{config.error}</Alert>}
      <nav className="tabs" aria-label="报销范围">
        <Btn
          kind={scope === "mine" ? "primary" : "ghost"}
          onClick={() => setScope("mine")}
        >
          我的报销
        </Btn>
        {reviewer && (
          <>
            <Btn
              kind={scope === "review" ? "primary" : "ghost"}
              onClick={() => setScope("review")}
            >
              待我处理
            </Btn>
            <Btn
              kind={scope === "all" ? "primary" : "ghost"}
              onClick={() => setScope("all")}
            >
              全部可见
            </Btn>
          </>
        )}
      </nav>
      <DataTablePage<Claim>
        key={`${scope}-${status}`}
        title="报销记录"
        description="每笔报销保留票据、识别建议与处理记录。"
        fetchPage={fetchPage}
        columns={columns}
        rowKey={(r) => r.id}
        filters={
          <SelectField
            label="报销状态"
            value={status}
            onChange={(e) => setStatus(e.target.value)}
          >
            <option value="">全部状态</option>
            {Object.entries(labels).map(([v, label]) => (
              <option key={v} value={v}>
                {label}
              </option>
            ))}
          </SelectField>
        }
        emptyText="还没有报销记录，可以先上传一张票据。"
      />
      {upload && (
        <FormDialog
          open
          title="上传报销票据"
          submitText="保存并开始识别"
          dirty={!!file}
          onClose={() => {
            setUpload(false);
            setFile(null);
          }}
          onSubmit={async () => {
            if (!file) throw new Error("请选择票据文件");
            if (file.size > 8 * 1024 * 1024)
              throw new Error("文件不能超过8MiB");
            const body = new FormData();
            body.append("receipt", file);
            const result = await request<{ id: number }>(
              "POST",
              "/reimbursements",
              body,
            );
            notify("票据已保存，识别完成后请核对并填写用途");
            setFile(null);
            open(result.id);
          }}
        >
          <Field
            label="票据文件"
            type="file"
            accept="image/png,image/jpeg,application/pdf"
            required
            hint="支持PNG、JPEG或单页PDF，最多8MiB；每笔报销上传一张票据。"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
          <p className="muted">
            识别结果用于辅助填写，核对确认后才可提交审批。
          </p>
        </FormDialog>
      )}
    </>
  );
}

function ClaimDetail({ id, onBack }: { id: number; onBack: () => void }) {
  const { user, notify } = useWorkspace();
  const [claim, setClaim] = useState<Claim | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [blob, setBlob] = useState("");
  const [blobError, setBlobError] = useState("");
  const [edit, setEdit] = useState<Fields | null>(null);
  const [baseline, setBaseline] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [action, setAction] = useState("");
  const [actionVersion, setActionVersion] = useState(0);
  const [note, setNote] = useState("");
  const [paymentDate, setPaymentDate] = useState(today());
  const generation = useRef(0);
  const load = useCallback(async () => {
    const current = ++generation.current;
    try {
      const r = await request<{ claim: Claim }>("GET", `/reimbursements/${id}`);
      if (generation.current === current) {
        setClaim(r.claim);
        setError("");
      }
    } catch (e) {
      if (generation.current === current) {
        setClaim(null);
        setAction("");
        setConfirmed(false);
        setError(
          e instanceof TypeError
            ? "网络连接失败，请检查后重试"
            : (e as Error).message,
        );
      }
    }
  }, [id]);
  useEffect(() => {
    void load();
    return () => {
      generation.current++;
    };
  }, [load]);
  useEffect(() => {
    if (!claim || !activeOCR(claim) || busy) return;
    const timer = setInterval(() => {
      void load();
    }, 2500);
    return () => clearInterval(timer);
  }, [claim?.ocr_status, busy, load]);
  useEffect(() => {
    let live = true,
      url = "";
    fetch(`${BASE}/reimbursements/${id}/receipt`, {
      headers: { Authorization: `Bearer ${getToken()}` },
    })
      .then(async (r) => {
        if (!r.ok) throw new Error("票据预览失败，请刷新重试");
        return r.blob();
      })
      .then((b) => {
        url = URL.createObjectURL(b);
        if (live) setBlob(url);
        else URL.revokeObjectURL(url);
      })
      .catch((e: Error) => {
        if (live) setBlobError(e.message);
      });
    return () => {
      live = false;
      if (url) URL.revokeObjectURL(url);
    };
  }, [id]);
  const events = useRemote(
    () =>
      request<{
        events: {
          id: number;
          action: string;
          note: string;
          actor_name: string | null;
          created_at: string;
        }[];
      }>("GET", `/reimbursements/${id}/events`),
    [id, claim?.updated_at],
  );
  const mutate = async (
    verb: string,
    body: Record<string, unknown>,
    version = claim?.version,
  ) => {
    generation.current++;
    const r = await request<{ claim: Claim }>(
      "POST",
      `/reimbursements/${id}/${verb}`,
      { ...body, version },
    );
    generation.current++;
    setClaim(r.claim);
    setConfirmed(false);
    notify("报销记录已更新");
  };
  const change = (key: Exclude<keyof Fields, "version">, value: string) =>
    setEdit((v) => (v ? { ...v, [key]: value } : null));
  const startAction = (value: string) => {
    setAction(value);
    setActionVersion(claim?.version ?? 0);
    setNote("");
    setPaymentDate(today());
  };
  if (!claim)
    return (
      <>
        <SimpleHeader title="报销详情">
          <Btn kind="ghost" onClick={onBack}>
            返回列表
          </Btn>
        </SimpleHeader>
        {error ? (
          <Alert retry={load}>{error}</Alert>
        ) : (
          <p role="status">正在载入报销记录…</p>
        )}
      </>
    );
  const owner = claim.employee_id === user.employee_id;
  const editable = owner && ["draft", "rejected"].includes(claim.status);
  const reviewer =
    !owner &&
    ["admin", "finance", "manager", "workplace_manager"].includes(user.role);
  const finance = !owner && ["admin", "finance"].includes(user.role);
  const preview = (
    <div className="receipt-preview">
      {blobError ? (
        <Alert>{blobError}</Alert>
      ) : !blob ? (
        <p role="status">正在载入原始票据…</p>
      ) : (
        <>
          {claim.receipt_mime === "application/pdf" ? (
            <iframe src={blob} title="原始报销票据" />
          ) : (
            <img src={blob} alt="本笔报销上传的原始票据" />
          )}
          <a
            href={blob}
            download={`报销-${id}.${claim.receipt_mime === "application/pdf" ? "pdf" : "png"}`}
          >
            下载原始票据
          </a>
        </>
      )}
    </div>
  );
  return (
    <>
      <SimpleHeader
        title={`报销单 BX-${String(id).padStart(6, "0")}`}
        description={`${claim.employee_name}的票据与处理记录`}
      >
        <Btn kind="ghost" onClick={onBack}>
          返回列表
        </Btn>
        <Btn
          kind="ghost"
          onClick={() => {
            void load();
          }}
        >
          刷新
        </Btn>
      </SimpleHeader>
      {error && <Alert retry={load}>{error}</Alert>}
      <section className="panel claim-state">
        <div className="section-heading">
          <h2>{labels[claim.status]}</h2>
          {claimStatus(claim.status)}
        </div>
        <p>
          {ocrLabels[claim.ocr_status]}
          {activeOCR(claim) && "，页面会自动更新。"}
          {claim.ocr_progress.totalPages
            ? ` ${claim.ocr_progress.extractedPages ?? 0}/${claim.ocr_progress.totalPages}页`
            : ""}
        </p>
        {claim.ocr_error && (
          <Alert tone={claim.ocr_status === "failed" ? "error" : "info"}>
            {claim.ocr_error}
          </Alert>
        )}
        {claim.review_note && (
          <Alert tone="info">审批说明：{claim.review_note}</Alert>
        )}
        <div className="actions">
          {editable && (
            <>
              <Btn
                disabled={activeOCR(claim) || busy}
                onClick={() => {
                  const values = fieldsFor(claim);
                  setEdit(values);
                  setBaseline(JSON.stringify(values));
                }}
              >
                核对并填写
              </Btn>
              {["ready", "failed"].includes(claim.ocr_status) && (
                <Btn
                  kind="ghost"
                  disabled={busy}
                  onClick={() => startAction("recognize")}
                >
                  重新识别
                </Btn>
              )}
            </>
          )}
          {reviewer && claim.status === "submitted" && (
            <Btn onClick={() => startAction("approve")}>批准报销</Btn>
          )}
          {reviewer &&
            (claim.status === "submitted" ||
              (finance && claim.status === "approved")) && (
              <Btn kind="danger" onClick={() => startAction("reject")}>
                驳回申请
              </Btn>
            )}
          {finance && claim.status === "approved" && (
            <Btn onClick={() => startAction("pay")}>登记付款</Btn>
          )}
          {owner && claim.status === "submitted" && (
            <Btn kind="ghost" onClick={() => startAction("withdraw")}>
              撤回申请
            </Btn>
          )}
          {owner &&
            ["draft", "rejected", "submitted"].includes(claim.status) && (
              <Btn kind="ghost" onClick={() => startAction("cancel")}>
                撤销报销
              </Btn>
            )}
        </div>
      </section>
      <div className="receipt-workspace">
        <section className="panel">
          <h2>原始票据</h2>
          {preview}
        </section>
        <section className="panel">
          <h2>{claim.title}</h2>
          <dl className="claim-facts">
            {[
              ["申请人", claim.employee_name],
              ["费用类别", categories[claim.category]],
              ["商家", claim.merchant || "待填写"],
              ["票据号码", claim.invoice_number || "待填写"],
              ["票据代码", claim.invoice_code || "—"],
              ["票据日期", claim.invoice_date || "待填写"],
              ["币种", claim.currency],
              [
                "票面金额",
                claim.invoice_total == null
                  ? "待填写"
                  : money(claim.invoice_total),
              ],
              [
                "申请报销",
                claim.amount == null ? "待填写" : money(claim.amount),
              ],
              ["用途说明", claim.reason || "待填写"],
            ].map(([label, value]) => (
              <div key={label}>
                <dt>{label}</dt>
                <dd>{value}</dd>
              </div>
            ))}
          </dl>
          {editable && (
            <>
              <label className="claim-confirm">
                <input
                  type="checkbox"
                  checked={confirmed}
                  disabled={activeOCR(claim) || busy}
                  onChange={(e) => setConfirmed(e.target.checked)}
                />
                我已核对原始票据、金额和用途
              </label>
              <Btn
                disabled={!confirmed || activeOCR(claim) || busy}
                onClick={async () => {
                  setBusy(true);
                  setError("");
                  try {
                    await mutate("submit", { confirmed: true });
                  } catch (e) {
                    setError((e as Error).message);
                  } finally {
                    setBusy(false);
                  }
                }}
              >
                提交审批
              </Btn>
              <p className="muted">
                提交前请补齐用途。识别及批准都不会自动生成付款流水。
              </p>
            </>
          )}
          {claim.finance_record_id && (
            <p>
              已关联支出流水 #{claim.finance_record_id}，付款日期{" "}
              {claim.payment_date}。
            </p>
          )}
          {owner && claim.status === "submitted" && (
            <p className="muted">
              申请人不能审批自己的报销，请由其他审批人处理。
            </p>
          )}
        </section>
      </div>
      {claim.ocr_result && (
        <details className="panel receipt-raw">
          <summary>识别建议与原文</summary>
          {claim.ocr_result.warnings.map((warning, i) => (
            <p key={i} className="muted">
              {warning}
            </p>
          ))}
          <pre>{claim.ocr_result.text}</pre>
        </details>
      )}
      <section className="panel">
        <h2>处理记录</h2>
        {events.error && <Alert retry={events.reload}>{events.error}</Alert>}
        {events.loading && <p role="status">正在载入处理记录…</p>}
        <ol className="claim-timeline">
          {events.data?.events.map((event) => (
            <li key={event.id}>
              <strong>
                {(
                  {
                    uploaded: "上传票据",
                    updated: "保存人工核对",
                    ocr_ready: "识别完成",
                    ocr_failed: "识别未完成",
                    ocr_retry: "重新识别",
                    submit: "提交审批",
                    approve: "批准报销",
                    reject: "驳回报销",
                    withdraw: "撤回申请",
                    cancel: "撤销报销",
                    pay: "登记付款",
                  } as Record<string, string>
                )[event.action] ?? event.action}
              </strong>
              <span>
                {event.actor_name || "识别服务"} ·{" "}
                {new Intl.DateTimeFormat("zh-CN", {
                  timeZone: "Asia/Shanghai",
                  dateStyle: "short",
                  timeStyle: "short",
                }).format(new Date(event.created_at))}
              </span>
              {event.note && <p>{event.note}</p>}
            </li>
          ))}
        </ol>
      </section>
      {edit && (
        <FormDialog
          open
          className="receipt-edit-dialog"
          title="核对并填写报销信息"
          dirty={JSON.stringify(edit) !== baseline}
          submitText="保存草稿"
          onClose={() => setEdit(null)}
          onSubmit={async () => {
            const r = await request<{ claim: Claim }>(
              "PUT",
              `/reimbursements/${id}`,
              {
                ...edit,
                invoiceTotal:
                  edit.invoiceTotal === "" ? null : Number(edit.invoiceTotal),
                amount: edit.amount === "" ? null : Number(edit.amount),
              },
            );
            generation.current++;
            setClaim(r.claim);
            setConfirmed(false);
            notify("草稿已保存，请核对后提交审批");
          }}
        >
          <div className="receipt-editor">
            <div>
              <h3>对照原始票据</h3>
              {preview}
            </div>
            <div className="form-grid">
              <Field
                label="报销标题"
                value={edit.title}
                maxLength={160}
                onChange={(e) => change("title", e.target.value)}
              />
              <SelectField
                label="费用类别"
                value={edit.category}
                onChange={(e) => change("category", e.target.value)}
              >
                {Object.entries(categories).map(([v, label]) => (
                  <option key={v} value={v}>
                    {label}
                  </option>
                ))}
              </SelectField>
              <Field
                label="商家名称"
                value={edit.merchant}
                maxLength={200}
                onChange={(e) => change("merchant", e.target.value)}
              />
              <Field
                label="票据号码"
                value={edit.invoiceNumber}
                maxLength={64}
                onChange={(e) => change("invoiceNumber", e.target.value)}
              />
              <Field
                label="票据代码（选填）"
                value={edit.invoiceCode}
                maxLength={64}
                onChange={(e) => change("invoiceCode", e.target.value)}
              />
              <Field
                label="票据日期"
                type="date"
                value={edit.invoiceDate}
                max={today()}
                onChange={(e) => change("invoiceDate", e.target.value)}
              />
              <SelectField
                label="币种"
                value={edit.currency}
                onChange={(e) => change("currency", e.target.value)}
              >
                {[...new Set(["CNY", edit.currency])].map((v) => (
                  <option key={v} value={v}>
                    {v === "CNY" ? "人民币 CNY" : v}
                  </option>
                ))}
              </SelectField>
              <Field
                label="票面金额（元）"
                type="number"
                step="0.01"
                min={0}
                value={edit.invoiceTotal}
                onChange={(e) => change("invoiceTotal", e.target.value)}
              />
              <Field
                label="申请报销金额（元）"
                type="number"
                step="0.01"
                min={0}
                value={edit.amount}
                onChange={(e) => change("amount", e.target.value)}
              />
              <TextAreaField
                label="用途说明"
                value={edit.reason}
                maxLength={2000}
                onChange={(e) => change("reason", e.target.value)}
              />
              {claim.ocr_result && (
                <Btn
                  kind="ghost"
                  onClick={() => {
                    const f = claim.ocr_result!.fields;
                    setEdit({
                      ...edit,
                      merchant: f.merchant,
                      invoiceNumber: f.invoiceNumber,
                      invoiceCode: f.invoiceCode,
                      invoiceDate: f.invoiceDate,
                      currency: f.currency,
                      category: f.category,
                      invoiceTotal: f.total == null ? "" : String(f.total),
                      amount: f.total == null ? "" : String(f.total),
                    });
                  }}
                >
                  应用识别建议
                </Btn>
              )}
            </div>
          </div>
        </FormDialog>
      )}
      {action && (
        <FormDialog
          open
          title={
            (
              {
                recognize: "重新识别票据",
                approve: "批准报销",
                reject: "驳回报销",
                pay: "登记报销付款",
                withdraw: "撤回申请",
                cancel: "撤销报销",
              } as Record<string, string>
            )[action]
          }
          submitText={action === "pay" ? "确认付款入账" : "确认"}
          dirty={note !== "" || paymentDate !== today()}
          onClose={() => setAction("")}
          onSubmit={async () => {
            if (action === "reject" && !note.trim())
              throw new Error("请填写驳回原因");
            await mutate(
              action,
              action === "recognize"
                ? {}
                : { note, paymentDate: action === "pay" ? paymentDate : "" },
              actionVersion,
            );
          }}
        >
          <p>
            报销单 BX-{String(id).padStart(6, "0")}，申请金额{" "}
            {claim.amount == null ? "待填写" : money(claim.amount)}。
          </p>
          {action === "pay" && (
            <>
              <p>核对已支付金额与日期，保存后生成一笔支出流水。</p>
              <Field
                label="付款日期"
                type="date"
                min={claim.invoice_date || undefined}
                max={today()}
                required
                value={paymentDate}
                onChange={(e) => setPaymentDate(e.target.value)}
              />
            </>
          )}
          {action === "recognize" ? (
            <p>会重新调用识别服务，已保存的人工填写信息保持原样。</p>
          ) : (
            <TextAreaField
              label={action === "reject" ? "驳回原因" : "处理说明（选填）"}
              required={action === "reject"}
              value={note}
              maxLength={2000}
              onChange={(e) => setNote(e.target.value)}
            />
          )}
        </FormDialog>
      )}
    </>
  );
}
