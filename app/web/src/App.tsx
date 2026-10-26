import React, { useEffect, useState } from 'react';
import { AppShell, LoginPage, DataTablePage, StatCard, Btn, FormDialog, Field, LevelBadge, MoneyCell, Column } from './components/kit';
import { employeesApi, performanceApi, salaryApi, workdayApi, getToken, setToken } from './lib/api';

import { money, LEVEL_ZH } from './lib/tokens';

import type { Employee, PerformanceRow as PerfRow, SalaryRow } from './lib/api';

type PageKey = 'dashboard' | 'employees' | 'performance' | 'salary' | 'workdays';

function Dashboard({ onNav }: { onNav: (p: PageKey) => void }) {
  const [stats, setStats] = useState<Record<string, number> | null>(null);
  const [err, setErr] = useState<string | null>(null);
  useEffect(() => {
    employeesApi.stats().then(setStats).catch((e) => setErr(e.message));
  }, []);
  return (
    <section>
      <h1 style={{ font: '700 22px/1.3 var(--f)', color: 'var(--ink)', margin: '0 0 6px' }}>今日概览</h1>
      <p style={{ font: '400 13px/1.6 var(--f)', color: 'var(--ink-soft)', margin: '0 0 20px' }}>
        团队在册、各等级分布一目了然；明细去对应页面查看。
      </p>
      {err && <div style={{ color: 'var(--out)', font: '400 13px var(--f)' }}>{err}</div>}
      {stats && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(150px, 1fr))', gap: 12 }}>
          <StatCard label="在册员工" value={stats.total ?? 0} />
          <StatCard label="在职" value={stats.active ?? 0} />
          <StatCard label="实习生" value={stats.trainees ?? 0} />
          <StatCard label="C 级" value={stats.c_level ?? 0} />
          <StatCard label="B 级" value={stats.b_level ?? 0} />
          <StatCard label="A 级" value={stats.a_level ?? 0} />
        </div>
      )}
      <div style={{ display: 'flex', gap: 8, marginTop: 24 }}>
        <Btn kind="ghost" onClick={() => onNav('performance')}>查看每日业绩</Btn>
        <Btn kind="ghost" onClick={() => onNav('salary')}>查看薪资</Btn>
      </div>
    </section>
  );
}

function EmployeesPage() {
  const fetchPage = async (page: number, limit: number) => {
    const r = await employeesApi.list({ page, limit });
    return { rows: r.employees, total: r.pagination.total };
  };
  const cols: Column<Employee>[] = [
    { key: 'name', title: '姓名', render: r => <strong>{r.name}</strong> },
    { key: 'employee_code', title: '工号' },
    { key: 'level', title: '等级', render: r => <LevelBadge level={r.level} /> },
    { key: 'department', title: '部门' },
    { key: 'status', title: '状态', render: r => r.status === 'active' ? '在职' : r.status },
    { key: 'join_date', title: '入职日期', render: r => r.join_date?.slice(0, 10) ?? '—' },
  ];
  return <DataTablePage title="员工管理" fetchPage={fetchPage} columns={cols} rowKey={r => r.id} emptyText="还没有员工，先由管理员创建账号" />;
}

function PerformancePage() {
  const fetchPage = async (page: number, limit: number) => {
    const r = await performanceApi.list({ page, limit });
    return { rows: r.performances, total: r.pagination.total };
  };
  const [submitOpen, setSubmitOpen] = useState(false);
  const [form, setForm] = useState({ employeeId: '', workDate: new Date().toISOString().slice(0, 10), orders: '1' });
  const cols: Column<PerfRow>[] = [
    { key: 'work_date', title: '日期' },
    { key: 'employee_name', title: '员工', render: r => r.employee_name ?? `#${r.employee_id}` },
    { key: 'product_name', title: '产品', render: r => r.product_name ?? '—' },
    { key: 'orders_count', title: '订单数', align: 'right' },
    { key: 'commission_amount', title: '提成', align: 'right', render: r => <MoneyCell v={r.commission_amount} tone="in" /> },
  ];
  return (
    <>
      <DataTablePage title="每日业绩" fetchPage={fetchPage} columns={cols} rowKey={r => r.id}
        emptyText="今日还没有业绩录入"
        toolbar={<Btn onClick={() => setSubmitOpen(true)}>代录业绩</Btn>} />
      <FormDialog open={submitOpen} title="代录业绩" onClose={() => setSubmitOpen(false)}
        onSubmit={async () => {
          await performanceApi.submit({
            employeeId: Number(form.employeeId), workDate: form.workDate, orders: Number(form.orders),
          });
        }} submitText="录入">
        <Field label="员工 ID" inputMode="numeric" value={form.employeeId}
               onChange={e => setForm({ ...form, employeeId: e.target.value })} placeholder="员工编号" />
        <Field label="工作日期" type="date" value={form.workDate}
               onChange={e => setForm({ ...form, workDate: e.target.value })} />
        <Field label="订单数" inputMode="numeric" value={form.orders}
               onChange={e => setForm({ ...form, orders: e.target.value })} />
        <div style={{ font: '400 12px/1.6 var(--f)', color: 'var(--ink-soft)' }}>
          提成按现有规则自动计算：1–3 单 ×10 元，4–5 单每单 20 元，6 单起每单 30 元。
        </div>
      </FormDialog>
    </>
  );
}

