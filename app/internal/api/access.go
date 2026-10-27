package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	businessaccess "github.com/nightgopher/cc4-journey/app/internal/access"
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
		case strings.HasPrefix(p, "simulation/"):
			allowed = role == "admin"
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
		if owner := r.Header.Get("X-CC4-Simulation-Owner"); write && owner != "" && p != "simulation/lease" {
			var value string
			err := repo.Pool.QueryRow(r.Context(), `SELECT value FROM system_settings WHERE key='business_simulation_lease_v1'`).Scan(&value)
			var lease struct {
				Owner     string    `json:"owner"`
				ExpiresAt time.Time `json:"expiresAt"`
			}
			if os.Getenv("BUSINESS_SIMULATION_ENABLED") != "true" || err != nil || !simulationOwnerPattern.MatchString(owner) || json.Unmarshal([]byte(value), &lease) != nil || lease.Owner != owner || lease.ExpiresAt.Before(time.Now().Add(2*time.Minute)) {
				writeErr(w, 409, "演练执行锁已失效，拒绝过期任务写入")
				return
			}
			// A mutation must finish well before this lease can expire and another
			// runner can acquire it, including pauses before the HTTP request.
			ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
			defer cancel()
			r = r.WithContext(ctx)
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
			uid, _ := mustUser(r.Context())
			principal := businessaccess.FromContext(r.Context())
			if principal == nil {
				principal, _ = businessaccess.Load(r.Context(), uid)
			}
			if principal == nil || principal.EmployeeID == 0 {
				writeErr(w, 403, "员工档案不存在")
				return
			}
			allowedModules, err := principal.Modules(r.Context())
			if err != nil {
				dbError(w, err)
				return
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
	case "mcp":
		return "profile"
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
	p := businessaccess.FromContext(r.Context())
	if p == nil {
		uid, _ := mustUser(r.Context())
		p, err = businessaccess.Load(r.Context(), uid)
		if err != nil {
			return nil, nil, err
		}
	}
	return p.Scope()
}

func canAccessEmployee(w http.ResponseWriter, r *http.Request, id int) bool {
	p := businessaccess.FromContext(r.Context())
	if p == nil {
		uid, _ := mustUser(r.Context())
		p, _ = businessaccess.Load(r.Context(), uid)
	}
	if p == nil || p.CheckEmployee(r.Context(), id) != nil {
		writeErr(w, 403, "无权限操作该员工的记录")
		return false
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
