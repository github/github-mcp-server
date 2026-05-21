package middleware

import (
	"net/http"
	"strings"

	"github.com/github/github-mcp-server/pkg/http/headers"
)

// SetCorsHeaders is middleware that sets CORS headers to allow browser-based
// MCP clients to connect from any origin. This is safe because the server
// authenticates via bearer tokens (not cookies), so cross-origin requests
// cannot exploit ambient credentials.
func SetCorsHeaders(h http.Handler) http.Handler {
	allowHeaders := strings.Join([]string{
		"Content-Type",
		"Mcp-Session-Id",
		"Mcp-Protocol-Version",
		"Last-Event-ID",
		headers.AuthorizationHeader,
		headers.MCPReadOnlyHeader,
		headers.MCPToolsetsHeader,
		headers.MCPToolsHeader,
		headers.MCPExcludeToolsHeader,
		headers.MCPFeaturesHeader,
		headers.MCPLockdownHeader,
		headers.MCPInsidersHeader,
	}, ", ")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id, WWW-Authenticate")
		w.Header().Set("Access-Control-Allow-Headers", allowHeaders)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		h.ServeHTTP(w, r)
	})
}
