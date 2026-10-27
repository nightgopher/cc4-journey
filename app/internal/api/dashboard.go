package api

import (
	"net/http"
	"time"

	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

func (h *apiHandler) dashboardToday(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if !validDate(date) {
		writeErr(w, 400, "日期格式须为 YYYY-MM-DD")
		return
	}
	emp, team, err := employeeScope(r)
	if err != nil {
		writeErr(w, 403, "员工档案或团队不存在")
		return
	}
	if team == nil {
		team = queryInt(r, "teamId")
	}
	products, err := repo.Objects(r.Context(), repo.Pool, `SELECT p.id AS product_id,COALESCE(p.name,'日汇总') AS product_name,SUM(COALESCE(d.orders_count,0)) AS orders,
		SUM(CASE WHEN COALESCE(d.sales_amount,0)>0 THEN d.sales_amount ELSE COALESCE(p.price,0)*COALESCE(d.orders_count,0) END) AS revenue
		FROM daily_performance_records d JOIN employees e ON e.id=d.employee_id LEFT JOIN products p ON p.id=d.product_id
		WHERE d.work_date=$1::text::date AND ($2::int IS NULL OR e.id=$2) AND ($3::int IS NULL OR e.team_id=$3) GROUP BY p.id,p.name ORDER BY revenue DESC`, date, emp, team)
	if err != nil {
		dbError(w, err)
		return
	}
	orders, revenue := 0.0, 0.0
	for _, p := range products {
		orders += p["orders"].(float64)
		revenue += p["revenue"].(float64)
	}
	attendance, err := repo.Objects(r.Context(), repo.Pool, `SELECT t.id AS team_id,COALESCE(t.name,'未分配团队') AS team_name,COUNT(DISTINCT e.id) AS expected,
		COUNT(DISTINCT e.id) FILTER(WHERE ar.status='work' OR COALESCE(ar.attendance_days,0)>0) AS present
		FROM employees e LEFT JOIN users u ON u.id=e.user_id LEFT JOIN teams t ON t.id=e.team_id
		LEFT JOIN attendance_records ar ON ar.employee_id=e.id AND ar.date=$1::text::date
		WHERE e.status='active' AND COALESCE(u.role,'employee')!='admin' AND ($2::int IS NULL OR e.id=$2) AND ($3::int IS NULL OR e.team_id=$3)
		GROUP BY t.id,t.name ORDER BY t.name`, date, emp, team)
	if err != nil {
		dbError(w, err)
		return
	}
	expected, present := 0.0, 0.0
	for _, a := range attendance {
		ex, pr := a["expected"].(float64), a["present"].(float64)
		rate := 0.0
		if ex > 0 {
			rate = pr / ex
		}
		a["rate"] = rate
		expected += ex
		present += pr
	}
	rate := 0.0
	if expected > 0 {
		rate = present / expected
	}
	funnel, err := repo.One(r.Context(), repo.Pool, `SELECT COUNT(*) FILTER(WHERE wl.wechat_added=true) AS wechat_added_count,
		COUNT(*) FILTER(WHERE COALESCE(wl.call_duration,0)>0) AS answered_count
		FROM wechat_leads wl LEFT JOIN employees e ON e.id=wl.assigned_to WHERE wl.call_time>=$1::text::date AND wl.call_time<$1::text::date+INTERVAL '1 day'
		AND ($2::int IS NULL OR e.id=$2) AND ($3::int IS NULL OR e.team_id=$3)`, date, emp, team)
	if err != nil {
		dbError(w, err)
		return
	}
	funnel["purchase_count"] = orders
	funnel["call_out_total"] = nil
	funnel["wechat_add_rate"] = nil
	writeJSON(w, 200, map[string]any{"success": true, "data": map[string]any{"date": date, "products": products, "totals": map[string]any{"orders": orders, "revenue": revenue}, "attendance": map[string]any{"present": present, "expected": expected, "rate": rate}, "attendanceByTeam": attendance, "funnel": funnel}})
}

func (h *apiHandler) dashboardTrend(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	start, end := q.Get("startDate"), q.Get("endDate")
	a, ae := time.Parse("2006-01-02", start)
	b, be := time.Parse("2006-01-02", end)
	if ae != nil || be != nil || a.After(b) || b.Sub(a) > 3*366*24*time.Hour {
		writeErr(w, 400, "日期范围无效，最长支持三年")
		return
	}
	kind := q.Get("type")
	if kind == "" {
		kind = "daily"
	}
	if kind != "daily" && kind != "monthly" {
		writeErr(w, 400, "type 须为 daily 或 monthly")
		return
	}
	emp, team, err := employeeScope(r)
	if err != nil {
		writeErr(w, 403, "员工档案或团队不存在")
		return
	}
	if team == nil {
		team = queryInt(r, "teamId")
	}
	rows, err := repo.Objects(r.Context(), repo.Pool, `WITH days AS (SELECT generate_series($1::text::date,$2::text::date,'1 day'::interval)::date AS day),
		perf AS (SELECT d.work_date AS day,SUM(COALESCE(d.orders_count,0)) AS orders,SUM(CASE WHEN COALESCE(d.sales_amount,0)>0 THEN d.sales_amount ELSE COALESCE(p.price,0)*COALESCE(d.orders_count,0) END) AS revenue
		FROM daily_performance_records d JOIN employees e ON e.id=d.employee_id LEFT JOIN products p ON p.id=d.product_id WHERE d.work_date BETWEEN $1::text::date AND $2::text::date AND ($3::int IS NULL OR e.id=$3) AND ($4::int IS NULL OR e.team_id=$4) GROUP BY d.work_date),
		leads AS (SELECT wl.call_time::date AS day,COUNT(*) FILTER(WHERE wl.wechat_added) AS wechat_added,COUNT(*) FILTER(WHERE wl.call_duration>0) AS answered FROM wechat_leads wl LEFT JOIN employees e ON e.id=wl.assigned_to WHERE wl.call_time::date BETWEEN $1::text::date AND $2::text::date AND ($3::int IS NULL OR e.id=$3) AND ($4::int IS NULL OR e.team_id=$4) GROUP BY wl.call_time::date),
		att AS (SELECT ar.date AS day,COUNT(DISTINCT ar.employee_id) FILTER(WHERE ar.status='work' OR ar.attendance_days>0) AS present FROM attendance_records ar JOIN employees e ON e.id=ar.employee_id LEFT JOIN users u ON u.id=e.user_id WHERE ar.date BETWEEN $1::text::date AND $2::text::date AND COALESCE(u.role,'employee')!='admin' AND ($3::int IS NULL OR e.id=$3) AND ($4::int IS NULL OR e.team_id=$4) GROUP BY ar.date),
		expected AS (SELECT COUNT(*) AS n FROM employees e LEFT JOIN users u ON u.id=e.user_id WHERE e.status='active' AND COALESCE(u.role,'employee')!='admin' AND ($3::int IS NULL OR e.id=$3) AND ($4::int IS NULL OR e.team_id=$4))
		SELECT TO_CHAR(days.day,CASE WHEN $5::text='monthly' THEN 'YYYY-MM' ELSE 'YYYY-MM-DD' END) AS date,
		SUM(COALESCE(perf.orders,0)) AS orders,SUM(COALESCE(perf.revenue,0)) AS revenue,SUM(COALESCE(leads.wechat_added,0)) AS wechat_added,SUM(COALESCE(leads.answered,0)) AS answered,
		NULL::int AS call_out,CASE WHEN MAX(expected.n)>0 THEN SUM(COALESCE(att.present,0))::float/(MAX(expected.n)*COUNT(*)) ELSE 0 END AS attendance_rate
		FROM days LEFT JOIN perf USING(day) LEFT JOIN leads USING(day) LEFT JOIN att USING(day) CROSS JOIN expected GROUP BY 1 ORDER BY 1`, start, end, emp, team, kind)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"success": true, "data": map[string]any{"startDate": start, "endDate": end, "type": kind, "series": rows}})
}
