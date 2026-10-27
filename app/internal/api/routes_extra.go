// routes_extra.go —— 补齐存量其余 11 组路由（leads/customers/products/attendance/teams/notifications/settings）。
// 端点路径、语义与 JS 后端逐组对齐。
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

// RegisterExtraRoutes 挂载额外路由组（在 /api/v1 认证区内调用）。
func (h *apiHandler) RegisterExtraRoutes(r chi.Router) {
	// ---- 线索 ----
	r.Route("/leads", func(r chi.Router) {
		r.Get("/", h.listLeads)
		r.Post("/", h.createLead)
		r.Put("/{id}", h.updateLead)
		r.Delete("/{id}", h.deleteLead)
		r.Post("/assign-batch", h.assignLeadsBatch)
	})
	// ---- 客户 ----
	r.Route("/customers", func(r chi.Router) {
		r.Get("/", h.listCustomers)
		r.Post("/", h.createCustomer)
		r.Put("/{id}", h.updateCustomer)
		r.Delete("/{id}", h.deleteCustomer)
		r.Get("/{id}/orders", h.listCustomerOrders)
		r.Post("/{id}/orders", h.createCustomerOrder)
	})
	// ---- 产品 ----
	r.Route("/products", func(r chi.Router) {
		r.Get("/", h.listProducts)
		r.Get("/all", h.listProducts)
		r.Post("/", h.upsertProduct)
		r.Put("/{id}", h.upsertProductBody)
		r.Delete("/{id}", h.deleteProduct)
	})
	// ---- 考勤 ----
	r.Route("/attendance", func(r chi.Router) {
		r.Get("/", h.listAttendance)
		r.Post("/", h.upsertAttendance)
	})
	// ---- 团队 ----
	r.Route("/teams", func(r chi.Router) {
		r.Get("/", h.listTeams)
		r.Get("/all", h.listTeams)
		r.Get("/{id}/members", h.teamMembers)
		r.Post("/", h.createTeam)
		r.Put("/{id}", h.updateTeam)
		r.Delete("/{id}", h.deleteTeam)
	})
	// ---- 通知 ----
	r.Route("/notifications", func(r chi.Router) {
		r.Get("/", h.listNotifications)
		r.Get("/unread-count", h.unreadCount)
		r.Put("/mark-all-read", h.markAllRead)
		r.Put("/{id}/read", h.markRead)
		r.Delete("/{id}", h.deleteNotification)
	})
	// ---- 系统设置 ----
	r.Route("/settings", func(r chi.Router) {
		r.Get("/", h.listSettings)
		r.Put("/", h.upsertSetting)
		r.Post("/", h.upsertSetting)
		r.Delete("/{key}", h.deleteSetting)
	})
}

// ---- helpers ----

func pathID(r *http.Request) (int, error) { return strconv.Atoi(chi.URLParam(r, "id")) }
func bodyMap(r *http.Request) (map[string]any, error) {
	var m map[string]any
	err := json.NewDecoder(r.Body).Decode(&m)
	return m, err
}

func decodeNormalized(r *http.Request, v any) error {
	m, err := bodyMap(r)
	if err != nil {
		return err
	}
	b, err := json.Marshal(normalize(m))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func leadAccess(w http.ResponseWriter, r *http.Request, id int) bool {
	row, err := repo.One(r.Context(), repo.Pool, `SELECT assigned_to FROM leads WHERE id=$1`, id)
	if err != nil {
		dbError(w, err)
		return false
	}
	_, role := mustUser(r.Context())
	if hasRole(role, "admin", "hr") {
		return true
	}
	v, ok := row["assigned_to"].(float64)
	if !ok {
		writeErr(w, 403, "无权限操作未分配线索")
		return false
	}
	return canAccessEmployee(w, r, int(v))
}
func str(m map[string]any, k string) string { v, _ := m[k].(string); return v }

// ---- 线索 handlers ----

func (h *apiHandler) listLeads(w http.ResponseWriter, r *http.Request) {
	page, limit, offset := pageParams(r)
	self, team, err := employeeScope(r)
	if err != nil {
		writeErr(w, 403, "员工档案或团队不存在")
		return
	}
	assigned := queryInt(r, "assigned_to")
	if self != nil {
		assigned = self
	}
	rows, total, err := repo.ListLeads(r.Context(), r.URL.Query().Get("status"), r.URL.Query().Get("search"), assigned, team, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"leads": rows, "pagination": pagination(page, limit, total)})
}

