package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
	"net/http"
)

// Lookups expose only the identifiers and labels needed to complete an authorized
// form. They do not expose a personnel module's salary or contact details.
func (h *apiHandler) registerLookups(r chi.Router) {
	r.Get("/lookups/approvers", func(w http.ResponseWriter, r *http.Request) {
		_, role := mustUser(r.Context())
		if !hasRole(role, "admin", "hr", "manager", "workplace_manager") {
			writeErr(w, 403, "无审批权限")
			return
		}
		rows, err := repo.Objects(r.Context(), repo.Pool, `SELECT e.id,e.name,e.position FROM employees e JOIN users u ON u.id=e.user_id WHERE e.status='active' AND u.status='active' AND u.role IN('admin','hr','manager','workplace_manager')ORDER BY e.name`)
		if err != nil {
			dbError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"approvers": rows})
	})
	r.Get("/lookups/employees", func(w http.ResponseWriter, r *http.Request) {
		self, team, err := employeeScope(r)
		if err != nil {
			writeErr(w, 403, "员工档案或团队不存在")
			return
		}
		rows, err := repo.Objects(r.Context(), repo.Pool, `SELECT id,name,employee_code,user_id,team_id FROM employees WHERE status='active' AND($1::int IS NULL OR id=$1)AND($2::int IS NULL OR team_id=$2)ORDER BY name LIMIT 500`, self, team)
		if err != nil {
			dbError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"employees": rows})
	})
	r.Get("/lookups/products", func(w http.ResponseWriter, r *http.Request) {
		rows, err := repo.Objects(r.Context(), repo.Pool, `SELECT id,name,code,price,status FROM products WHERE status='active' ORDER BY name`)
		if err != nil {
			dbError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"products": rows})
	})
	r.Get("/lookups/teams", func(w http.ResponseWriter, r *http.Request) {
		uid, role := mustUser(r.Context())
		var self, team *int
		if hasRole(role, "employee", "manager", "workplace_manager") {
			e, err := repo.GetEmployeeByUserID(r.Context(), uid)
			if err != nil {
				writeErr(w, 403, "员工档案不存在")
				return
			}
			self = &e.ID
			team = e.TeamID
			if team == nil {
				team = new(int)
			}
		}
		rows, err := repo.Objects(r.Context(), repo.Pool, `SELECT id,name,code FROM teams WHERE status='active' AND($1::int IS NULL OR id=$1 OR manager_id=$2)ORDER BY name`, team, self)
		if err != nil {
			dbError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"teams": rows})
	})
}
