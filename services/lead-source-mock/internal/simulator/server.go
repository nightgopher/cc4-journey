// Package simulator provides a deterministic, synthetic upstream call-record API.
// It has no dependency on the CC4 database and never makes outbound calls.
package simulator

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

//go:embed openapi.json
var specification []byte

type Config struct {
	Username, Password, TokenSecret string
	RecordsPerDay                   int
	Scenario                        string
	Delay                           time.Duration
	Location                        *time.Location
	TokenTTL                        time.Duration
}

func FromEnvironment() (Config, error) {
	c := Config{Username: os.Getenv("MOCK_USERNAME"), Password: os.Getenv("MOCK_PASSWORD"), TokenSecret: os.Getenv("MOCK_TOKEN_SECRET"), RecordsPerDay: 120, Scenario: os.Getenv("MOCK_SCENARIO"), Delay: 2 * time.Second, TokenTTL: time.Hour}
	var err error
	zone := os.Getenv("BUSINESS_TIMEZONE")
	if zone == "" {
		zone = "Asia/Shanghai"
	}
	c.Location, err = time.LoadLocation(zone)
	if err != nil {
		return c, errors.New("BUSINESS_TIMEZONE is invalid")
	}
	if v := os.Getenv("MOCK_RECORDS_PER_DAY"); v != "" {
		c.RecordsPerDay, err = strconv.Atoi(v)
		if err != nil {
			return c, errors.New("MOCK_RECORDS_PER_DAY must be an integer")
		}
	}
	if v := os.Getenv("MOCK_DELAY"); v != "" {
		c.Delay, err = time.ParseDuration(v)
		if err != nil {
			return c, errors.New("MOCK_DELAY must be a duration")
		}
	}
	return c, nil
}

type Server struct {
	cfg      Config
	revision string
	mux      *http.ServeMux
	now      func() time.Time
}

func New(cfg Config, revision string) (*Server, error) {
	if strings.TrimSpace(cfg.Username) == "" || len(cfg.Password) < 16 || len(cfg.TokenSecret) < 32 {
		return nil, errors.New("set MOCK_USERNAME, MOCK_PASSWORD (16+ characters) and MOCK_TOKEN_SECRET (32+ characters)")
	}
	if cfg.RecordsPerDay < 1 || cfg.RecordsPerDay > 500 {
		return nil, errors.New("MOCK_RECORDS_PER_DAY must be 1–500")
	}
	if cfg.Scenario == "" {
		cfg.Scenario = "normal"
	}
	if !scenario(cfg.Scenario) {
		return nil, errors.New("MOCK_SCENARIO must be normal, empty, error or slow")
	}
	if cfg.Location == nil {
		cfg.Location, _ = time.LoadLocation("Asia/Shanghai")
	}
	if cfg.TokenTTL == 0 {
		cfg.TokenTTL = time.Hour
	}
	if cfg.TokenTTL < time.Minute || cfg.TokenTTL > 24*time.Hour {
		return nil, errors.New("token lifetime must be 1 minute–24 hours")
	}
	if cfg.Delay == 0 {
		cfg.Delay = 2 * time.Second
	}
	if cfg.Delay < 0 || cfg.Delay > 30*time.Second {
		return nil, errors.New("MOCK_DELAY must be greater than zero and at most 30s")
	}
	s := &Server{cfg: cfg, revision: revision, mux: http.NewServeMux(), now: time.Now}
	s.mux.HandleFunc("/health", s.health)
	s.mux.HandleFunc("/healthz", s.health)
	s.mux.HandleFunc("/token", s.token)
	s.mux.HandleFunc("/task/cdr/all2", s.records)
	s.mux.HandleFunc("/scenarios", func(w http.ResponseWriter, r *http.Request) {
		if !getOnly(w, r) {
			return
		}
		write(w, 200, map[string]any{"mock": true, "default": cfg.Scenario, "scenarios": []string{"normal", "empty", "error", "slow"}})
	})
	s.mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		if !getOnly(w, r) {
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write(specification)
	})
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			failure(w, 404, "接口不存在")
			return
		}
		if !getOnly(w, r) {
			return
		}
		write(w, 200, map[string]any{"mock": true, "service": "cc4-lead-source-mock", "revision": revision, "documentation": "openapi.json", "notice": "全部记录为合成数据，000 开头的手机号不可用于拨打"})
	})
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Mock-Data", "true")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	s.mux.ServeHTTP(w, r)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if !getOnly(w, r) {
		return
	}
	write(w, 200, map[string]any{"status": "ok", "mock": true, "service": "cc4-lead-source-mock", "revision": s.revision, "recordsPerDay": s.cfg.RecordsPerDay, "timezone": s.cfg.Location.String()})
}

