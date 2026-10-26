/**
 * 模板组件 —— DataTablePage / StatCard / FormDialog / AppShell
 * 23 个 CRUD 页面全部声明式套用，消除旧前端的巨型文件。
 * 组件用原生 CSS 变量 + 极简内联样式（不引第三方 UI 库——避免 shadcn 模板脸）。
 */
import React, { useEffect, useState } from 'react';
import { tokens as t, money } from '../lib/tokens';
import { auth, setToken, ApiError } from '../lib/api';

// ---------- 基础原子 ----------

export const Btn: React.FC<React.ButtonHTMLAttributes<HTMLButtonElement> & {
  kind?: 'primary' | 'ghost' | 'danger'
}> = ({ kind = 'primary', style, ...rest }) => (
  <button {...rest} style={{
    font: `600 13px/1 ${t.type.family}`,
    padding: '8px 14px', borderRadius: t.radius.sm, cursor: 'pointer',
    border: kind === 'ghost' ? `1px solid ${t.color.line}` : '1px solid transparent',
    background: kind === 'primary' ? t.color.accent : kind === 'danger' ? t.color.moneyOut : 'transparent',
    color: kind === 'ghost' ? t.color.ink : '#fff',
    opacity: rest.disabled ? 0.5 : 1,
    ...style,
  }} />
);

export const Field: React.FC<{ label: string } & React.InputHTMLAttributes<HTMLInputElement>> = ({ label, ...rest }) => (
  <label style={{ display: 'block' }}>
    <div style={{ font: `400 12px/1.5 ${t.type.family}`, color: t.color.inkSoft, marginBottom: 4 }}>{label}</div>
    <input {...rest} style={{
      width: '100%', boxSizing: 'border-box', font: `400 14px/1.4 ${t.type.family}`,
      padding: '8px 10px', borderRadius: t.radius.sm, border: `1px solid ${t.color.line}`,
      background: '#fff', color: t.color.ink, outline: 'none',
    }} />
  </label>
);

const StatusDot: React.FC<{ color: string; text: string }> = ({ color, text }) => (
  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, font: `400 13px/1 ${t.type.family}` }}>
    <span style={{ width: 7, height: 7, borderRadius: '50%', background: color, display: 'inline-block' }} />
    {text}
  </span>
);

export function LevelBadge({ level }: { level: string }) {
  const zh: Record<string, string> = { trainee: '实习生', c_level: 'C级', b_level: 'B级', a_level: 'A级', suggested_elimination: '建议淘汰', eliminated: '已淘汰' };
  const color = level === 'a_level' ? t.color.moneyIn : level === 'b_level' ? t.color.accent
    : level === 'c_level' ? t.color.warn : level === 'trainee' ? t.color.inkSoft : t.color.moneyOut;
  return <StatusDot color={color} text={zh[level] ?? level} />;
}

// ---------- StatCard：数字优先，钱用等宽强调 ----------

export const StatCard: React.FC<{ label: string; value: React.ReactNode; money?: boolean }> = ({ label, value, money: isMoney }) => (
  <div style={{
    padding: '14px 18px', background: '#fff', borderRadius: t.radius.md,
    border: `1px solid ${t.color.line}`, borderLeft: `3px solid ${t.color.ink}`,
  }}>
    <div style={{ font: `400 12px/1.4 ${t.type.family}`, color: t.color.inkSoft }}>{label}</div>
    <div style={{
      font: isMoney ? `650 22px/1.3 ${t.type.family}` : `700 22px/1.3 ${t.type.family}`,
      color: t.color.ink, marginTop: 4, fontVariantNumeric: 'tabular-nums',
    }}>{value}</div>
  </div>
);

// ---------- DataTablePage：声明式列表页模板 ----------

export interface Column<T> {
  key: string;
  title: string;
  render?: (row: T) => React.ReactNode;
  align?: 'left' | 'right';
}

export interface DataTablePageProps<T> {
  title: string;
  fetchPage: (page: number, limit: number) => Promise<{ rows: T[]; total: number }>;
  columns: Column<T>[];
  rowKey: (row: T) => React.Key;
  toolbar?: React.ReactNode;
  emptyText?: string;
}

