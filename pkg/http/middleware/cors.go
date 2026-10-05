package middleware

import (
	"net/http"
	"sort"
	"strings"

	"github.com/github/github-mcp-server/pkg/http/headers"
	"golang.org/x/net/http/httpguts"
)

const maxCORSProjectedRequestHeaders = 64

var corsAllowedRequestHeaders = []string{
	headers.ContentTypeHeader,
	"Mcp-Session-Id",
	"Mcp-Protocol-Version",
	headers.MCPMethodHeader,
	headers.MCPNameHeader,
	"Last-Event-ID",
	headers.AuthorizationHeader,
	headers.MCPReadOnlyHeader,
	headers.MCPToolsetsHeader,
	headers.MCPToolsHeader,
	headers.MCPExcludeToolsHeader,
	headers.MCPFeaturesHeader,
	headers.MCPLockdownHeader,
	headers.MCPInsidersHeader,
	headers.MCPParamOwnerHeader,
	headers.MCPParamRepoHeader,
}

// SetCorsHeaders is middleware that sets CORS headers to allow browser-based
// MCP clients to connect from any origin. This is safe because the server
// authenticates via bearer tokens (not cookies), so cross-origin requests
// cannot exploit ambient credentials.
func SetCorsHeaders(h http.Handler) http.Handler {
	fixedAllowHeaders := strings.Join(corsAllowedRequestHeaders, ", ")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.Header().Add("Access-Control-Expose-Headers", "Mcp-Session-Id, WWW-Authenticate")
		w.Header().Set("Access-Control-Allow-Headers", fixedAllowHeaders)

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Headers", corsPreflightAllowedRequestHeaders(r.Header))
			w.WriteHeader(http.StatusOK)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// corsPreflightAllowedRequestHeaders reflects validated projected arguments because CORS has no prefix wildcard.
func corsPreflightAllowedRequestHeaders(requestHeaders http.Header) string {
	allowed := make([]string, 0, len(corsAllowedRequestHeaders))
	allowed = append(allowed, corsAllowedRequestHeaders...)

	seen := make(map[string]struct{}, len(allowed))
	for _, header := range allowed {
		seen[strings.ToLower(header)] = struct{}{}
	}

	prefix := strings.ToLower(headers.MCPParamHeaderPrefix)
	projected := make([]string, 0, maxCORSProjectedRequestHeaders)
requestedHeaders:
	for _, value := range requestHeaders.Values("Access-Control-Request-Headers") {
		for header := range strings.SplitSeq(value, ",") {
			header = strings.TrimSpace(header)
			if !httpguts.ValidHeaderFieldName(header) {
				continue
			}

			key := strings.ToLower(header)
			if !strings.HasPrefix(key, prefix) || len(key) == len(prefix) {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}

			seen[key] = struct{}{}
			projected = append(projected, key)
			if len(projected) == maxCORSProjectedRequestHeaders {
				break requestedHeaders
			}
		}
	}

	sort.Strings(projected)
	for _, header := range projected {
		allowed = append(allowed, http.CanonicalHeaderKey(header))
	}
	return strings.Join(allowed, ", ")
}