function SalaryPage() {
  const [year, setYear] = useState(new Date().getFullYear());
  const [month, setMonth] = useState(new Date().getMonth() + 1);
  const fetchPage = async (page: number, limit: number) => {
    const r = await salaryApi.list({ page, limit, year, month });
    return { rows: r.salaries, total: r.pagination.total };
  };
  const [calcOpen, setCalcOpen] = useState(false);
  const [form, setForm] = useState({ employeeId: '' });
  const cols: Column<SalaryRow>[] = [
    { key: 'employee_id', title: '员工 ID' },
    { key: 'period', title: '账期', render: r => `${r.period_year}-${String(r.period_month).padStart(2, '0')}` },
    { key: 'base_salary', title: '底薪', align: 'right', render: r => <MoneyCell v={r.base_salary} /> },
    { key: 'commission_amount', title: '提成', align: 'right', render: r => <MoneyCell v={r.commission_amount} tone="in" /> },
    { key: 'bonus_amount', title: '奖金', align: 'right', render: r => <MoneyCell v={r.bonus_amount} /> },
    { key: 'net_salary', title: '应发', align: 'right', render: r => <MoneyCell v={r.net_salary} /> },
  ];
  return (
    <>
      <DataTablePage title="薪资管理" fetchPage={fetchPage} columns={cols} rowKey={r => r.id}
        emptyText="本月还没有薪资记录"
        toolbar={
          <>
            <select value={month} onChange={e => setMonth(Number(e.target.value))}
                    style={{ font: '400 13px var(--f)', padding: '7px 10px', borderRadius: 4, border: '1px solid var(--line)', background: '#fff' }}>
              {Array.from({ length: 12 }, (_, i) => i + 1).map(m => <option key={m} value={m}>{m} 月</option>)}
            </select>
            <Btn onClick={() => setCalcOpen(true)}>计算薪资</Btn>
          </>} />
      <FormDialog open={calcOpen} title="计算单人薪资" onClose={() => setCalcOpen(false)}
        onSubmit={async () => { await salaryApi.calculate(Number(form.employeeId), year, month); }}
        submitText="计算">
        <Field label="员工 ID" inputMode="numeric" value={form.employeeId}
               onChange={e => setForm({ employeeId: e.target.value })} />
        <div style={{ font: '400 12px/1.6 var(--f)', color: 'var(--ink-soft)' }}>
          账期：{year} 年 {month} 月。计算规则按等级自动套用。
        </div>
      </FormDialog>
    </>
  );
}