func (h *apiHandler) createLead(w http.ResponseWriter, r *http.Request) {
	var l repo.Lead
	if err := decodeNormalized(r, &l); err != nil || l.CustomerName == "" || l.Phone == "" {
		writeErr(w, http.StatusBadRequest, "缺少必要参数：customer_name/phone")
		return
	}
	if l.NextFollowUp != nil && *l.NextFollowUp != "" && !validDate(*l.NextFollowUp) {
		writeErr(w, 400, "下次跟进日期格式须为 YYYY-MM-DD")
		return
	}
	uid, role := mustUser(r.Context())
	if role == "employee" || (hasRole(role, "manager", "workplace_manager") && l.AssignedTo == nil) {
		e, err := repo.GetEmployeeByUserID(r.Context(), uid)
		if err != nil {
			writeErr(w, 403, "员工档案不存在")
			return
		}
		l.AssignedTo = &e.ID
	}
	if l.AssignedTo != nil && !canAccessEmployee(w, r, *l.AssignedTo) {
		return
	}
	id, err := repo.CreateLead(r.Context(), &l)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"message": "线索创建成功", "id": id})
}

func (h *apiHandler) updateLead(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "无效ID")
		return
	}
	if !leadAccess(w, r, id) {
		return
	}
	m, err := bodyMap(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "请求参数错误")
		return
	}
	fields := normalize(m)
	if date, ok := fields["next_follow_up"].(string); ok && date != "" && !validDate(date) {
		writeErr(w, 400, "下次跟进日期无效")
		return
	}
	if _, ok := fields["assigned_to"]; ok {
		_, role := mustUser(r.Context())
		if role == "employee" {
			writeErr(w, 403, "无权限转移线索")
			return
		}
		if v, ok := fields["assigned_to"].(float64); ok && !canAccessEmployee(w, r, int(v)) {
			return
		}
	}
	if err := repo.UpdateLead(r.Context(), id, fields); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "线索更新成功"})
}

func (h *apiHandler) deleteLead(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "无效ID")
		return
	}
	if !leadAccess(w, r, id) {
		return
	}
	if err := repo.DeleteLead(r.Context(), id); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "线索删除成功"})
}

