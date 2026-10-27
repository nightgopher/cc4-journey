package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	protocol "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

type mcpIntegrationAuth struct{ token string }

func (a mcpIntegrationAuth) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.Header = request.Header.Clone()
	copy.Header.Set("Authorization", "Bearer "+a.token)
	return http.DefaultTransport.RoundTrip(copy)
}

func TestMCPBusinessIntegration(t *testing.T) {
	dsn := os.Getenv("CC4_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CC4_TEST_DATABASE_URL to an isolated cc4_test_* database")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil || !strings.HasPrefix(config.ConnConfig.Database, "cc4_test_") {
		t.Fatal("MCP integration only runs against an isolated cc4_test_* database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := repo.Connect(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	var userID, employeeID int
	if err := repo.Pool.QueryRow(ctx, `INSERT INTO users(phone,password_hash,role,status) VALUES('00090000001','test-only','admin','active') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Pool.QueryRow(ctx, `INSERT INTO employees(user_id,employee_code,name,level,status,join_date) VALUES($1,'MCP_TEST','MCP 验收员工','c_level','active','2026-10-01') RETURNING id`, userID).Scan(&employeeID); err != nil {
		t.Fatal(err)
	}
	ensureMCPCredentialsSchema(t)
	_, token, err := repo.MintMCPCredential(ctx, userID, userID, "Integration", "read_write")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MCP_ALLOWED_ORIGINS", "")
	server := httptest.NewServer(Router("website-test-secret"))
	defer server.Close()
	client := protocol.NewClient(&protocol.Implementation{Name: "business-agent-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &protocol.StreamableClientTransport{
		Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: mcpIntegrationAuth{token: token}},
		MaxRetries: -1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	call := func(name string, arguments map[string]any) map[string]any {
		t.Helper()
		result, err := session.CallTool(ctx, &protocol.CallToolParams{Name: name, Arguments: arguments})
		if err != nil || result.IsError {
			t.Fatalf("%s: result=%+v error=%v", name, result, err)
		}
		raw, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		var object map[string]any
		if err := json.Unmarshal(raw, &object); err != nil {
			t.Fatal(err)
		}
		return object
	}
	tools, err := session.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 7 {
		t.Fatalf("tools/list: %+v, %v", tools, err)
	}
	preview := call("commission_preview", map[string]any{"dailyOrders": []int{3, 5, 8}})
	if preview["monthlyTotal"] != float64(260) {
		t.Fatalf("commission preview: %v", preview)
	}
	employee := call("employee_get", map[string]any{"employeeId": employeeID})
	if employee["id"] != float64(employeeID) || employee["name"] != "MCP 验收员工" {
		t.Fatalf("employee projection: %v", employee)
	}
	call("workday_get", map[string]any{"year": 2026, "month": 10})
	call("salary_preview", map[string]any{"employeeId": employeeID, "periodYear": 2026, "periodMonth": 10})
	call("level_transition_check", map[string]any{"level": "b_level", "workdaysInLevel": 3, "recentOrders": []int{1, 2, 1}})
	var persistedSalaries int
	if err := repo.Pool.QueryRow(ctx, `SELECT count(*) FROM salary_records WHERE employee_id=$1`, employeeID).Scan(&persistedSalaries); err != nil || persistedSalaries != 0 {
		t.Fatalf("salary preview persisted business data: %d, %v", persistedSalaries, err)
	}
	created := call("performance_submit", map[string]any{"employeeId": employeeID, "workDate": "2026-10-08", "orders": 2, "salesAmount": 60})
	var orders int
	var commission float64
	if err := repo.Pool.QueryRow(ctx, `SELECT orders_count,commission_amount FROM daily_performance_records WHERE id=$1`, int(created["recordId"].(float64))).Scan(&orders, &commission); err != nil || orders != 2 || commission != 20 {
		t.Fatalf("MCP submit did not persist expected business data: orders=%d commission=%.2f err=%v", orders, commission, err)
	}
	var attributed int
	if err := repo.Pool.QueryRow(ctx, `SELECT count(*) FROM agent_tool_audit WHERE actor='mcp' AND action='performance_submit' AND detail LIKE $1`, fmt.Sprintf("%% by=%d", employeeID)).Scan(&attributed); err != nil || attributed != 1 {
		t.Fatalf("submit was not attributed to the actual employee: %d %v", attributed, err)
	}
	pending, err := repo.SubmitOrder(ctx, repo.OrderSubmission{EmployeeID: employeeID, WorkDate: "2026-10-09", WorkPeriod: "full_day", ParentPhone: "00090000002", OrderID: "MCP_APPROVAL"})
	if err != nil {
		t.Fatal(err)
	}
	detailID := int(pending["id"].(float64))
	approved := call("performance_approve", map[string]any{"detailId": detailID, "approved": true})
	if approved["detailId"] != float64(detailID) {
		t.Fatalf("approval response identified the wrong detail: %v", approved)
	}
	var status string
	if err := repo.Pool.QueryRow(ctx, `SELECT approval_status FROM daily_performance_order_details WHERE id=$1`, detailID).Scan(&status); err != nil || status != "approved" {
		t.Fatalf("approval was not persisted: %q, %v", status, err)
	}
	repeated, err := session.CallTool(ctx, &protocol.CallToolParams{Name: "performance_approve", Arguments: map[string]any{"detailId": detailID, "approved": true}})
	if err != nil || !repeated.IsError {
		t.Fatalf("repeated approval must be a tool error: %+v %v", repeated, err)
	}
	t.Log(fmt.Sprintf("Streamable HTTP: discovered 7 tools; previews/queries, real submit, real approval and repeat rejection passed for isolated employee %d", employeeID))
}

func ensureMCPCredentialsSchema(t *testing.T) {
	t.Helper()
	var exists bool
	if err := repo.Pool.QueryRow(context.Background(), "SELECT to_regclass('public.mcp_credentials') IS NOT NULL").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		body, err := os.ReadFile("../../migrations/003_mcp_credentials.sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = repo.Pool.Exec(context.Background(), string(body)); err != nil {
			t.Fatal(err)
		}
	}
}
