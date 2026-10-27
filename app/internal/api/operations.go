package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
	"golang.org/x/crypto/bcrypt"
)

func (h *apiHandler) registerOperations(r chi.Router) {
	r.Get("/products/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeErr(w, 400, "无效ID")
			return
		}
		row, err := repo.One(r.Context(), repo.Pool, `SELECT * FROM products WHERE id=$1`, id)
		if err != nil {
			dbError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"product": row})
	})
	r.Get("/teams/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeErr(w, 400, "无效ID")
			return
		}
		if !canAccessTeam(w, r, id) {
			return
		}
		row, err := repo.One(r.Context(), repo.Pool, `SELECT * FROM teams WHERE id=$1`, id)
		if err != nil {
			dbError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"team": row})
	})
	r.Post("/leads/{id}/mark-paid", h.leadOutcome)
	r.Post("/leads/{id}/mark-invalid", h.leadOutcome)
	r.Post("/notifications", h.createNotification)
	r.Post("/workdays/generate", h.generateWorkdays)
	r.Post("/workdays/range", h.workdayRange)
	r.Put("/auth/change-password", h.changePassword)
	r.Put("/auth/profile", h.updateProfile)
	r.Get("/auth/profile/stats", h.profileStats)
	r.Get("/auth/users", h.listUsers)
	r.Put("/auth/users/{id}/status", h.userStatus)
	r.Get("/salary/{id}/detail", h.salaryDetail)
	r.Put("/salary/{id}/payment-status", h.salaryPayment)
	r.Put("/salary/batch/payment-status", h.salaryPayment)
	r.Get("/performance", h.monthlyPerformance)
	r.Post("/performance/aggregate-to-monthly", h.aggregatePerformance)
}
func (h *apiHandler) leadOutcome(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	if !leadAccess(w, r, id) {
		return
	}
	status := "converted"
	if strings.HasSuffix(r.URL.Path, "mark-invalid") {
		status = "lost"
	}
	if err = repo.UpdateLead(r.Context(), id, map[string]any{"status": status}); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"message": "线索状态已更新"})
}
func (h *apiHandler) createNotification(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID  int    `json:"userId"`
		UserIDs []int  `json:"userIds"`
		Title   string `json:"title"`
		Message string `json:"message"`
		Type    string `json:"type"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Message) == "" {
		writeErr(w, 400, "通知标题和内容不能为空")
		return
	}
	if in.Type == "" {
		in.Type = "info"
	}
	if in.UserID > 0 {
		in.UserIDs = append(in.UserIDs, in.UserID)
	}
	if len(in.UserIDs) == 0 {
		writeErr(w, 400, "请选择通知接收人")
		return
	}
	_, role := mustUser(r.Context())
	if role != "admin" {
		for _, uid := range in.UserIDs {
			e, err := repo.GetEmployeeByUserID(r.Context(), uid)
			if err != nil {
				writeErr(w, 400, "接收人没有员工档案")
				return
			}
			if !canAccessEmployee(w, r, e.ID) {
				return
			}
		}
	}
	_, err := repo.Pool.Exec(r.Context(), `INSERT INTO notifications(user_id,title,message,type) SELECT DISTINCT unnest($1::int[]),$2,$3,$4`, in.UserIDs, in.Title, in.Message, in.Type)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 201, map[string]string{"message": "通知已创建"})
}
func (h *apiHandler) generateWorkdays(w http.ResponseWriter, r *http.Request) {
	_, role := mustUser(r.Context())
	if !hasRole(role, "admin", "workplace_manager") {
		writeErr(w, 403, "无权限修改工作日")
		return
	}
	var in struct {
		Year   int   `json:"year"`
		Months []int `json:"months"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.Months) == 0 {
		writeErr(w, 400, "请选择年份和月份")
		return
	}
	for _, m := range in.Months {
		if !validPeriod(in.Year, m) {
			writeErr(w, 400, "年份或月份无效")
			return
		}
	}
	configs := []*repo.WorkdayConfig{}
	for _, m := range in.Months {
		days := []int{}
		for d := time.Date(in.Year, time.Month(m), 1, 0, 0, 0, 0, time.UTC); int(d.Month()) == m; d = d.AddDate(0, 0, 1) {
			if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
				days = append(days, d.Day())
			}
		}
		cfg, err := repo.UpsertWorkdayConfig(r.Context(), in.Year, m, days)
		if err != nil {
			dbError(w, err)
			return
		}
		configs = append(configs, cfg)
	}
	writeJSON(w, 200, map[string]any{"message": "工作日已生成", "workdayConfigs": configs})
}
func (h *apiHandler) workdayRange(w http.ResponseWriter, r *http.Request) {
	var in struct {
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || !validDate(in.StartDate) || !validDate(in.EndDate) || in.StartDate > in.EndDate {
		writeErr(w, 400, "日期范围无效")
		return
	}
	start, _ := time.Parse("2006-01-02", in.StartDate)
	end, _ := time.Parse("2006-01-02", in.EndDate)
	if end.Sub(start) > 3*366*24*time.Hour {
		writeErr(w, 400, "日期范围最长三年")
		return
	}
	days, err := repo.WorkdaysUpTo(r.Context(), start, end)
	if err != nil {
		dbError(w, err)
		return
	}
	out := []string{}
	for _, d := range days {
		out = append(out, d.Format("2006-01-02"))
	}
	writeJSON(w, 200, map[string]any{"startDate": in.StartDate, "endDate": in.EndDate, "workdays": out, "count": len(out)})
}
func (h *apiHandler) changePassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CurrentPassword string `json:"currentPassword"`
		OldPassword     string `json:"oldPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.NewPassword) < 6 {
		writeErr(w, 400, "新密码至少 6 位")
		return
	}
	if in.CurrentPassword == "" {
		in.CurrentPassword = in.OldPassword
	}
	uid, _ := mustUser(r.Context())
	var hash string
	if err := repo.Pool.QueryRow(r.Context(), `SELECT password_hash FROM users WHERE id=$1`, uid).Scan(&hash); err != nil {
		dbError(w, err)
		return
	}
	if !checkPassword(in.CurrentPassword, hash) {
		writeErr(w, 400, "当前密码不正确")
		return
	}
	b, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		dbError(w, err)
		return
	}
	if _, err = repo.Pool.Exec(r.Context(), `UPDATE users SET password_hash=$2,updated_at=CURRENT_TIMESTAMP WHERE id=$1`, uid, string(b)); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"message": "密码已更新"})
}
func (h *apiHandler) updateProfile(w http.ResponseWriter, r *http.Request) {
	uid, _ := mustUser(r.Context())
	m, err := bodyMap(r)
	if err != nil {
		writeErr(w, 400, "请求参数错误")
		return
	}
	e, err := repo.GetEmployeeByUserID(r.Context(), uid)
	if err != nil {
		dbError(w, err)
		return
	}
	if err = repo.UpdateObject(r.Context(), repo.Pool, "employees", e.ID, normalize(m), map[string]string{"name": "", "email": "", "phone": "", "avatar_url": ""}); err != nil {
		dbError(w, err)
		return
	}
	h.session(w, r)
}
func (h *apiHandler) profileStats(w http.ResponseWriter, r *http.Request) {
	me := recorderID(r)
	if me == nil {
		writeJSON(w, 200, map[string]any{"total_orders": 0, "total_sales": 0})
		return
	}
	row, err := repo.One(r.Context(), repo.Pool, `SELECT COALESCE(SUM(orders_count),0)AS total_orders,COALESCE(SUM(sales_amount),0)AS total_sales,COUNT(DISTINCT work_date)AS work_days FROM daily_performance_records WHERE employee_id=$1`, *me)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, row)
}
func (h *apiHandler) listUsers(w http.ResponseWriter, r *http.Request) {
	_, role := mustUser(r.Context())
	if role != "admin" {
		writeErr(w, 403, "仅管理员可查看账号列表")
		return
	}
	page, limit, offset := pageParams(r)
	var total int
	if err := repo.Pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		dbError(w, err)
		return
	}
	rows, err := repo.Objects(r.Context(), repo.Pool, `SELECT u.id,u.phone,u.email,u.role,u.status,u.last_login,e.name,e.id AS employee_id FROM users u LEFT JOIN employees e ON e.user_id=u.id ORDER BY u.id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"users": rows, "pagination": pagination(page, limit, total)})
}
func (h *apiHandler) userStatus(w http.ResponseWriter, r *http.Request) {
	uid, role := mustUser(r.Context())
	if role != "admin" {
		writeErr(w, 403, "仅管理员可修改账号状态")
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || !hasRole(in.Status, "active", "inactive", "suspended") {
		writeErr(w, 400, "账号状态无效")
		return
	}
	if id == uid && in.Status != "active" {
		writeErr(w, 400, "不能停用当前账号")
		return
	}
	if err = repo.UpdateObject(r.Context(), repo.Pool, "users", id, repo.Object{"status": in.Status}, map[string]string{"status": ""}); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"message": "账号状态已更新"})
}
func (h *apiHandler) salaryDetail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	row, err := repo.One(r.Context(), repo.Pool, `SELECT s.*,e.name AS employee_name,e.level FROM salary_records s JOIN employees e ON e.id=s.employee_id WHERE s.id=$1`, id)
	if err != nil {
		dbError(w, err)
		return
	}
	if !canAccessEmployee(w, r, int(row["employee_id"].(float64))) {
		return
	}
	writeJSON(w, 200, map[string]any{"salary": row, "detail": row["calculation_details"]})
}
func (h *apiHandler) salaryPayment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SalaryIDs     []int  `json:"salaryIds"`
		IDs           []int  `json:"ids"`
		PaymentStatus string `json:"paymentStatus"`
		Status        string `json:"status"`
		PaymentDate   string `json:"paymentDate"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeErr(w, 400, "请求参数错误")
		return
	}
	if in.PaymentStatus == "" {
		in.PaymentStatus = in.Status
	}
	if !hasRole(in.PaymentStatus, "pending", "paid", "cancelled") || (in.PaymentDate != "" && !validDate(in.PaymentDate)) {
		writeErr(w, 400, "发放状态或日期无效")
		return
	}
	if id, err := strconv.Atoi(chi.URLParam(r, "id")); err == nil && id > 0 {
		in.SalaryIDs = []int{id}
	}
	if len(in.SalaryIDs) == 0 {
		in.SalaryIDs = in.IDs
	}
	if len(in.SalaryIDs) == 0 {
		writeErr(w, 400, "请选择薪资记录")
		return
	}
	for _, id := range in.SalaryIDs {
		row, err := repo.One(r.Context(), repo.Pool, `SELECT employee_id FROM salary_records WHERE id=$1`, id)
		if err != nil {
			dbError(w, err)
			return
		}
		if !canAccessEmployee(w, r, int(row["employee_id"].(float64))) {
			return
		}
	}
	if err := repo.SetSalaryPayment(r.Context(), in.SalaryIDs, in.PaymentStatus, in.PaymentDate); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"message": "薪资发放状态已更新"})
}
func (h *apiHandler) monthlyPerformance(w http.ResponseWriter, r *http.Request) {
	page, limit, offset := pageParams(r)
	emp, team, err := employeeScope(r)
	if err != nil {
		writeErr(w, 403, "员工档案或团队不存在")
		return
	}
	if emp == nil {
		emp = queryInt(r, "employee_id")
	}
	year, month := queryInt(r, "year"), queryInt(r, "month")
	where := `($1::int IS NULL OR p.employee_id=$1) AND ($2::int IS NULL OR e.team_id=$2) AND ($3::int IS NULL OR p.period_year=$3) AND ($4::int IS NULL OR p.period_month=$4)`
	var total int
	if err = repo.Pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM performance_records p JOIN employees e ON e.id=p.employee_id WHERE `+where, emp, team, year, month).Scan(&total); err != nil {
		dbError(w, err)
		return
	}
	rows, err := repo.Objects(r.Context(), repo.Pool, `SELECT p.*,e.name AS employee_name,product.name AS product_name FROM performance_records p JOIN employees e ON e.id=p.employee_id LEFT JOIN products product ON product.id=p.product_id WHERE `+where+` ORDER BY p.period_year DESC,p.period_month DESC,p.id DESC LIMIT $5 OFFSET $6`, emp, team, year, month, limit, offset)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"performances": rows, "pagination": pagination(page, limit, total)})
}
func (h *apiHandler) aggregatePerformance(w http.ResponseWriter, r *http.Request) {
	_, role := mustUser(r.Context())
	if !hasRole(role, "admin", "hr", "manager", "workplace_manager") {
		writeErr(w, 403, "无权限汇总业绩")
		return
	}
	var in struct {
		Year  int `json:"year"`
		Month int `json:"month"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || !validPeriod(in.Year, in.Month) {
		writeErr(w, 400, "年份或月份无效")
		return
	}
	_, team, err := employeeScope(r)
	if err != nil {
		writeErr(w, 403, "员工档案或团队不存在")
		return
	}
	count, err := repo.AggregateMonthlyPerformance(r.Context(), in.Year, in.Month, team)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"message": fmt.Sprintf("已汇总 %d 条月度业绩", count), "count": count})
}
