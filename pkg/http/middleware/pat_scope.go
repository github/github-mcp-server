package middleware

import (
	"log/slog"
	"net/http"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/utils"
)

// WithPATScopes is a middleware that fetches and stores scopes for classic Personal Access Tokens (PATs) in the request context.
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
				existingScopes, ok := ghcontext.GetTokenScopes(ctx)
				if ok {
					logger.Debug("using existing scopes from context", "scopes", existingScopes)
					next.ServeHTTP(w, r)
					return
				}

				scopesList, err := scopeFetcher.FetchTokenScopes(ctx, tokenInfo.Token)
				if err != nil {
					logger.Warn("failed to fetch PAT scopes", "error", err)
					next.ServeHTTP(w, r)
					return
				}

				// Store fetched scopes in context for downstream use
				ctx = ghcontext.WithTokenScopes(ctx, scopesList)

				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
