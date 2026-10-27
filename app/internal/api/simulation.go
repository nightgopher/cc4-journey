package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
	"github.com/nightgopher/cc4-journey/app/internal/service"
)

var simulationOwnerPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

// A single writer owns the demo timeline. The runner renews this lease before
// every business mutation; the server chooses its lifetime, never the caller.
func (h *apiHandler) simulationLease(w http.ResponseWriter, r *http.Request) {
	if _, role := mustUser(r.Context()); role != "admin" || os.Getenv("BUSINESS_SIMULATION_ENABLED") != "true" {
		writeErr(w, 403, "仅启用演练的环境允许管理员推进业务")
		return
	}
	var body struct {
		Owner  string `json:"owner"`
		Action string `json:"action"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil || !simulationOwnerPattern.MatchString(body.Owner) || !hasRole(body.Action, "acquire", "renew", "release") {
		writeErr(w, 400, "无效的演练执行编号")
		return
	}
	tx, err := repo.Pool.Begin(r.Context())
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended('business-simulation-lease',0))`); err != nil {
		dbError(w, err)
		return
	}
	var value string
	err = tx.QueryRow(r.Context(), `SELECT value FROM system_settings WHERE key='business_simulation_lease_v1'`).Scan(&value)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		dbError(w, err)
		return
	}
	var lease struct {
		Owner     string    `json:"owner"`
		ExpiresAt time.Time `json:"expiresAt"`
	}
	if value != "" && json.Unmarshal([]byte(value), &lease) != nil {
		writeErr(w, 500, "演练锁状态无效")
		return
	}
	if body.Action != "acquire" && (lease.Owner != body.Owner || !time.Now().Before(lease.ExpiresAt)) {
		writeErr(w, 409, "演练执行锁已失效，请重新读取检查点后启动")
		return
	}
	if body.Action == "acquire" && lease.Owner != "" && time.Now().Before(lease.ExpiresAt) {
		writeErr(w, 409, "已有业务演练正在执行，请等待当前任务完成")
		return
	}
	if body.Action == "release" {
		_, err = tx.Exec(r.Context(), `DELETE FROM system_settings WHERE key='business_simulation_lease_v1'`)
	} else {
		lease.Owner, lease.ExpiresAt = body.Owner, time.Now().UTC().Add(10*time.Minute)
		encoded, _ := json.Marshal(lease)
		_, err = tx.Exec(r.Context(), `INSERT INTO system_settings(key,value,category,is_public) VALUES('business_simulation_lease_v1',$1,'simulation',false) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,is_public=false,updated_at=CURRENT_TIMESTAMP`, string(encoded))
	}
	if err != nil {
		dbError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"success": true})
}

func (h *apiHandler) simulationReport(w http.ResponseWriter, r *http.Request) {
	if _, role := mustUser(r.Context()); role != "admin" {
		writeErr(w, 403, "仅管理员可查看业务演练报告")
		return
	}
	var value string
	err := repo.Pool.QueryRow(r.Context(), `SELECT value FROM system_settings WHERE key='business_simulation_report_v1'`).Scan(&value)
	var report json.RawMessage
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		dbError(w, err)
		return
	}
	if err == nil {
		if !json.Valid([]byte(value)) {
			writeErr(w, 500, "业务演练报告格式无效")
			return
		}
		report = json.RawMessage(value)
	}
	writeJSON(w, 200, map[string]any{"report": report, "enabled": os.Getenv("BUSINESS_SIMULATION_ENABLED") == "true", "businessDate": service.BusinessNow().Format("2006-01-02"), "businessTimezone": service.BusinessLocation().String()})
}
