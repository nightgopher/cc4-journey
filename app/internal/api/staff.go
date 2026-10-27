package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
	"golang.org/x/crypto/bcrypt"
)

func (h *apiHandler) registerStaff(r chi.Router) {
	r.Post("/employees", h.createEmployee)
	r.Put("/employees/{id}", h.updateEmployee)
	r.Put("/employees/{id}/user-info", h.employeeUserInfo)
	r.Post("/employees/{id}/level-change", h.changeEmployeeLevel)
	r.Post("/employees/{id}/confirm-level-change", h.changeEmployeeLevel)
	r.Get("/employees/{id}/level-history", h.employeeLevelHistory)
	r.Get("/employees/{id}/performance-stats", h.employeePerformanceStats)
	r.Put("/employees/batch-status", h.batchEmployeeStatus)
	r.Delete("/employees/batch-delete", h.batchDeleteEmployees)
}
func (h *apiHandler) createEmployee(w http.ResponseWriter, r *http.Request) {
	_, role := mustUser(r.Context())
	m, err := bodyMap(r)
	if err != nil {
		writeErr(w, 400, "请求参数错误")
		return
	}
	b, _ := json.Marshal(normalize(m))
	var in repo.EmployeeInput
	if json.Unmarshal(b, &in) != nil {
		writeErr(w, 400, "字段类型错误")
		return
	}
	if !hasRole(in.Role, "", "employee", "manager", "workplace_manager", "hr", "admin", "finance") {
		writeErr(w, 400, "无效的角色")
		return
	}
	if role != "admin" && in.Role != "" && in.Role != "employee" {
		writeErr(w, 403, "仅管理员可分配管理角色")
		return
	}
	if hasRole(role, "manager", "workplace_manager") {
		_, team, err := employeeScope(r)
		if err != nil {
			writeErr(w, 403, "未分配团队")
			return
		}
		if in.TeamID != nil && *in.TeamID != *team {
			writeErr(w, 403, "只能创建本团队员工")
			return
		}
		in.TeamID = team
	}
	e, err := repo.CreateEmployee(r.Context(), in)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"message": "员工创建成功", "employee": e, "id": e.ID})
}
func (h *apiHandler) updateEmployee(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	if !canAccessEmployee(w, r, id) {
		return
	}
	m, err := bodyMap(r)
	if err != nil {
		writeErr(w, 400, "请求参数错误")
		return
	}
	fields := normalize(m)
	_, role := mustUser(r.Context())
	allowed := repo.EmployeeFields
	if role == "employee" {
		allowed = map[string]string{"phone": "", "email": "", "avatar_url": ""}
	}
	if role == "finance" {
		writeErr(w, 403, "无权限修改员工")
		return
	}
	if hasRole(role, "manager", "workplace_manager") {
		if v, ok := fields["team_id"]; ok {
			_, team, err := employeeScope(r)
			n, valid := v.(float64)
			if err != nil || !valid || team == nil || int(n) != *team {
				writeErr(w, 403, "不能转移员工到其他团队")
				return
			}
		}
	}
	if err = repo.UpdateObject(r.Context(), repo.Pool, "employees", id, fields, allowed); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"message": "员工更新成功"})
}
func (h *apiHandler) employeeUserInfo(w http.ResponseWriter, r *http.Request) {
	_, role := mustUser(r.Context())
	if !hasRole(role, "admin", "hr", "workplace_manager") {
		writeErr(w, 403, "无权限修改账号")
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	if !canAccessEmployee(w, r, id) {
		return
	}
	var in struct {
		Phone    string `json:"phone"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		Password string `json:"password"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeErr(w, 400, "请求参数错误")
		return
	}
	if in.Role != "" && (role != "admin" || !hasRole(in.Role, "admin", "hr", "manager", "workplace_manager", "employee", "finance")) {
		writeErr(w, 403, "无权限分配该角色")
		return
	}
	e, err := repo.GetEmployeeByID(r.Context(), id)
	if err != nil || e.UserID == nil {
		writeErr(w, 404, "关联账号不存在")
		return
	}
	fields := repo.Object{}
	if in.Phone != "" {
		fields["phone"] = in.Phone
	}
	if in.Email != "" {
		fields["email"] = in.Email
	}
	if in.Role != "" {
		fields["role"] = in.Role
	}
	if in.Password != "" {
		if len(in.Password) < 6 {
			writeErr(w, 400, "密码至少 6 位")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if err != nil {
			dbError(w, err)
			return
		}
		fields["password_hash"] = string(hash)
	}
	if err = repo.UpdateObject(r.Context(), repo.Pool, "users", *e.UserID, fields, map[string]string{"phone": "", "email": "", "role": "", "password_hash": ""}); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"message": "账号更新成功"})
}
func (h *apiHandler) changeEmployeeLevel(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	if !canAccessEmployee(w, r, id) {
		return
	}
	var in struct {
		ToLevel    string `json:"toLevel"`
		NewLevel   string `json:"newLevel"`
		ChangeDate string `json:"changeDate"`
		Reason     string `json:"reason"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeErr(w, 400, "请求参数错误")
		return
	}
	if in.ToLevel == "" {
		in.ToLevel = in.NewLevel
	}
	if !hasRole(in.ToLevel, "trainee", "c_level", "b_level", "a_level", "eliminated") {
		writeErr(w, 400, "无效的目标等级")
		return
	}
	if err = repo.ChangeEmployeeLevel(r.Context(), id, in.ToLevel, in.ChangeDate, in.Reason, recorderID(r)); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"message": "等级变更成功"})
}
func (h *apiHandler) employeeLevelHistory(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	if !canAccessEmployee(w, r, id) {
		return
	}
	rows, err := repo.Objects(r.Context(), repo.Pool, `SELECT * FROM employee_level_history WHERE employee_id=$1 ORDER BY change_date DESC,id DESC`, id)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"history": rows})
}
func (h *apiHandler) employeePerformanceStats(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	if !canAccessEmployee(w, r, id) {
		return
	}
	row, err := repo.One(r.Context(), repo.Pool, `SELECT COALESCE(SUM(orders_count),0)AS total_orders,COALESCE(SUM(sales_amount),0)AS total_sales,COALESCE(SUM(commission_amount),0)AS total_commission,COUNT(DISTINCT work_date)AS work_days FROM daily_performance_records WHERE employee_id=$1`, id)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, row)
}
func (h *apiHandler) batchEmployeeStatus(w http.ResponseWriter, r *http.Request) {
	var in struct {
		EmployeeIDs []int  `json:"employeeIds"`
		Status      string `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.EmployeeIDs) == 0 || !hasRole(in.Status, "active", "inactive", "suspended") {
		writeErr(w, 400, "员工列表或状态无效")
		return
	}
	uid, _ := mustUser(r.Context())
	for _, id := range in.EmployeeIDs {
		if !canAccessEmployee(w, r, id) {
			return
		}
		e, err := repo.GetEmployeeByID(r.Context(), id)
		if err == nil && e.UserID != nil && *e.UserID == uid && in.Status != "active" {
			writeErr(w, 400, "不能停用当前账号")
			return
		}
	}
	tag, err := repo.Pool.Exec(r.Context(), `UPDATE employees SET status=$2,updated_at=CURRENT_TIMESTAMP WHERE id=ANY($1)`, in.EmployeeIDs, in.Status)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"message": "员工状态已更新", "updated": tag.RowsAffected()})
}
func (h *apiHandler) batchDeleteEmployees(w http.ResponseWriter, r *http.Request) {
	var in struct {
		EmployeeIDs []int `json:"employeeIds"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.EmployeeIDs) == 0 {
		writeErr(w, 400, "请选择员工")
		return
	}
	uid, _ := mustUser(r.Context())
	for _, id := range in.EmployeeIDs {
		if !canAccessEmployee(w, r, id) {
			return
		}
		e, err := repo.GetEmployeeByID(r.Context(), id)
		if err == nil && e.UserID != nil && *e.UserID == uid {
			writeErr(w, 400, "不能删除当前账号")
			return
		}
	}
	tag, err := repo.Pool.Exec(r.Context(), `DELETE FROM employees WHERE id=ANY($1)`, in.EmployeeIDs)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"message": "员工已删除", "deleted": tag.RowsAffected(), "count": strconv.FormatInt(tag.RowsAffected(), 10)})
}