function WorkdaysPage() {
  const [year, setYear] = useState(new Date().getFullYear());
  const [month, setMonth] = useState(new Date().getMonth() + 1);
  const [cfg, setCfg] = useState<{ workdays: number[]; total: number; note?: string } | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = async () => {
    setErr(null);
    try {
      const w = await workdayApi.get(year, month);
      setCfg({ workdays: w.workdays, total: w.total_workdays });
    } catch {
      setCfg(null);
      setErr('该月未配置——下方按钮可一键生成周一至周五。');
    }
  };
  useEffect(() => { load(); }, [year, month]);

  const daysInMonth = new Date(year, month, 0).getDate();
  const toggle = (d: number) => {
    if (!cfg) return;
    const set = new Set(cfg.workdays);
    set.has(d) ? set.delete(d) : set.add(d);
    setCfg({ workdays: [...set].sort((a, b) => a - b), total: set.size });
  };

  return (
    <section>
      <h1 style={{ font: '600 18px/1.3 var(--f)', color: 'var(--ink)', margin: '0 0 6px' }}>工作日管理</h1>
      <p style={{ font: '400 13px/1.6 var(--f)', color: 'var(--ink-soft)', margin: '0 0 16px' }}>
        所有「X 个工作日」的考核、工资、全勤都以这份配置为准。
      </p>
      <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
        <select value={year} onChange={e => setYear(Number(e.target.value))}
                style={{ font: '400 13px var(--f)', padding: '7px 10px', borderRadius: 4, border: '1px solid var(--line)' }}>
          {[2025, 2026, 2027].map(y => <option key={y} value={y}>{y} 年</option>)}
        </select>
        <select value={month} onChange={e => setMonth(Number(e.target.value))}
                style={{ font: '400 13px var(--f)', padding: '7px 10px', borderRadius: 4, border: '1px solid var(--line)' }}>
          {Array.from({ length: 12 }, (_, i) => i + 1).map(m => <option key={m} value={m}>{m} 月</option>)}
        </select>
        <Btn onClick={async () => {
          setBusy(true);
          try {
            const days: number[] = [];
            for (let d = 1; d <= daysInMonth; d++) {
              const wd = new Date(year, month - 1, d).getDay();
              if (wd !== 0 && wd !== 6) days.push(d);
            }
            await workdayApi.save(year, month, days);
            await load();
          } finally { setBusy(false); }
        }}>生成周一至周五</Btn>
        {cfg && <Btn disabled={busy} onClick={async () => { await workdayApi.save(year, month, cfg.workdays); await load(); }}>
          保存配置（{cfg.total} 天）</Btn>}
      </div>
      {err && <div style={{ color: 'var(--warn)', font: '400 13px/1.6 var(--f)', marginBottom: 12 }}>{err}</div>}
      {cfg && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)', gap: 6, maxWidth: 480 }}>
          {Array.from({ length: daysInMonth }, (_, i) => i + 1).map(d => {
            const on = cfg.workdays.includes(d);
            return (
              <button key={d} onClick={() => toggle(d)} aria-pressed={on} style={{
                padding: '10px 0', font: '500 13px var(--f)', cursor: 'pointer',
                borderRadius: 4, border: `1px solid ${on ? 'var(--accent)' : 'var(--line)'}`,
                background: on ? 'var(--accent)' : '#fff', color: on ? '#fff' : 'var(--ink-soft)',
              }}>{d}</button>
            );
          })}
        </div>
      )}
    </section>
  );
}

export default function App() {
  const [page, setPage] = useState<PageKey>('dashboard');
  const [user, setUser] = useState<{ name: string; role: string } | null>(() => {
    if (!getToken()) return null;
    try { return JSON.parse(localStorage.getItem('cc4_user') || 'null'); } catch { return null; }
  });

  const handleLogin = (name: string, role: string) => {
    localStorage.setItem('cc4_user', JSON.stringify({ name, role }));
    setUser({ name, role });
  };
  const handleLogout = () => { setToken(null); localStorage.removeItem('cc4_user'); setUser(null); };

  if (!user) return <LoginPage onLogin={handleLogin} />;

  const nav = [
    { key: 'dashboard', label: '概览', icon: '◉' },
    { key: 'employees', label: '员工管理', icon: '☰' },
    { key: 'performance', label: '每日业绩', icon: '▤' },
    { key: 'salary', label: '薪资', icon: '¥' },
    { key: 'workdays', label: '工作日', icon: '▦' },
  ] as const;

  return (
    <AppShell nav={[...nav]} active={page} onNav={p => setPage(p as PageKey)}
              userName={user.name} userRole={user.role} onLogout={handleLogout}>
      {page === 'dashboard' && <Dashboard onNav={setPage} />}
      {page === 'employees' && <EmployeesPage />}
      {page === 'performance' && <PerformancePage />}
      {page === 'salary' && <SalaryPage />}
      {page === 'workdays' && <WorkdaysPage />}
    </AppShell>
  );
}
