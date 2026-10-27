package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

func (h *apiHandler) registerFinance(r chi.Router) {
	r.Route("/finance", func(r chi.Router) {
		for _, kind := range []string{"income", "expense", "receivable"} {
			r.Get("/"+kind, h.financeList(kind))
			r.Post("/"+kind, h.financeCreate(kind))
		}
		for _, kind := range []string{"income", "expense"} {
			r.Put("/"+kind+"/{id}", h.financeUpdate(kind))
			r.Delete("/"+kind+"/{id}", h.financeDelete(kind))
		}
		r.Post("/expense/recurring", h.financeCreate("expense"))
		r.Get("/cost-types", h.costTypes)
		r.Post("/cost-types", h.createCostType)
		r.Delete("/cost-types/{id}", h.deleteCostType)
		r.Get("/receivable/{id}", h.receivableDetail)
		r.Get("/receivable/{id}/payments", h.listPayments)
		r.Post("/receivable/{id}/payments", h.recordPayment)
		r.Put("/receivable/{id}/payment", h.recordPayment)
		r.Post("/sync-income", h.syncIncome)
	})
}

func recorderID(r *http.Request) *int {
	uid, _ := mustUser(r.Context())
	e, err := repo.GetEmployeeByUserID(r.Context(), uid)
	if err != nil {
		return nil
	}
	return &e.ID
}

func financeAccess(w http.ResponseWriter, r *http.Request, id int) bool {
	_, role := mustUser(r.Context())
	if hasRole(role, "admin", "finance") {
		return true
	}
	row, err := repo.One(r.Context(), repo.Pool, `SELECT recorded_by FROM finance_records WHERE id=$1`, id)
	if err != nil {
		dbError(w, err)
		return false
	}
	owner, ok := row["recorded_by"].(float64)
	if !ok {
		writeErr(w, 403, "无权限操作全局财务记录")
		return false
	}
	return canAccessEmployee(w, r, int(owner))
}

func (h *apiHandler) financeList(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, limit, offset := pageParams(r)
		q := r.URL.Query()
		start, end := q.Get("start_date"), q.Get("end_date")
		if kind == "receivable" {
			start = q.Get("due_date_from")
			end = q.Get("due_date_to")
		}
		if (start != "" && !validDate(start)) || (end != "" && !validDate(end)) || (start != "" && end != "" && start > end) {
			writeErr(w, 400, "日期范围无效")
			return
		}
		category := q.Get("category")
		if kind == "income" {
			category = q.Get("income_type")
		}
		_, team, err := employeeScope(r)
		if err != nil {
			writeErr(w, 403, "员工档案或团队不存在")
			return
		}
		rows, total, err := repo.ListFinance(r.Context(), repo.FinanceQuery{Type: kind, StartDate: start, EndDate: end, Category: category, Status: q.Get("status"), Limit: limit, Offset: offset, TeamID: team})
		if err != nil {
			dbError(w, err)
			return
		}
		key := kind
		if kind == "receivable" {
			key = "receivables"
		}
		writeJSON(w, 200, map[string]any{key: rows, "pagination": pagination(page, limit, total)})
	}
}

func (h *apiHandler) financeCreate(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, err := bodyMap(r)
		if err != nil {
			writeErr(w, 400, "请求参数错误")
			return
		}
		var in repo.FinanceInput
		// Accept legacy camelCase and snake_case clients through a single mapping.
		n := normalize(m)
		fields := map[string]string{"category": "category", "amount": "amount", "transaction_date": "transactionDate", "description": "description", "reference_id": "invoiceNumber", "customer_name": "customerName", "product_name": "productName", "order_count": "orderCount", "commission_per_order": "commissionPerOrder", "cost_type_id": "costTypeId", "cost_type": "costType", "total_amount": "totalAmount", "recurrence_frequency": "frequency", "recurrence_start_date": "startDate", "recurrence_end_date": "endDate", "account_title": "accountTitle"}
		values := map[string]any{}
		for key, target := range fields {
			if v, ok := n[key]; ok {
				values[target] = v
			}
		}
		for _, key := range []string{"frequency", "startDate", "endDate", "accountTitle"} {
			if v, ok := m[key]; ok {
				values[key] = v
			}
		}
		if kind == "receivable" {
			values["customerName"] = m["customer"]
			values["transactionDate"] = m["dueDate"]
			values["description"] = m["note"]
		}
		if strings.HasSuffix(r.URL.Path, "/recurring") {
			values["costType"] = "recurring"
		}
		b, _ := json.Marshal(values)
		if err = json.Unmarshal(b, &in); err != nil {
			writeErr(w, 400, "字段类型错误")
			return
		}
		row, err := repo.CreateFinance(r.Context(), kind, in, recorderID(r))
		if err != nil {
			dbError(w, err)
			return
		}
		writeJSON(w, 201, map[string]any{"success": true, "message": "财务记录创建成功", kind: row, "id": row["id"]})
	}
}

