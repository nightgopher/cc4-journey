package simulator

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func fixture(t *testing.T) *Server {
	t.Helper()
	s, err := New(Config{Username: "test-user", Password: "test-password-for-fixtures", TokenSecret: strings.Repeat("s", 32), RecordsPerDay: 120}, "test-revision")
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC) }
	return s
}
func request(s *Server, method, path, token string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	r.Header.Set("Authorization", token)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func login(t *testing.T, s *Server) string {
	t.Helper()
	w := request(s, "POST", "/token", "", []byte(`{"username":"test-user","password":"test-password-for-fixtures"}`))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var result struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Data.Token == "" {
		t.Fatal(w.Body.String())
	}
	return result.Data.Token
}
func query(page, size string) string {
	return "/task/cdr/all2?" + url.Values{"page": {page}, "pageSize": {size}, "startTime": {"2026-10-25 00:00:00"}, "endTime": {"2026-10-26 23:59:59"}}.Encode()
}

type result struct {
	Code int  `json:"code"`
	Mock bool `json:"mock"`
	Data struct {
		Total int          `json:"totalCount"`
		Rows  []CallRecord `json:"data"`
	} `json:"data"`
}

func decode(t *testing.T, w *httptest.ResponseRecorder) result {
	t.Helper()
	var r result
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &r) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	return r
}

func TestAuthenticationAndRevision(t *testing.T) {
	s := fixture(t)
	if got := request(s, "GET", query("1", "100"), "", nil); got.Code != 401 {
		t.Fatal(got.Code)
	}
	if got := request(s, "GET", "/token?username=test-user&password=wrong", "", nil); got.Code != 401 {
		t.Fatal(got.Code)
	}
	if got := request(s, "GET", "/token?username=test-user&password=test-password-for-fixtures", "", nil); got.Code != 200 {
		t.Fatal(got.Code)
	}
	token := login(t, s)
	for _, header := range []string{token, "Bearer " + token} {
		decode(t, request(s, "GET", query("1", "100"), header, nil))
	}
	for _, bad := range []string{token + "x", strings.Replace(token, ".", ".tampered", 1), strings.Repeat("x", 5000)} {
		if got := request(s, "GET", query("1", "100"), bad, nil); got.Code != 401 {
			t.Fatal(got.Code)
		}
	}
	s.now = func() time.Time { return time.Date(2026, 10, 26, 14, 0, 0, 0, time.UTC) }
	if got := request(s, "GET", query("1", "100"), token, nil); got.Code != 401 {
		t.Fatal("expired token accepted")
	}
	health := request(s, "GET", "/health", "", nil)
	if health.Code != 200 || !strings.Contains(health.Body.String(), "test-revision") || strings.Contains(health.Body.String(), s.cfg.Password) {
		t.Fatal(health.Body.String())
	}
	if health.Header().Get("X-Mock-Data") != "true" || health.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(health.Header())
	}
}

func TestDeterministicPaginationAndSyntheticContract(t *testing.T) {
	s := fixture(t)
	token := login(t, s)
	ids := map[int64]bool{}
	wechat := 0
	for _, page := range []string{"1", "2", "3"} {
		r := decode(t, request(s, "GET", query(page, "100"), token, nil))
		if r.Data.Total != 240 || !r.Mock {
			t.Fatal(r)
		}
		for _, row := range r.Data.Rows {
			if ids[row.ID] {
				t.Fatal("duplicate ID across pages")
			}
			ids[row.ID] = true
			if !strings.HasPrefix(row.RealPhone, "000") || len(row.RealPhone) != 11 || !strings.HasPrefix(row.CustomerName, "模拟家长") {
				t.Fatal(row)
			}
			if strings.HasPrefix(row.Grade, "AF3") {
				wechat++
			}
			if row.CallTime < 1761321600 {
				t.Fatal("invalid Unix seconds")
			}
		}
	}
	if len(ids) != 240 || wechat != 60 {
		t.Fatalf("ids=%d wechat=%d", len(ids), wechat)
	}
	a := decode(t, request(s, "GET", query("2", "100"), token, nil))
	b := decode(t, request(s, "GET", query("2", "100"), token, nil))
	if !reflect.DeepEqual(a, b) {
		t.Fatal("repeat request changed records")
	}
	if got := decode(t, request(s, "GET", query("99", "100"), token, nil)); len(got.Data.Rows) != 0 || got.Data.Total != 240 {
		t.Fatal(got)
	}
}

