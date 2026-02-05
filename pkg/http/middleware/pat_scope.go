package middleware

import (
	"log/slog"
	"net/http"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/utils"
)

// WithScopeChallenge creates a new middleware that determines if an OAuth request contains sufficient scopes to
// complete the request and returns a scope challenge if not.
func WithPATScopes(logger *slog.Logger, scopeFetcher scopes.FetcherInterface) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			tokenInfo, ok := ghcontext.GetTokenInfo(ctx)
			if !ok || tokenInfo == nil {
				logger.Warn("no token info found in context")
				next.ServeHTTP(w, r)
				return
			}

			// Fetch token scopes for scope-based tool filtering (PAT tokens only)
			// Only classic PATs (ghp_ prefix) return OAuth scopes via X-OAuth-Scopes header.
			// Fine-grained PATs and other token types don't support this, so we skip filtering.
			if tokenInfo.TokenType == utils.TokenTypePersonalAccessToken {
				scopesList, err := scopeFetcher.FetchTokenScopes(ctx, tokenInfo.Token)
				if err != nil {
					logger.Warn("failed to fetch PAT scopes", "error", err)
					next.ServeHTTP(w, r)
					return
				}

				tokenInfo.Scopes = scopesList
				tokenInfo.ScopesFetched = true

				// Store fetched scopes in context for downstream use
				ctx := ghcontext.WithTokenInfo(ctx, tokenInfo)

				next.ServeHTTP(w, r.WithContext(ctx))
			}
		}
		return http.HandlerFunc(fn)
	}
}
