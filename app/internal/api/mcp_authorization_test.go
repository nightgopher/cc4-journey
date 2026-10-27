package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	protocol "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

// This suite exercises the public REST and Streamable HTTP boundaries against
// real PostgreSQL. It must never be pointed at a business database.
func TestMCPAuthorizationIntegration(t *testing.T) {
	dsn := os.Getenv("CC4_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CC4_TEST_DATABASE_URL to an isolated cc4_test_* database")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil || !strings.HasPrefix(cfg.ConnConfig.Database, "cc4_test_") {
		t.Fatal("MCP authorization tests require an isolated cc4_test_* database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	if err := repo.Connect(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(repo.Close)
	ensureMCPCredentialsSchema(t)
	t.Setenv("MCP_PUBLIC_URL", "https://cc4.example/mcp")
	t.Setenv("MCP_ALLOWED_ORIGINS", "")
	t.Setenv("MCP_TOKEN", "legacy-environment-key")
	handler := Router("mcp-authorization-test-secret")
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := repo.Pool.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	type actor struct {
		user, employee int
		jwt, position  string
	}
	sequence := 0
	newActor := func(role string, team *int) actor {
		t.Helper()
		sequence++
		var a actor
		if err := repo.Pool.QueryRow(ctx, `INSERT INTO users(phone,password_hash,role,status) VALUES($1,'test-only',$2,'active') RETURNING id`, fmt.Sprintf("00091%06d", sequence), role).Scan(&a.user); err != nil {
			t.Fatal(err)
		}
		a.position = fmt.Sprintf("MCP_AUTH_%d", a.user)
		if err := repo.Pool.QueryRow(ctx, `INSERT INTO employees(user_id,employee_code,name,position,level,status,team_id,join_date) VALUES($1,$2,$2,$2,'c_level','active',$3,'2026-10-01') RETURNING id`, a.user, a.position, team).Scan(&a.employee); err != nil {
			t.Fatal(err)
		}
		a.jwt, err = signToken(a.user, []byte("mcp-authorization-test-secret"))
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	rest := func(t *testing.T, a actor, method, path string, body any, want int) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, "/api/v1"+path, bytes.NewReader(raw))
		r.Host = "untrusted-client.example"
		r.Header.Set("Authorization", "Bearer "+a.jwt)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		if want < 300 && w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("credential responses must not be cached")
		}
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	issue := func(t *testing.T, a actor, mode string) (int64, string) {
		t.Helper()
		out := rest(t, a, "POST", "/mcp/credentials", map[string]string{"accessMode": mode}, 201)
		if out["endpoint"] != "https://cc4.example/mcp" {
			t.Fatal("client Host header changed generated endpoint")
		}
		return int64(out["credential"].(map[string]any)["id"].(float64)), out["token"].(string)
	}
	connect := func(t *testing.T, token string) *protocol.ClientSession {
		t.Helper()
		client := protocol.NewClient(&protocol.Implementation{Name: "authorization-test", Version: "1"}, nil)
		session, err := client.Connect(ctx, &protocol.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: mcpIntegrationAuth{token}}, MaxRetries: -1}, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = session.Close() })
		return session
	}
	tools := func(t *testing.T, s *protocol.ClientSession) []string {
		t.Helper()
		out, err := s.ListTools(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, tool := range out.Tools {
			names = append(names, tool.Name)
		}
		return names
	}
	call := func(t *testing.T, s *protocol.ClientSession, name string, args map[string]any, allowed bool) {
		t.Helper()
		out, err := s.CallTool(ctx, &protocol.CallToolParams{Name: name, Arguments: args})
		if err != nil || out == nil || out.IsError == allowed {
			t.Fatalf("%s allowed=%t: %+v, %v", name, allowed, out, err)
		}
	}
	status := func(t *testing.T, token string) int {
		t.Helper()
		r, _ := http.NewRequestWithContext(ctx, "POST", server.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`))
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode
	}

	t.Run("owner-only issuance hash storage and one active credential", func(t *testing.T) {
		admin, employee, other := newActor("admin", nil), newActor("employee", nil), newActor("employee", nil)
		id, token := issue(t, employee, "read_only")
		exec(`UPDATE mcp_credentials SET created_at=now()-interval '10 years' WHERE id=$1`, id)
		if status(t, token) != 200 {
			t.Fatal("long-lived credential unexpectedly expired")
		}
		var hash, prefix string
		if err := repo.Pool.QueryRow(ctx, `SELECT token_hash,token_prefix FROM mcp_credentials WHERE id=$1`, id).Scan(&hash, &prefix); err != nil {
			t.Fatal(err)
		}
		if len(token) != 72 || hash != repo.MCPTokenHash(token) || strings.Contains(hash, token) || prefix != token[:16] {
			t.Fatal("credential was not stored as a one-way hash")
		}
		metadata, _ := json.Marshal(rest(t, employee, "GET", "/mcp/credentials", nil, 200))
		if bytes.Contains(metadata, []byte(token)) || bytes.Contains(metadata, []byte(hash)) {
			t.Fatal("metadata endpoint revealed a secret")
		}
		rest(t, employee, "POST", "/mcp/credentials", map[string]any{"ownerUserId": other.user}, 403)
		rest(t, employee, "GET", fmt.Sprintf("/mcp/credentials?userId=%d", other.user), nil, 403)
		rest(t, employee, "POST", "/mcp/credentials", map[string]any{"role": "admin"}, 400)
		rest(t, employee, "POST", "/mcp/credentials", map[string]any{"accessMode": "read_write"}, 403)
		foreignID, _ := issue(t, other, "read_only")
		rest(t, employee, "DELETE", fmt.Sprintf("/mcp/credentials/%d", foreignID), nil, 403)
		out := rest(t, admin, "POST", "/mcp/credentials", map[string]any{"ownerUserId": other.user, "accessMode": "read_only"}, 201)
		if int(out["credential"].(map[string]any)["userId"].(float64)) != other.user {
			t.Fatal("admin issuance was not bound to requested account")
		}
		session := connect(t, token)
		_, replacement := issue(t, employee, "read_only")
		if status(t, token) != 401 || status(t, replacement) != 200 {
			t.Fatal("reset did not immediately replace the credential")
		}
		if _, err := session.ListTools(ctx, nil); err == nil {
			t.Fatal("reset credential retained an initialized session")
		}
		if status(t, "legacy-environment-key") != 401 {
			t.Fatal("legacy environment token still authenticates")
		}
	})

	t.Run("finance cannot escalate writes and CI identity stays read only", func(t *testing.T) {
		finance := newActor("finance", nil)
		rest(t, finance, "POST", "/mcp/credentials", map[string]string{"accessMode": "read_write"}, 403)
		_, token := issue(t, finance, "read_only")
		s := connect(t, token)
		if slices.Contains(tools(t, s), "performance_submit") {
			t.Fatal("finance discovered administrative writes")
		}
		call(t, s, "performance_submit", map[string]any{"employeeId": finance.employee, "workDate": "2026-10-22", "orders": 1}, false)
		ci := newActor("admin", nil)
		exec(`DELETE FROM employees WHERE id=$1`, ci.employee)
		rest(t, ci, "POST", "/mcp/credentials", map[string]string{"accessMode": "read_write"}, 403)
		_, ciToken := issue(t, ci, "read_only")
		if names := tools(t, connect(t, ciToken)); len(names) != 5 {
			t.Fatalf("CI identity tools: %v", names)
		}
	})

	t.Run("read only tools and denied writes have no business effect", func(t *testing.T) {
		a := newActor("admin", nil)
		id, token := issue(t, a, "read_only")
		s := connect(t, token)
		names := tools(t, s)
		if len(names) != 5 || slices.Contains(names, "performance_submit") || slices.Contains(names, "performance_approve") {
			t.Fatalf("read-only discovery: %v", names)
		}
		call(t, s, "performance_submit", map[string]any{"employeeId": a.employee, "workDate": "2026-10-20", "orders": 1}, false)
		call(t, s, "performance_approve", map[string]any{"detailId": 999999, "approved": true}, false)
		var count int
		if err := repo.Pool.QueryRow(ctx, `SELECT count(*) FROM daily_performance_records WHERE employee_id=$1`, a.employee).Scan(&count); err != nil || count != 0 {
			t.Fatalf("denied write changed business data: %d %v", count, err)
		}
		if err := repo.Pool.QueryRow(ctx, `SELECT count(*) FROM agent_tool_audit WHERE actor=$1 AND action='mcp_performance_submit' AND detail::jsonb->>'credentialId'=$2 AND detail::jsonb->>'result'='denied'`, fmt.Sprintf("user:%d", a.user), fmt.Sprint(id)).Scan(&count); err != nil || count != 1 {
			t.Fatalf("missing attributable denial audit: %d %v", count, err)
		}
		if err := repo.Pool.QueryRow(ctx, `SELECT count(*) FROM mcp_credentials WHERE id=$1 AND last_used_at IS NOT NULL`, id).Scan(&count); err != nil || count != 1 {
			t.Fatal("last use not recorded")
		}
	})

	t.Run("team scope current role current modules and real approver", func(t *testing.T) {
		var teamA, teamB int
		if err := repo.Pool.QueryRow(ctx, `INSERT INTO teams(name,code) VALUES('MCP team A','MCP_AUTH_A') RETURNING id`).Scan(&teamA); err != nil {
			t.Fatal(err)
		}
		if err := repo.Pool.QueryRow(ctx, `INSERT INTO teams(name,code) VALUES('MCP team B','MCP_AUTH_B') RETURNING id`).Scan(&teamB); err != nil {
			t.Fatal(err)
		}
		manager := newActor("manager", &teamA)
		member, outsider := newActor("employee", &teamA), newActor("employee", &teamB)
		_, token := issue(t, manager, "read_write")
		s := connect(t, token)
		if !slices.Contains(tools(t, s), "performance_submit") {
			t.Fatal("manager lost authorized write tool")
		}
		call(t, s, "employee_get", map[string]any{"employeeId": member.employee}, true)
		call(t, s, "employee_get", map[string]any{"employeeId": outsider.employee}, false)
		call(t, s, "salary_preview", map[string]any{"employeeId": outsider.employee, "periodYear": 2026, "periodMonth": 10}, false)
		call(t, s, "level_transition_check", map[string]any{"employeeId": outsider.employee}, false)
		call(t, s, "performance_submit", map[string]any{"employeeId": outsider.employee, "workDate": "2026-10-20", "orders": 1}, false)
		pending, err := repo.SubmitOrder(ctx, repo.OrderSubmission{EmployeeID: outsider.employee, WorkDate: "2026-10-21", WorkPeriod: "full_day", ParentPhone: "00091999999", OrderID: "MCP_SCOPE_APPROVAL"})
		if err != nil {
			t.Fatal(err)
		}
		detailID := int(pending["id"].(float64))
		call(t, s, "performance_approve", map[string]any{"detailId": detailID, "approved": true}, false)
		exec(`UPDATE employees SET team_id=$1 WHERE id=$2`, teamB, manager.employee)
		call(t, s, "employee_get", map[string]any{"employeeId": member.employee}, false)
		call(t, s, "employee_get", map[string]any{"employeeId": outsider.employee}, true)
		call(t, s, "performance_approve", map[string]any{"detailId": detailID, "approved": true}, true)
		var approvedBy int
		if err := repo.Pool.QueryRow(ctx, `SELECT approved_by FROM daily_performance_order_details WHERE id=$1`, detailID).Scan(&approvedBy); err != nil || approvedBy != manager.employee {
			t.Fatalf("real approver not persisted: %d %v", approvedBy, err)
		}
		if err := repo.SavePositionModules(ctx, manager.position, []string{"profile"}); err != nil {
			t.Fatal(err)
		}
		if names := tools(t, s); !slices.Equal(names, []string{"employee_get"}) {
			t.Fatalf("position change did not update existing session: %v", names)
		}
		call(t, s, "performance_submit", map[string]any{"employeeId": outsider.employee, "workDate": "2026-10-22", "orders": 1}, false)
		exec(`DELETE FROM system_settings WHERE key=$1`, "position_permissions_"+manager.position)
		exec(`DELETE FROM position_module_permissions WHERE position_name=$1`, manager.position)
		exec(`UPDATE users SET role='employee' WHERE id=$1`, manager.user)
		if slices.Contains(tools(t, s), "performance_submit") {
			t.Fatal("role downgrade retained write tool")
		}
		call(t, s, "employee_get", map[string]any{"employeeId": manager.employee}, true)
		call(t, s, "employee_get", map[string]any{"employeeId": outsider.employee}, false)
		call(t, s, "salary_preview", map[string]any{"employeeId": outsider.employee, "periodYear": 2026, "periodMonth": 10}, false)
		call(t, s, "performance_submit", map[string]any{"employeeId": manager.employee, "workDate": "2026-10-22", "orders": 1}, false)
	})

	t.Run("revocation disabled account and departed employee affect existing sessions", func(t *testing.T) {
		for _, scenario := range []string{"revoked", "account-disabled", "employee-departed", "ambiguous-employee"} {
			t.Run(scenario, func(t *testing.T) {
				a := newActor("employee", nil)
				id, token := issue(t, a, "read_only")
				s := connect(t, token)
				call(t, s, "employee_get", map[string]any{"employeeId": a.employee}, true)
				switch scenario {
				case "revoked":
					rest(t, a, "DELETE", fmt.Sprintf("/mcp/credentials/%d", id), nil, 200)
				case "account-disabled":
					exec(`UPDATE users SET status='inactive' WHERE id=$1`, a.user)
				case "employee-departed":
					exec(`UPDATE employees SET status='inactive' WHERE id=$1`, a.employee)
				case "ambiguous-employee":
					exec(`INSERT INTO employees(user_id,employee_code,name) VALUES($1,$2,'Ambiguous')`, a.user, a.position+"_X")
				}
				if status(t, token) != 401 {
					t.Fatal("inactive credential/account still authenticates")
				}
				if _, err := s.ListTools(ctx, nil); err == nil {
					t.Fatal("existing session survived revocation/status change")
				}
			})
		}
	})

	t.Run("a session cannot be reused with another valid credential", func(t *testing.T) {
		a, b := newActor("admin", nil), newActor("admin", nil)
		_, tokenA := issue(t, a, "read_only")
		_, tokenB := issue(t, b, "read_only")
		send := func(token, sessionID, body string) (int, string) {
			r, _ := http.NewRequestWithContext(ctx, "POST", server.URL+"/mcp", strings.NewReader(body))
			r.Header.Set("Authorization", "Bearer "+token)
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Accept", "application/json, text/event-stream")
			if sessionID != "" {
				r.Header.Set("Mcp-Session-Id", sessionID)
				r.Header.Set("MCP-Protocol-Version", "2025-03-26")
			}
			resp, err := http.DefaultClient.Do(r)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			_, _ = io.Copy(io.Discard, resp.Body)
			return resp.StatusCode, resp.Header.Get("Mcp-Session-Id")
		}
		code, sessionID := send(tokenA, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
		if code != 200 || sessionID == "" {
			t.Fatalf("initialize status=%d missing session=%t", code, sessionID == "")
		}
		code, _ = send(tokenB, sessionID, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
		if code != 403 {
			t.Fatalf("credential B reused credential A session: HTTP %d", code)
		}
	})

	t.Run("concurrent resets leave exactly one active token", func(t *testing.T) {
		a := newActor("admin", nil)
		var wg sync.WaitGroup
		errors := make(chan error, 6)
		for range 6 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _, err := repo.MintMCPCredential(ctx, a.user, a.user, "Concurrent", "read_only")
				errors <- err
			}()
		}
		wg.Wait()
		close(errors)
		for err := range errors {
			if err != nil {
				t.Fatal(err)
			}
		}
		var active int
		if err := repo.Pool.QueryRow(ctx, `SELECT count(*) FROM mcp_credentials WHERE user_id=$1 AND revoked_at IS NULL`, a.user).Scan(&active); err != nil || active != 1 {
			t.Fatalf("active=%d err=%v", active, err)
		}
	})
}