type claims struct {
	Subject string `json:"sub"`
	Expires int64  `json:"exp"`
}

func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	var username, password string
	switch r.Method {
	case http.MethodGet:
		username = r.URL.Query().Get("username")
		password = r.URL.Query().Get("password")
	case http.MethodPost:
		var in struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if decoder.Decode(&in) != nil {
			failure(w, 400, "登录参数须为 JSON username/password")
			return
		}
		username, password = in.Username, in.Password
	default:
		w.Header().Set("Allow", "GET, POST")
		failure(w, 405, "请求方法不支持")
		return
	}
	a, b := sha256.Sum256([]byte(password)), sha256.Sum256([]byte(s.cfg.Password))
	u, v := sha256.Sum256([]byte(username)), sha256.Sum256([]byte(s.cfg.Username))
	if subtle.ConstantTimeCompare(a[:], b[:]) != 1 || subtle.ConstantTimeCompare(u[:], v[:]) != 1 {
		failure(w, 401, "模拟来源账号或密码错误")
		return
	}
	expires := s.now().Add(s.cfg.TokenTTL).Unix()
	raw, _ := json.Marshal(claims{Subject: s.cfg.Username, Expires: expires})
	payload := base64.RawURLEncoding.EncodeToString(raw)
	write(w, 200, map[string]any{"code": 200, "mock": true, "data": map[string]any{"token": payload + "." + s.sign(payload), "expiresIn": int(s.cfg.TokenTTL.Seconds()), "expiresAt": expires}})
}

