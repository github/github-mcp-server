package middleware

import (
	"errors"
	"net/http"

	ghcontext "github.com/github/github-mcp-server/v2/pkg/context"
	"github.com/github/github-mcp-server/v2/pkg/http/oauth"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
)

// WithInvalidTokenChallenge responds with a transport-level HTTP 401 when GitHub
// rejects the token as invalid or expired, so OAuth clients refresh instead of
// seeing isError: true inside an HTTP 200 (which never triggers a refresh).
//
// It must run before dispatch: tool responses stream as SSE, so the status is
// flushed before the handler runs and cannot be rewritten afterward. The check
// is narrow by design — only a 401 challenges; every other error stays
// isError: true per the project's error model (#420).
func WithInvalidTokenChallenge(oauthCfg *oauth.Config, scopeFetcher scopes.FetcherInterface) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Skip health check endpoints.
			if r.URL.Path == "/_ping" {
				next.ServeHTTP(w, r)
				return
			}

			tokenInfo, ok := ghcontext.GetTokenInfo(ctx)
			if !ok || tokenInfo == nil || tokenInfo.Token == "" {
				// Missing tokens are handled upstream by ExtractUserToken.
				next.ServeHTTP(w, r)
				return
			}

			// Only tools/call needs a valid credential; discovery methods
			// (tools/list, initialize) must stay reachable pre-auth.
			methodInfo, ok := ghcontext.MCPMethod(ctx)
			if !ok || methodInfo == nil || methodInfo.Method != "tools/call" {
				next.ServeHTTP(w, r)
				return
			}

			// Classic PATs were already probed by WithPATScopes; scopes in
			// context mean the token authenticated, so skip a second call.
			if _, ok := ghcontext.GetTokenScopes(ctx); ok {
				next.ServeHTTP(w, r)
				return
			}

			// Other token types (ghu_, gho_, fine-grained) have no prior probe.
			// FetchTokenScopes returns ErrInvalidToken on 401.
			_, err := scopeFetcher.FetchTokenScopes(ctx, tokenInfo.Token)
			if errors.Is(err, scopes.ErrInvalidToken) {
				sendInvalidTokenChallenge(w, r, oauthCfg)
				return
			}

			// Success, network error, or 5xx must not become a 401.
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
