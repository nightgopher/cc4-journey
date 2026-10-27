package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
	"github.com/nightgopher/cc4-journey/app/internal/service"
)

func (h *apiHandler) registerWechat(r chi.Router) {
	r.Route("/wechat-leads", func(r chi.Router) {
		r.Get("/", h.wechatList)
		r.Get("/stats/overview", h.wechatStats)
		r.Get("/export/full", h.wechatExport)
		r.Get("/export/phone-only", h.wechatExport)
		r.Post("/sync", h.wechatSync)
		r.Post("/import", h.wechatImport)
		r.Get("/{id}", h.wechatDetail)
		r.Put("/{id}", h.wechatUpdate)
		r.Put("/{id}/assign", h.wechatAssign)
		r.Post("/{id}/follow-up", h.wechatFollowUp)
	})
}

func wechatFilter(r *http.Request) (string, []any, error) {
	self, team, err := employeeScope(r)
	if err != nil {
		return "", nil, err
	}
	where := `1=1`
	args := []any{}
	add := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	if self != nil {
		where += ` AND wl.assigned_to=` + add(*self)
	}
	if team != nil {
		where += ` AND wl.assigned_to IN(SELECT id FROM employees WHERE team_id=` + add(*team) + `)`
	}
	q := r.URL.Query()
	for _, key := range []string{"status", "city", "province"} {
		if v := q.Get(key); v != "" {
			where += ` AND wl.` + key + `=` + add(v)
		}
	}
	if q.Has("wechatAdded") {
		if q.Get("wechatAdded") != "true" && q.Get("wechatAdded") != "false" {
			return "", nil, fmt.Errorf("wechatAdded 格式无效")
		}
		where += ` AND wl.wechat_added=` + add(q.Get("wechatAdded") == "true")
	}
	if id := queryInt(r, "assignedTo"); id != nil {
		where += ` AND wl.assigned_to=` + add(*id)
	}
	if v := q.Get("keyword"); v != "" {
		n := add("%" + v + "%")
		where += ` AND (wl.real_phone ILIKE ` + n + ` OR wl.customer_name ILIKE ` + n + ` OR wl.call_status ILIKE ` + n + `)`
	}
	if v := q.Get("startDate"); v != "" {
		if !validDate(v) {
			return "", nil, fmt.Errorf("开始日期无效")
		}
		where += ` AND wl.call_time>=` + add(v) + `::date`
	}
	if v := q.Get("endDate"); v != "" {
		if !validDate(v) {
			return "", nil, fmt.Errorf("结束日期无效")
		}
		where += ` AND wl.call_time<` + add(v) + `::date+INTERVAL '1 day'`
	}
	return where, args, nil
}
func (h *apiHandler) wechatList(w http.ResponseWriter, r *http.Request) {
	where, args, err := wechatFilter(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	page, limit, offset := pageParams(r)
	if n := queryInt(r, "pageSize"); n != nil && *n > 0 && *n <= 500 {
		limit = *n
		offset = (page - 1) * limit
	}
	var total int
	if err = repo.Pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM wechat_leads wl WHERE `+where, args...).Scan(&total); err != nil {
		dbError(w, err)
		return
	}
	sql := `SELECT wl.*,e.name AS assigned_employee_name FROM wechat_leads wl LEFT JOIN employees e ON e.id=wl.assigned_to WHERE ` + where + fmt.Sprintf(` ORDER BY wl.created_at DESC,wl.id DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	rows, err := repo.Objects(r.Context(), repo.Pool, sql, append(args, limit, offset)...)
	if err != nil {
		dbError(w, err)
		return
	}
	p := pagination(page, limit, total)
	p["pageSize"] = limit
	p["totalCount"] = total
	writeJSON(w, 200, map[string]any{"success": true, "leads": rows, "pagination": p, "data": map[string]any{"list": rows, "pagination": p}})
}
func wechatAccess(w http.ResponseWriter, r *http.Request, id int) bool {
	row, err := repo.One(r.Context(), repo.Pool, `SELECT assigned_to FROM wechat_leads WHERE id=$1`, id)
	if err != nil {
		dbError(w, err)
		return false
	}
	_, role := mustUser(r.Context())
	if hasRole(role, "admin", "hr", "finance") {
		return true
	}
	n, ok := row["assigned_to"].(float64)
	if !ok {
		writeErr(w, 403, "线索尚未分配")
		return false
	}
	return canAccessEmployee(w, r, int(n))
}
func (h *apiHandler) wechatDetail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	if !wechatAccess(w, r, id) {
		return
	}
	row, err := repo.One(r.Context(), repo.Pool, `SELECT * FROM wechat_leads WHERE id=$1`, id)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"success": true, "data": row})
}
func (h *apiHandler) wechatUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	if !wechatAccess(w, r, id) {
		return
	}
	m, err := bodyMap(r)
	if err != nil {
		writeErr(w, 400, "请求参数错误")
		return
	}
	if err = repo.UpdateObject(r.Context(), repo.Pool, "wechat_leads", id, repo.Object(m), map[string]string{"customer_name": "", "status": "", "notes": "", "is_converted": ""}); err != nil {
		dbError(w, err)
		return
	}
	if v, ok := m["is_converted"].(bool); ok {
		_, err = repo.Pool.Exec(r.Context(), `UPDATE wechat_leads SET converted_at=CASE WHEN $2 THEN COALESCE(converted_at,CURRENT_TIMESTAMP) ELSE NULL END,status=CASE WHEN $2 THEN 'converted' ELSE status END WHERE id=$1`, id, v)
		if err != nil {
			dbError(w, err)
			return
		}
	}
	h.wechatDetail(w, r)
}
// wechatClosed rejects work on a lead that has reached a final state (PRD 3.2):
// a converted, lost or duplicate lead can no longer be reassigned or followed up,
// so the owner recorded at conversion stays the owner for attribution (F6).
func wechatClosed(w http.ResponseWriter, r *http.Request, id int, what string) bool {
	var status string
	if err := repo.Pool.QueryRow(r.Context(), `SELECT COALESCE(status,'') FROM wechat_leads WHERE id=$1`, id).Scan(&status); err != nil {
		writeErr(w, 404, "线索不存在")
		return true
	}
	switch status {
	case "converted", "lost", "duplicate":
		writeErr(w, 409, "已转化、无效或重复的线索不能再"+what)
		return true
	}
	return false
}
func (h *apiHandler) wechatAssign(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	_, role := mustUser(r.Context())
	if role != "admin" && !wechatAccess(w, r, id) {
		return
	}
	var in struct {
		EmployeeID int `json:"employeeId"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.EmployeeID <= 0 {
		writeErr(w, 400, "请选择员工")
		return
	}
	if !canAccessEmployee(w, r, in.EmployeeID) || wechatClosed(w, r, id, "分配") {
		return
	}
	tag, err := repo.Pool.Exec(r.Context(), `UPDATE wechat_leads SET assigned_to=$2,assigned_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE id=$1`, id, in.EmployeeID)
	if err != nil {
		dbError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, 404, "线索不存在")
		return
	}
	writeJSON(w, 200, map[string]any{"success": true, "message": "线索分配成功"})
}
func (h *apiHandler) wechatFollowUp(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	if !wechatAccess(w, r, id) {
		return
	}
	var in struct {
		Content string `json:"content"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Content) == "" {
		writeErr(w, 400, "请填写跟进内容")
		return
	}
	if wechatClosed(w, r, id, "跟进") {
		return
	}
	_, err = repo.Pool.Exec(r.Context(), `UPDATE wechat_leads SET last_follow_up_time=CURRENT_TIMESTAMP,last_follow_up_content=$2,follow_up_count=COALESCE(follow_up_count,0)+1,status=CASE WHEN status='new' THEN 'contacted' ELSE status END,updated_at=CURRENT_TIMESTAMP WHERE id=$1`, id, in.Content)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"success": true, "message": "跟进已记录"})
}
func (h *apiHandler) wechatStats(w http.ResponseWriter, r *http.Request) {
	where, args, err := wechatFilter(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	row, err := repo.One(r.Context(), repo.Pool, `SELECT COUNT(*) AS total_leads,COUNT(*) FILTER(WHERE wechat_added)AS wechat_added_count,COUNT(*) FILTER(WHERE status='new')AS new_leads,COUNT(*) FILTER(WHERE status='contacted')AS contacted_leads,COUNT(*) FILTER(WHERE status='qualified')AS qualified_leads,COUNT(*) FILTER(WHERE status='converted')AS converted_leads,COUNT(*) FILTER(WHERE assigned_to IS NOT NULL)AS assigned_leads,COALESCE(AVG(call_duration),0)AS avg_call_duration,COUNT(DISTINCT city)AS city_count,COUNT(DISTINCT province)AS province_count FROM wechat_leads wl WHERE `+where, args...)
	if err != nil {
		dbError(w, err)
		return
	}
	rate := 0.0
	if n := row["total_leads"].(float64); n > 0 {
		rate = row["wechat_added_count"].(float64) / n
	}
	row["wechat_add_rate"] = rate
	writeJSON(w, 200, map[string]any{"success": true, "data": row})
}

func (h *apiHandler) wechatImport(w http.ResponseWriter, r *http.Request) {
	_, role := mustUser(r.Context())
	if !hasRole(role, "admin", "manager", "workplace_manager") {
		writeErr(w, 403, "无权限导入线索")
		return
	}
	var in struct {
		Records []service.WechatRecord `json:"records"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.Records) == 0 || len(in.Records) > 1000 {
		writeErr(w, 400, "records 须包含 1–1000 条记录")
		return
	}
	for _, record := range in.Records {
		if record.AssignedTo != nil && !canAccessEmployee(w, r, *record.AssignedTo) {
			return
		}
	}
	if hasRole(role, "manager", "workplace_manager") {
		me := recorderID(r)
		if me == nil {
			writeErr(w, 403, "员工档案不存在")
			return
		}
		for i := range in.Records {
			if in.Records[i].AssignedTo == nil {
				in.Records[i].AssignedTo = me
			}
		}
	}
	saved, duplicate, err := service.ImportWechatRecords(r.Context(), in.Records)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"success": true, "data": map[string]any{"savedCount": saved, "duplicateCount": duplicate}})
}
func (h *apiHandler) wechatSync(w http.ResponseWriter, r *http.Request) {
	var in struct {
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.StartTime) < 10 || len(in.EndTime) < 10 || !validDate(in.StartTime[:10]) || !validDate(in.EndTime[:10]) || in.StartTime > in.EndTime {
		writeErr(w, 400, "请提供有效的 startTime/endTime")
		return
	}
	if !service.WechatConfigured() {
		writeErr(w, 503, "外呼同步尚未配置，请设置 GOOKI_HOST、GOOKI_USERNAME、GOOKI_PASSWORD")
		return
	}
	var assignee *int
	_, role := mustUser(r.Context())
	if hasRole(role, "manager", "workplace_manager") {
		assignee = recorderID(r)
	}
	result, err := service.SyncWechat(r.Context(), in.StartTime, in.EndTime, assignee)
	if err != nil {
		writeErr(w, 502, "外呼同步失败，请检查服务配置后重试")
		return
	}
	writeJSON(w, 200, map[string]any{"success": true, "message": "微信线索同步完成", "data": result})
}

func (h *apiHandler) wechatExport(w http.ResponseWriter, r *http.Request) {
	where, args, err := wechatFilter(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	rows, err := repo.Objects(r.Context(), repo.Pool, `SELECT wl.* FROM wechat_leads wl WHERE `+where+` ORDER BY wl.id LIMIT 5001`, args...)
	if err != nil {
		dbError(w, err)
		return
	}
	if len(rows) > 5000 {
		writeErr(w, 400, "导出最多 5000 条，请缩小筛选范围")
		return
	}
	headers := []string{"客户姓名", "手机号", "状态", "城市", "省份", "跟进内容"}
	fields := []string{"customer_name", "real_phone", "status", "city", "province", "last_follow_up_content"}
	if strings.HasSuffix(r.URL.Path, "phone-only") {
		headers = []string{"手机号"}
		fields = []string{"real_phone"}
	}
	data := [][]string{headers}
	seen := map[string]bool{}
	for _, row := range rows {
		if len(fields) == 1 {
			phone, _ := row["real_phone"].(string)
			if seen[phone] {
				continue
			}
			seen[phone] = true
		}
		values := []string{}
		for _, key := range fields {
			v := ""
			if row[key] != nil {
				v = fmt.Sprint(row[key])
			}
			values = append(values, v)
		}
		data = append(data, values)
	}
	b, err := xlsx(data)
	if err != nil {
		dbError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="wechat-leads-`+time.Now().Format("20060102")+`.xlsx"`)
	w.Write(b)
}

// Minimal OOXML workbook: user content is an inline string, never an Excel formula.
func xlsx(rows [][]string) ([]byte, error) {
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	parts := map[string]string{
		"[Content_Types].xml":        `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`,
		"_rels/.rels":                `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="微信线索" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
	}
	var sheet strings.Builder
	sheet.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for i, row := range rows {
		sheet.WriteString(`<row r="` + strconv.Itoa(i+1) + `">`)
		for _, value := range row {
			var escaped bytes.Buffer
			xml.EscapeText(&escaped, []byte(value))
			sheet.WriteString(`<c t="inlineStr"><is><t xml:space="preserve">` + escaped.String() + `</t></is></c>`)
		}
		sheet.WriteString(`</row>`)
	}
	sheet.WriteString(`</sheetData></worksheet>`)
	parts["xl/worksheets/sheet1.xml"] = sheet.String()
	for name, body := range parts {
		w, err := z.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>` + body)); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
