package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/github/github-mcp-server/pkg/http/oauth"
	"github.com/github/github-mcp-server/pkg/utils"
)

func ExtractUserToken(oauthCfg *oauth.Config) func(next http.Handler) http.Handler {
	return ExtractUserTokenWithFallback(oauthCfg, "")
}

// ExtractUserTokenWithFallback extracts a per-request token, falling back to a
// configured token only when the Authorization header is absent. An explicitly
// provided header, including an empty or malformed one, is never replaced.
func ExtractUserTokenWithFallback(oauthCfg *oauth.Config, fallbackToken string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Check if token info already exists in context, if it does, skip extraction.
			// In remote setup, we may have already extracted token info earlier.
			if _, ok := ghcontext.GetTokenInfo(ctx); ok {
				// Token info already exists in context, skip extraction
				next.ServeHTTP(w, r)
				return
			}

			tokenType, token, err := utils.ParseAuthorizationHeader(r)
			if err != nil {
				if errors.Is(err, utils.ErrMissingAuthorizationHeader) {
					if !hasAuthorizationHeader(r.Header) && fallbackToken != "" {
						tokenType, tokenErr := utils.ParseToken(fallbackToken)
						if tokenErr != nil {
							http.Error(w, "configured static token is invalid", http.StatusInternalServerError)
							return
						}
						ctx = ghcontext.WithTokenInfo(ctx, &ghcontext.TokenInfo{
							Token:     fallbackToken,
							TokenType: tokenType,
						})
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}

					// For missing Authorization header, return 401 with WWW-Authenticate header per MCP spec
					sendAuthChallenge(w, r, oauthCfg)
					return
				}
				// For other auth errors (bad format, unsupported), return 400
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			ctx = ghcontext.WithTokenInfo(ctx, &ghcontext.TokenInfo{
				Token:     token,
				TokenType: tokenType,
			})
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

func hasAuthorizationHeader(header http.Header) bool {
	for name := range header {
		if strings.EqualFold(name, headers.AuthorizationHeader) {
			return true
		}
	}
	return false
}

// sendAuthChallenge sends a 401 Unauthorized response with WWW-Authenticate header
// containing the OAuth protected resource metadata URL as per RFC 6750 and MCP spec.
func sendAuthChallenge(w http.ResponseWriter, r *http.Request, oauthCfg *oauth.Config) {
	resourcePath := oauth.ResolveResourcePath(r, oauthCfg)
	resourceMetadataURL := oauth.BuildResourceMetadataURL(r, oauthCfg, resourcePath)
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer resource_metadata=%q`, resourceMetadataURL))
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}
