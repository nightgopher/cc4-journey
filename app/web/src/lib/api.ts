/**
 * API 客户端 —— 对齐 Go 后端 (app/cmd/server) 与存量响应包。
 * 统一处理 session.access_token、错误中文提示、分页包。
 */
const BASE = (import.meta as any).env?.VITE_API_BASE_URL || 'http://localhost:3001/api/v1';

let accessToken: string | null = localStorage.getItem('cc4_token');

export function setToken(t: string | null) {
  accessToken = t;
  if (t) localStorage.setItem('cc4_token', t);
  else localStorage.removeItem('cc4_token');
}
export function getToken() { return accessToken; }

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) { super(message); this.status = status; }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (accessToken) headers['Authorization'] = `Bearer ${accessToken}`;
  const res = await fetch(`${BASE}${path}`, {
    method, headers, body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    if (res.status === 401) { setToken(null); }
    throw new ApiError(res.status, data.error || `请求失败（${res.status}）`);
  }
  return data as T;
}

// ---- 认证（对齐 POST /auth/login：token 在 session.access_token）----
export interface LoginResponse {
  message: string;
  user: { id: number; email: string | null; role: string; employee_id: number | null;
          name: string | null; department: string | null; level: string | null };
  session: { access_token: string; expires_in: string };
}
export const auth = {
  login: (phone: string, password: string) => request<LoginResponse>('POST', '/auth/login', { phone, password }),
  session: () => request<{ userId: number }>('GET', '/auth/session'),
};

export interface Pagination { page: number; limit: number; total: number; totalPages: number }
export interface Employee {
  id: number; employee_code: string | null; name: string; level: string; status: string;
  department: string | null; position: string | null; join_date: string | null;
  team_id: number | null; salary_base: number | string;
}
export const employeesApi = {
  list: (params: Record<string, string | number> = {}) => {
    const qs = new URLSearchParams(Object.entries(params).map(([k, v]) => [k, String(v)])).toString();
    return request<{ employees: Employee[]; pagination: Pagination }>('GET', `/employees?${qs}`);
  },
  get: (id: number) => request<Employee>('GET', `/employees/${id}`),
  stats: () => request<Record<string, number>>('GET', '/employees/stats/overview'),
};

export interface PerformanceRow {
  id: number; employee_id: number; work_date: string; orders_count: number;
  sales_amount: number; commission_amount: number; product_id: number | null;
  employee_name?: string | null; product_name?: string | null;
}
export const performanceApi = {
  list: (params: Record<string, string | number> = {}) => {
    const qs = new URLSearchParams(Object.entries(params).map(([k, v]) => [k, String(v)])).toString();
    return request<{ performances: PerformanceRow[]; pagination: Pagination }>('GET', `/daily-performance?${qs}`);
  },
  submit: (body: { employeeId: number; workDate: string; orders: number; productId?: number; workPeriod?: string }) =>
    request<{ message: string; dailyPerformance: { recordId: number; commissionAmount: number } }>(
      'POST', '/daily-performance', body),
  approve: (detailId: number, approved: boolean, rejectionReason?: string) =>
    request<{ message: string }>(`POST`, `/daily-performance/order-details/${detailId}/approve`,
      { approved, rejectionReason }),
};

export interface SalaryRow {
  id: number; employee_id: number; period_year: number; period_month: number;
  base_salary: number; commission_amount: number; bonus_amount: number;
  net_salary: number; payment_status: string;
}
export const salaryApi = {
  list: (params: Record<string, string | number> = {}) => {
    const qs = new URLSearchParams(Object.entries(params).map(([k, v]) => [k, String(v)])).toString();
    return request<{ salaries: SalaryRow[]; pagination: Pagination }>('GET', `/salary?${qs}`);
  },
  calculate: (employeeId: number, periodYear: number, periodMonth: number) =>
    request<{ netSalary: number; baseSalary: number; commissionAmount: number }>(
      'POST', '/salary/calculate', { employeeId, periodYear, periodMonth }),
  calculateBatch: (periodYear: number, periodMonth: number) =>
    request<{ message: string }>('POST', '/salary/calculate-batch', { periodYear, periodMonth }),
};

export interface WorkdayConfig { id: number; year: number; month: number; workdays: number[]; total_workdays: number }
export const workdayApi = {
  get: (year: number, month: number) => request<WorkdayConfig>('GET', `/workdays/${year}/${month}`),
  save: (year: number, month: number, workdays: number[]) =>
    request<{ message: string }>('POST', '/workdays', { year, month, workdays }),
};
