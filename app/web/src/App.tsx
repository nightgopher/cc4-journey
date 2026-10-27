import React, { useCallback, useEffect, useMemo, useState } from "react";
import {
  Alert,
  AppShell,
  Btn,
  Icon,
  LoginPage,
  NavItem,
} from "./components/kit";
import { auth, getToken, request, Row, setToken, User } from "./lib/api";
import {
  AttendancePage,
  CustomersPage,
  FinancePage,
  LeadsPage,
  NotificationsPage,
  PermissionsPage,
  ProductsPage,
  SettingsPage,
  TeamsPage,
  WechatPage,
} from "./pages/business";
import {
  ApprovalsPage,
  DashboardPage,
  EmployeesPage,
  MonthlyPerformancePage,
  PerformancePage,
  ProfilePage,
  SalaryPage,
  WorkdaysPage,
} from "./pages/core";
import { WorkspaceContext } from "./pages/shared";
import { SimulationPage } from "./pages/simulation";

const NAV: (NavItem & { module: string })[] = [
  {
    key: "simulation",
    label: "业务演练",
    icon: "list",
    group: "演示验证",
    module: "simulation",
  },
  {
    key: "dashboard",
    label: "经营概览",
    icon: "dashboard",
    group: "经营",
    module: "dashboard",
  },
  {
    key: "leads",
    label: "线索管理",
    icon: "phone",
    group: "销售业务",
    module: "leads",
  },
  {
    key: "wechat",
    label: "微信线索",
    icon: "phone",
    group: "销售业务",
    module: "wechat-leads",
  },
  {
    key: "customers",
    label: "客户管理",
    icon: "people",
    group: "销售业务",
    module: "customers",
  },
  {
    key: "performance",
    label: "每日业绩",
    icon: "list",
    group: "销售业务",
    module: "daily-performance",
  },
  {
    key: "monthly",
    label: "月度业绩",
    icon: "calendar",
    group: "销售业务",
    module: "performance",
  },
  {
    key: "products",
    label: "产品管理",
    icon: "box",
    group: "销售业务",
    module: "products",
  },
  {
    key: "employees",
    label: "员工管理",
    icon: "people",
    group: "团队协作",
    module: "employees",
  },
  {
    key: "teams",
    label: "团队管理",
    icon: "people",
    group: "团队协作",
    module: "teams",
  },
  {
    key: "attendance",
    label: "考勤管理",
    icon: "calendar",
    group: "团队协作",
    module: "attendance",
  },
  {
    key: "approvals",
    label: "审批管理",
    icon: "check",
    group: "团队协作",
    module: "approval",
  },
  {
    key: "salary",
    label: "薪资管理",
    icon: "money",
    group: "财务与配置",
    module: "salary",
  },
  {
    key: "finance",
    label: "财务管理",
    icon: "money",
    group: "财务与配置",
    module: "financial",
  },
  {
    key: "workdays",
    label: "工作日配置",
    icon: "calendar",
    group: "财务与配置",
    module: "workdays",
  },
  {
    key: "notifications",
    label: "通知中心",
    icon: "bell",
    group: "系统",
    module: "notifications",
  },
  {
    key: "settings",
    label: "系统设置",
    icon: "settings",
    group: "系统",
    module: "settings",
  },
  {
    key: "permissions",
    label: "职位权限",
    icon: "shield",
    group: "系统",
    module: "admin",
  },
  {
    key: "profile",
    label: "个人中心",
    icon: "people",
    group: "系统",
    module: "profile",
  },
];
export default function App() {
  const [authenticated, setAuthenticated] = useState(!!getToken()),
    [user, setUser] = useState<User | null>(null),
    [modules, setModules] = useState<string[]>([]),
    [authError, setAuthError] = useState(""),
    [authRetry, setAuthRetry] = useState(0);
  const [page, setPage] = useState(
    () => new URLSearchParams(location.search).get("view") ?? "dashboard",
  );
  const [employees, setEmployees] = useState<Row[]>([]),
    [approvers, setApprovers] = useState<Row[]>([]),
    [products, setProducts] = useState<Row[]>([]),
    [teams, setTeams] = useState<Row[]>([]),
    [lookupVersion, setLookupVersion] = useState(0);
  const [notice, setNotice] = useState<{ text: string; error: boolean } | null>(
    null,
  );
  const notify = useCallback(
    (text: string, error = false) => setNotice({ text, error }),
    [],
  );
  const refresh = useCallback(() => setLookupVersion((v) => v + 1), []);
  const logout = useCallback(() => {
    setToken(null);
    localStorage.removeItem("cc4_user");
    setAuthenticated(false);
    setUser(null);
    setModules([]);
  }, []);
  useEffect(() => {
    const pop = () =>
      setPage(new URLSearchParams(location.search).get("view") ?? "dashboard");
    addEventListener("popstate", pop);
    addEventListener("cc4:unauthorized", logout);
    return () => {
      removeEventListener("popstate", pop);
      removeEventListener("cc4:unauthorized", logout);
    };
  }, [logout]);
  useEffect(() => {
    if (!authenticated) return;
    let live = true;
    setAuthError("");
    Promise.all([
      auth.session(),
      request<{ data: { allowedModules: string[] } }>("GET", "/permissions/me"),
    ])
      .then(([session, permissions]) => {
        if (live) {
          setUser(session.user);
          setModules(permissions.data.allowedModules);
        }
      })
      .catch((e: Error) => {
        if (live) setAuthError(e.message);
      });
    return () => {
      live = false;
    };
  }, [authenticated, authRetry]);
  useEffect(() => {
    if (!user) return;
    let live = true;
    Promise.all([
      request<{ employees: Row[] }>("GET", "/lookups/employees"),
      request<{ products: Row[] }>("GET", "/lookups/products"),
      request<{ teams: Row[] }>("GET", "/lookups/teams"),
      ["admin", "hr", "manager", "workplace_manager"].includes(user.role)
        ? request<{ approvers: Row[] }>("GET", "/lookups/approvers")
        : Promise.resolve({ approvers: [] as Row[] }),
    ])
      .then(([e, p, t, a]) => {
        if (live) {
          setEmployees(e.employees);
          setProducts(p.products.filter((r) => r.status === "active"));
          setTeams(t.teams);
          setApprovers(a.approvers);
        }
      })
      .catch((e: Error) => {
        if (live) notify(`基础资料加载失败：${e.message}`, true);
      });
    return () => {
      live = false;
    };
  }, [user, lookupVersion, notify]);
  const nav = useMemo(
    () =>
      NAV.filter((n) => user?.role === "admin" || modules.includes(n.module)),
    [user, modules],
  );
  const navigate = useCallback((key: string) => {
    const url = new URL(location.href);
    url.search = "";
    url.searchParams.set("view", key);
    history.pushState(null, "", url);
    setPage(key);
    document.getElementById("main-content")?.focus();
  }, []);
  useEffect(() => {
    if (!user || !nav.length) return;
    if (!nav.some((n) => n.key === page)) {
      const next = nav.some((n) => n.key === "performance")
        ? "performance"
        : nav[0].key;
      const url = new URL(location.href);
      url.search = `?view=${next}`;
      history.replaceState(null, "", url);
      setPage(next);
    }
  }, [user, nav, page]);
  if (!authenticated)
    return (
      <LoginPage
        onLogin={() => {
          setUser(null);
          setAuthenticated(true);
        }}
      />
    );
  if (authError)
    return (
      <main className="session-state">
        <h1>暂时无法进入工作台</h1>
        <Alert>{authError}</Alert>
        <div className="actions">
          <Btn onClick={() => setAuthRetry((v) => v + 1)}>重新连接</Btn>
          <Btn kind="ghost" onClick={logout}>
            返回登录
          </Btn>
        </div>
      </main>
    );
  if (!user)
    return (
      <main className="session-state" role="status">
        正在连接工作台…
      </main>
    );
  const views: Record<string, React.ReactNode> = {
    dashboard: <DashboardPage navigate={navigate} />,
    leads: <LeadsPage />,
    wechat: <WechatPage />,
    customers: <CustomersPage />,
    performance: <PerformancePage />,
    monthly: <MonthlyPerformancePage />,
    products: <ProductsPage />,
    employees: <EmployeesPage />,
    teams: <TeamsPage />,
    attendance: <AttendancePage />,
    approvals: <ApprovalsPage />,
    salary: <SalaryPage />,
    finance: <FinancePage />,
    workdays: <WorkdaysPage />,
    notifications: <NotificationsPage />,
    settings: <SettingsPage />,
    permissions: <PermissionsPage />,
    profile: <ProfilePage />,
    simulation: <SimulationPage />,
  };
  return (
    <WorkspaceContext.Provider
      value={{ user, notify, employees, products, teams, approvers, refresh }}
    >
      <AppShell
        nav={nav}
        active={page}
        onNav={navigate}
        userName={user.name}
        userRole={user.role}
        onLogout={logout}
      >
        {user.demo_mode && (
          <div className="environment-note" role="note">
            模拟经营环境 · 业务日期按北京时间，演练收付款为模拟记录。
          </div>
        )}
        <React.Fragment key={page}>
          {nav.some((n) => n.key === page) ? views[page] : null}
        </React.Fragment>
      </AppShell>
      {notice && (
        <div
          className="notice"
          role={notice.error ? "alert" : "status"}
          aria-live="polite"
        >
          <span>{notice.text}</span>
          <button
            className="icon-button"
            aria-label="关闭提示"
            onClick={() => setNotice(null)}
          >
            <Icon name="close" />
          </button>
        </div>
      )}
    </WorkspaceContext.Provider>
  );
}
