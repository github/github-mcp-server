package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/oauth"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/utils"
)

// WithScopeChallenge creates a new middleware that determines if an OAuth request contains sufficient scopes to
// complete the request and returns a scope challenge if not.
func WithScopeChallenge(oauthCfg *oauth.Config, scopeFetcher scopes.FetcherInterface) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Skip health check endpoints
			if r.URL.Path == "/_ping" {
				next.ServeHTTP(w, r)
				return
			}

			// Get user from context
			tokenInfo, ok := ghcontext.GetTokenInfo(ctx)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			// Only check OAuth tokens - scope challenge allows OAuth apps to request additional scopes
			if tokenInfo.TokenType != utils.TokenTypeOAuthAccessToken {
				next.ServeHTTP(w, r)
				return
			}

			// Use pre-parsed envelope information when WithMCPParse ran earlier.
			var methodInfo *ghcontext.MCPMethodInfo
			if parsedMethodInfo, ok := ghcontext.MCPMethod(ctx); ok && parsedMethodInfo != nil {
				// Only check tools/call requests
				if parsedMethodInfo.Method != "tools/call" {
					next.ServeHTTP(w, r)
					return
				}
				methodInfo = parsedMethodInfo
			} else {
				// Fallback: parse the request body directly
				body, err := io.ReadAll(r.Body)
				if err != nil {
					if isMaxBytesError(err) {
						writeRequestTooLarge(w)
						return
					}
					next.ServeHTTP(w, r)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))

				methodInfo, err = parseMCPMethodInfo(body)
				if err != nil || methodInfo == nil {
					next.ServeHTTP(w, r)
					return
				}

				// Only check tools/call requests
				if methodInfo.Method != "tools/call" {
					next.ServeHTTP(w, r)
					return
				}
			}
			scopeAccess, ok := scopes.GetToolScopeAccess(methodInfo.ItemName)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			// Get OAuth scopes for Token. First check if scopes are already in context,  then fetch from GitHub if not present.
			// This allows Remote Server to pass scope info to avoid redundant GitHub API calls.
			activeScopes, ok := ghcontext.GetTokenScopes(ctx)
			if !ok || (len(activeScopes) == 0 && tokenInfo.Token != "") {
				fetchedScopes, err := scopeFetcher.FetchTokenScopes(ctx, tokenInfo.Token)
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}
				activeScopes = fetchedScopes
			}

			// Store active scopes in context for downstream use
			ctx = ghcontext.WithTokenScopes(ctx, activeScopes)
			r = r.WithContext(ctx)

			// Most calls already have sufficient scopes. Compare against the
			// exhaustive upper bound before materializing tool arguments.
			if scopeAccess.MaximumScopesSatisfied(activeScopes) {
				next.ServeHTTP(w, r)
				return
			}

			arguments, err := methodInfo.DecodeArguments()
			if err != nil {
				// Preserve normal MCP handler validation for invalid arguments.
				next.ServeHTTP(w, r)
				return
			}
			challengeScopes := scopeAccess.ResolveChallenge(arguments, activeScopes)
			if len(challengeScopes) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			// Build the resource metadata URL using the shared utility
			// GetEffectiveResourcePath returns the original path (e.g., /mcp or /mcp/x/all)
			// which is used to construct the well-known OAuth protected resource URL
			resourcePath := oauth.ResolveResourcePath(r, oauthCfg)
			resourceMetadataURL := oauth.BuildResourceMetadataURL(r, oauthCfg, resourcePath)

			// Build the WWW-Authenticate header value
			wwwAuthenticateHeader := fmt.Sprintf(`Bearer error="insufficient_scope", scope=%q, resource_metadata=%q, error_description=%q`,
				strings.Join(challengeScopes, " "),
				resourceMetadataURL,
				"Additional scopes required: "+strings.Join(challengeScopes, ", "),
			)

			// Send the exact scope set returned by the tool's scope check.
			w.Header().Set("WWW-Authenticate", wwwAuthenticateHeader)
			http.Error(w, "Forbidden: insufficient scopes", http.StatusForbidden)
		}
		return http.HandlerFunc(fn)
	}
}