func (s *Server) sign(payload string) string {
	h := hmac.New(sha256.New, []byte(s.cfg.TokenSecret))
	_, _ = h.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
func (s *Server) authorized(r *http.Request) bool {
	token := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(token) > 4096 {
		return false
	}
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 || !hmac.Equal([]byte(parts[1]), []byte(s.sign(parts[0]))) {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	var c claims
	return json.Unmarshal(raw, &c) == nil && c.Subject == s.cfg.Username && c.Expires > s.now().Unix()
}

type CallRecord struct {
	ID           int64  `json:"id"`
	CustomerName string `json:"customerName"`
	RealPhone    string `json:"realPhone"`
	Phone        string `json:"phone"`
	Grade        string `json:"grade"`
	Duration     int    `json:"duration"`
	CallTime     int64  `json:"callTime"`
	Audio        string `json:"audio"`
	City         string `json:"city"`
	Province     string `json:"province"`
}

func (s *Server) records(w http.ResponseWriter, r *http.Request) {
	if !getOnly(w, r) {
		return
	}
	if !s.authorized(r) {
		failure(w, 401, "请提供有效的模拟来源令牌")
		return
	}
	q := r.URL.Query()
	mode := q.Get("scenario")
	if mode == "" {
		mode = s.cfg.Scenario
	}
	if !scenario(mode) {
		failure(w, 400, "scenario 须为 normal、empty、error 或 slow")
		return
	}
	page, err := integer(q.Get("page"), 1, 1, 100000)
	if err != nil {
		failure(w, 400, "page 须为 1–100000 的整数")
		return
	}
	size, err := integer(q.Get("pageSize"), 100, 1, 500)
	if err != nil {
		failure(w, 400, "pageSize 须为 1–500 的整数")
		return
	}
	start, err := parseTime(q.Get("startTime"), false, s.cfg.Location)
	if err != nil {
		failure(w, 400, "startTime 日期格式无效")
		return
	}
	end, err := parseTime(q.Get("endTime"), true, s.cfg.Location)
	if err != nil || end.Before(start) {
		failure(w, 400, "endTime 格式无效或早于 startTime")
		return
	}
	days := 0
	for d := dayStart(start, s.cfg.Location); !d.After(dayStart(end, s.cfg.Location)); d = d.AddDate(0, 0, 1) {
		days++
		if days > 31 {
			failure(w, 400, "单次时间范围最多 31 个自然日")
			return
		}
	}
	if mode == "error" {
		failure(w, 502, "模拟上游暂时不可用")
		return
	}
	if mode == "slow" {
		timer := time.NewTimer(s.cfg.Delay)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
		}
	}
	rows := make([]CallRecord, 0, size)
	total := 0
	offset := (page - 1) * size
	if mode != "empty" {
		for day := dayStart(start, s.cfg.Location); !day.After(end); day = day.AddDate(0, 0, 1) {
			next := day.AddDate(0, 0, 1)
			for i := 0; i < s.cfg.RecordsPerDay; i++ {
				when := day.Add(time.Duration(i) * next.Sub(day) / time.Duration(s.cfg.RecordsPerDay))
				if when.Before(start) || when.After(end) {
					continue
				}
				if total >= offset && len(rows) < size {
					rows = append(rows, record(day, when, i))
				}
				total++
			}
		}
	}
	write(w, 200, map[string]any{"code": 200, "mock": true, "scenario": mode, "data": map[string]any{"totalCount": total, "page": page, "pageSize": size, "data": rows}, "notice": "合成外呼记录；AF3 为加微未成交，AF2 为当场成交，000 手机号不用于拨打"})
}

func record(day, when time.Time, index int) CallRecord {
	grades := []string{"AF3加微信成功挂断（模拟未成交）", "AF1接通未加微信（模拟）", "AF2当场成交（模拟）", "AF0未接通（模拟）"}
	places := [][2]string{{"杭州", "浙江"}, {"南京", "江苏"}, {"成都", "四川"}, {"武汉", "湖北"}}
	dayCode, _ := strconv.ParseInt(day.Format("20060102"), 10, 64)
	phone := fmt.Sprintf("000%s%03d", day.Format("06002"), index+1)
	duration := 30 + index%120
	if index%4 == 3 {
		duration = 0
	}
	return CallRecord{ID: 9_000_000_000_000 + dayCode*1000 + int64(index+1), CustomerName: fmt.Sprintf("模拟家长 %s-%03d", day.Format("0102"), index+1), RealPhone: phone, Phone: phone[:3] + "****" + phone[7:], Grade: grades[index%4], Duration: duration, CallTime: when.Unix(), City: places[index%4][0], Province: places[index%4][1]}
}

func parseTime(v string, end bool, loc *time.Location) (time.Time, error) {
	if len(v) == 10 {
		t, err := time.ParseInLocation("2006-01-02", v, loc)
		if err != nil {
			return t, err
		}
		if t.Year() < 2000 || t.Year() > 2100 {
			return t, errors.New("year out of range")
		}
		if end {
			t = t.AddDate(0, 0, 1).Add(-time.Nanosecond)
		}
		return t, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if t, err := time.ParseInLocation(layout, v, loc); err == nil && t.Year() >= 2000 && t.Year() <= 2100 {
			return t, nil
		}
	}
	return time.Time{}, errors.New("invalid timestamp")
}
func dayStart(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}
func scenario(s string) bool { return s == "normal" || s == "empty" || s == "error" || s == "slow" }
func integer(s string, fallback, min, max int) (int, error) {
	if s == "" {
		return fallback, nil
	}
	n, e := strconv.Atoi(s)
	if e != nil || n < min || n > max {
		return 0, errors.New("out of range")
	}
	return n, nil
}
func getOnly(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	w.Header().Set("Allow", "GET, HEAD")
	failure(w, 405, "请求方法不支持")
	return false
}
func write(w http.ResponseWriter, status int, body any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func failure(w http.ResponseWriter, status int, message string) {
	write(w, status, map[string]any{"code": status, "mock": true, "message": message})
}
