package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

func (h *apiHandler) registerAttendance(r chi.Router) {
	r.Post("/attendance/batch", h.batchAttendance)
	r.Put("/attendance/{id}", h.updateAttendance)
	r.Get("/attendance/monthly-summary", h.monthlyAttendance)
	r.Post("/attendance/monthly-summary/sync", h.syncMonthlyAttendance)
	r.Post("/attendance/monthly-summary/initialize", h.syncMonthlyAttendance)
	r.Get("/attendance/leave-requests", h.leaveRequests)
	r.Post("/attendance/leave-requests", h.requestLeave)
	r.Put("/attendance/leave-requests/{id}/approve", h.approveLeave)
	r.Post("/attendance/generate-daily-records", h.generateAttendance)
	r.Post("/daily-performance/sync-attendance", h.syncMonthlyAttendance)
	r.Post("/daily-performance/sync-attendance-batch", h.syncMonthlyAttendance)
}
func (h *apiHandler) batchAttendance(w http.ResponseWriter, r *http.Request) {
	_, role := mustUser(r.Context())
	if !hasRole(role, "admin", "hr", "manager", "workplace_manager") {
		writeErr(w, 403, "无权限批量录入考勤")
		return
	}
	var in struct {
		Records []struct {
			EmployeeID int    `json:"employeeId"`
			Date       string `json:"date"`
			Status     string `json:"status"`
			Notes      string `json:"notes"`
		} `json:"records"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.Records) == 0 || len(in.Records) > 500 {
		writeErr(w, 400, "请提供 1–500 条考勤记录")
		return
	}
	for _, a := range in.Records {
		if !validDate(a.Date) || !hasRole(a.Status, "work", "partial", "absent") {
			writeErr(w, 400, "考勤日期或状态无效")
			return
		}
		if !canAccessEmployee(w, r, a.EmployeeID) {
			return
		}
	}
	tx, err := repo.Pool.Begin(r.Context())
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	for _, a := range in.Records {
		_, err = tx.Exec(r.Context(), `INSERT INTO attendance_records(employee_id,date,status,attendance_days,notes,source)VALUES($1,$2,$3::text,CASE WHEN $3::text='work' THEN 1 WHEN $3::text='partial' THEN 0.5 ELSE 0 END,$4,'manual') ON CONFLICT(employee_id,date)DO UPDATE SET status=EXCLUDED.status,attendance_days=EXCLUDED.attendance_days,notes=EXCLUDED.notes,source='manual',updated_at=CURRENT_TIMESTAMP`, a.EmployeeID, a.Date, a.Status, a.Notes)
		if err != nil {
			dbError(w, err)
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"message": "批量考勤已保存", "count": len(in.Records)})
}
func (h *apiHandler) updateAttendance(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	row, err := repo.One(r.Context(), repo.Pool, `SELECT employee_id FROM attendance_records WHERE id=$1`, id)
	if err != nil {
		dbError(w, err)
		return
	}
	if !canAccessEmployee(w, r, int(row["employee_id"].(float64))) {
		return
	}
	m, err := bodyMap(r)
	if err != nil {
		writeErr(w, 400, "请求参数错误")
		return
	}
	fields := normalize(m)
	fields["source"] = "manual"
	if _, ok := fields["attendance_days"]; ok {
		writeErr(w, 400, "出勤天数由出勤状态自动计算，不能直接修改")
		return
	}
	if v, ok := fields["status"].(string); ok {
		if !hasRole(v, "work", "partial", "absent") {
			writeErr(w, 400, "考勤状态无效")
			return
		}
		fields["attendance_days"] = map[string]float64{"work": 1, "partial": .5, "absent": 0}[v]
	}
	if err = repo.UpdateObject(r.Context(), repo.Pool, "attendance_records", id, fields, map[string]string{"status": "", "attendance_days": "", "notes": "", "leave_type": "", "source": ""}); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"message": "考勤已更新"})
}

const monthlyAttendanceSQL = `SELECT e.id AS employee_id,e.name AS employee_name,e.employee_code,$1::int AS year,$2::int AS month,
	COALESCE(wc.total_workdays,(SELECT COUNT(*) FROM generate_series(make_date($1,$2,1),(make_date($1,$2,1)+INTERVAL '1 month - 1 day')::date,'1 day'::interval) d WHERE EXTRACT(ISODOW FROM d)<6))AS total_workdays,
	COALESCE(SUM(COALESCE(ar.attendance_days,CASE WHEN ar.status='work' THEN 1 WHEN ar.status='partial' THEN 0.5 ELSE 0 END))FILTER(WHERE CASE WHEN wc.workdays IS NULL THEN EXTRACT(ISODOW FROM ar.date)<6 ELSE EXTRACT(DAY FROM ar.date)::int=ANY(wc.workdays) END),0)AS actual_work_days
	FROM employees e LEFT JOIN users u ON u.id=e.user_id LEFT JOIN workday_config wc ON wc.year=$1 AND wc.month=$2
	LEFT JOIN attendance_records ar ON ar.employee_id=e.id AND EXTRACT(YEAR FROM ar.date)=$1 AND EXTRACT(MONTH FROM ar.date)=$2
	WHERE e.status='active' AND COALESCE(u.role,'employee')!='admin' AND($3::int IS NULL OR e.id=$3) AND($4::int IS NULL OR e.team_id=$4)
	GROUP BY e.id,e.name,e.employee_code,wc.total_workdays,wc.workdays ORDER BY e.id`

func (h *apiHandler) monthlyAttendance(w http.ResponseWriter, r *http.Request) {
	y, m := queryInt(r, "year"), queryInt(r, "month")
	if y == nil || m == nil || !validPeriod(*y, *m) {
		writeErr(w, 400, "请选择有效年份和月份")
		return
	}
	self, team, err := employeeScope(r)
	if err != nil {
		writeErr(w, 403, "员工档案或团队不存在")
		return
	}
	if self == nil {
		self = queryInt(r, "employee_id")
	}
	rows, err := repo.Objects(r.Context(), repo.Pool, monthlyAttendanceSQL, *y, *m, self, team)
	if err != nil {
		dbError(w, err)
		return
	}
	for _, row := range rows {
		rate := 0.0
		total := row["total_workdays"].(float64)
		if total > 0 {
			rate = row["actual_work_days"].(float64) / total
		}
		row["attendance_rate"] = rate
	}
	page, limit, offset := pageParams(r)
	total := len(rows)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	writeJSON(w, 200, map[string]any{"summary": rows[offset:end], "pagination": pagination(page, limit, total)})
}
func (h *apiHandler) syncMonthlyAttendance(w http.ResponseWriter, r *http.Request) {
	_, role := mustUser(r.Context())
	if !hasRole(role, "admin", "hr", "manager", "workplace_manager") {
		writeErr(w, 403, "无权限同步考勤")
		return
	}
	var in struct {
		Year       int  `json:"year"`
		Month      int  `json:"month"`
		EmployeeID *int `json:"employeeId"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || !validPeriod(in.Year, in.Month) {
		writeErr(w, 400, "请选择有效年份和月份")
		return
	}
	self, team, err := employeeScope(r)
	if err != nil {
		writeErr(w, 403, "员工档案或团队不存在")
		return
	}
	if in.EmployeeID != nil {
		if !canAccessEmployee(w, r, *in.EmployeeID) {
			return
		}
		self = in.EmployeeID
	}
	tag, err := repo.Pool.Exec(r.Context(), `INSERT INTO monthly_attendance_summary(employee_id,year,month,total_workdays,actual_work_days)
	SELECT employee_id,year,month,total_workdays,FLOOR(actual_work_days)::int FROM (`+monthlyAttendanceSQL+`) q
	ON CONFLICT(employee_id,year,month)DO UPDATE SET total_workdays=EXCLUDED.total_workdays,actual_work_days=EXCLUDED.actual_work_days,updated_at=CURRENT_TIMESTAMP`, in.Year, in.Month, self, team)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"message": "月度考勤已同步", "count": tag.RowsAffected()})
}
func (h *apiHandler) requestLeave(w http.ResponseWriter, r *http.Request) {
	var in struct {
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
		LeaveType string `json:"leaveType"`
		Reason    string `json:"reason"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || !validDate(in.StartDate) || !validDate(in.EndDate) || in.StartDate > in.EndDate || strings.TrimSpace(in.Reason) == "" {
		writeErr(w, 400, "请填写有效的请假日期和原因")
		return
	}
	start, _ := time.Parse("2006-01-02", in.StartDate)
	end, _ := time.Parse("2006-01-02", in.EndDate)
	if end.Sub(start) > 366*24*time.Hour {
		writeErr(w, 400, "请假范围最长一年")
		return
	}
	me := recorderID(r)
	if me == nil {
		writeErr(w, 403, "员工档案不存在")
		return
	}
	id, err := repo.InsertApprovalRequest(r.Context(), &repo.ApprovalRequest{RequestType: "leave", Title: in.StartDate + " 至 " + in.EndDate + " 请假", Description: &in.Reason, Priority: "medium", RequestedBy: *me, Metadata: map[string]any{"start_date": in.StartDate, "end_date": in.EndDate, "leave_type": in.LeaveType, "reason": in.Reason}})
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"message": "请假申请已提交", "id": id})
}
func (h *apiHandler) leaveRequests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	q.Set("request_type", "leave")
	r.URL.RawQuery = q.Encode()
	w2 := newCaptureWriter()
	h.listApprovals(w2, r)
	var result map[string]any
	if json.Unmarshal(w2.body, &result) != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	result["leaveRequests"] = result["approvals"]
	writeJSON(w, w2.status, result)
}

type captureWriter struct {
	header http.Header
	status int
	body   []byte
}

func newCaptureWriter() *captureWriter       { return &captureWriter{header: http.Header{}, status: 200} }
func (c *captureWriter) Header() http.Header { return c.header }
func (c *captureWriter) WriteHeader(n int)   { c.status = n }
func (c *captureWriter) Write(b []byte) (int, error) {
	c.body = append(c.body, b...)
	return len(b), nil
}
func (h *apiHandler) approveLeave(w http.ResponseWriter, r *http.Request) { h.approvalAction(w, r) }
func (h *apiHandler) generateAttendance(w http.ResponseWriter, r *http.Request) {
	_, role := mustUser(r.Context())
	if !hasRole(role, "admin", "hr") {
		writeErr(w, 403, "无权限生成考勤")
		return
	}
	var in struct {
		Date string `json:"date"`
	}
	if r.ContentLength != 0 && json.NewDecoder(r.Body).Decode(&in) != nil {
		writeErr(w, 400, "请求参数错误")
		return
	}
	if in.Date == "" {
		in.Date = time.Now().Format("2006-01-02")
	}
	if !validDate(in.Date) {
		writeErr(w, 400, "日期无效")
		return
	}
	tag, err := repo.Pool.Exec(r.Context(), `INSERT INTO attendance_records(employee_id,date,status,attendance_days,source)SELECT e.id,$1::text::date,'absent',0,'default' FROM employees e LEFT JOIN users u ON u.id=e.user_id WHERE e.status='active' AND COALESCE(u.role,'employee')!='admin' ON CONFLICT(employee_id,date)DO NOTHING`, in.Date)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"message": "每日考勤已生成", "count": tag.RowsAffected()})
}
