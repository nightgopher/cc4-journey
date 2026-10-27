/**
 * API 客户端 —— 对齐 Go 后端 (app/cmd/server) 与存量响应包。
 * 统一处理 session.access_token、错误中文提示、分页包。
 */
export const BASE =
  (import.meta as ImportMeta & { env?: Record<string, string> }).env
    ?.VITE_API_BASE_URL || "http://localhost:3101/api/v1";

let accessToken: string | null = localStorage.getItem("cc4_token");

export function setToken(t: string | null) {
  accessToken = t;
  if (t) localStorage.setItem("cc4_token", t);
  else localStorage.removeItem("cc4_token");
}
export function getToken() {
  return accessToken;
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export async function request<T = Record<string, unknown>>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const headers: Record<string, string> = {};
  if (!(body instanceof FormData)) headers["Content-Type"] = "application/json";
  if (accessToken) headers["Authorization"] = `Bearer ${accessToken}`;
  const res = await fetch(`${BASE}${path}`, {
    method,
    headers,
    body:
      body === undefined
        ? undefined
        : body instanceof FormData
          ? body
          : JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    if (res.status === 401) {
      setToken(null);
      window.dispatchEvent(new Event("cc4:unauthorized"));
    }
    throw new ApiError(res.status, data.error || `请求失败（${res.status}）`);
  }
  return data as T;
}

// ---- 认证（对齐 POST /auth/login：token 在 session.access_token）----
export interface LoginResponse {
  message: string;
  user: {
    id: number;
    email: string | null;
    role: string;
    employee_id: number | null;
    name: string | null;
    department: string | null;
    level: string | null;
  };
  session: { access_token: string; expires_in: string };
}
export const auth = {
  login: (phone: string, password: string) =>
    request<LoginResponse>("POST", "/auth/login", { phone, password }),
  session: () =>
    request<{ userId: number; user: User }>("GET", "/auth/session"),
};

export interface User {
  id: number;
  name: string | null;
  role: string;
  employee_id: number | null;
  phone: string | null;
  email: string | null;
  level: string | null;
  position: string | null;
  team_id: number | null;
}
export type Row = Record<string, unknown>;
export function queryString(
  params: Record<string, string | number | undefined>,
) {
  return new URLSearchParams(
    Object.entries(params)
      .filter(([, v]) => v !== undefined && v !== "")
      .map(([k, v]) => [k, String(v)]),
  ).toString();
}
export async function collection(
  path: string,
  key: string,
  page = 1,
  limit = 20,
  params: Record<string, string | number | undefined> = {},
) {
  const result = await request<Record<string, unknown>>(
    "GET",
    `${path}?${queryString({ page, limit, ...params })}`,
  );
  const rows = result[key];
  if (!Array.isArray(rows)) throw new Error("服务返回格式异常，请刷新重试");
  const pagination = result.pagination as Pagination | undefined;
  return pagination
    ? { rows: rows as Row[], total: pagination.total }
    : {
        rows: (rows as Row[]).slice((page - 1) * limit, page * limit),
        total: rows.length,
      };
}
export async function download(path: string) {
  const headers: Record<string, string> = {};
  if (accessToken) headers.Authorization = `Bearer ${accessToken}`;
  const res = await fetch(`${BASE}${path}`, { headers });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new ApiError(
      res.status,
      data.error || "导出失败，请缩小筛选范围后重试",
    );
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download =
    res.headers.get("Content-Disposition")?.match(/filename="([^"]+)"/)?.[1] ||
    (path.startsWith("/uploads/") ? path.split("/").pop() : "导出.xlsx") ||
    "附件";
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export interface Pagination {
  page: number;
  limit: number;
  total: number;
  totalPages: number;
}
export interface Employee {
  id: number;
  employee_code: string | null;
  name: string;
  level: string;
  status: string;
  department: string | null;
  position: string | null;
  join_date: string | null;
  team_id: number | null;
  salary_base: number | string;
}
export const employeesApi = {
  list: (params: Record<string, string | number> = {}) => {
    const qs = new URLSearchParams(
      Object.entries(params).map(([k, v]) => [k, String(v)]),
    ).toString();
    return request<{ employees: Employee[]; pagination: Pagination }>(
      "GET",
      `/employees?${qs}`,
    );
  },
  get: (id: number) => request<Employee>("GET", `/employees/${id}`),
  stats: () =>
    request<Record<string, number>>("GET", "/employees/stats/overview"),
};

export interface PerformanceRow {
  id: number;
  employee_id: number;
  work_date: string;
  orders_count: number;
  sales_amount: number;
  commission_amount: number;
  product_id: number | null;
  employee_name?: string | null;
  product_name?: string | null;
}
export const performanceApi = {
  list: (params: Record<string, string | number> = {}) => {
    const qs = new URLSearchParams(
      Object.entries(params).map(([k, v]) => [k, String(v)]),
    ).toString();
    return request<{ performances: PerformanceRow[]; pagination: Pagination }>(
      "GET",
      `/daily-performance?${qs}`,
    );
  },
  submit: (body: {
    employeeId: number;
    workDate: string;
    orders: number;
    productId?: number;
    workPeriod?: string;
  }) =>
    request<{
      message: string;
      dailyPerformance: { recordId: number; commissionAmount: number };
    }>("POST", "/daily-performance", body),
  approve: (detailId: number, approved: boolean, rejectionReason?: string) =>
    request<{ message: string }>(
      `POST`,
      `/daily-performance/order-details/${detailId}/approve`,
      { approved, rejectionReason },
    ),
};

export interface SalaryRow {
  id: number;
  employee_id: number;
  period_year: number;
  period_month: number;
  base_salary: number;
  commission_amount: number;
  bonus_amount: number;
  net_salary: number;
  payment_status: string;
}
export const salaryApi = {
  list: (params: Record<string, string | number> = {}) => {
    const qs = new URLSearchParams(
      Object.entries(params).map(([k, v]) => [k, String(v)]),
    ).toString();
    return request<{ salaries: SalaryRow[]; pagination: Pagination }>(
      "GET",
      `/salary?${qs}`,
    );
  },
  calculate: (employeeId: number, periodYear: number, periodMonth: number) =>
    request<{
      netSalary: number;
      baseSalary: number;
      commissionAmount: number;
    }>("POST", "/salary/calculate", { employeeId, periodYear, periodMonth }),
  calculateBatch: (periodYear: number, periodMonth: number) =>
    request<{ message: string }>("POST", "/salary/calculate-batch", {
      periodYear,
      periodMonth,
    }),
};

export interface WorkdayConfig {
  id: number;
  year: number;
  month: number;
  workdays: number[];
  total_workdays: number;
}
export const workdayApi = {
  get: (year: number, month: number) =>
    request<WorkdayConfig>("GET", `/workdays/${year}/${month}`),
  save: (year: number, month: number, workdays: number[]) =>
    request<{ message: string }>("POST", "/workdays", {
      year,
      month,
      workdays,
    }),
};