func (h *apiHandler) assignLeadsBatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LeadIDs    []int `json:"leadIds"`
		AssignedTo int   `json:"assignedTo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.LeadIDs) == 0 || body.AssignedTo == 0 {
		writeErr(w, http.StatusBadRequest, "缺少必要参数：leadIds/assignedTo")
		return
	}
	if !canAccessEmployee(w, r, body.AssignedTo) {
		return
	}
	for _, id := range body.LeadIDs {
		if !leadAccess(w, r, id) {
			return
		}
	}
	assignee, err := repo.GetEmployeeByID(r.Context(), body.AssignedTo)
	if err != nil || assignee.Status != "active" {
		writeErr(w, 400, "只能分配给在职员工")
		return
	}
	n, err := repo.AssignLeadsBatch(r.Context(), body.LeadIDs, body.AssignedTo)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "分配失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "批量分配成功", "assigned": n})
}

// ---- 客户 handlers ----

func (h *apiHandler) listCustomers(w http.ResponseWriter, r *http.Request) {
	page, limit, offset := pageParams(r)
	rows, total, err := repo.ListCustomers(r.Context(), r.URL.Query().Get("search"), limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"customers": rows, "pagination": pagination(page, limit, total)})
}

func (h *apiHandler) createCustomer(w http.ResponseWriter, r *http.Request) {
	var c repo.Customer
	if err := decodeNormalized(r, &c); err != nil || c.Name == "" || c.Phone == "" {
		writeErr(w, http.StatusBadRequest, "缺少必要参数：name/phone")
		return
	}
	id, err := repo.CreateCustomer(r.Context(), &c)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"message": "客户创建成功", "id": id})
}

func (h *apiHandler) updateCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "无效ID")
		return
	}
	m, err := bodyMap(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "请求参数错误")
		return
	}
	if err := repo.UpdateCustomer(r.Context(), id, normalize(m)); err != nil {
		writeErr(w, http.StatusInternalServerError, "更新失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "客户更新成功"})
}

func (h *apiHandler) deleteCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "无效ID")
		return
	}
	if err := repo.DeleteCustomer(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, "删除失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "客户删除成功"})
}

func (h *apiHandler) listCustomerOrders(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "无效ID")
		return
	}
	rows, err := repo.ListCustomerOrders(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": rows})
}

func (h *apiHandler) createCustomerOrder(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "无效ID")
		return
	}
	var o repo.CustomerOrder
	if err := decodeNormalized(r, &o); err != nil || o.TotalAmount <= 0 {
		writeErr(w, http.StatusBadRequest, "缺少必要参数：totalAmount")
		return
	}
	o.CustomerID = id
	oid, err := repo.CreateCustomerOrder(r.Context(), &o)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"message": "订单创建成功", "id": oid})
}

// ---- 产品 handlers ----

func (h *apiHandler) listProducts(w http.ResponseWriter, r *http.Request) {
	rows, err := repo.ListProducts(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"products": rows})
}

func (h *apiHandler) upsertProduct(w http.ResponseWriter, r *http.Request) {
	var p repo.Product
	if err := decodeProduct(r, &p); err != nil || p.Name == "" {
		writeErr(w, http.StatusBadRequest, "缺少必要参数：name")
		return
	}
	if p.Price < 0 {
		writeErr(w, 400, "产品价格不能为负数")
		return
	}
	id, err := repo.UpsertProduct(r.Context(), &p)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"message": "产品保存成功", "id": id})
}

func (h *apiHandler) upsertProductBody(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "无效ID")
		return
	}
	var p repo.Product
	if err := decodeProduct(r, &p); err != nil || p.Name == "" {
		writeErr(w, http.StatusBadRequest, "缺少必要参数：name")
		return
	}
	p.ID = id
	if p.Price < 0 {
		writeErr(w, 400, "产品价格不能为负数")
		return
	}
	if _, err := repo.UpsertProduct(r.Context(), &p); err != nil {
		writeErr(w, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "产品更新成功", "id": id})
}

func (h *apiHandler) deleteProduct(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "无效ID")
		return
	}
	if err := repo.DeleteProduct(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, "删除失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "产品删除成功"})
}

// ---- 考勤 handlers ----

func (h *apiHandler) listAttendance(w http.ResponseWriter, r *http.Request) {
	page, limit, offset := pageParams(r)
	employeeID := queryInt(r, "employee_id")
	self, team, err := employeeScope(r)
	if err != nil {
		writeErr(w, 403, "员工档案或团队不存在")
		return
	}
	if self != nil {
		employeeID = self
	}
	rows, total, err := repo.ListAttendance(r.Context(), employeeID, team,
		r.URL.Query().Get("start_date"), r.URL.Query().Get("end_date"), limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"attendance": rows, "pagination": pagination(page, limit, total)})
}

func (h *apiHandler) upsertAttendance(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EmployeeID int    `json:"employeeId"`
		Date       string `json:"date"`
		Status     string `json:"status"`
		LeaveType  string `json:"leaveType,omitempty"`
		Notes      string `json:"notes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.EmployeeID == 0 || body.Date == "" || body.Status == "" {
		writeErr(w, http.StatusBadRequest, "缺少必要参数：employeeId/date/status")
		return
	}
	if _, err := time.Parse("2006-01-02", body.Date); err != nil {
		writeErr(w, http.StatusBadRequest, "date 格式须为 YYYY-MM-DD")
		return
	}
	if !canAccessEmployee(w, r, body.EmployeeID) {
		return
	}
	if body.Status == "present" {
		body.Status = "work"
	}
	if body.Status == "late" {
		body.Status = "partial"
	}
	if body.Status == "leave" {
		body.Status = "absent"
	}
	if !hasRole(body.Status, "work", "partial", "absent") {
		writeErr(w, 400, "考勤状态无效")
		return
	}
	id, err := repo.UpsertAttendance(r.Context(), body.EmployeeID, body.Date, body.Status, body.LeaveType, body.Notes)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"message": "考勤记录保存成功", "id": id})
}

// ---- 团队 handlers ----

func (h *apiHandler) listTeams(w http.ResponseWriter, r *http.Request) {
	rows, err := repo.ListTeams(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	uid, role := mustUser(r.Context())
	if hasRole(role, "manager", "workplace_manager") {
		e, err := repo.GetEmployeeByUserID(r.Context(), uid)
		if err != nil {
			writeErr(w, 403, "员工档案不存在")
			return
		}
		filtered := []repo.Team{}
		for _, tm := range rows {
			if (e.TeamID != nil && tm.ID == *e.TeamID) || (tm.ManagerID != nil && *tm.ManagerID == e.ID) {
				filtered = append(filtered, tm)
			}
		}
		rows = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"teams": rows})
}

func (h *apiHandler) teamMembers(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "无效ID")
		return
	}
	if !canAccessTeam(w, r, id) {
		return
	}
	rows, err := repo.TeamMembers(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": rows})
}

func (h *apiHandler) createTeam(w http.ResponseWriter, r *http.Request) {
	var tm repo.Team
	if err := decodeNormalized(r, &tm); err != nil || tm.Name == "" || tm.ManagerID == nil {
		writeErr(w, http.StatusBadRequest, "缺少必要参数：name/managerId（团队必须指派经理）")
		return
	}
	id, err := repo.CreateTeam(r.Context(), &tm)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"message": "团队创建成功", "id": id})
}