export function DataTablePage<T>({ title, fetchPage, columns, rowKey, toolbar, emptyText = '暂无数据' }: DataTablePageProps<T>) {
  const [rows, setRows] = useState<T[]>([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const limit = 20;

  useEffect(() => {
    let live = true;
    setLoading(true); setErr(null);
    fetchPage(page, limit)
      .then(({ rows: r, total: tt }) => { if (live) { setRows(r); setTotal(tt); } })
      .catch((e: ApiError) => { if (live) setErr(e.message); })
      .finally(() => { if (live) setLoading(false); });
    return () => { live = false; };
  }, [page]);

  const totalPages = Math.max(1, Math.ceil(total / limit));

  return (
    <section>
      <header style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', margin: '0 0 14px' }}>
        <h1 style={{ font: `600 18px/1.3 ${t.type.family}`, color: t.color.ink, margin: 0 }}>{title}</h1>
        <div style={{ display: 'flex', gap: 8 }}>{toolbar}</div>
      </header>

      {err && <div style={{ padding: 12, background: '#FBEFEC', borderRadius: t.radius.sm, color: t.color.moneyOut, font: `400 13px/1.5 ${t.type.family}` }}>{err} —— 请刷新重试</div>}
      {loading && <div style={{ padding: 24, color: t.color.inkSoft, font: `400 13px ${t.type.family}` }}>数据待抓取…</div>}

      {!loading && !err && (
        <div style={{ background: '#fff', border: `1px solid ${t.color.line}`, borderRadius: t.radius.md, overflow: 'hidden' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr>
                {columns.map(c => (
                  <th key={c.key} style={{
                    font: `600 12px/1.4 ${t.type.family}`, color: t.color.inkSoft,
                    textAlign: c.align ?? 'left', padding: '10px 14px',
                    borderBottom: `1px solid ${t.color.line}`, background: t.color.paper,
                  }}>{c.title}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.length === 0 && (
                <tr><td colSpan={columns.length} style={{ padding: 28, textAlign: 'center', color: t.color.inkSoft, font: `400 13px ${t.type.family}` }}>{emptyText}</td></tr>
              )}
              {rows.map(row => (
                <tr key={rowKey(row)} style={{ borderBottom: `1px solid ${t.color.line}` }}>
                  {columns.map(c => (
                    <td key={c.key} style={{
                      padding: '10px 14px', font: `400 13px/1.5 ${t.type.family}`, color: t.color.ink,
                      textAlign: c.align ?? 'left', fontVariantNumeric: 'tabular-nums',
                    }}>{c.render ? c.render(row) : (row as any)[c.key]}</td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {!loading && !err && total > limit && (
        <nav style={{ display: 'flex', gap: 8, alignItems: 'center', justifyContent: 'flex-end', marginTop: 12 }}
             aria-label="分页">
          <span style={{ font: `400 12px ${t.type.family}`, color: t.color.inkSoft }}>共 {total} 条</span>
          <Btn kind="ghost" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>上一页</Btn>
          <span style={{ font: `400 12px ${t.type.family}` }}>{page} / {totalPages}</span>
          <Btn kind="ghost" disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>下一页</Btn>
        </nav>
      )}
    </section>
  );
}

// ---------- FormDialog ----------

export const FormDialog: React.FC<{
  open: boolean; title: string; onClose: () => void; onSubmit: () => Promise<void>;
  submitText?: string; children: React.ReactNode;
}> = ({ open, title, onClose, onSubmit, submitText = '保存', children }) => {
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  if (!open) return null;
  return (
    <div role="dialog" aria-modal="true" aria-label={title} onClick={onClose}
         style={{ position: 'fixed', inset: 0, background: 'rgba(22,48,43,.4)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 50 }}>
      <div onClick={e => e.stopPropagation()} style={{
        width: 420, maxWidth: '92vw', background: '#fff', borderRadius: t.radius.md,
        border: `1px solid ${t.color.line}`, padding: 24,
      }}>
        <h2 style={{ font: `600 16px/1.4 ${t.type.family}`, margin: '0 0 16px', color: t.color.ink }}>{title}</h2>
        <div style={{ display: 'grid', gap: 12 }}>{children}</div>
        {err && <div style={{ marginTop: 10, color: t.color.moneyOut, font: `400 13px ${t.type.family}` }}>{err}</div>}
        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 20 }}>
          <Btn kind="ghost" onClick={onClose}>取消</Btn>
          <Btn disabled={busy} onClick={async () => {
            setBusy(true); setErr(null);
            try { await onSubmit(); onClose(); } catch (e) { setErr((e as ApiError).message); } finally { setBusy(false); }
          }}>{busy ? '保存中…' : submitText}</Btn>
        </div>
      </div>
    </div>
  );
};

// ---------- AppShell：左窄导航 ----------

export interface NavItem { key: string; label: string; icon: string }
export const AppShell: React.FC<{
  nav: NavItem[]; active: string; onNav: (key: string) => void;
  userName?: string | null; userRole?: string | null; onLogout: () => void;
  children: React.ReactNode;
}> = ({ nav, active, onNav, userName, userRole, onLogout, children }) => (
  <div style={{ display: 'flex', minHeight: '100vh', background: t.color.paper }}>
    <aside style={{
      width: 200, flexShrink: 0, background: t.color.ink, color: '#E8EDEA',
      display: 'flex', flexDirection: 'column', padding: '18px 0',
    }}>
      <div style={{ font: `700 15px/1.4 ${t.type.family}`, padding: '0 20px 18px', borderBottom: `1px solid rgba(255,255,255,.12)`, marginBottom: 10 }}>
        呼叫中心<span style={{ color: '#7FB8A8' }}> 4.0</span>
      </div>
      {nav.map(n => (
        <button key={n.key} onClick={() => onNav(n.key)} aria-current={active === n.key}
          style={{
            display: 'flex', alignItems: 'center', gap: 10, padding: '11px 20px',
            font: `500 13px/1 ${t.type.family}`, textAlign: 'left', cursor: 'pointer',
            background: active === n.key ? 'rgba(255,255,255,.1)' : 'transparent',
            color: active === n.key ? '#fff' : '#B9C8C3', border: 'none',
            borderLeft: active === n.key ? `2px solid #7FB8A8` : '2px solid transparent',
          }}>
          <span aria-hidden style={{ fontSize: 14 }}>{n.icon}</span>{n.label}
        </button>
      ))}
      <div style={{ marginTop: 'auto', padding: '14px 20px 0', borderTop: `1px solid rgba(255,255,255,.12)` }}>
        <div style={{ font: `500 13px/1.4 ${t.type.family}`, color: '#fff' }}>{userName ?? '未登录'}</div>
        <div style={{ font: `400 11px/1.6 ${t.type.family}`, color: '#7F948E', marginBottom: 8 }}>
          {userRole === 'admin' ? '管理员' : userRole === 'manager' ? '团队经理' : userRole === 'finance' ? '财务' : userRole === 'employee' ? '销售' : userRole}
        </div>
        <button onClick={onLogout} style={{
          font: `400 12px/1 ${t.type.family}`, color: '#B9C8C3', background: 'transparent',
          border: 'none', cursor: 'pointer', padding: 0,
        }}>退出登录</button>
      </div>
    </aside>
    <main style={{ flex: 1, padding: 24, maxWidth: 1200, minWidth: 0 }}>{children}</main>
  </div>
);

// ---------- LoginPage：话务台签名 ----------

export const LoginPage: React.FC<{ onLogin: (name: string, role: string) => void }> = ({ onLogin }) => {
  const [phone, setPhone] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true); setErr(null);
    try {
      const res = await auth.login(phone, password);
      setToken(res.session.access_token);
      onLogin(res.user.name ?? '', res.user.role);
    } catch (e2) { setErr((e2 as ApiError).message); } finally { setBusy(false); }
  };

  return (
    <div style={{ minHeight: '100vh', display: 'grid', placeItems: 'center', background: t.color.ink }}>
      <form onSubmit={submit} style={{
        width: 360, maxWidth: '92vw', background: '#fff', borderRadius: t.radius.lg, padding: 32,
        border: `1px solid ${t.color.line}`,
      }}>
        <div style={{ font: `700 22px/1.3 ${t.type.family}`, color: t.color.ink, marginBottom: 4 }}>
          呼叫中心<span style={{ color: t.color.accent }}> 4.0</span>
        </div>
        <div style={{ font: `400 13px/1.6 ${t.type.family}`, color: t.color.inkSoft, marginBottom: 24 }}>
          教育体验课电销管理台——线索、业绩、薪资，一个地方看全。
        </div>
        <div style={{ display: 'grid', gap: 14 }}>
          <Field label="手机号" inputMode="tel" autoComplete="tel" required
                 value={phone} onChange={e => setPhone(e.target.value)} placeholder="登录手机号" />
          <Field label="密码" type="password" autoComplete="current-password" required
                 value={password} onChange={e => setPassword(e.target.value)} placeholder="登录密码" />
        </div>
        {err && <div role="alert" style={{ marginTop: 12, color: t.color.moneyOut, font: `400 13px/1.5 ${t.type.family}` }}>{err}</div>}
        <Btn type="submit" disabled={busy} style={{ width: '100%', marginTop: 20, padding: '11px 14px', fontSize: 14 }}>
          {busy ? '登录中…' : '进入工作台'}
        </Btn>
      </form>
    </div>
  );
};

// 金额单元格（等宽 + 绿色只给钱）
export const MoneyCell: React.FC<{ v: number | string | null; tone?: 'in' | 'out' }> = ({ v, tone }) => (
  <span style={{
    color: tone === 'in' ? t.color.moneyIn : tone === 'out' ? t.color.moneyOut : t.color.ink,
    fontWeight: 600,
  }}>{money(v)}</span>
);
