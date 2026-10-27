package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

func (h *apiHandler) registerPermissions(r chi.Router) {
	r.Get("/permissions/modules", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"success": true, "data": repo.Modules})
	})
	r.Get("/permissions/me", h.myPermissions)
	r.Get("/permissions/positions", func(w http.ResponseWriter, r *http.Request) {
		rows, err := repo.ListPositions(r.Context())
		if err != nil {
			dbError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"success": true, "data": rows})
	})
	r.Post("/permissions/positions", h.mutatePosition)
	r.Put("/permissions/positions/{positionId}", h.mutatePosition)
	r.Delete("/permissions/positions/{positionId}", h.mutatePosition)
	r.Get("/permissions/position/{positionName}", h.getPositionPermissions)
	r.Put("/permissions/position/{positionName}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			AllowedModules []string `json:"allowedModules"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil || in.AllowedModules == nil {
			writeErr(w, 400, "allowedModules 须为数组")
			return
		}
		if err := repo.SavePositionModules(r.Context(), chi.URLParam(r, "positionName"), in.AllowedModules); err != nil {
			dbError(w, err)
			return
		}
		h.getPositionPermissions(w, r)
	})
	r.Delete("/permissions/position/{positionName}", func(w http.ResponseWriter, r *http.Request) {
		tx, err := repo.Pool.Begin(r.Context())
		if err != nil {
			dbError(w, err)
			return
		}
		defer tx.Rollback(r.Context())
		name := chi.URLParam(r, "positionName")
		if _, err = tx.Exec(r.Context(), `DELETE FROM system_settings WHERE key=$1`, "position_permissions_"+name); err == nil {
			_, err = tx.Exec(r.Context(), `DELETE FROM position_module_permissions WHERE position_name=$1`, name)
		}
		if err != nil {
			dbError(w, err)
			return
		}
		if err = tx.Commit(r.Context()); err != nil {
			dbError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"success": true, "message": "权限已恢复默认"})
	})
}
func (h *apiHandler) myPermissions(w http.ResponseWriter, r *http.Request) {
	uid, role := mustUser(r.Context())
	allowed := repo.DefaultModules(role)
	source := "role"
	configured := false
	var position *string
	if role == "admin" {
		source = "admin"
		configured = true
	} else {
		err := repo.Pool.QueryRow(r.Context(), `SELECT position FROM employees WHERE user_id=$1`, uid).Scan(&position)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			dbError(w, err)
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			allowed = []string{"profile"}
			source = "unbound"
		}
		if position != nil {
			modules, ok, err := repo.PositionModules(r.Context(), repo.Pool, *position)
			if err != nil {
				dbError(w, err)
				return
			}
			if ok {
				allowed = modules
				source = "position"
				configured = true
			}
		}
	}
	writeJSON(w, 200, map[string]any{"success": true, "data": map[string]any{"allowedModules": allowed, "source": source, "configured": configured, "role": role, "position": position}})
}
func (h *apiHandler) getPositionPermissions(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "positionName")
	modules, configured, err := repo.PositionModules(r.Context(), repo.Pool, name)
	if err != nil {
		dbError(w, err)
		return
	}
	role := "employee"
	if !configured {
		rows, err := repo.ListPositions(r.Context())
		if err != nil {
			dbError(w, err)
			return
		}
		for _, p := range rows {
			if p.Name == name {
				role = p.RoleHint
			}
		}
		modules = repo.DefaultModules(role)
	}
	writeJSON(w, 200, map[string]any{"success": true, "data": map[string]any{"positionName": name, "configured": configured, "allowedModules": modules, "previewSource": role}})
}
func (h *apiHandler) mutatePosition(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name     string `json:"name"`
		RoleHint string `json:"role_hint"`
	}
	remove := r.Method == "DELETE"
	if !remove && json.NewDecoder(r.Body).Decode(&in) != nil {
		writeErr(w, 400, "请求参数错误")
		return
	}
	if in.RoleHint != "" && !hasRole(in.RoleHint, "employee", "manager", "workplace_manager", "hr", "finance") {
		writeErr(w, 400, "无效的角色")
		return
	}
	row, err := repo.MutatePosition(r.Context(), chi.URLParam(r, "positionId"), in.Name, in.RoleHint, remove)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"success": true, "message": "职位已保存", "data": row})
}
