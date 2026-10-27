package api

import (
	"errors"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

func hasRole(role string, allowed ...string) bool {
	for _, v := range allowed {
		if role == v {
			return true
		}
	}
	return false
}

// authorize centralizes operation permissions; row ownership is checked separately.
func (h *apiHandler) authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, role := mustUser(r.Context())
		p := strings.TrimPrefix(r.URL.Path, "/api/v1/")
		if strings.HasPrefix(p, "/api/wechat-leads") {
			p = strings.TrimPrefix(p, "/api/")
		}
		write := r.Method != "GET"
		allowed := true
		switch {
		case strings.HasPrefix(p, "permissions/"):
			allowed = (!write && (p == "permissions/me" || p == "permissions/modules")) || role == "admin"
		case strings.HasPrefix(p, "finance/"):
			allowed = hasRole(role, "admin", "finance", "manager", "workplace_manager")
		case strings.HasPrefix(p, "settings"):
			allowed = !write || hasRole(role, "admin", "workplace_manager")
		case strings.HasPrefix(p, "teams"):
			allowed = !write || hasRole(role, "admin", "workplace_manager")
		case strings.HasPrefix(p, "products"):
			allowed = !write || hasRole(role, "admin", "workplace_manager")
		case strings.HasPrefix(p, "employees/") && (strings.Contains(p, "level") || strings.Contains(p, "batch")) && write:
			allowed = hasRole(role, "admin", "hr", "workplace_manager")
		case p == "employees" && write:
			allowed = hasRole(role, "admin", "hr", "manager", "workplace_manager")
		case strings.HasPrefix(p, "salary") && write:
			allowed = hasRole(role, "admin", "hr", "finance", "manager")
		case (strings.HasPrefix(p, "leads") || strings.HasPrefix(p, "customers")) && r.Method == "DELETE":
			allowed = hasRole(role, "admin", "manager", "workplace_manager")
		case p == "leads/assign-batch":
			allowed = hasRole(role, "admin", "manager", "workplace_manager")
		case strings.HasPrefix(p, "wechat-leads") && (strings.Contains(p, "/assign") || strings.Contains(p, "/sync")):
			allowed = hasRole(role, "admin", "manager", "workplace_manager")
		case strings.HasPrefix(p, "notifications") && r.Method == "POST":
			allowed = hasRole(role, "admin", "hr", "manager", "workplace_manager")
		}
		if !allowed {
			writeErr(w, http.StatusForbidden, "无权限执行此操作")
			return
		}
		if strings.HasPrefix(p, "finance/") && hasRole(role, "manager", "workplace_manager") {
			if _, _, err := employeeScope(r); err != nil {
				writeErr(w, 403, "员工档案或团队不存在")
				return
			}
		}
		if role != "admin" && !strings.HasPrefix(p, "auth/") && !strings.HasPrefix(p, "permissions/") && !strings.HasPrefix(p, "lookups/") {
			module := strings.Split(strings.Trim(p, "/"), "/")[0]
			module = mapModule(module)
			allowedModules := repo.DefaultModules(role)
			uid, _ := mustUser(r.Context())
			var position *string
			err := repo.Pool.QueryRow(r.Context(), `SELECT position FROM employees WHERE user_id=$1`, uid).Scan(&position)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					writeErr(w, 403, "员工档案不存在")
				} else {
					dbError(w, err)
				}
				return
			}
			if position != nil {
				configured, ok, err := repo.PositionModules(r.Context(), repo.Pool, *position)
				if err != nil {
					dbError(w, err)
					return
				}
				if ok {
					allowedModules = configured
				}
			}
			if !slices.Contains(allowedModules, module) {
				writeErr(w, 403, "当前职位无权访问此模块")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func mapModule(module string) string {
	switch module {
	case "finance":
		return "financial"
	case "approvals":
		return "approval"
	case "uploads":
		return "daily-performance"
	}
	return module
}

func canAccessTeam(w http.ResponseWriter, r *http.Request, id int) bool {
	uid, role := mustUser(r.Context())
	if hasRole(role, "admin", "hr") {
		return true
	}
	if role == "employee" {
		writeErr(w, 403, "无权限查看团队成员资料")
		return false
	}
	if hasRole(role, "manager", "workplace_manager") {
		e, err := repo.GetEmployeeByUserID(r.Context(), uid)
		if err != nil {
			writeErr(w, 403, "员工档案不存在")
			return false
		}
		if e.TeamID != nil && *e.TeamID == id {
			return true
		}
		var manages bool
		if err = repo.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM teams WHERE id=$1 AND manager_id=$2)`, id, e.ID).Scan(&manages); err == nil && manages {
			return true
		}
	}
	writeErr(w, 403, "无权限查看其他团队")
	return false
}

func employeeScope(r *http.Request) (employeeID, teamID *int, err error) {
	uid, role := mustUser(r.Context())
	if !hasRole(role, "employee", "manager", "workplace_manager") {
		return nil, nil, nil
	}
	e, err := repo.GetEmployeeByUserID(r.Context(), uid)
	if err != nil {
		return nil, nil, err
	}
	if role == "employee" {
		return &e.ID, nil, nil
	}
	if e.TeamID == nil {
		return nil, nil, errors.New("未分配团队")
	}
	return nil, e.TeamID, nil
}

func canAccessEmployee(w http.ResponseWriter, r *http.Request, id int) bool {
	emp, team, err := employeeScope(r)
	if err != nil {
		writeErr(w, 403, "员工档案或团队不存在")
		return false
	}
	if emp != nil && *emp != id {
		writeErr(w, 403, "只能操作自己的记录")
		return false
	}
	if team != nil {
		target, err := repo.TeamIDOfEmployee(r.Context(), id)
		if err != nil || target == nil || *target != *team {
			writeErr(w, 403, "只能操作本团队的记录")
			return false
		}
	}
	return true
}

func dbError(w http.ResponseWriter, err error) {
	if errors.Is(err, repo.ErrForbidden) {
		writeErr(w, 403, "无权限执行此操作")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, 404, "记录不存在")
		return
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			writeErr(w, 409, "记录已存在，请勿重复提交")
			return
		case "23503":
			writeErr(w, 409, "关联记录不存在或仍被使用")
			return
		case "23514", "23502", "22P02", "22007", "22008", "22003":
			writeErr(w, 400, "字段值不符合业务规则")
			return
		default:
			log.Printf("database: %v", err)
			writeErr(w, 500, "数据操作失败")
			return
		}
	}
	writeErr(w, 400, err.Error())
}

func validDate(s string) bool   { _, err := time.Parse("2006-01-02", s); return err == nil }
func validPeriod(y, m int) bool { return y >= 2000 && y <= 2100 && m >= 1 && m <= 12 }

func normalize(m map[string]any) repo.Object {
	aliases := map[string]string{"customerName": "customer_name", "assignedTo": "assigned_to", "nextFollowUp": "next_follow_up",
		"estimatedValue": "estimated_value", "customerType": "customer_type", "creditLimit": "credit_limit", "totalAmount": "total_amount",
		"orderDate": "order_date", "employeeId": "employee_id", "managerId": "manager_id", "commissionRules": "commission_rules",
		"transactionDate": "transaction_date", "invoiceNumber": "reference_id", "incomeType": "category", "vendor": "reference_id",
		"productName": "product_name", "orderCount": "order_count", "commissionPerOrder": "commission_per_order", "costTypeId": "cost_type_id",
		"costType": "cost_type", "recurrenceFrequency": "recurrence_frequency", "recurrenceStartDate": "recurrence_start_date", "recurrenceEndDate": "recurrence_end_date",
		"employeeCode": "employee_code", "joinDate": "join_date", "salaryBase": "salary_base", "teamId": "team_id"}
	out := repo.Object{}
	for k, v := range m {
		if mapped, ok := aliases[k]; ok {
			k = mapped
		}
		out[k] = v
	}
	return out
}
