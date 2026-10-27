package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
	"github.com/nightgopher/cc4-journey/app/internal/service"
)

func (h *apiHandler) registerOrders(r chi.Router) {
	r.Get("/daily-performance/order-details", h.orderDetails)
	r.Get("/daily-performance/pending-approval", h.orderDetails)
	r.Post("/daily-performance/{id}/approve", h.approveOrderDetail)
	r.Put("/daily-performance/{id}", h.updatePerformance)
	r.Delete("/daily-performance/{id}", h.deletePerformance)
	r.Delete("/daily-performance/order-details/{id}", h.deleteOrder)
	r.Put("/daily-performance/order-details/{id}/screenshots", h.updateScreenshots)
	r.Get("/daily-performance/order-details/{id}/edit-history", h.orderHistory)
	r.Get("/uploads/{name}", h.getUpload)
}
func uploadDir() string {
	if dir := os.Getenv("UPLOAD_DIR"); dir != "" {
		return dir
	}
	return "uploads"
}
func readScreenshots(w http.ResponseWriter, r *http.Request) (map[string]string, func(), error) {
	paths := []string{}
	cleanup := func() {
		for _, p := range paths {
			os.Remove(p)
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 13<<20)
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		return nil, cleanup, fmt.Errorf("图片总量不能超过 12 MB")
	}
	defer r.MultipartForm.RemoveAll()
	urls := map[string]string{}
	for _, key := range []string{"wechatPayScreenshot", "wechatChatScreenshot", "wechatContactScreenshot"} {
		file, _, err := r.FormFile(key)
		if err == http.ErrMissingFile {
			continue
		}
		if err != nil {
			return nil, cleanup, err
		}
		b, err := io.ReadAll(io.LimitReader(file, (4<<20)+1))
		file.Close()
		if err != nil || len(b) > 4<<20 {
			return nil, cleanup, fmt.Errorf("每张图片须小于 4 MB")
		}
		ext := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp"}[http.DetectContentType(b)]
		if ext == "" {
			return nil, cleanup, fmt.Errorf("仅支持 PNG、JPEG 或 WebP 图片")
		}
		id := make([]byte, 16)
		if _, err = rand.Read(id); err != nil {
			return nil, cleanup, err
		}
		name := hex.EncodeToString(id) + ext
		if err = os.MkdirAll(uploadDir(), 0700); err != nil {
			return nil, cleanup, err
		}
		p := filepath.Join(uploadDir(), name)
		if err = os.WriteFile(p, b, 0600); err != nil {
			return nil, cleanup, err
		}
		paths = append(paths, p)
		urls[key] = "/uploads/" + name
	}
	return urls, cleanup, nil
}
func (h *apiHandler) submitSelfOrder(w http.ResponseWriter, r *http.Request) {
	var in repo.OrderSubmission
	orders := 1
	var cleanup = func() {}
	success := false
	defer func() {
		if !success {
			cleanup()
		}
	}()
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		urls, clean, err := readScreenshots(w, r)
		cleanup = clean
		if err != nil {
			writeErr(w, 400, err.Error())
			return
		}
		in.EmployeeID, _ = strconv.Atoi(r.FormValue("employeeId"))
		in.WorkDate = r.FormValue("workDate")
		if id, err := strconv.Atoi(r.FormValue("productId")); err == nil && id > 0 {
			in.ProductID = &id
		}
		in.ParentPhone = r.FormValue("parentPhone")
		in.OrderID = r.FormValue("orderId")
		in.Notes = r.FormValue("notes")
		in.WorkPeriod = r.FormValue("workPeriod")
		in.ParentAddedTeacherWechat = r.FormValue("parentAddedTeacherWechat") == "true"
		in.PayURL = urls["wechatPayScreenshot"]
		in.ChatURL = urls["wechatChatScreenshot"]
		in.ContactURL = urls["wechatContactScreenshot"]
		value := r.FormValue("orders")
		if value == "" {
			value = r.FormValue("ordersCount")
		}
		if value != "" {
			orders, _ = strconv.Atoi(value)
		}
	} else {
		var raw map[string]json.RawMessage
		if json.NewDecoder(r.Body).Decode(&raw) != nil {
			writeErr(w, 400, "请求参数错误")
			return
		}
		b, _ := json.Marshal(raw)
		if json.Unmarshal(b, &in) != nil {
			writeErr(w, 400, "字段类型错误")
			return
		}
		if b, ok := raw["orders"]; ok {
			if json.Unmarshal(b, &orders) != nil {
				writeErr(w, 400, "订单数无效")
				return
			}
		} else if b, ok := raw["ordersCount"]; ok {
			if json.Unmarshal(b, &orders) != nil {
				writeErr(w, 400, "订单数无效")
				return
			}
		}
	}
	me := recorderID(r)
	if me == nil {
		writeErr(w, 403, "员工档案不存在")
		return
	}
	if in.EmployeeID == 0 {
		in.EmployeeID = *me
	}
	if in.EmployeeID != *me {
		writeErr(w, 403, "只能录入自己的业绩")
		return
	}
	if !validDate(in.WorkDate) || orders < 0 || orders > 1 {
		writeErr(w, 400, "请填写有效工作日期；自录每次提交 1 单或零业绩")
		return
	}
	if in.WorkPeriod == "" {
		in.WorkPeriod = "full_day"
	}
	if !hasRole(in.WorkPeriod, "full_day", "morning", "afternoon") {
		writeErr(w, 400, "工作时段无效")
		return
	}
	if orders == 0 {
		result, err := service.SubmitPerformance(r.Context(), service.SubmitPerformanceInput{EmployeeID: *me, WorkDate: in.WorkDate, Orders: 0, WorkPeriod: in.WorkPeriod})
		if err != nil {
			dbError(w, err)
			return
		}
		writeJSON(w, 201, map[string]any{"message": "零业绩及出勤已记录", "dailyPerformance": result, "needsApproval": false})
		return
	}
	if in.ProductID == nil || in.ParentPhone == "" || in.PayURL == "" || in.ChatURL == "" || in.ContactURL == "" {
		writeErr(w, 400, "请填写产品、家长手机号并提供支付、聊天和联系页三张截图")
		return
	}
	row, err := repo.SubmitOrder(r.Context(), in)
	if err != nil {
		dbError(w, err)
		return
	}
	success = true
	writeJSON(w, 201, map[string]any{"message": "业绩已提交，等待审核", "orderDetail": row, "needsApproval": true})
}
func (h *apiHandler) orderDetails(w http.ResponseWriter, r *http.Request) {
	self, team, err := employeeScope(r)
	if err != nil {
		writeErr(w, 403, "员工档案或团队不存在")
		return
	}
	if self == nil {
		self = queryInt(r, "employee_id")
	}
	status := r.URL.Query().Get("status")
	if strings.HasSuffix(r.URL.Path, "pending-approval") {
		status = "pending_approval"
	}
	page, limit, offset := pageParams(r)
	where := `($1::int IS NULL OR od.employee_id=$1)AND($2::int IS NULL OR e.team_id=$2)AND($3::text='' OR od.approval_status=$3)`
	var total int
	if err = repo.Pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM daily_performance_order_details od JOIN employees e ON e.id=od.employee_id WHERE `+where, self, team, status).Scan(&total); err != nil {
		dbError(w, err)
		return
	}
	rows, err := repo.Objects(r.Context(), repo.Pool, `SELECT od.*,e.name AS employee_name,p.name AS product_name FROM daily_performance_order_details od JOIN employees e ON e.id=od.employee_id LEFT JOIN products p ON p.id=od.product_id WHERE `+where+` ORDER BY od.created_at DESC,od.id DESC LIMIT $4 OFFSET $5`, self, team, status, limit, offset)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"orderDetails": rows, "pendingApprovals": rows, "pagination": pagination(page, limit, total)})
}
func (h *apiHandler) deleteOrder(w http.ResponseWriter, r *http.Request) {
	_, role := mustUser(r.Context())
	if !hasRole(role, "admin", "hr", "manager", "workplace_manager") {
		writeErr(w, 403, "无权限删除业绩明细")
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	row, err := repo.GetOrderDetail(r.Context(), id)
	if err != nil {
		dbError(w, err)
		return
	}
	if !canAccessEmployee(w, r, row.EmployeeID) {
		return
	}
	if err = repo.DeleteOrderDetail(r.Context(), id); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"message": "明细已删除，业绩已更新"})
}
func (h *apiHandler) updatePerformance(w http.ResponseWriter, r *http.Request) {
	_, role := mustUser(r.Context())
	if !hasRole(role, "admin", "hr", "manager", "workplace_manager") {
		writeErr(w, 403, "请通过明细修改自录业绩")
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	row, err := repo.One(r.Context(), repo.Pool, `SELECT employee_id FROM daily_performance_records WHERE id=$1`, id)
	if err != nil {
		dbError(w, err)
		return
	}
	if !canAccessEmployee(w, r, int(row["employee_id"].(float64))) {
		return
	}
	var in struct {
		OrdersCount int    `json:"ordersCount"`
		WorkPeriod  string `json:"workPeriod"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeErr(w, 400, "请求参数错误")
		return
	}
	if err = repo.UpdatePerformance(r.Context(), id, in.OrdersCount, in.WorkPeriod); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"message": "业绩已更新"})
}
func (h *apiHandler) deletePerformance(w http.ResponseWriter, r *http.Request) {
	_, role := mustUser(r.Context())
	if !hasRole(role, "admin", "hr") {
		writeErr(w, 403, "无权限删除业绩")
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	row, err := repo.One(r.Context(), repo.Pool, `SELECT employee_id,TO_CHAR(work_date,'YYYY-MM-DD')AS date,product_id FROM daily_performance_records WHERE id=$1`, id)
	if err != nil {
		dbError(w, err)
		return
	}
	tx, err := repo.Pool.Begin(r.Context())
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	product := "null"
	if n, ok := row["product_id"].(float64); ok {
		product = strconv.Itoa(int(n))
	}
	ref := fmt.Sprintf("daily-%d-%s-%s", int(row["employee_id"].(float64)), row["date"], product)
	if _, err = tx.Exec(r.Context(), `DELETE FROM finance_records WHERE reference_type='daily_performance' AND reference_id=$1`, ref); err == nil {
		_, err = tx.Exec(r.Context(), `DELETE FROM daily_performance_records WHERE id=$1`, id)
	}
	if err != nil {
		dbError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"message": "业绩已删除"})
}
func (h *apiHandler) updateScreenshots(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	row, err := repo.GetOrderDetail(r.Context(), id)
	if err != nil {
		dbError(w, err)
		return
	}
	if !canAccessEmployee(w, r, row.EmployeeID) {
		return
	}
	urls, cleanup, err := readScreenshots(w, r)
	if err != nil {
		cleanup()
		writeErr(w, 400, err.Error())
		return
	}
	success := false
	defer func() {
		if !success {
			cleanup()
		}
	}()
	if len(urls) == 0 {
		writeErr(w, 400, "请选择需要更新的截图")
		return
	}
	fields := repo.Object{}
	for field, column := range map[string]string{"wechatPayScreenshot": "wechat_pay_screenshot_url", "wechatChatScreenshot": "wechat_chat_screenshot_url", "wechatContactScreenshot": "wechat_contact_screenshot_url"} {
		if url, ok := urls[field]; ok {
			fields[column] = url
		}
	}
	tx, err := repo.Pool.Begin(r.Context())
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if err = repo.UpdateObject(r.Context(), tx, "daily_performance_order_details", id, fields, map[string]string{"wechat_pay_screenshot_url": "", "wechat_chat_screenshot_url": "", "wechat_contact_screenshot_url": ""}); err != nil {
		dbError(w, err)
		return
	}
	b, _ := json.Marshal(fields)
	if _, err = tx.Exec(r.Context(), `INSERT INTO order_edit_history(order_detail_id,edited_by,changes)VALUES($1,$2,$3::jsonb)`, id, recorderID(r), string(b)); err != nil {
		dbError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		dbError(w, err)
		return
	}
	success = true
	writeJSON(w, 200, map[string]string{"message": "截图已更新"})
}
func (h *apiHandler) orderHistory(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效ID")
		return
	}
	row, err := repo.GetOrderDetail(r.Context(), id)
	if err != nil {
		dbError(w, err)
		return
	}
	if !canAccessEmployee(w, r, row.EmployeeID) {
		return
	}
	rows, err := repo.Objects(r.Context(), repo.Pool, `SELECT h.*,e.name AS editor_name FROM order_edit_history h LEFT JOIN employees e ON e.id=h.edited_by WHERE order_detail_id=$1 ORDER BY created_at DESC`, id)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"history": rows})
}

var uploadName = regexp.MustCompile(`^[a-f0-9]{32}\.(png|jpg|webp)$`)

func (h *apiHandler) getUpload(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !uploadName.MatchString(name) {
		writeErr(w, 404, "文件不存在")
		return
	}
	self, team, err := employeeScope(r)
	if err != nil {
		writeErr(w, 403, "无权限读取文件")
		return
	}
	var allowed bool
	err = repo.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM daily_performance_order_details od JOIN employees e ON e.id=od.employee_id WHERE $1 IN(od.wechat_pay_screenshot_url,od.wechat_chat_screenshot_url,od.wechat_contact_screenshot_url) AND($2::int IS NULL OR e.id=$2)AND($3::int IS NULL OR e.team_id=$3))`, "/uploads/"+name, self, team).Scan(&allowed)
	if err != nil || !allowed {
		writeErr(w, 404, "文件不存在")
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeFile(w, r, filepath.Join(uploadDir(), name))
}
