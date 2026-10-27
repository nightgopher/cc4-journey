import React, { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { money, LEVEL_ZH, ROLE_ZH, STATUS_ZH } from "../lib/tokens";
import { auth, setToken } from "../lib/api";

export function Icon({ name, size = 18 }: { name: string; size?: number }) {
  const paths: Record<string, React.ReactNode> = {
    dashboard: <path d="M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z" />,
    people: (
      <>
        <circle cx="9" cy="8" r="3" />
        <path d="M3 21v-3a6 6 0 0 1 12 0v3M16 5a3 3 0 0 1 0 6M18 15a5 5 0 0 1 3 5" />
      </>
    ),
    phone: (
      <path d="M5 3h4l2 5-3 2a16 16 0 0 0 6 6l2-3 5 2v4c0 2-2 3-4 2A22 22 0 0 1 3 7c-1-2 0-4 2-4Z" />
    ),
    list: <path d="M8 5h13M8 12h13M8 19h13M3 5h.01M3 12h.01M3 19h.01" />,
    calendar: (
      <>
        <rect x="3" y="5" width="18" height="16" rx="2" />
        <path d="M7 3v4M17 3v4M3 11h18M7 15h2M13 15h2" />
      </>
    ),
    money: (
      <>
        <rect x="3" y="5" width="18" height="14" rx="2" />
        <circle cx="12" cy="12" r="3" />
        <path d="M6 9v6M18 9v6" />
      </>
    ),
    box: <path d="m3 7 9-4 9 4v10l-9 4-9-4Zm0 0 9 4 9-4M12 11v10M7 5l10 4" />,
    bell: <path d="M5 17h14l-2-4V9a5 5 0 0 0-10 0v4Zm5 3h4" />,
    check: (
      <>
        <rect x="4" y="3" width="16" height="18" rx="2" />
        <path d="m8 12 3 3 5-6" />
      </>
    ),
    settings: (
      <>
        <path d="M4 7h16M4 17h16" />
        <circle cx="9" cy="7" r="3" fill="currentColor" />
        <circle cx="15" cy="17" r="3" fill="currentColor" />
      </>
    ),
    shield: (
      <>
        <path d="m12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6Z" />
        <path d="m8 12 3 3 5-6" />
      </>
    ),
    menu: <path d="M4 6h16M4 12h16M4 18h16" />,
    close: <path d="m6 6 12 12M6 18 18 6" />,
    logout: <path d="M10 4H4v16h6M10 12h11m-4-4 4 4-4 4" />,
    caret: <path d="m6 9 6 6 6-6" />,
  };
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {paths[name] ?? paths.list}
    </svg>
  );
}
export const Btn: React.FC<
  React.ButtonHTMLAttributes<HTMLButtonElement> & {
    kind?: "primary" | "ghost" | "danger" | "ghost-danger";
  }
> = ({ kind = "primary", className = "", type = "button", ...rest }) => (
  <button type={type} className={`btn btn--${kind} ${className}`} {...rest} />
);

export interface RowMenuItem {
  label: string;
  danger?: boolean;
  onSelect: () => void;
}

/**
 * 行内「更多」菜单：渲染到 body 并 fixed 定位。操作列是 sticky 单元格，会形成独立层叠上下文，
 * 菜单留在单元格内时会被下一行的操作列遮住、被表格裁剪。
 */
