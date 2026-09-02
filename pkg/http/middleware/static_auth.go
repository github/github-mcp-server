package middleware

import (
	"net/http"
	"strings"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/github/github-mcp-server/pkg/http/oauth"
)

// StaticAuthBrowserGuard prevents browser requests from implicitly consuming a
// shared static credential. It must run before CORS response headers are set.
func StaticAuthBrowserGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isOAuthMetadataPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("Origin") == "" {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := ghcontext.GetTokenInfo(r.Context()); ok {
			next.ServeHTTP(w, r)
			return
		}

		if r.Method == http.MethodOptions {
			if headerListContains(r.Header.Get("Access-Control-Request-Headers"), headers.AuthorizationHeader) {
				next.ServeHTTP(w, r)
				return
			}
			rejectStaticAuthBrowserRequest(w)
			return
		}

		if hasAuthorizationHeader(r.Header) {
			next.ServeHTTP(w, r)
			return
		}
		rejectStaticAuthBrowserRequest(w)
	})
}

func isOAuthMetadataPath(path string) bool {
	return path == oauth.OAuthProtectedResourcePrefix || strings.HasPrefix(path, oauth.OAuthProtectedResourcePrefix+"/")
}

func headerListContains(value, target string) bool {
	for header := range strings.SplitSeq(value, ",") {
		if strings.EqualFold(strings.TrimSpace(header), target) {
			return true
		}
	}
	return false
}

func rejectStaticAuthBrowserRequest(w http.ResponseWriter) {
	http.Error(w, "Forbidden", http.StatusForbidden)
}
