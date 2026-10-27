package mcp

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nightgopher/cc4-journey/app/internal/mcpaccess"
)

// HTTPHandler accepts only active, user-bound database credentials. The old
// environment-wide MCP_TOKEN is deliberately not an authentication path.
func HTTPHandler() http.Handler {
	verify := func(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		grant, err := mcpaccess.Authenticate(ctx, token)
		if err != nil {
			return nil, auth.ErrInvalidToken
		}
		return &auth.TokenInfo{
			// Bind sessions to a credential, so even another token belonging to
			// the same account cannot reuse a reset/revoked session.
			UserID: "cc4-credential:" + strconv.FormatInt(grant.Credential.ID, 10),
			Extra:  map[string]any{"credentialId": grant.Credential.ID},
		}, nil
	}
	return newHTTPHandler(verify, mcpaccess.Resolve)
}

func newHTTPHandler(verify auth.TokenVerifier, resolve grantResolver) http.Handler {
	transport := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return newAuthorizedServer(0, resolve)
	}, &mcp.StreamableHTTPOptions{SessionTimeout: 15 * time.Minute, MaxRequestBodyBytes: 1 << 20})
	// These opaque demo credentials intentionally have no expiration. Their
	// revocation and current account status are checked by the verifier.
	protected := auth.RequireBearerToken(verify, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(transport)
	origins := strings.Split(os.Getenv("MCP_ALLOWED_ORIGINS"), ",")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if origin := r.Header.Get("Origin"); origin != "" {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			allowed := origin == scheme+"://"+r.Host
			for _, configured := range origins {
				allowed = allowed || origin == strings.TrimSpace(configured)
			}
			if !allowed {
				http.Error(w, `{"error":"Origin is not allowed"}`, 403)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, MCP-Protocol-Version, Mcp-Session-Id, Last-Event-ID")
			w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
		protected.ServeHTTP(w, r)
	})
}
