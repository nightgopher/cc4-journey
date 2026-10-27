package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const receiptSample = "Merchant: Demo Office Supplies\nInvoice No: DEMO-20261027-001\nInvoice Date: 2026-10-27\nCurrency: CNY\nSubtotal: 118.40\nTax: 7.10\nTOTAL: CNY 125.50"

func TestReceiptFieldExtraction(t *testing.T) {
	f, warnings := ExtractReceiptFields(receiptSample)
	if f.Merchant != "Demo Office Supplies" || f.InvoiceNumber != "DEMO-20261027-001" || f.InvoiceDate != "2026-10-27" || f.Total == nil || *f.Total != 125.5 || f.Category != "office" || len(warnings) != 0 {
		t.Fatalf("fields=%+v warnings=%v", f, warnings)
	}
	chinese := "销售方名称：演示办公用品公司\n发票号码：20261027000000000001\n开票日期：2026年10月27日\n价税合计（小写）￥125.50"
	f, _ = ExtractReceiptFields(chinese)
	if f.Merchant != "演示办公用品公司" || f.Total == nil || *f.Total != 125.5 || f.InvoiceDate != "2026-10-27" {
		t.Fatalf("Chinese fields=%+v", f)
	}
}
func TestReceiptDoesNotUseSubtotalOrInventMissingAmounts(t *testing.T) {
	for _, text := range []string{"Subtotal: 118.40\nTax: 7.10", "TOTAL: 125.50\nTOTAL: 130.00", "invoice number 9999999"} {
		f, _ := ExtractReceiptFields(text)
		if f.Total != nil {
			t.Fatalf("invented amount from %q", text)
		}
	}
}
func TestPaddleModelsParseOnlyRecognizedContent(t *testing.T) {
	for _, model := range PaddleModels {
		t.Run(model, func(t *testing.T) {
			var result any
			if model == "PP-OCRv6" {
				result = map[string]any{"ocrResults": []any{map[string]any{"prunedResult": map[string]any{"rec_texts": strings.Split(receiptSample, "\n")}, "ocrImage": "https://storage.invalid/private-token"}}}
			} else {
				result = map[string]any{"layoutParsingResults": []any{map[string]any{"markdown": map[string]any{"text": receiptSample, "images": map[string]string{"../secret": "https://storage.invalid/private-token"}}, "outputImages": map[string]string{"foo": "https://storage.invalid/private-token"}}}}
			}
			b, _ := json.Marshal(map[string]any{"result": result})
			r, err := ParsePaddleJSONL(b, model)
			if err != nil || r.Fields.Total == nil || *r.Fields.Total != 125.5 || strings.Contains(r.Text, "private-token") || strings.Contains(r.Text, "../secret") {
				t.Fatalf("result=%+v error=%v", r, err)
			}
		})
	}
	for _, body := range []string{"", `{"result":{}}`, `not json`} {
		if _, err := ParsePaddleJSONL([]byte(body), PaddleModels[0]); err == nil {
			t.Fatal("empty or invalid OCR result passed")
		}
	}
}
func TestPaddleSubmitAndUnauthenticatedResultDownload(t *testing.T) {
	var url string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/jobs":
			if r.Method != "POST" || r.Header.Get("Authorization") != "bearer test-only" {
				t.Error("missing supplier auth")
			}
			if err := r.ParseMultipartForm(1 << 20); err != nil || r.FormValue("model") != PaddleModels[0] {
				t.Error("invalid multipart contract")
			}
			if r.MultipartForm != nil {
				defer r.MultipartForm.RemoveAll()
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"jobId": "job_1"}})
		case "/jobs/job_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"state": "done", "extractProgress": map[string]int{"totalPages": 1, "extractedPages": 1}, "resultUrl": map[string]string{"jsonUrl": url + "/result"}}})
		case "/result":
			if r.Header.Get("Authorization") != "" {
				t.Error("supplier token leaked to result storage")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"layoutParsingResults": []any{map[string]any{"markdown": map[string]string{"text": receiptSample}}}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	url = server.URL
	client, err := NewPaddleClient(url+"/jobs", "test-only", PaddleModels[0], true)
	if err != nil {
		t.Fatal(err)
	}
	path := t.TempDir() + "/receipt.png"
	_ = os.WriteFile(path, []byte("synthetic upload"), 0600)
	id, err := client.Submit(context.Background(), path)
	if err != nil || id != "job_1" {
		t.Fatalf("id=%s err=%v", id, err)
	}
	r, err := client.Poll(context.Background(), id)
	if err != nil || r.Recognition == nil || r.Recognition.Fields.Total == nil || *r.Recognition.Fields.Total != 125.5 {
		t.Fatalf("poll=%+v error=%v", r, err)
	}
}
func TestPaddleRejectsUnknownStateAndDoesNotExposeErrorBody(t *testing.T) {
	for _, state := range []string{"unexpected", "failed"} {
		t.Run(state, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"state": state, "errorMsg": "private-provider-secret"}})
			}))
			defer server.Close()
			client, _ := NewPaddleClient(server.URL, "test-only", PaddleModels[0], true)
			if _, err := client.Poll(context.Background(), "job_1"); err == nil || strings.Contains(err.Error(), "private-provider-secret") {
				t.Fatal("unknown state passed or provider details exposed")
			}
		})
	}
}
