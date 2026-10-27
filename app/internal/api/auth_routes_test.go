package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestProtectedRoutesRequireAuthentication(t *testing.T) {
	router := Router("route-test-secret")
	count := 0
	err := chi.Walk(router.(chi.Routes), func(method, path string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if !strings.HasPrefix(path, "/api/") || path == "/api/v1/auth/login" || path == "/api/v1/settings/public" {
			return nil
		}
		if method == "OPTIONS" {
			return nil
		}
		count++
		t.Run(method+" "+path, func(t *testing.T) {
			request := httptest.NewRequest(method, strings.ReplaceAll(path, "*", "missing"), nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("unauthenticated route returned %d: %s", response.Code, response.Body.String())
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count < 100 {
		t.Fatalf("route walk unexpectedly found only %d protected routes", count)
	}
	t.Logf("verified %d protected routes", count)
}