func (h *apiHandler) updateTeam(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "无效ID")
		return
	}
	var tm repo.Team
	if err := decodeNormalized(r, &tm); err != nil || tm.Name == "" {
		writeErr(w, http.StatusBadRequest, "缺少必要参数：name")
		return
	}
	if err := repo.UpdateTeam(r.Context(), id, &tm); err != nil {
		writeErr(w, http.StatusInternalServerError, "更新失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "团队更新成功"})
}

func decodeProduct(r *http.Request, p *repo.Product) error {
	m, err := bodyMap(r)
	if err != nil {
		return err
	}
	n := normalize(m)
	if rules, ok := n["commission_rules"]; ok && rules != nil {
		if _, ok := rules.(string); !ok {
			b, err := json.Marshal(rules)
			if err != nil {
				return err
			}
			n["commission_rules"] = string(b)
		}
	}
	b, err := json.Marshal(n)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, p); err != nil {
		return err
	}
	if p.CommissionRules != nil {
		var rules struct {
			Tiers []repo.CommissionTier `json:"tiers"`
		}
		if err = json.Unmarshal([]byte(*p.CommissionRules), &rules); err != nil {
			return err
		}
		for _, t := range rules.Tiers {
			if t.From < 1 || (t.To != nil && *t.To < t.From) || t.Amount < 0 {
				return fmt.Errorf("提成档位无效")
			}
		}
	}
	return nil
}

func (h *apiHandler) deleteTeam(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "无效ID")
		return
	}
	if err := repo.DeleteTeam(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, "删除失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "团队删除成功"})
}

// ---- 通知 handlers ----

func (h *apiHandler) listNotifications(w http.ResponseWriter, r *http.Request) {
	uid, _ := mustUser(r.Context())
	page, limit, offset := pageParams(r)
	unreadOnly := r.URL.Query().Get("unread") == "true"
	rows, total, err := repo.ListNotifications(r.Context(), uid, unreadOnly, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": rows, "pagination": pagination(page, limit, total)})
}

func (h *apiHandler) unreadCount(w http.ResponseWriter, r *http.Request) {
	uid, _ := mustUser(r.Context())
	n, err := repo.UnreadCount(r.Context(), uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unreadCount": n})
}

func (h *apiHandler) markRead(w http.ResponseWriter, r *http.Request) {
	uid, _ := mustUser(r.Context())
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "无效ID")
		return
	}
	if err := repo.MarkNotificationRead(r.Context(), id, uid); err != nil {
		writeErr(w, http.StatusInternalServerError, "操作失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "已标记已读"})
}

func (h *apiHandler) markAllRead(w http.ResponseWriter, r *http.Request) {
	uid, _ := mustUser(r.Context())
	if err := repo.MarkAllNotificationsRead(r.Context(), uid); err != nil {
		writeErr(w, http.StatusInternalServerError, "操作失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "全部已读"})
}

func (h *apiHandler) deleteNotification(w http.ResponseWriter, r *http.Request) {
	uid, _ := mustUser(r.Context())
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "无效ID")
		return
	}
	if err := repo.DeleteNotification(r.Context(), id, uid); err != nil {
		writeErr(w, http.StatusInternalServerError, "删除失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "通知已删除"})
}

// ---- 设置 handlers ----

func (h *apiHandler) listSettings(w http.ResponseWriter, r *http.Request) {
	rows, err := repo.ListSettings(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	_, role := mustUser(r.Context())
	if !hasRole(role, "admin", "workplace_manager") {
		filtered := []repo.Setting{}
		for _, s := range rows {
			if s.IsPublic {
				filtered = append(filtered, s)
			}
		}
		rows = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": rows})
}

func (h *apiHandler) upsertSetting(w http.ResponseWriter, r *http.Request) {
	if _, role := mustUser(r.Context()); !hasRole(role, "admin", "workplace_manager") {
		writeErr(w, http.StatusForbidden, "仅管理员可修改设置")
		return
	}
	var s repo.Setting
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil || s.Key == "" {
		writeErr(w, http.StatusBadRequest, "缺少必要参数：key/value")
		return
	}
	if err := repo.UpsertSetting(r.Context(), &s); err != nil {
		writeErr(w, http.StatusInternalServerError, "保存失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "设置保存成功"})
}

func (h *apiHandler) deleteSetting(w http.ResponseWriter, r *http.Request) {
	if _, role := mustUser(r.Context()); !hasRole(role, "admin", "workplace_manager") {
		writeErr(w, http.StatusForbidden, "仅管理员可修改设置")
		return
	}
	key := chi.URLParam(r, "key")
	if err := repo.DeleteSetting(r.Context(), key); err != nil {
		writeErr(w, http.StatusInternalServerError, "删除失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "设置已删除"})
}

func (h *apiHandler) publicSettings(w http.ResponseWriter, r *http.Request) {
	rows, err := repo.ListSettings(r.Context())
	if err != nil {
		dbError(w, err)
		return
	}
	out := []repo.Setting{}
	for _, s := range rows {
		if s.IsPublic {
			out = append(out, s)
		}
	}
	writeJSON(w, 200, map[string]any{"settings": out})
}
