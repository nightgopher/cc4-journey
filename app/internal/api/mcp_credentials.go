package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/nightgopher/cc4-journey/app/internal/access"
	"github.com/nightgopher/cc4-journey/app/internal/mcpaccess"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

func (h *apiHandler) registerMCPCredentials(r chi.Router) {
	r.Get("/mcp/credentials", h.getMCPCredential)
	r.Post("/mcp/credentials", h.createMCPCredential)
	r.Delete("/mcp/credentials/{id}", h.revokeMCPCredential)
}

func mcpEndpoint() (string, error) {
	endpoint := os.Getenv("MCP_PUBLIC_URL")
	if endpoint == "" {
		endpoint = "https://150.5.143.32/mcp"
	}
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("MCP_PUBLIC_URL 配置无效")
	}
	return endpoint, nil
}

func credentialOwner(w http.ResponseWriter, r *http.Request, requested int) *access.Principal {
	caller := access.FromContext(r.Context())
	if caller == nil {
		writeErr(w, 401, "需要登录")
		return nil
	}
	if requested == 0 || requested == caller.UserID {
		return caller
	}
	if caller.Role != "admin" {
		writeErr(w, 403, "只能管理自己的 Agent 接入")
		return nil
	}
	owner, err := access.Load(r.Context(), requested)
	if err != nil {
		writeErr(w, 400, "接入账号不存在或不可用")
		return nil
	}
	return owner
}

func (h *apiHandler) getMCPCredential(w http.ResponseWriter, r *http.Request) {
	requested := 0
	if value := r.URL.Query().Get("userId"); value != "" {
		var err error
		requested, err = strconv.Atoi(value)
		if err != nil || requested <= 0 {
			writeErr(w, 400, "userId 无效")
			return
		}
	}
	owner := credentialOwner(w, r, requested)
	if owner == nil {
		return
	}
	c, err := repo.LatestMCPCredential(r.Context(), owner.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		c, err = nil, nil
	}
	if err != nil {
		dbError(w, err)
		return
	}
	endpoint, err := mcpEndpoint()
	if err != nil {
		writeErr(w, 503, "Agent 接入地址暂不可用")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"credential": c, "endpoint": endpoint, "canWrite": mcpaccess.CanWrite(r.Context(), owner)})
}

func (h *apiHandler) createMCPCredential(w http.ResponseWriter, r *http.Request) {
	var in struct {
		OwnerUserID int    `json:"ownerUserId"`
		Label       string `json:"label"`
		AccessMode  string `json:"accessMode"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil || in.OwnerUserID < 0 {
		writeErr(w, 400, "接入参数无效")
		return
	}
	owner := credentialOwner(w, r, in.OwnerUserID)
	if owner == nil {
		return
	}
	if in.AccessMode == "" {
		in.AccessMode = "read_only"
	}
	if in.AccessMode != "read_only" && in.AccessMode != "read_write" {
		writeErr(w, 400, "请选择仅查询或允许业务写入")
		return
	}
	if in.AccessMode == "read_write" && !mcpaccess.CanWrite(r.Context(), owner) {
		writeErr(w, 403, "该账号当前没有业务写入权限")
		return
	}
	endpoint, err := mcpEndpoint()
	if err != nil {
		writeErr(w, 503, "Agent 接入地址暂不可用")
		return
	}
	label := strings.TrimSpace(in.Label)
	if label == "" {
		label = "Agent"
	}
	issuer, _ := mustUser(r.Context())
	c, token, err := repo.MintMCPCredential(r.Context(), owner.UserID, issuer, label, in.AccessMode)
	if err != nil {
		dbError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	config := map[string]any{"mcpServers": map[string]any{"cc4-cloud": map[string]any{
		"type": "http", "url": endpoint, "headers": map[string]string{"Authorization": "Bearer " + token},
	}}}
	writeJSON(w, 201, map[string]any{"credential": c, "token": token, "endpoint": endpoint, "config": config})
}

func (h *apiHandler) revokeMCPCredential(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, 400, "接入凭据 ID 无效")
		return
	}
	uid, role := mustUser(r.Context())
	if err := repo.RevokeMCPCredential(r.Context(), id, uid, role == "admin"); err != nil {
		dbError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]string{"message": "Agent 已断开，原 Token 已失效"})
}
