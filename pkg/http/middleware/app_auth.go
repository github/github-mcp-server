package middleware

import (
	"log/slog"
	"net/http"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/github/appauth"
	"github.com/github/github-mcp-server/pkg/utils"
)

// WithGitHubAppToken injects a GitHub App installation token into the request
// context when the incoming request does not already carry one. This lets the
// HTTP server authenticate as a GitHub App installation instead of requiring
// every caller to send an Authorization header.
//
// If the request already carries TokenInfo (e.g., an explicit Authorization
// header parsed earlier), this middleware is a no-op so per-request tokens
// take precedence.
//
// Why: the HTTP server's downstream pipeline (ExtractUserToken,
// RequestDeps.GetClient) is built around a per-request bearer token. Rather
// than rewire that pipeline, we synthesize a TokenInfo from the installation
// token so the existing flow works unchanged.
func WithGitHubAppToken(transport *appauth.Transport, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if _, ok := ghcontext.GetTokenInfo(ctx); ok {
				next.ServeHTTP(w, r)
				return
			}

			token, err := transport.Token(ctx)
			if err != nil {
				logger.Error("failed to obtain GitHub App installation token", "error", err)
				http.Error(w, "failed to obtain GitHub App installation token", http.StatusInternalServerError)
				return
			}

			ctx = ghcontext.WithTokenInfo(ctx, &ghcontext.TokenInfo{
				Token:     token,
				TokenType: utils.TokenTypeServerToServerGitHubAppToken,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