func (h *apiHandler) financeUpdate(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeErr(w, 400, "无效ID")
			return
		}
		if !financeAccess(w, r, id) {
			return
		}
		automatic, err := repo.One(r.Context(), repo.Pool, `SELECT COALESCE(source,'manual') AS source FROM finance_records WHERE id=$1`, id)
		if err != nil {
			dbError(w, err)
			return
		}
		if automatic["source"] == "auto" {
			writeErr(w, 409, "自动流水请修改来源记录")
			return
		}
		if _, err = repo.One(r.Context(), repo.Pool, `SELECT id FROM finance_records WHERE id=$1 AND type=$2`, id, kind); err != nil {
			dbError(w, err)
			return
		}
		m, err := bodyMap(r)
		if err != nil {
			writeErr(w, 400, "请求参数错误")
			return
		}
		fields := normalize(m)
		if amount, ok := fields["amount"]; ok {
			n, ok := amount.(float64)
			if !ok || !repo.ValidMoney(n) {
				writeErr(w, 400, "金额格式无效")
				return
			}
		}
		if err = repo.UpdateObject(r.Context(), repo.Pool, "finance_records", id, fields, repo.FinanceFields); err != nil {
			dbError(w, err)
			return
		}
		row, err := repo.One(r.Context(), repo.Pool, `SELECT * FROM finance_records WHERE id=$1`, id)
		if err != nil {
			dbError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"message": "更新成功", kind: row})
	}
}

func (h *apiHandler) financeDelete(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeErr(w, 400, "无效ID")
			return
		}
		if !financeAccess(w, r, id) {
			return
		}
		tag, err := repo.Pool.Exec(r.Context(), `DELETE FROM finance_records WHERE id=$1 AND type=$2 AND COALESCE(reference_type,'') NOT IN ('receivable_payment','daily_performance','salary')`, id, kind)
		if err != nil {
			dbError(w, err)
			return
		}
		if tag.RowsAffected() == 0 {
			writeErr(w, 409, "记录不存在或为自动生成的流水，请修改来源记录")
			return
		}
		writeJSON(w, 200, map[string]string{"message": "删除成功"})
	}
}

func (h *apiHandler) costTypes(w http.ResponseWriter, r *http.Request) {
	rows, err := repo.Objects(r.Context(), repo.Pool, `SELECT * FROM cost_types WHERE is_active=true ORDER BY category,name`)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"costTypes": rows})
}
func (h *apiHandler) createCostType(w http.ResponseWriter, r *http.Request) {
	_, role := mustUser(r.Context())
	if !hasRole(role, "admin", "workplace_manager") {
		writeErr(w, 403, "无权限管理成本类型")
		return
	}
	var in struct {
		Name        string `json:"name"`
		Category    string `json:"category"`
		Description string `json:"description"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Name) == "" {
		writeErr(w, 400, "请输入成本类型名称")
		return
	}
	var raw []byte
	err := repo.Pool.QueryRow(r.Context(), `INSERT INTO cost_types(name,category,description)VALUES($1,$2,$3)RETURNING to_jsonb(cost_types.*)`, in.Name, in.Category, in.Description).Scan(&raw)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"costType": json.RawMessage(raw)})
}
func (h *apiHandler) deleteCostType(w http.ResponseWriter, r *http.Request) {
	_, role := mustUser(r.Context())
	if !hasRole(role, "admin", "workplace_manager") {
		writeErr(w, 403, "无权限管理成本类型")
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	tag, err := repo.Pool.Exec(r.Context(), `UPDATE cost_types SET is_active=false WHERE id=$1`, id)
	if err != nil {
		dbError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, 404, "成本类型不存在")
		return
	}
	writeJSON(w, 200, map[string]string{"message": "成本类型已删除"})
}
func (h *apiHandler) receivableDetail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	if !financeAccess(w, r, id) {
		return
	}
	row, err := repo.GetReceivable(r.Context(), id)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"receivable": row})
}
func (h *apiHandler) listPayments(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	if !financeAccess(w, r, id) {
		return
	}
	if _, err = repo.GetReceivable(r.Context(), id); err != nil {
		dbError(w, err)
		return
	}
	rows, err := repo.Objects(r.Context(), repo.Pool, `SELECT * FROM payment_records WHERE receivable_id=$1 ORDER BY payment_date DESC,id DESC`, id)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"payments": rows})
}
func (h *apiHandler) recordPayment(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	if !financeAccess(w, r, id) {
		return
	}
	var in repo.PaymentInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeErr(w, 400, "请求参数错误")
		return
	}
	row, err := repo.RecordPayment(r.Context(), id, in, recorderID(r))
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"success": true, "message": "回款成功", "receivable": row})
}
func (h *apiHandler) syncIncome(w http.ResponseWriter, r *http.Request) {
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
	total, err := repo.SyncMonthlyIncome(r.Context(), in.Year, in.Month, team)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"success": true, "message": fmt.Sprintf("收入同步完成，共 %.2f 元", total), "amount": total})
}
