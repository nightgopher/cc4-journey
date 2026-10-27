package api

import (
	"net/http/httptest"
	"testing"
)

func TestMCPRouteHasIndependentAuthenticationAndOriginChecks(t *testing.T) {
	t.Setenv("MCP_ALLOWED_ORIGINS", "https://trusted.example")
	router := Router("website-jwt-secret")
	for _, test := range []struct {
		method, origin string
		status         int
	}{
		{"POST", "", 401},
		{"OPTIONS", "https://foreign.example", 403},
		{"OPTIONS", "https://trusted.example", 204},
	} {
		request := httptest.NewRequest(test.method, "/mcp", nil)
		request.Header.Set("Origin", test.origin)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("%s origin=%s got %d, want %d", test.method, test.origin, response.Code, test.status)
		}
		if response.Header().Get("Access-Control-Allow-Origin") == "*" {
			t.Fatal("MCP inherited the REST API's wildcard CORS policy")
		}
	}
}