func TestTimeRangesAndBadParameters(t *testing.T) {
	s := fixture(t)
	token := login(t, s)
	p := "/task/cdr/all2?" + url.Values{"startTime": {"2026-10-26 10:00:00"}, "endTime": {"2026-10-26T10:59:59+08:00"}}.Encode()
	if r := decode(t, request(s, "GET", p, token, nil)); r.Data.Total != 5 {
		t.Fatal(r)
	}
	for _, params := range []url.Values{
		{"startTime": {"2026-02-30"}, "endTime": {"2026-10-26"}},
		{"startTime": {"2026-10-27"}, "endTime": {"2026-10-26"}},
		{"startTime": {"2026-01-01"}, "endTime": {"2026-02-01"}},
		{"startTime": {"2026-10-25"}, "endTime": {"2026-10-26"}, "page": {"-1"}},
		{"startTime": {"2026-10-25"}, "endTime": {"2026-10-26"}, "pageSize": {"501"}},
		{"startTime": {"2026-10-25"}, "endTime": {"2026-10-26"}, "scenario": {"unknown"}},
	} {
		if w := request(s, "GET", "/task/cdr/all2?"+params.Encode(), token, nil); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	for _, day := range []string{"2026-03-08", "2026-11-01"} {
		loc, _ := time.LoadLocation("America/Los_Angeles")
		s.cfg.Location = loc
		r := decode(t, request(s, "GET", "/task/cdr/all2?"+url.Values{"startTime": {day}, "endTime": {day}}.Encode(), token, nil))
		if r.Data.Total != 120 {
			t.Fatal("DST civil-day count", r.Data.Total)
		}
	}
}

func TestScenariosAndCancellation(t *testing.T) {
	s := fixture(t)
	token := login(t, s)
	empty := request(s, "GET", query("1", "100")+"&scenario=empty", token, nil)
	r := decode(t, empty)
	if r.Data.Total != 0 || r.Data.Rows == nil || len(r.Data.Rows) != 0 {
		t.Fatal(empty.Body.String())
	}
	if w := request(s, "GET", query("1", "100")+"&scenario=error", token, nil); w.Code != 502 {
		t.Fatal(w.Code)
	}
	req := httptest.NewRequest("GET", query("1", "100")+"&scenario=slow", nil)
	req.Header.Set("Authorization", token)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)
	start := time.Now()
	s.ServeHTTP(httptest.NewRecorder(), req)
	if time.Since(start) > time.Second {
		t.Fatal("slow handler ignored cancellation")
	}
}

func TestConcurrentRequestsAndSpecification(t *testing.T) {
	s := fixture(t)
	token := login(t, s)
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := request(s, "GET", query("1", "100"), token, nil)
			if w.Code != 200 {
				t.Errorf("status=%d", w.Code)
			}
		}()
	}
	wg.Wait()
	var spec struct {
		Version string         `json:"openapi"`
		Paths   map[string]any `json:"paths"`
	}
	w := request(s, "GET", "/openapi.json", "", nil)
	if json.Unmarshal(w.Body.Bytes(), &spec) != nil || spec.Version != "3.1.0" || spec.Paths["/token"] == nil {
		t.Fatal(w.Body.String())
	}
	if w := request(s, "DELETE", "/token", "", nil); w.Code != 405 {
		t.Fatal(w.Code)
	}
	if w := request(s, "GET", "/missing", "", nil); w.Code != 404 {
		t.Fatal(w.Code)
	}
}

func TestConfigurationRejectsMissingCredentials(t *testing.T) {
	for _, cfg := range []Config{{}, {Username: "user", Password: "short", TokenSecret: strings.Repeat("x", 32), RecordsPerDay: 120}, {Username: "user", Password: strings.Repeat("p", 16), TokenSecret: strings.Repeat("s", 32), RecordsPerDay: 501}} {
		if _, err := New(cfg, "test"); err == nil {
			t.Fatal("unsafe or invalid configuration accepted")
		}
	}
}
