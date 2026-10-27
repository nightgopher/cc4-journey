package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
	"github.com/nightgopher/cc4-journey/app/internal/service"
)

func TestBusinessIntegration(t *testing.T) {
	dsn := os.Getenv("CC4_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CC4_TEST_DATABASE_URL to an isolated cc4_test_* database")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cfg.ConnConfig.Database, "cc4_test_") {
		t.Fatal("integration tests only run in a cc4_test_* database")
	}
	ctx := context.Background()
	if err = repo.Connect(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	var adminUser, adminEmp int
	if err = repo.Pool.QueryRow(ctx, `INSERT INTO users(phone,password_hash,role,status)VALUES('18800000001','testpass','admin','active')RETURNING id`).Scan(&adminUser); err != nil {
		t.Fatal(err)
	}
	if err = repo.Pool.QueryRow(ctx, `INSERT INTO employees(user_id,employee_code,name,level,status,join_date)VALUES($1,'TEST_ADMIN','测试管理员','a_level','active','2026-10-01')RETURNING id`, adminUser).Scan(&adminEmp); err != nil {
		t.Fatal(err)
	}
	adminToken, _ := signToken(adminUser, []byte("integration-secret"))
	handler := Router("integration-secret")
	request := func(t *testing.T, token, method, path string, body any, want int) map[string]any {
		t.Helper()
		var b []byte
		if body != nil {
			b, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		r := httptest.NewRequest(method, "/api/v1"+path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s got %d want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		var result map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("invalid JSON: %s", w.Body.String())
		}
		return result
	}
	var employeeID int
	var employeeToken string
	var productID int
	t.Run("authentication and employee account", func(t *testing.T) {
		request(t, "", "GET", "/employees", nil, 401)
		row := request(t, adminToken, "POST", "/employees", map[string]any{"name": "测试销售", "phone": "18800000002", "password": "testpass", "employeeCode": "TEST_SALES", "level": "b_level", "joinDate": "2026-10-01"}, 201)
		employeeID = int(row["id"].(float64))
		login := request(t, "", "POST", "/auth/login", map[string]string{"phone": "18800000002", "password": "testpass"}, 200)
		employeeToken = login["session"].(map[string]any)["access_token"].(string)
		request(t, employeeToken, "POST", "/salary/calculate-batch", map[string]int{"periodYear": 2026, "periodMonth": 10}, 403)
		request(t, employeeToken, "POST", "/products", map[string]any{"name": "非法产品"}, 403)
		request(t, employeeToken, "GET", fmt.Sprintf("/employees/%d", adminEmp), nil, 403)
	})
	t.Run("lead date, assignment and ownership", func(t *testing.T) {
		row := request(t, adminToken, "POST", "/leads", map[string]any{"customerName": "测试家长", "phone": "18800000003", "nextFollowUp": "2026-11-02"}, 201)
		id := int(row["id"].(float64))
		request(t, adminToken, "POST", "/leads/assign-batch", map[string]any{"leadIds": []int{id}, "assignedTo": employeeID}, 200)
		list := request(t, employeeToken, "GET", "/leads", nil, 200)["leads"].([]any)
		if len(list) != 1 {
			t.Fatalf("want one assigned lead: %v", list)
		}
		l := list[0].(map[string]any)
		if l["status"] != "new" || l["next_follow_up"] != "2026-11-02" {
			t.Fatal(l)
		}
		request(t, employeeToken, "PUT", fmt.Sprintf("/leads/%d", id), map[string]any{"status": "contacted", "nextFollowUp": ""}, 200)
		request(t, adminToken, "POST", "/leads", map[string]any{"customerName": "无效日期", "phone": "18800000004", "nextFollowUp": "2026-02-30"}, 400)
	})
	t.Run("product update preserves identity", func(t *testing.T) {
		rules := map[string]any{"tiers": []map[string]any{{"from": 1, "to": 3, "amount": 10}, {"from": 4, "to": 5, "amount": 20}, {"from": 6, "to": nil, "amount": 30}}}
		row := request(t, adminToken, "POST", "/products", map[string]any{"name": "体验课", "code": "TEST_PRODUCT", "price": 9, "commissionRules": rules}, 201)
		productID = int(row["id"].(float64))
		request(t, adminToken, "PUT", fmt.Sprintf("/products/%d", productID), map[string]any{"name": "体验课更新", "code": "TEST_PRODUCT", "price": 12, "commissionRules": rules}, 200)
		var count int
		if err = repo.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM products WHERE code='TEST_PRODUCT'`).Scan(&count); err != nil || count != 1 {
			t.Fatalf("product duplicated: %d %v", count, err)
		}
	})
	t.Run("concurrent ordinal commission and NULL product accumulation", func(t *testing.T) {
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := service.SubmitPerformance(ctx, service.SubmitPerformanceInput{EmployeeID: employeeID, WorkDate: "2026-10-05", ProductID: &productID, Orders: 1})
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		var orders, count int
		var sales, commission float64
		if err = repo.Pool.QueryRow(ctx, `SELECT orders_count,sales_amount,commission_amount FROM daily_performance_records WHERE employee_id=$1 AND product_id=$2 AND work_date='2026-10-05'`, employeeID, productID).Scan(&orders, &sales, &commission); err != nil {
			t.Fatal(err)
		}
		if orders != 8 || sales != 96 || commission != 160 {
			t.Fatalf("orders=%d sales=%v commission=%v", orders, sales, commission)
		}
		for range 4 {
			if _, err = service.SubmitPerformance(ctx, service.SubmitPerformanceInput{EmployeeID: employeeID, WorkDate: "2026-10-06", Orders: 1}); err != nil {
				t.Fatal(err)
			}
		}
		if err = repo.Pool.QueryRow(ctx, `SELECT COUNT(*),SUM(orders_count),SUM(commission_amount) FROM daily_performance_records WHERE employee_id=$1 AND product_id IS NULL AND work_date='2026-10-06'`, employeeID).Scan(&count, &orders, &commission); err != nil {
			t.Fatal(err)
		}
		if count != 1 || orders != 4 || commission != 50 {
			t.Fatalf("rows=%d orders=%d commission=%v", count, orders, commission)
		}
	})
	t.Run("monthly wage commission keeps model one across products", func(t *testing.T) {
		rules := map[string]any{"tiers": []map[string]any{{"from": 1, "to": nil, "amount": 99}}}
		row := request(t, adminToken, "POST", "/products", map[string]any{"name": "独立档位产品", "code": "TEST_SPECIAL", "price": 9, "commissionRules": rules}, 201)
		id := int(row["id"].(float64))
		result, err := service.SubmitPerformance(ctx, service.SubmitPerformanceInput{EmployeeID: employeeID, WorkDate: "2026-10-05", ProductID: &id, Orders: 3})
		if err != nil {
			t.Fatal(err)
		}
		var recorded float64
		if err = repo.Pool.QueryRow(ctx, `SELECT commission_amount FROM daily_performance_records WHERE id=$1`, result.RecordID).Scan(&recorded); err != nil || recorded != 297 {
			t.Fatalf("product commission=%v err=%v", recorded, err)
		}
		salary, _, err := service.CalculateSalary(ctx, employeeID, 2026, 10)
		if err != nil || salary.CommissionAmount != 300 {
			t.Fatalf("D2 salary=%+v err=%v", salary, err)
		}
	})
	t.Run("receivable transaction, partial, completed and overpayment", func(t *testing.T) {
		row := request(t, adminToken, "POST", "/finance/receivable", map[string]any{"customer": "测试应收客户", "amount": 100, "dueDate": "2026-11-01"}, 201)
		id := int(row["id"].(float64))
		path := fmt.Sprintf("/finance/receivable/%d/payments", id)
		request(t, adminToken, "POST", path, map[string]any{"amount": 30, "paymentDate": "2026-10-26", "paymentMethod": "bank"}, 201)
		request(t, adminToken, "POST", path, map[string]any{"amount": 71, "paymentDate": "2026-10-26"}, 400)
		request(t, adminToken, "POST", path, map[string]any{"amount": 70, "paymentDate": "2026-10-26"}, 201)
		detail := request(t, adminToken, "GET", fmt.Sprintf("/finance/receivable/%d", id), nil, 200)["receivable"].(map[string]any)
		if detail["paid_amount"] != float64(100) || detail["payment_status"] != "completed" {
			t.Fatal(detail)
		}
		payments := request(t, adminToken, "GET", path, nil, 200)["payments"].([]any)
		if len(payments) != 2 {
			t.Fatalf("failed payment leaked: %v", payments)
		}
		request(t, employeeToken, "GET", "/finance/income", nil, 403)
	})
	t.Run("position permissions roundtrip", func(t *testing.T) {
		request(t, adminToken, "PUT", "/permissions/position/员工", map[string]any{"allowedModules": []string{"daily-performance", "notifications"}}, 200)
		row := request(t, employeeToken, "GET", "/permissions/me", nil, 200)["data"].(map[string]any)
		if row["source"] != "position" {
			t.Fatal(row)
		}
		request(t, employeeToken, "GET", "/permissions/positions", nil, 403)
		request(t, adminToken, "DELETE", "/permissions/position/员工", nil, 200)
	})
	t.Run("workdays, attendance, dashboard and notifications", func(t *testing.T) {
		request(t, adminToken, "POST", "/workdays", map[string]any{"year": 2026, "month": 10, "workdays": []int{5, 6, 7}}, 200)
		request(t, adminToken, "POST", "/workdays", map[string]any{"year": 2026, "month": 2, "workdays": []int{30}}, 400)
		request(t, adminToken, "POST", "/attendance", map[string]any{"employeeId": employeeID, "date": "2026-10-07", "status": "work"}, 201)
		request(t, employeeToken, "GET", "/attendance", nil, 200)
		request(t, adminToken, "GET", "/dashboard/today?date=2026-10-05", nil, 200)
		row := request(t, adminToken, "GET", "/dashboard/trend?startDate=2026-10-05&endDate=2026-10-07", nil, 200)
		if len(row["data"].(map[string]any)["series"].([]any)) != 3 {
			t.Fatal(row)
		}
		request(t, adminToken, "GET", "/dashboard/trend?startDate=bad&endDate=2026-10-07", nil, 400)
		request(t, employeeToken, "GET", "/notifications", nil, 200)
	})
	t.Run("employee order submission, approval ordinal, rejection and screenshots", func(t *testing.T) {
		t.Setenv("UPLOAD_DIR", t.TempDir())
		var firstID int
		for i := 0; i < 2; i++ {
			var b bytes.Buffer
			form := multipart.NewWriter(&b)
			for key, value := range map[string]string{"employeeId": fmt.Sprint(employeeID), "workDate": "2026-10-08", "productId": fmt.Sprint(productID), "orders": "1", "parentPhone": fmt.Sprintf("1880010000%d", i), "workPeriod": "full_day"} {
				if err := form.WriteField(key, value); err != nil {
					t.Fatal(err)
				}
			}
			for _, key := range []string{"wechatPayScreenshot", "wechatChatScreenshot", "wechatContactScreenshot"} {
				part, err := form.CreateFormFile(key, "sample.png")
				if err != nil {
					t.Fatal(err)
				}
				if _, err = part.Write([]byte("\x89PNG\r\n\x1a\n" + strings.Repeat("x", 64))); err != nil {
					t.Fatal(err)
				}
			}
			form.Close()
			r := httptest.NewRequest("POST", "/api/v1/daily-performance", &b)
			r.Header.Set("Content-Type", form.FormDataContentType())
			r.Header.Set("Authorization", "Bearer "+employeeToken)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != 201 {
				t.Fatalf("submit: %d %s", w.Code, w.Body.String())
			}
			var result map[string]any
			json.Unmarshal(w.Body.Bytes(), &result)
			detail := result["orderDetail"].(map[string]any)
			id := int(detail["id"].(float64))
			if i == 0 {
				firstID = id
			}
			request(t, adminToken, "POST", fmt.Sprintf("/daily-performance/order-details/%d/approve", id), map[string]any{"approved": true}, 200)
			request(t, adminToken, "POST", fmt.Sprintf("/daily-performance/order-details/%d/approve", id), map[string]any{"approved": true}, 400)
			var commission float64
			var approvedBy int
			if err = repo.Pool.QueryRow(ctx, `SELECT d.commission_amount,od.approved_by FROM daily_performance_records d JOIN daily_performance_order_details od ON od.performance_record_id=d.id WHERE od.id=$1`, id).Scan(&commission, &approvedBy); err != nil {
				t.Fatal(err)
			}
			if commission != float64((i+1)*10) || approvedBy != adminEmp {
				t.Fatalf("commission=%v approver=%d", commission, approvedBy)
			}
			imageReq := httptest.NewRequest("GET", "/api/v1"+detail["wechat_pay_screenshot_url"].(string), nil)
			imageReq.Header.Set("Authorization", "Bearer "+employeeToken)
			imageResp := httptest.NewRecorder()
			handler.ServeHTTP(imageResp, imageReq)
			if imageResp.Code != 200 {
				t.Fatalf("screenshot: %d %s", imageResp.Code, imageResp.Body.String())
			}
		}
		request(t, employeeToken, "POST", "/daily-performance", map[string]any{"employeeId": adminEmp, "workDate": "2026-10-09", "orders": 0}, 403)
		request(t, employeeToken, "POST", "/daily-performance", map[string]any{"employeeId": employeeID, "workDate": "2026-10-09", "orders": 0}, 201)
		request(t, adminToken, "DELETE", fmt.Sprintf("/daily-performance/order-details/%d", firstID), nil, 200)
		var n int
		var commission float64
		if err = repo.Pool.QueryRow(ctx, `SELECT orders_count,commission_amount FROM daily_performance_records WHERE employee_id=$1 AND product_id=$2 AND work_date='2026-10-08'`, employeeID, productID).Scan(&n, &commission); err != nil || n != 1 || commission != 10 {
			t.Fatalf("delete reversal: %d %v %v", n, commission, err)
		}
	})
	t.Run("customers, orders, teams, notifications and public settings", func(t *testing.T) {
		customer := request(t, adminToken, "POST", "/customers", map[string]any{"name": "测试客户", "phone": "18800000008"}, 201)
		cid := int(customer["id"].(float64))
		request(t, adminToken, "POST", fmt.Sprintf("/customers/%d/orders", cid), map[string]any{"totalAmount": 99.5, "orderDate": "2026-10-26"}, 201)
		request(t, adminToken, "GET", fmt.Sprintf("/customers/%d/orders", cid), nil, 200)
		team := request(t, adminToken, "POST", "/teams", map[string]any{"name": "测试团队", "code": "TEST_TEAM", "managerId": adminEmp}, 201)
		tid := int(team["id"].(float64))
		request(t, adminToken, "GET", fmt.Sprintf("/teams/%d", tid), nil, 200)
		request(t, adminToken, "GET", fmt.Sprintf("/teams/%d/members", tid), nil, 200)
		var userID int
		repo.Pool.QueryRow(ctx, `SELECT user_id FROM employees WHERE id=$1`, employeeID).Scan(&userID)
		request(t, adminToken, "POST", "/notifications", map[string]any{"userId": userID, "title": "测试通知", "message": "请核对订单"}, 201)
		request(t, employeeToken, "PUT", "/notifications/mark-all-read", nil, 200)
		request(t, adminToken, "POST", "/settings", map[string]any{"key": "test_public", "value": "欢迎使用", "is_public": true}, 200)
		result := request(t, "", "GET", "/settings/public", nil, 200)
		if len(result["settings"].([]any)) != 1 {
			t.Fatal(result)
		}
	})
	t.Run("monthly attendance, leave approval and monthly performance", func(t *testing.T) {
		request(t, adminToken, "POST", "/attendance/batch", map[string]any{"records": []map[string]any{{"employeeId": employeeID, "date": "2026-10-07", "status": "partial"}}}, 200)
		summary := request(t, adminToken, "GET", "/attendance/monthly-summary?year=2026&month=10", nil, 200)["summary"].([]any)
		if len(summary) != 1 {
			t.Fatal(summary)
		}
		if summary[0].(map[string]any)["actual_work_days"] != 2.5 {
			t.Fatal(summary)
		}
		request(t, adminToken, "POST", "/attendance/monthly-summary/sync", map[string]any{"year": 2026, "month": 10}, 200)
		leave := request(t, employeeToken, "POST", "/attendance/leave-requests", map[string]any{"startDate": "2026-10-12", "endDate": "2026-10-13", "leaveType": "personal", "reason": "个人事务"}, 201)
		id := int(leave["id"].(float64))
		request(t, adminToken, "POST", fmt.Sprintf("/approvals/%d/action", id), map[string]any{"action": "approved"}, 200)
		request(t, adminToken, "POST", fmt.Sprintf("/approvals/%d/action", id), map[string]any{"action": "approved"}, 400)
		request(t, employeeToken, "GET", "/attendance/leave-requests", nil, 200)
		request(t, adminToken, "POST", "/performance/aggregate-to-monthly", map[string]any{"year": 2026, "month": 10}, 200)
		request(t, adminToken, "GET", "/performance?year=2026&month=10", nil, 200)
	})
	t.Run("salary payment creates and reverses one finance expense", func(t *testing.T) {
		request(t, adminToken, "POST", "/salary/calculate", map[string]any{"employeeId": employeeID, "periodYear": 2026, "periodMonth": 10}, 200)
		var id int
		if err = repo.Pool.QueryRow(ctx, `SELECT id FROM salary_records WHERE employee_id=$1 AND period_year=2026 AND period_month=10`, employeeID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		request(t, adminToken, "PUT", fmt.Sprintf("/salary/%d/payment-status", id), map[string]any{"paymentStatus": "paid", "paymentDate": "2026-10-26"}, 200)
		var count int
		repo.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM finance_records WHERE reference_type='salary'`).Scan(&count)
		if count != 1 {
			t.Fatalf("salary expenses=%d", count)
		}
		request(t, adminToken, "POST", "/salary/calculate", map[string]any{"employeeId": employeeID, "periodYear": 2026, "periodMonth": 10}, 400)
		request(t, adminToken, "PUT", fmt.Sprintf("/salary/%d/payment-status", id), map[string]any{"paymentStatus": "pending"}, 200)
		repo.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM finance_records WHERE reference_type='salary'`).Scan(&count)
		if count != 0 {
			t.Fatal("salary expense not reversed")
		}
	})
	t.Run("wechat import deduplication, following, stats and exports", func(t *testing.T) {
		records := []map[string]any{{"call_record_id": "TEST_CALL", "customer_name": "微信客户测试", "real_phone": "18800000009", "wechat_added": true, "assigned_to": employeeID, "call_time": "2026-10-26T12:00:00Z"}}
		request(t, adminToken, "POST", "/wechat-leads/import", map[string]any{"records": records}, 201)
		r := request(t, adminToken, "POST", "/wechat-leads/import", map[string]any{"records": records}, 201)
		if r["data"].(map[string]any)["duplicateCount"] != float64(1) {
			t.Fatal(r)
		}
		list := request(t, employeeToken, "GET", "/wechat-leads", nil, 200)["leads"].([]any)
		if len(list) != 1 {
			t.Fatal(list)
		}
		id := int(list[0].(map[string]any)["id"].(float64))
		request(t, employeeToken, "POST", fmt.Sprintf("/wechat-leads/%d/follow-up", id), map[string]string{"content": "已联系，安排体验课"}, 200)
		request(t, employeeToken, "GET", "/wechat-leads/stats/overview", nil, 200)
		rq := httptest.NewRequest("GET", "/api/v1/wechat-leads/export/full", nil)
		rq.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, rq)
		if rr.Code != 200 {
			t.Fatalf("export: %d %s", rr.Code, rr.Body.String())
		}
		if !bytes.HasPrefix(rr.Body.Bytes(), []byte("PK")) {
			t.Fatal("export must be an OOXML ZIP")
		}
		request(t, adminToken, "POST", "/wechat-leads/sync", map[string]string{"startTime": "2026-10-26 00:00:00", "endTime": "2026-10-26 23:59:59"}, 503)
	})
	t.Run("manager scope, position revocation, lookups and visible lead creation", func(t *testing.T) {
		managerTokens := []string{}
		managerIDs := []int{}
		teamIDs := []int{}
		receivables := []int{}
		for i := 0; i < 2; i++ {
			e, err := repo.CreateEmployee(ctx, repo.EmployeeInput{Name: fmt.Sprintf("主管%d", i), EmployeeCode: fmt.Sprintf("MGR_%d", i), Phone: fmt.Sprintf("1880000010%d", i), Password: "testpass", Role: "manager", Position: "团队经理", Level: "a_level", JoinDate: "2026-10-01"})
			if err != nil {
				t.Fatal(err)
			}
			managerIDs = append(managerIDs, e.ID)
			token, _ := signToken(*e.UserID, []byte("integration-secret"))
			managerTokens = append(managerTokens, token)
			team := request(t, adminToken, "POST", "/teams", map[string]any{"name": fmt.Sprintf("独立团队%d", i), "code": fmt.Sprintf("ISO_%d", i), "managerId": e.ID}, 201)
			teamID := int(team["id"].(float64))
			teamIDs = append(teamIDs, teamID)
			if _, err = repo.Pool.Exec(ctx, `UPDATE employees SET team_id=$2 WHERE id=$1`, e.ID, teamID); err != nil {
				t.Fatal(err)
			}
			r := request(t, token, "POST", "/finance/receivable", map[string]any{"customer": fmt.Sprintf("团队%d客户", i), "amount": 80, "dueDate": "2026-11-01"}, 201)
			receivables = append(receivables, int(r["id"].(float64)))
		}
		list := request(t, managerTokens[0], "GET", "/finance/receivable", nil, 200)["receivables"].([]any)
		if len(list) != 1 || int(list[0].(map[string]any)["id"].(float64)) != receivables[0] {
			t.Fatal(list)
		}
		request(t, managerTokens[0], "POST", fmt.Sprintf("/finance/receivable/%d/payments", receivables[1]), map[string]any{"amount": 1, "paymentDate": "2026-10-26"}, 403)
		request(t, managerTokens[0], "GET", fmt.Sprintf("/teams/%d/members", teamIDs[1]), nil, 403)
		request(t, employeeToken, "GET", fmt.Sprintf("/teams/%d/members", teamIDs[0]), nil, 403)
		request(t, managerTokens[0], "POST", "/leads", map[string]any{"customer_name": "主管自己创建", "phone": "18800000199"}, 201)
		leads := request(t, managerTokens[0], "GET", "/leads", nil, 200)["leads"].([]any)
		if len(leads) != 1 || int(leads[0].(map[string]any)["assigned_to"].(float64)) != managerIDs[0] {
			t.Fatal(leads)
		}
		request(t, employeeToken, "GET", "/salary", nil, 200)
		if _, err = repo.Pool.Exec(ctx, `UPDATE employees SET position='受限销售' WHERE id=$1`, employeeID); err != nil {
			t.Fatal(err)
		}
		request(t, adminToken, "PUT", "/permissions/position/受限销售", map[string]any{"allowedModules": []string{"profile"}}, 200)
		request(t, employeeToken, "GET", "/daily-performance", nil, 403)
		request(t, employeeToken, "GET", "/salary", nil, 403)
		// An unrelated nullable legacy field must not bypass position restrictions.
		if _, err = repo.Pool.Exec(ctx, `UPDATE employees SET created_at=NULL WHERE id=$1`, employeeID); err != nil {
			t.Fatal(err)
		}
		request(t, employeeToken, "GET", "/daily-performance", nil, 403)
		perms := request(t, employeeToken, "GET", "/permissions/me", nil, 200)["data"].(map[string]any)
		if perms["source"] != "position" {
			t.Fatal(perms)
		}
		if _, err = repo.Pool.Exec(ctx, `UPDATE employees SET created_at=CURRENT_TIMESTAMP WHERE id=$1`, employeeID); err != nil {
			t.Fatal(err)
		}
		lookup := request(t, employeeToken, "GET", "/lookups/employees", nil, 200)["employees"].([]any)
		if len(lookup) != 1 {
			t.Fatal(lookup)
		}
		if _, exists := lookup[0].(map[string]any)["salary_base"]; exists {
			t.Fatal("lookup leaked salary")
		}
		if _, err = repo.Pool.Exec(ctx, `UPDATE employees SET position='员工' WHERE id=$1`, employeeID); err != nil {
			t.Fatal(err)
		}
		approval := request(t, adminToken, "POST", "/approvals", map[string]any{"requestType": "other", "title": "未指派审批", "requestedBy": managerIDs[0]}, 201)
		id := int(approval["id"].(float64))
		request(t, managerTokens[0], "POST", fmt.Sprintf("/approvals/%d/action", id), map[string]any{"action": "approved"}, 403)
		request(t, adminToken, "POST", fmt.Sprintf("/approvals/%d/action", id), map[string]any{"action": "approved"}, 200)
		// Explicitly assigned approvers can receive a cross-team handoff; other
		// managers still cannot decide the request.
		approval = request(t, managerTokens[0], "POST", "/approvals", map[string]any{"requestType": "other", "title": "跨团队转交", "currentApprover": managerIDs[1]}, 201)
		id = int(approval["id"].(float64))
		pending := request(t, managerTokens[1], "GET", "/approvals/pending", nil, 200)["approvals"].([]any)
		if len(pending) != 1 || int(pending[0].(map[string]any)["id"].(float64)) != id {
			t.Fatal(pending)
		}
		request(t, managerTokens[0], "POST", fmt.Sprintf("/approvals/%d/action", id), map[string]any{"action": "approved"}, 403)
		request(t, managerTokens[1], "POST", fmt.Sprintf("/approvals/%d/action", id), map[string]any{"action": "forwarded", "nextApproverId": employeeID}, 400)
		request(t, managerTokens[1], "POST", fmt.Sprintf("/approvals/%d/action", id), map[string]any{"action": "forwarded", "nextApproverId": adminEmp}, 200)
		request(t, adminToken, "POST", fmt.Sprintf("/approvals/%d/action", id), map[string]any{"action": "approved"}, 200)
	})
	t.Run("ordersCount alias and model one edit retains sales", func(t *testing.T) {
		row := request(t, adminToken, "POST", "/daily-performance", map[string]any{"employeeId": employeeID, "workDate": "2026-10-15", "ordersCount": 5, "salesAmount": 100}, 201)
		id := int(row["dailyPerformance"].(map[string]any)["recordId"].(float64))
		request(t, adminToken, "PUT", fmt.Sprintf("/daily-performance/%d", id), map[string]any{"ordersCount": 3}, 200)
		var orders int
		var sales float64
		if err = repo.Pool.QueryRow(ctx, `SELECT orders_count,sales_amount FROM daily_performance_records WHERE id=$1`, id).Scan(&orders, &sales); err != nil || orders != 3 || sales != 100 {
			t.Fatalf("orders %d sales %v err %v", orders, sales, err)
		}
		var attendanceID int
		if err = repo.Pool.QueryRow(ctx, `SELECT id FROM attendance_records WHERE employee_id=$1 AND date='2026-10-15'`, employeeID).Scan(&attendanceID); err != nil {
			t.Fatal(err)
		}
		request(t, employeeToken, "PUT", fmt.Sprintf("/attendance/%d", attendanceID), map[string]any{"attendance_days": 99}, 400)
	})
	t.Run("civil dates across zones and explicit absence wage precedence", func(t *testing.T) {
		savedLocation := time.Local
		defer func() { time.Local = savedLocation }()
		time.Local, _ = time.LoadLocation("America/Los_Angeles")
		repo.UpsertWorkdayConfig(ctx, 2025, 10, []int{26, 29, 30})
		repo.UpsertWorkdayConfig(ctx, 2025, 11, []int{1, 2, 5, 6})
		for i, tc := range []struct {
			level, date string
			oct, nov    float64
		}{{"b_level", "2025-10-26", 264, 264}, {"c_level", "2025-10-30", 30, 60}} {
			e, err := repo.CreateEmployee(ctx, repo.EmployeeInput{Name: "跨月测试", EmployeeCode: fmt.Sprintf("ZONE_%d", i), Phone: fmt.Sprintf("1880000020%d", i), Password: "testpass", Level: tc.level, JoinDate: tc.date})
			if err != nil {
				t.Fatal(err)
			}
			for _, zone := range []string{"America/Los_Angeles", "UTC", "Asia/Shanghai"} {
				time.Local, _ = time.LoadLocation(zone)
				oct, _, err := service.CalculateSalary(ctx, e.ID, 2025, 10)
				if err != nil {
					t.Fatal(err)
				}
				nov, _, err := service.CalculateSalary(ctx, e.ID, 2025, 11)
				if err != nil {
					t.Fatal(err)
				}
				if oct.BaseSalary != tc.oct || nov.BaseSalary != tc.nov {
					t.Fatalf("zone %s %s: %v/%v expected %v/%v", zone, tc.level, oct.BaseSalary, nov.BaseSalary, tc.oct, tc.nov)
				}
			}
		}
		a, err := repo.CreateEmployee(ctx, repo.EmployeeInput{Name: "A级考勤测试", EmployeeCode: "A_ATTENDANCE", Phone: "18800000209", Password: "testpass", Level: "a_level", JoinDate: "2026-01-01"})
		if err != nil {
			t.Fatal(err)
		}
		repo.UpsertWorkdayConfig(ctx, 2026, 9, []int{1})
		request(t, adminToken, "POST", "/daily-performance", map[string]any{"employeeId": a.ID, "workDate": "2026-09-01", "orders": 1}, 201)
		request(t, adminToken, "POST", "/attendance", map[string]any{"employeeId": a.ID, "date": "2026-09-01", "status": "absent"}, 201)
		request(t, adminToken, "POST", "/daily-performance", map[string]any{"employeeId": a.ID, "workDate": "2026-09-01", "orders": 1}, 201)
		detail, _, err := service.CalculateSalary(ctx, a.ID, 2026, 9)
		if err != nil || detail.BaseSalary != 0 {
			t.Fatalf("absence paid base: %v %v", detail, err)
		}
		request(t, adminToken, "POST", "/attendance", map[string]any{"employeeId": a.ID, "date": "2026-09-01", "status": "partial"}, 201)
		detail, _, err = service.CalculateSalary(ctx, a.ID, 2026, 9)
		if err != nil || detail.BaseSalary != 1100 {
			t.Fatalf("half day: %v %v", detail, err)
		}
		request(t, adminToken, "POST", "/daily-performance", map[string]any{"employeeId": a.ID, "workDate": "2026-08-03", "orders": 1}, 201)
		detail, _, err = service.CalculateSalary(ctx, a.ID, 2026, 8)
		if err != nil || detail.BaseSalary != 104 {
			t.Fatalf("weekday fallback: %v %v", detail, err)
		}
		_, err = service.RunDailyLevelCheck(ctx, time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		_, err = service.RunDailyLevelCheck(ctx, time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		var count int
		if err = repo.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM approval_requests WHERE requested_by=$1 AND request_type='level_review' AND status='pending'`, a.ID).Scan(&count); err != nil || count != 1 {
			t.Fatalf("D4 review count %d err %v", count, err)
		}
	})
	t.Run("upstream HTTP pagination filtering provenance and deduplication", func(t *testing.T) {
		var unavailable atomic.Bool
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/token" {
				writeJSON(w, 200, map[string]any{"code": 200, "data": map[string]any{"token": "fixture-token"}})
				return
			}
			if r.URL.Path != "/task/cdr/all2" || r.Header.Get("Authorization") != "fixture-token" {
				w.WriteHeader(401)
				return
			}
			if unavailable.Load() {
				w.WriteHeader(502)
				return
			}
			page := r.URL.Query().Get("page")
			offset := 0
			if page == "2" {
				offset = 100
			} else if page != "1" {
				t.Error("unexpected upstream page")
			}
			rows := []map[string]any{}
			for n := offset; n < offset+100 && n < 120; n++ {
				grade := "AF1接通未加微信"
				if n%4 == 0 {
					grade = "AF3加微信成功挂断（模拟未成交）"
				}
				rows = append(rows, map[string]any{"id": 9200000 + n, "customerName": fmt.Sprintf("模拟家长 %03d", n), "realPhone": fmt.Sprintf("000%08d", n), "phone": "000****0000", "grade": grade, "duration": 60, "callTime": 1792944000 + 60*n, "city": "杭州", "province": "浙江"})
			}
			writeJSON(w, 200, map[string]any{"code": 200, "mock": true, "data": map[string]any{"totalCount": 120, "data": rows}})
		}))
		defer upstream.Close()
		remote := os.Getenv("CC4_UPSTREAM_TEST_URL")
		if remote == "" {
			t.Setenv("GOOKI_HOST", upstream.URL)
			t.Setenv("GOOKI_USERNAME", "fixture-user")
			t.Setenv("GOOKI_PASSWORD", "fixture-password")
		} else {
			t.Setenv("GOOKI_HOST", remote)
			if os.Getenv("GOOKI_USERNAME") == "" || os.Getenv("GOOKI_PASSWORD") == "" {
				t.Fatal("remote upstream test requires GOOKI_USERNAME and GOOKI_PASSWORD")
			}
		}
		body := map[string]any{"startTime": "2026-10-26 00:00:00", "endTime": "2026-10-26 23:59:59"}
		first := request(t, adminToken, "POST", "/wechat-leads/sync", body, 200)["data"].(map[string]any)
		if first["savedCount"] != float64(30) || first["processedCount"] != float64(120) || first["mock"] != true {
			t.Fatal(first)
		}
		second := request(t, adminToken, "POST", "/wechat-leads/sync", body, 200)["data"].(map[string]any)
		if second["savedCount"] != float64(0) || second["duplicateCount"] != float64(30) {
			t.Fatal(second)
		}
		var count int
		if err := repo.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM wechat_leads WHERE data_source='mock' AND customer_name LIKE '模拟家长%' AND real_phone LIKE '000%'`).Scan(&count); err != nil || count != 30 {
			t.Fatalf("mock provenance rows=%d err=%v", count, err)
		}
		if remote == "" {
			unavailable.Store(true)
			request(t, adminToken, "POST", "/wechat-leads/sync", body, 502)
		}
	})

}