export function RowMenu({ items }: { items: RowMenuItem[] }) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null);
  const root = useRef<HTMLDivElement>(null);
  const pop = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (!open) return;
    const close = () => setOpen(false);
    const onDoc = (e: MouseEvent) => {
      const target = e.target as Node;
      if (!root.current?.contains(target) && !pop.current?.contains(target))
        setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        setOpen(false);
        trigger.current?.focus();
      }
    };
    document.addEventListener("mousedown", onDoc);
    document.addEventListener("keydown", onKey);
    window.addEventListener("scroll", close, true);
    window.addEventListener("resize", close);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      document.removeEventListener("keydown", onKey);
      window.removeEventListener("scroll", close, true);
      window.removeEventListener("resize", close);
    };
  }, [open]);
  const toggle = () => {
    if (open) {
      setOpen(false);
      return;
    }
    const r = trigger.current?.getBoundingClientRect();
    if (r) {
      // Open upward when the menu would run past the bottom of the viewport.
      const height = items.length * 32 + 10;
      const top =
        r.bottom + 4 + height > window.innerHeight
          ? Math.max(8, r.top - 4 - height)
          : r.bottom + 4;
      setPos({ top, left: Math.max(8, r.right - 148) });
    }
    setOpen(true);
  };
  return (
    <div className="row-menu" ref={root}>
      <button
        ref={trigger}
        type="button"
        className="btn btn--ghost"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={toggle}
      >
        更多
        <Icon name="caret" size={14} />
      </button>
      {open &&
        pos &&
        createPortal(
          <div className="row-menu-pop" role="menu" style={pos} ref={pop}>
            {items.map((it) => (
              <button
                key={it.label}
                type="button"
                role="menuitem"
                className={it.danger ? "row-menu-item danger" : "row-menu-item"}
                onClick={() => {
                  setOpen(false);
                  it.onSelect();
                }}
              >
                {it.label}
              </button>
            ))}
          </div>,
          document.body,
        )}
    </div>
  );
}
export function Field({
  label,
  hint,
  ...rest
}: {
  label: string;
  hint?: string;
} & React.InputHTMLAttributes<HTMLInputElement>) {
  const id = useId();
  return (
    <label className="field" htmlFor={id}>
      <span>{label}</span>
      <input id={id} name={rest.name ?? label} autoComplete="off" {...rest} />
      {hint && <small>{hint}</small>}
    </label>
  );
}
export function SelectField({
  label,
  children,
  ...rest
}: {
  label: string;
  children: React.ReactNode;
} & React.SelectHTMLAttributes<HTMLSelectElement>) {
  const id = useId();
  return (
    <label className="field" htmlFor={id}>
      <span>{label}</span>
      <select id={id} name={rest.name ?? label} {...rest}>
        {children}
      </select>
    </label>
  );
}
export function TextAreaField({
  label,
  ...rest
}: { label: string } & React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  const id = useId();
  return (
    <label className="field" htmlFor={id}>
      <span>{label}</span>
      <textarea id={id} name={rest.name ?? label} rows={3} {...rest} />
    </label>
  );
}
export function Status({
  value,
  label,
}: {
  value: string | null | undefined;
  label?: string;
}) {
  if (!value) return <span className="muted">—</span>;
  const tone = [
    "active",
    "approved",
    "confirmed",
    "completed",
    "converted",
    "work",
    "paid",
  ].includes(value)
    ? "good"
    : [
          "rejected",
          "lost",
          "cancelled",
          "absent",
          "inactive",
          "suspended",
        ].includes(value)
      ? "bad"
      : "neutral";
  return (
    <span className={`status status--${tone}`}>
      {label ?? STATUS_ZH[value] ?? value}
    </span>
  );
}
export const LevelBadge = ({ level }: { level: string }) => (
  <span className={`level level--${level}`}>{LEVEL_ZH[level] ?? level}</span>
);
export const MoneyCell = ({
  v,
  tone,
}: {
  v: number | string | null;
  tone?: "in" | "out";
}) => {
  const n = typeof v === "number" ? v : Number(v);
  const active = tone && n !== 0 && !Number.isNaN(n) ? tone : undefined;
  return (
    <span className={`money ${active ? `money--${active}` : ""}`}>
      {money(v)}
    </span>
  );
};
export const StatCard = ({
  label,
  value,
}: {
  label: string;
  value: React.ReactNode;
  money?: boolean;
}) => (
  <div className="stat">
    <div className="muted">{label}</div>
    <strong>{value}</strong>
  </div>
);
export function Alert({
  children,
  retry,
  tone = "error",
}: {
  children: React.ReactNode;
  retry?: () => void;
  tone?: "error" | "success" | "info";
}) {
  return (
    <div
      className={`alert alert--${tone}`}
      role={tone === "error" ? "alert" : "status"}
    >
      <span>{children}</span>
      {retry && (
        <Btn kind="ghost" onClick={retry}>
          重试
        </Btn>
      )}
    </div>
  );
}
export interface Column<T> {
  key: string;
  title: string;
  render?: (row: T) => React.ReactNode;
  align?: "left" | "right";
}
export interface DataTablePageProps<T> {
  title: string;
  description?: string;
  fetchPage: (
    page: number,
    limit: number,
  ) => Promise<{ rows: T[]; total: number }>;
  columns: Column<T>[];
  rowKey: (row: T) => React.Key;
  toolbar?: React.ReactNode;
  filters?: React.ReactNode;
  emptyText?: string;
  refreshKey?: string | number;
}
export function DataTablePage<T>({
  title,
  description,
  fetchPage,
  columns,
  rowKey,
  toolbar,
  filters,
  emptyText = "暂无记录。",
  refreshKey,
}: DataTablePageProps<T>) {
  const [rows, setRows] = useState<T[]>([]);
  const [page, setPage] = useState(() =>
    Math.max(1, Number(new URLSearchParams(location.search).get("page")) || 1),
  );
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [retry, setRetry] = useState(0);
  const limit = 20;
  const scrollRef = useRef<HTMLDivElement>(null);
  const [scrollable, setScrollable] = useState(false);
  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const check = () =>
      setScrollable(
        el.scrollWidth > el.clientWidth + 1 &&
          el.scrollLeft < el.scrollWidth - el.clientWidth - 1,
      );
    check();
    el.addEventListener("scroll", check, { passive: true });
    window.addEventListener("resize", check);
    return () => {
      el.removeEventListener("scroll", check);
      window.removeEventListener("resize", check);
    };
  }, [rows, loading]);
  useEffect(() => {
    let live = true;
    setLoading(true);
    setErr("");
    fetchPage(page, limit)
      .then((result) => {
        if (!live) return;
        if (page > 1 && result.total <= (page - 1) * limit) {
          setPage(Math.max(1, Math.ceil(result.total / limit)));
          return;
        }
        setRows(result.rows);
        setTotal(result.total);
      })
      .catch((e: Error) => {
        if (live) setErr(e.message);
      })
      .finally(() => {
        if (live) setLoading(false);
      });
    return () => {
      live = false;
    };
  }, [page, fetchPage, refreshKey, retry]);
  const goPage = (p: number) => {
    setPage(p);
    const url = new URL(location.href);
    url.searchParams.set("page", String(p));
    history.replaceState(null, "", url);
  };
  return (
    <section aria-busy={loading}>
      <header className="page-heading">
        <div>
          <h1>{title}</h1>
          {description && <p>{description}</p>}
        </div>
        <div className="actions">
          {toolbar}
          <Btn
            kind="ghost"
            onClick={() => setRetry((v) => v + 1)}
            disabled={loading}
          >
            刷新
          </Btn>
        </div>
      </header>
      {filters && <div className="filters">{filters}</div>}
      {err && <Alert retry={() => setRetry((v) => v + 1)}>{err}</Alert>}
      {loading ? (
        <div className="table-skeleton" role="status" aria-label="正在加载记录">
          <span>正在加载…</span>
          {[1, 2, 3, 4].map((n) => (
            <div key={n} />
          ))}
        </div>
      ) : (
        !err && (
          <>
            <div
              className="table-wrap"
              data-scrollable={scrollable || undefined}
            >
              <div
                ref={scrollRef}
                className="table-scroll"
                tabIndex={0}
                role="region"
                aria-label={`${title}数据表`}
              >
                {rows.length === 0 ? (
                  <div className="empty">
                    <Icon name="list" size={28} />
                    <p>{emptyText}</p>
                  </div>
                ) : (
                  <table>
                    <thead>
                      <tr>
                        {columns.map((c) => (
                          <th
                            key={c.key}
                            scope="col"
                            className={
                              c.align === "right"
                                ? "number"
                                : c.title === "操作"
                                  ? "actions-cell"
                                  : ""
                            }
                          >
                            {c.title}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {rows.map((row) => (
                        <tr key={rowKey(row)}>
                          {columns.map((c) => (
                            <td
                              key={c.key}
                              className={
                                c.align === "right"
                                  ? "number"
                                  : c.title === "操作"
                                    ? "actions-cell"
                                    : ""
                              }
                            >
                              {c.render
                                ? c.render(row)
                                : String(
                                    (row as Record<string, unknown>)[c.key] ??
                                      "—",
                                  )}
                            </td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            </div>
            <nav className="pagination" aria-label="分页">
              <span>共 {total.toLocaleString("zh-CN")} 条</span>
              <div className="actions">
                <Btn
                  kind="ghost"
                  disabled={page <= 1}
                  onClick={() => goPage(page - 1)}
                >
                  上一页
                </Btn>
                <span>
                  {page} / {Math.max(1, Math.ceil(total / limit))}
                </span>
                <Btn
                  kind="ghost"
                  disabled={page * limit >= total}
                  onClick={() => goPage(page + 1)}
                >
                  下一页
                </Btn>
              </div>
            </nav>
          </>
        )
      )}
    </section>
  );
}
export function FormDialog({
  open,
  title,
  onClose,
  onSubmit,
  submitText = "保存",
  children,
  dirty = false,
  className,
}: {
  open: boolean;
  title: string;
  onClose: () => void;
  onSubmit: () => Promise<void>;
  submitText?: string;
  children: React.ReactNode;
  dirty?: boolean;
  className?: string;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [discard, setDiscard] = useState(false);
  const titleID = useId();
  useEffect(() => {
    if (!open) return;
    const dialog = ref.current;
    const opener = document.activeElement as HTMLElement | null;
    setErr("");
    setDiscard(false);
    dialog?.showModal();
    dialog
      ?.querySelector<HTMLElement>(
        "input:not([disabled]),select:not([disabled]),textarea:not([disabled])",
      )
      ?.focus();
    return () => {
      dialog?.close();
      if (opener?.isConnected) opener.focus();
    };
  }, [open]);
  useEffect(() => {
    if (!open || !dirty) return;
    const warn = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = "";
    };
    addEventListener("beforeunload", warn);
    return () => removeEventListener("beforeunload", warn);
  }, [open, dirty]);
  const close = () => {
    if (busy) return;
    if (dirty && !discard) {
      setDiscard(true);
      return;
    }
    onClose();
  };
  return (
    <dialog
      ref={ref}
      aria-labelledby={titleID}
      className={`dialog ${className ?? ""}`}
      onCancel={(e) => {
        e.preventDefault();
        close();
      }}
      onClick={(e) => {
        if (e.target === ref.current) close();
      }}
    >
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setErr("");
          try {
            await onSubmit();
            onClose();
          } catch (e) {
            setErr((e as Error).message);
          } finally {
            setBusy(false);
          }
        }}
      >
        <header className="dialog-heading">
          <h2 id={titleID}>{title}</h2>
          <button
            type="button"
            className="icon-button"
            aria-label="关闭弹窗"
            onClick={close}
            disabled={busy}
          >
            <Icon name="close" />
          </button>
        </header>
        <div className="form-grid">{children}</div>
        {err && <Alert>{err}</Alert>}
        {discard && (
          <Alert tone="info">
            有尚未保存的内容。继续编辑，或选择放弃更改。
          </Alert>
        )}
        <footer className="dialog-footer">
          {discard ? (
            <>
              <Btn
                kind="ghost"
                disabled={busy}
                onClick={() => setDiscard(false)}
              >
                继续编辑
              </Btn>
              <Btn kind="danger" disabled={busy} onClick={onClose}>
                放弃更改
              </Btn>
            </>
          ) : (
            <Btn kind="ghost" onClick={close} disabled={busy}>
              取消
            </Btn>
          )}
          <Btn type="submit" disabled={busy}>
            {busy ? "保存中…" : submitText}
          </Btn>
        </footer>
      </form>
    </dialog>
  );
}
export interface NavItem {
  key: string;
  label: string;
  icon: string;
  group?: string;
}
export function AppShell({
  nav,
  active,
  onNav,
  userName,
  userRole,
  onLogout,
  children,
}: {
  nav: NavItem[];
  active: string;
  onNav: (key: string) => void;
  userName?: string | null;
  userRole?: string | null;
  onLogout: () => void;
  children: React.ReactNode;
}) {
  const [mobileOpen, setMobileOpen] = useState(false);
  const sidebar = useRef<HTMLElement>(null);
  const workspace = useRef<HTMLDivElement>(null);
  const activeNavRef = useRef<HTMLAnchorElement>(null);
  const current = nav.find((n) => n.key === active);
  useEffect(() => {
    setMobileOpen(false);
    document.title = `${current?.label ?? "工作台"} · 呼叫中心 4.0`;
    activeNavRef.current?.scrollIntoView({ block: "nearest" });
  }, [active, current?.label]);
  useEffect(() => {
    if (!mobileOpen) return;
    const panel = sidebar.current;
    const content = workspace.current;
    const opener = document.activeElement as HTMLElement | null;
    const oldOverflow = document.body.style.overflow;
    if (content) content.inert = true;
    document.body.style.overflow = "hidden";
    const focusable = () =>
      Array.from(
        panel?.querySelectorAll<HTMLElement>(
          "a[href],button:not([disabled])",
        ) ?? [],
      ).filter((el) => el.getClientRects().length > 0);
    focusable()[0]?.focus();
    const keys = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        setMobileOpen(false);
      }
      if (e.key !== "Tab") return;
      const items = focusable();
      const first = items[0],
        last = items[items.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last?.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first?.focus();
      }
    };
    const resized = () => {
      if (matchMedia("(min-width: 761px)").matches) setMobileOpen(false);
    };
    addEventListener("keydown", keys);
    addEventListener("resize", resized);
    return () => {
      if (content) content.inert = false;
      document.body.style.overflow = oldOverflow;
      removeEventListener("keydown", keys);
      removeEventListener("resize", resized);
      if (opener?.isConnected) opener.focus();
    };
  }, [mobileOpen]);
  const groups = [...new Set(nav.map((n) => n.group ?? "工作台"))];
  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        跳到主要内容
      </a>
      <aside
        ref={sidebar}
        className={`sidebar ${mobileOpen ? "sidebar--open" : ""}`}
        id="app-navigation"
        role={mobileOpen ? "dialog" : undefined}
        aria-modal={mobileOpen || undefined}
        aria-label="工作台导航"
      >
        <a
          className="brand"
          href="?view=dashboard"
          onClick={(e) => {
            if (e.metaKey || e.ctrlKey) return;
            e.preventDefault();
            onNav(nav[0]?.key ?? "profile");
          }}
        >
          <Icon name="phone" size={21} />
          <span>
            呼叫中心 <b>4.0</b>
          </span>
        </a>
        {mobileOpen && (
          <button
            className="icon-button mobile-nav-close"
            aria-label="关闭导航"
            onClick={() => setMobileOpen(false)}
          >
            <Icon name="close" />
          </button>
        )}
        <nav className="side-nav" aria-label="业务导航">
          {groups.map((group) => (
            <div className="nav-group" key={group}>
              <span className="nav-group-name">{group}</span>
              {nav
                .filter((n) => (n.group ?? "工作台") === group)
                .map((n) => (
                  <a
                    href={`?view=${n.key}`}
                    key={n.key}
                    ref={active === n.key ? activeNavRef : undefined}
                    aria-current={active === n.key ? "page" : undefined}
                    onClick={(e) => {
                      if (e.metaKey || e.ctrlKey || e.shiftKey) return;
                      e.preventDefault();
                      onNav(n.key);
                    }}
                  >
                    <Icon name={n.icon} />
                    <span>{n.label}</span>
                  </a>
                ))}
            </div>
          ))}
        </nav>
        <div className="sidebar-user">
          <div className="avatar" aria-hidden="true">
            {userName?.slice(-2) ?? "用户"}
          </div>
          <div>
            <strong>{userName || "当前用户"}</strong>
            <span>{ROLE_ZH[userRole ?? ""] ?? userRole}</span>
          </div>
          <button
            className="icon-button"
            onClick={onLogout}
            aria-label="退出登录"
          >
            <Icon name="logout" />
          </button>
        </div>
      </aside>
      {mobileOpen && (
        <button
          className="nav-scrim"
          aria-label="收起导航"
          tabIndex={-1}
          onClick={() => setMobileOpen(false)}
        />
      )}
      <div className="workspace" ref={workspace}>
        <header className="workspace-bar">
          <button
            className="icon-button mobile-menu"
            aria-controls="app-navigation"
            aria-expanded={mobileOpen}
            aria-label={mobileOpen ? "收起导航" : "打开导航"}
            onClick={() => setMobileOpen((v) => !v)}
          >
            <Icon name="menu" />
          </button>
          <span>运营工作台</span>
          <span className="workspace-location">{current?.label}</span>
          <span className="workspace-date">
            {new Intl.DateTimeFormat("zh-CN", {
              timeZone: "Asia/Shanghai",
              month: "long",
              day: "numeric",
              weekday: "long",
            }).format(new Date())}
          </span>
        </header>
        <main id="main-content" tabIndex={-1}>
          {children}
        </main>
      </div>
    </div>
  );
}
export function LoginPage({
  onLogin,
}: {
  onLogin: (name: string, role: string) => void;
}) {
  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  return (
    <main className="login-page">
      <section className="login-intro">
        <div className="login-brand">
          <Icon name="phone" size={28} />
          呼叫中心 4.0
        </div>
        <h1>
          从一条线索，
          <br />
          到每一笔业绩。
        </h1>
        <p>客户跟进、团队协作、业绩审核与薪资核算，回到同一个工作台。</p>
        <div className="login-workflow">
          <span>跟进客户</span>
          <span>记录业绩</span>
          <span>核对薪资</span>
        </div>
      </section>
      <form
        className="login-form"
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setErr("");
          try {
            const result = await auth.login(phone, password);
            setToken(result.session.access_token);
            onLogin(result.user.name ?? "", result.user.role);
          } catch (e) {
            setErr((e as Error).message);
          } finally {
            setBusy(false);
          }
        }}
      >
        <h2>登录工作台</h2>
        <p className="muted">使用分配给你的工作账号</p>
        <Field
          label="手机号"
          name="phone"
          type="tel"
          autoComplete="username"
          required
          value={phone}
          onChange={(e) => setPhone(e.target.value)}
          placeholder="输入登录手机号…"
        />
        <Field
          label="密码"
          name="password"
          type="password"
          autoComplete="current-password"
          required
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="输入密码…"
        />
        {err && <Alert>{err}</Alert>}
        <Btn type="submit" disabled={busy}>
          {busy ? "登录中…" : "登录工作台"}
        </Btn>
        <p className="login-help">账号无法使用？请联系团队管理员。</p>
      </form>
    </main>
  );
}
