package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

func (h *apiHandler) registerApprovals(r chi.Router) {
	r.Get("/approvals", h.listApprovals)
	r.Post("/approvals", h.createApproval)
	r.Get("/approvals/my-requests", h.listApprovals)
	r.Get("/approvals/pending", h.listApprovals)
	r.Post("/approvals/{id}/action", h.approvalAction)
}
func (h *apiHandler) listApprovals(w http.ResponseWriter, r *http.Request) {
	page, limit, offset := pageParams(r)
	emp, team, err := employeeScope(r)
	if err != nil {
		writeErr(w, 403, "员工档案或团队不存在")
		return
	}
	where := `1=1`
	args := []any{}
	add := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	if emp != nil {
		p := add(*emp)
		where += ` AND(ar.requested_by=` + p + ` OR ar.current_approver=` + p + `)`
	}
	if team != nil {
		where += ` AND(requester.team_id=` + add(*team) + ` OR ar.current_approver=` + add(recorderID(r)) + `)`
	}
	if strings.HasSuffix(r.URL.Path, "/my-requests") {
		me := recorderID(r)
		if me == nil {
			writeJSON(w, 200, map[string]any{"myRequests": []any{}, "approvals": []any{}, "pagination": pagination(page, limit, 0)})
			return
		}
		where += ` AND ar.requested_by=` + add(*me)
	}
	if strings.HasSuffix(r.URL.Path, "/pending") {
		where += ` AND ar.status='pending'`
		_, role := mustUser(r.Context())
		if role != "admin" {
			me := recorderID(r)
			if me == nil {
				writeErr(w, 403, "员工档案不存在")
				return
			}
			where += ` AND (ar.current_approver IS NULL OR ar.current_approver=` + add(*me) + `)`
		}
	}
	for _, key := range []string{"status", "request_type"} {
		if v := r.URL.Query().Get(key); v != "" {
			where += ` AND ar.` + key + `=` + add(v)
		}
	}
	if v := queryInt(r, "requested_by"); v != nil {
		where += ` AND ar.requested_by=` + add(*v)
	}
	var total int
	if err = repo.Pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM approval_requests ar LEFT JOIN employees requester ON requester.id=ar.requested_by WHERE `+where, args...).Scan(&total); err != nil {
		dbError(w, err)
		return
	}
	rows, err := repo.Objects(r.Context(), repo.Pool, `SELECT ar.*,requester.name AS requester_name,approver.name AS current_approver_name FROM approval_requests ar LEFT JOIN employees requester ON requester.id=ar.requested_by LEFT JOIN employees approver ON approver.id=ar.current_approver WHERE `+where+` ORDER BY ar.requested_at DESC,ar.id DESC LIMIT `+add(limit)+` OFFSET `+add(offset), args...)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"approvals": rows, "myRequests": rows, "pendingApprovals": rows, "pagination": pagination(page, limit, total)})
}
func (h *apiHandler) createApproval(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RequestType     string         `json:"requestType"`
		Title           string         `json:"title"`
		Description     *string        `json:"description"`
		Priority        string         `json:"priority"`
		RequestedBy     *int           `json:"requestedBy"`
		CurrentApprover *int           `json:"currentApprover"`
		Metadata        map[string]any `json:"metadata"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.RequestType == "" || strings.TrimSpace(in.Title) == "" {
		writeErr(w, 400, "审批类型和标题为必填项")
		return
	}
	me := recorderID(r)
	if me == nil {
		writeErr(w, 403, "员工档案不存在")
		return
	}
	target := *me
	if in.RequestedBy != nil {
		target = *in.RequestedBy
	}
	if !canAccessEmployee(w, r, target) {
		return
	}
	_, role := mustUser(r.Context())
	if hasRole(in.RequestType, "promotion", "demotion", "elimination") && !hasRole(role, "admin", "hr", "manager", "workplace_manager") {
		writeErr(w, 403, "无权限发起等级审批")
		return
	}
	if in.Priority == "" {
		in.Priority = "medium"
	}
	if in.CurrentApprover != nil && !validApprover(r, *in.CurrentApprover) {
		writeErr(w, 400, "请选择在职的有效审批人")
		return
	}
	id, err := repo.InsertApprovalRequest(r.Context(), &repo.ApprovalRequest{RequestType: in.RequestType, Title: in.Title, Description: in.Description, Priority: in.Priority, RequestedBy: target, CurrentApprover: in.CurrentApprover, Metadata: in.Metadata})
	if err != nil {
		dbError(w, err)
		return
	}
	row, err := repo.PendingApproval(r.Context(), id)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"message": "审批申请创建成功", "approval": row, "id": id})
}
func (h *apiHandler) approvalAction(w http.ResponseWriter, r *http.Request) {
	_, role := mustUser(r.Context())
	if !hasRole(role, "admin", "hr", "manager", "workplace_manager") {
		writeErr(w, 403, "无权限处理审批")
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	a, err := repo.PendingApproval(r.Context(), id)
	if err != nil {
		dbError(w, err)
		return
	}
	me := recorderID(r)
	if me == nil {
		writeErr(w, 403, "审批人未绑定员工档案")
		return
	}
	assigned, hasAssigned := a["current_approver"].(float64)
	if (!hasAssigned && role != "admin") || (hasAssigned && int(assigned) != *me) {
		writeErr(w, 403, "当前用户不是指定审批人")
		return
	}
	var in struct {
		Action         string `json:"action"`
		Comments       string `json:"comments"`
		NextApproverID *int   `json:"nextApproverId"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeErr(w, 400, "请求参数错误")
		return
	}
	if in.NextApproverID != nil && !validApprover(r, *in.NextApproverID) {
		writeErr(w, 400, "请选择在职的有效审批人")
		return
	}
	if err = repo.DecideApproval(r.Context(), id, *me, role == "admin", in.Action, in.Comments, in.NextApproverID); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"message": "审批处理成功"})
}

func validApprover(r *http.Request, id int) bool {
	var valid bool
	err := repo.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM employees e JOIN users u ON u.id=e.user_id WHERE e.id=$1 AND e.status='active' AND u.status='active' AND u.role IN('admin','hr','manager','workplace_manager'))`, id).Scan(&valid)
	return err == nil && valid
}
