package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	protocol "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nightgopher/cc4-journey/app/internal/access"
	"github.com/nightgopher/cc4-journey/app/internal/mcpaccess"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

type authenticatedTransport struct{ token string }

func (a authenticatedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.Header = request.Header.Clone()
	copy.Header.Set("Authorization", "Bearer "+a.token)
	return http.DefaultTransport.RoundTrip(copy)
}

func TestHTTPAuthenticationAndOrigins(t *testing.T) {
	t.Setenv("MCP_ALLOWED_ORIGINS", "https://trusted.example")
	for _, test := range []struct {
		name, token, authorization, origin, method string
		status                                     int
	}{
		{"no credential", "", "", "", "POST", 401},
		{"missing token", "secret", "", "", "POST", 401},
		{"wrong token", "secret", "Bearer wrong", "", "GET", 401},
		{"foreign origin", "secret", "Bearer secret", "https://foreign.example", "POST", 403},
		{"foreign preflight", "secret", "", "https://foreign.example", "OPTIONS", 403},
		{"trusted preflight", "secret", "", "https://trusted.example", "OPTIONS", 204},
		{"trusted origin still needs auth", "secret", "", "https://trusted.example", "POST", 401},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, "http://127.0.0.1/mcp", nil)
			request.Header.Set("Authorization", test.authorization)
			request.Header.Set("Origin", test.origin)
			response := httptest.NewRecorder()
			fixtureHTTPHandler(test.token).ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("got %d, want %d: %s", response.Code, test.status, response.Body.String())
			}
		})
	}
}

func TestStreamableHTTPSessionsSurviveMultipleRequests(t *testing.T) {
	t.Setenv("MCP_ALLOWED_ORIGINS", "")
	server := httptest.NewServer(fixtureHTTPHandler("integration-secret"))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := protocol.NewClient(&protocol.Implementation{Name: "protocol-regression", Version: "1"}, nil)
	connect := func(disableSSE bool) *protocol.ClientSession {
		t.Helper()
		session, err := client.Connect(ctx, &protocol.StreamableClientTransport{
			Endpoint: server.URL + "/mcp",
			HTTPClient: &http.Client{Transport: authenticatedTransport{
				token: "integration-secret",
			}},
			MaxRetries: -1, DisableStandaloneSSE: disableSSE,
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = session.Close() })
		return session
	}
	first, second := connect(false), connect(true)
	want := []string{"commission_preview", "employee_get", "level_transition_check", "performance_approve", "performance_submit", "salary_preview", "workday_get"}
	for _, session := range []*protocol.ClientSession{first, first, second} {
		result, err := session.ListTools(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, tool := range result.Tools {
			names = append(names, tool.Name)
		}
		slices.Sort(names)
		if !slices.Equal(names, want) {
			t.Fatalf("tools: %v", names)
		}
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := second.ListTools(ctx, nil); err != nil {
		t.Fatalf("closing one session affected another: %v", err)
	}
}

func fixtureHTTPHandler(token string, modes ...string) http.Handler {
	mode := "read_write"
	if len(modes) > 0 {
		mode = modes[0]
	}
	verify := func(ctx context.Context, raw string, r *http.Request) (*auth.TokenInfo, error) {
		if token == "" || raw != token {
			return nil, auth.ErrInvalidToken
		}
		return &auth.TokenInfo{UserID: "credential:1", Extra: map[string]any{"credentialId": int64(1)}}, nil
	}
	resolve := func(ctx context.Context, id int64) (*mcpaccess.Grant, error) {
		return &mcpaccess.Grant{Credential: &repo.MCPCredential{ID: id, UserID: 1, AccessMode: mode}, Principal: &access.Principal{UserID: 1, EmployeeID: 1, Role: "admin"}, Modules: repo.DefaultModules("admin")}, nil
	}
	return newHTTPHandler(verify, resolve)
}

func TestReadOnlyCredentialRejectsGuessedWriteToolOverHTTP(t *testing.T) {
	t.Setenv("MCP_ALLOWED_ORIGINS", "")
	server := httptest.NewServer(fixtureHTTPHandler("read-only", "read_only"))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := protocol.NewClient(&protocol.Implementation{Name: "read-only-regression", Version: "1"}, nil)
	session, err := client.Connect(ctx, &protocol.StreamableClientTransport{Endpoint: server.URL, HTTPClient: &http.Client{Transport: authenticatedTransport{"read-only"}}, MaxRetries: -1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	list, err := session.ListTools(ctx, nil)
	if err != nil || len(list.Tools) != 5 {
		t.Fatalf("read-only tools: %+v %v", list, err)
	}
	result, err := session.CallTool(ctx, &protocol.CallToolParams{Name: "performance_submit", Arguments: map[string]any{"employeeId": 1, "workDate": "2026-10-01", "orders": 1}})
	if err != nil || result == nil || !result.IsError {
		t.Fatalf("guessed write tool was not denied: %+v %v", result, err)
	}
}

func TestLegacyEnvironmentTokenIsNotAnAuthenticationPath(t *testing.T) {
	t.Setenv("MCP_TOKEN", "legacy-global-token")
	r := httptest.NewRequest("POST", "http://127.0.0.1/mcp", nil)
	r.Header.Set("Authorization", "Bearer legacy-global-token")
	w := httptest.NewRecorder()
	HTTPHandler().ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("legacy global token must be rejected: %d", w.Code)
	}
}
