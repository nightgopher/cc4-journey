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
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nightgopher/cc4-journey/app/internal/domain"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
	"github.com/nightgopher/cc4-journey/app/internal/service"
)

func TestReimbursementIntegration(t *testing.T) {
	dsn := os.Getenv("CC4_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set isolated cc4_test_* database")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil || !strings.HasPrefix(cfg.ConnConfig.Database, "cc4_test_") {
		t.Fatal("reimbursement tests require an isolated cc4_test_* database")
	}
	ctx := context.Background()
	if err = repo.Connect(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	t.Setenv("PADDLEOCR_API_KEY", "test-only")
	t.Setenv("PADDLEOCR_MODEL", service.PaddleModels[0])
	directory := t.TempDir()
	t.Setenv("UPLOAD_DIR", directory)
	tokens, ids := map[string]string{}, map[string]int{}
	var team1, team2 int
	if err = repo.Pool.QueryRow(ctx, `INSERT INTO teams(name,code) VALUES('OCR测试一组','OCR_T1') RETURNING id`).Scan(&team1); err != nil {
		t.Fatal(err)
	}
	if err = repo.Pool.QueryRow(ctx, `INSERT INTO teams(name,code) VALUES('OCR测试二组','OCR_T2') RETURNING id`).Scan(&team2); err != nil {
		t.Fatal(err)
	}
	for i, spec := range []struct {
		name, role string
		team       int
	}{{"owner", "employee", team1}, {"other", "employee", team2}, {"manager", "manager", team1}, {"outsider", "manager", team2}, {"finance", "finance", team2}, {"admin", "admin", team1}} {
		var uid, eid int
		if err = repo.Pool.QueryRow(ctx, `INSERT INTO users(phone,password_hash,role,status)VALUES($1,'test-only',$2,'active') RETURNING id`, fmt.Sprintf("000880000%02d", i), spec.role).Scan(&uid); err != nil {
			t.Fatal(err)
		}
		if err = repo.Pool.QueryRow(ctx, `INSERT INTO employees(user_id,employee_code,name,level,status,team_id,join_date)VALUES($1,$2,$2,'trainee','active',$3,'2026-01-01') RETURNING id`, uid, "OCR_"+spec.name, spec.team).Scan(&eid); err != nil {
			t.Fatal(err)
		}
		ids[spec.name] = eid
		tokens[spec.name], _ = signToken(uid, []byte("ocr-integration-test"))
	}
	handler := Router("ocr-integration-test")
	call := func(who, method, path string, body any, want int) map[string]any {
		t.Helper()
		var raw []byte
		if body != nil {
			raw, _ = json.Marshal(body)
		}
		r := httptest.NewRequest(method, "/api/v1"+path, bytes.NewReader(raw))
		r.Header.Set("Authorization", "Bearer "+tokens[who])
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s %s got %d want %d: %s", who, method, path, w.Code, want, w.Body.String())
		}
		var result map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		return result
	}
	image, err := os.ReadFile("../../test/fixtures/reimbursement/invoice-demo.png")
	if err != nil {
		t.Fatal(err)
	}
	upload := func(raw []byte, name string, want int) int {
		t.Helper()
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		part, _ := form.CreateFormFile("receipt", name)
		_, _ = part.Write(raw)
		_ = form.Close()
		r := httptest.NewRequest("POST", "/api/v1/reimbursements", &body)
		r.Header.Set("Authorization", "Bearer "+tokens["owner"])
		r.Header.Set("Content-Type", form.FormDataContentType())
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("upload got %d want %d: %s", w.Code, want, w.Body.String())
		}
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		if n, ok := out["id"].(float64); ok {
			return int(n)
		}
		return 0
	}
	var supplierURL string
	var posts atomic.Int32
	var invoice atomic.Value
	invoice.Store("DEMO-20261027-001")
	supplier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/jobs" {
			n := posts.Add(1)
			if r.Header.Get("Authorization") != "bearer test-only" {
				t.Error("supplier auth missing")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"jobId": fmt.Sprintf("job-%d", n)}})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/jobs/") {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"state": "done", "extractProgress": map[string]int{"totalPages": 1, "extractedPages": 1}, "resultUrl": map[string]string{"jsonUrl": supplierURL + "/result"}}})
			return
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("OCR key forwarded to storage")
		}
		text := "Merchant: Demo Office Supplies\nInvoice No: " + invoice.Load().(string) + "\nInvoice Date: 2026-10-27\nCurrency: CNY\nSubtotal: 118.40\nTax: 7.10\nTOTAL: CNY 125.50"
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"layoutParsingResults": []any{map[string]any{"markdown": map[string]string{"text": text}}}}})
	}))
	defer supplier.Close()
	supplierURL = supplier.URL
	client, _ := service.NewPaddleClient(supplier.URL+"/jobs", "test-only", service.PaddleModels[0], true)
	process := func(id int) {
		t.Helper()
		for range 2 {
			_, err := repo.Pool.Exec(ctx, `UPDATE reimbursement_claims SET ocr_next_poll=now() WHERE id=$1`, id)
			if err != nil {
				t.Fatal(err)
			}
			if ok, err := service.RunReimbursementOCR(ctx, client, filepath.Join(directory, "receipts")); err != nil || !ok {
				t.Fatalf("OCR step=%v err=%v", ok, err)
			}
		}
	}
	path := func(id int) string { return fmt.Sprintf("/reimbursements/%d", id) }
	input := func(number string, amount float64) map[string]any {
		return map[string]any{"title": "模拟办公报销", "category": "office", "merchant": "Demo Office Supplies", "invoiceNumber": number, "invoiceCode": "", "invoiceDate": "2026-10-27", "currency": "CNY", "invoiceTotal": 125.5, "amount": amount, "reason": "模拟课堂办公用品"}
	}
	var first, financeID int
	t.Run("receipt upload and ownership privacy", func(t *testing.T) {
		first = upload(image, "demo.png", 201)
		call("other", "GET", path(first), nil, 403)
		call("outsider", "GET", path(first)+"/receipt", nil, 403)
		call("other", "GET", path(first)+"/events", nil, 403)
		config := call("owner", "GET", "/reimbursements/config", nil, 200)
		raw, _ := json.Marshal(config)
		if strings.Contains(string(raw), "test-only") || strings.Contains(string(raw), "API_KEY") {
			t.Fatal("key exposed in config")
		}
	})
	t.Run("asynchronous supplier contract and immutable suggestions", func(t *testing.T) {
		process(first)
		r := call("owner", "GET", path(first), nil, 200)["claim"].(map[string]any)
		if r["ocr_status"] != "ready" || r["amount"] != 125.5 || r["merchant"] != "Demo Office Supplies" {
			t.Fatal(r)
		}
	})
	t.Run("confirmation money limits and self approval", func(t *testing.T) {
		call("owner", "PUT", path(first), input("DEMO-20261027-001", 126), 200)
		call("owner", "POST", path(first)+"/submit", map[string]bool{"confirmed": true}, 400)
		call("owner", "PUT", path(first), input("DEMO-20261027-001", 125.5), 200)
		call("owner", "POST", path(first)+"/submit", map[string]bool{"confirmed": false}, 400)
		call("owner", "POST", path(first)+"/submit", map[string]bool{"confirmed": true}, 200)
		call("owner", "POST", path(first)+"/approve", map[string]any{}, 403)
		call("owner", "POST", path(first)+"/pay", map[string]any{}, 403)
		call("outsider", "POST", path(first)+"/approve", map[string]any{}, 403)
		call("manager", "POST", path(first)+"/approve", map[string]any{}, 200)
		call("owner", "PUT", path(first), input("DEMO-20261027-001", 1), 409)
		call("owner", "POST", path(first)+"/cancel", map[string]any{}, 409)
		var count int
		_ = repo.Pool.QueryRow(ctx, `SELECT count(*) FROM finance_records WHERE reference_type='reimbursement'`).Scan(&count)
		if count != 0 {
			t.Fatal("approval created expense before payment")
		}
	})
	t.Run("payment authorization and concurrent idempotency", func(t *testing.T) {
		call("manager", "POST", path(first)+"/pay", map[string]string{"paymentDate": "2026-10-27"}, 403)
		call("finance", "POST", path(first)+"/pay", map[string]any{"paymentDate": "2026-10-27", "amount": 999}, 400)
		call("finance", "POST", path(first)+"/pay", map[string]string{"paymentDate": "2100-01-01"}, 400)
		var wg sync.WaitGroup
		failures := make(chan string, 6)
		for range 6 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				r := httptest.NewRequest("POST", "/api/v1"+path(first)+"/pay", strings.NewReader(`{"paymentDate":"2026-10-27"}`))
				r.Header.Set("Authorization", "Bearer "+tokens["finance"])
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				if w.Code != 200 {
					failures <- w.Body.String()
				}
			}()
		}
		wg.Wait()
		close(failures)
		for message := range failures {
			t.Error(message)
		}
		var count int
		var sum float64
		_ = repo.Pool.QueryRow(ctx, `SELECT count(*),coalesce(sum(amount),0),max(id) FROM finance_records WHERE reference_type='reimbursement' AND reference_id=$1`, fmt.Sprintf("reimbursement-%d", first)).Scan(&count, &sum, &financeID)
		if count != 1 || sum != 125.5 {
			t.Fatalf("expense count=%d amount=%.2f", count, sum)
		}
	})
	t.Run("paid expense cannot be edited or deleted through finance", func(t *testing.T) {
		call("finance", "PUT", fmt.Sprintf("/finance/expense/%d", financeID), map[string]float64{"amount": 999}, 409)
		call("finance", "DELETE", fmt.Sprintf("/finance/expense/%d", financeID), nil, 409)
		call("owner", "POST", path(first)+"/withdraw", map[string]any{}, 409)
	})
	t.Run("duplicate file and invoice are rejected", func(t *testing.T) {
		upload(image, "duplicate.png", 409)
		duplicate := upload(append(append([]byte{}, image...), []byte("same-invoice")...), "variant.png", 201)
		process(duplicate)
		call("owner", "PUT", path(duplicate), input("DEMO-20261027-001", 125.5), 409)
	})
	t.Run("rejection editing and resubmission", func(t *testing.T) {
		invoice.Store("DEMO-REJECT-002")
		id := upload(append(append([]byte{}, image...), []byte("rejection")...), "rejected.png", 201)
		process(id)
		call("owner", "PUT", path(id), input("DEMO-REJECT-002", 100), 200)
		call("owner", "POST", path(id)+"/submit", map[string]bool{"confirmed": true}, 200)
		call("manager", "POST", path(id)+"/reject", map[string]string{"note": ""}, 400)
		call("manager", "POST", path(id)+"/reject", map[string]string{"note": "请补充用途"}, 200)
		call("owner", "PUT", path(id), input("DEMO-REJECT-002", 100), 200)
		call("owner", "POST", path(id)+"/submit", map[string]bool{"confirmed": true}, 200)
		call("owner", "POST", path(id)+"/withdraw", map[string]any{}, 200)
	})
	t.Run("interrupted submission is not automatically replayed", func(t *testing.T) {
		invoice.Store("DEMO-RECOVER-003")
		id := upload(append(append([]byte{}, image...), []byte("interrupted")...), "interrupted.png", 201)
		before := posts.Load()
		job, err := repo.AcquireOCRJob(ctx, strings.Repeat("a", 32))
		if err != nil || job.ID != id {
			t.Fatalf("job=%+v err=%v", job, err)
		}
		_, _ = repo.Pool.Exec(ctx, `UPDATE reimbursement_claims SET ocr_locked_until=now()-interval '1 second',ocr_next_poll=now() WHERE id=$1`, id)
		if _, err = service.RunReimbursementOCR(ctx, client, filepath.Join(directory, "receipts")); err != nil {
			t.Fatal(err)
		}
		row := call("owner", "GET", path(id), nil, 200)["claim"].(map[string]any)
		if row["ocr_status"] != "failed" || posts.Load() != before {
			t.Fatal("uncertain POST replayed")
		}
		call("owner", "POST", path(id)+"/recognize", map[string]any{}, 200)
		process(id)
		if posts.Load() != before+1 {
			t.Fatal("explicit retry did not submit exactly once")
		}
	})
	t.Run("cancelled jobs reject stale results and release receipt reservation", func(t *testing.T) {
		raw := append(append([]byte{}, image...), []byte("cancelled")...)
		id := upload(raw, "cancelled.png", 201)
		job, err := repo.AcquireOCRJob(ctx, strings.Repeat("b", 32))
		if err != nil || job.ID != id {
			t.Fatal(err)
		}
		call("owner", "POST", path(id)+"/cancel", map[string]any{}, 200)
		if err = repo.SaveOCRJob(ctx, job, "ready", nil, nil, &domain.ReceiptRecognition{}, ""); err != nil {
			t.Fatal(err)
		}
		row := call("owner", "GET", path(id), nil, 200)["claim"].(map[string]any)
		if row["status"] != "cancelled" || row["ocr_result"] != nil {
			t.Fatal("stale worker overwrote cancellation")
		}
		fresh := upload(raw, "retry-file.png", 201)
		call("owner", "POST", path(fresh)+"/cancel", map[string]any{}, 200)
	})
	t.Run("invalid receipt content and single page PDF", func(t *testing.T) {
		upload([]byte("<svg onload=alert(1)></svg>"), "fake.png", 400)
		pdf, err := os.ReadFile("../../test/fixtures/reimbursement/invoice-demo.pdf")
		if err != nil {
			t.Fatal(err)
		}
		id := upload(pdf, "receipt.pdf", 201)
		call("owner", "POST", path(id)+"/cancel", map[string]any{}, 200)
		bad := bytes.Replace(pdf, []byte("/Count 1"), []byte("/Count 2"), 1)
		upload(bad, "two-pages.pdf", 400)
	})
	if target := os.Getenv("CC4_REIMBURSEMENT_REPORT"); target != "" {
		if t.Failed() {
			return
		}
		b, _ := json.MarshalIndent(map[string]any{"status": "passed", "supplierMode": "HTTP contract mock", "model": service.PaddleModels[0], "expenseCount": 1, "expenseAmount": 125.5, "concurrentPayments": 6, "privacy": "passed", "duplicateReceipt": "rejected", "duplicateInvoice": "rejected", "uncertainSubmission": "not replayed", "staleWorker": "rejected"}, "", "  ")
		if err = os.WriteFile(target, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
