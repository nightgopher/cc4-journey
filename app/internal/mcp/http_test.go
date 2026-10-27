package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	protocol "github.com/modelcontextprotocol/go-sdk/mcp"
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
		{"disabled", "", "", "", "POST", 503},
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
			HTTPHandler(test.token).ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("got %d, want %d: %s", response.Code, test.status, response.Body.String())
			}
		})
	}
}

func TestStreamableHTTPSessionsSurviveMultipleRequests(t *testing.T) {
	t.Setenv("MCP_ALLOWED_ORIGINS", "")
	server := httptest.NewServer(HTTPHandler("integration-secret"))
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
