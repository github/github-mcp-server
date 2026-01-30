package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/oauth"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/utils"
)

// FetchScopesFromGitHubAPI fetches the OAuth scopes from the GitHub API by making
// a HEAD request and reading the X-OAuth-Scopes header. This is used as a fallback
// when scopes are not provided in the token info header.
func FetchScopesFromGitHubAPI(ctx context.Context, token string, apiHost utils.APIHostResolver) ([]string, error) {
	baseUrl, err := apiHost.BaseRESTURL(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, strings.TrimSuffix(baseUrl.String(), "/")+"/user", http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	scopeHeader := resp.Header.Get("X-OAuth-Scopes")
	if scopeHeader == "" {
		return []string{}, nil
	}

	// Parse comma-separated scopes and trim whitespace
	rawScopes := strings.Split(scopeHeader, ",")
	parsedScopes := make([]string, 0, len(rawScopes))
	for _, s := range rawScopes {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			parsedScopes = append(parsedScopes, trimmed)
		}
	}
	return parsedScopes, nil
}

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

			// Try to use pre-parsed MCP method info first (performance optimization)
			// This avoids re-parsing the JSON body if WithMCPParse middleware ran earlier
			var toolName string
			if methodInfo, ok := ghcontext.MCPMethod(ctx); ok && methodInfo != nil {
				// Only check tools/call requests
				if methodInfo.Method != "tools/call" {
					next.ServeHTTP(w, r)
					return
				}
				toolName = methodInfo.ItemName
			} else {
				// Fallback: parse the request body directly
				body, err := io.ReadAll(r.Body)
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))

				var mcpRequest struct {
					JSONRPC string `json:"jsonrpc"`
					Method  string `json:"method"`
					Params  struct {
						Name      string         `json:"name,omitempty"`
						Arguments map[string]any `json:"arguments,omitempty"`
					} `json:"params"`
				}

				err = json.Unmarshal(body, &mcpRequest)
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}

				// Only check tools/call requests
				if mcpRequest.Method != "tools/call" {
					next.ServeHTTP(w, r)
					return
				}

				toolName = mcpRequest.Params.Name
			}
			toolScopeInfo, err := scopes.GetToolScopeInfo(toolName)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			// If tool not found in scope map, allow the request
			if toolScopeInfo == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Get OAuth scopes from GitHub API
			activeScopes, err := scopeFetcher.FetchTokenScopes(ctx, tokenInfo.Token)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			// Check if user has the required scopes
			if toolScopeInfo.HasAcceptedScope(activeScopes...) {
				next.ServeHTTP(w, r)
				return
			}

			// User lacks required scopes - get the scopes they need
			requiredScopes := toolScopeInfo.GetRequiredScopesSlice()

			// Build the resource metadata URL using the shared utility
			// GetEffectiveResourcePath returns the original path (e.g., /mcp or /mcp/x/all)
			// which is used to construct the well-known OAuth protected resource URL
			resourcePath := oauth.ResolveResourcePath(r, oauthCfg)
			resourceMetadataURL := oauth.BuildResourceMetadataURL(r, oauthCfg, resourcePath)

			// Build recommended scopes: existing scopes + required scopes
			recommendedScopes := make([]string, 0, len(activeScopes)+len(requiredScopes))
			recommendedScopes = append(recommendedScopes, activeScopes...)
			recommendedScopes = append(recommendedScopes, requiredScopes...)

			// Build the WWW-Authenticate header value
			wwwAuthenticateHeader := fmt.Sprintf(`Bearer error="insufficient_scope", scope=%q, resource_metadata=%q, error_description=%q`,
				strings.Join(recommendedScopes, " "),
				resourceMetadataURL,
				"Additional scopes required: "+strings.Join(requiredScopes, ", "),
			)

			// Send scope challenge response with the superset of existing and required scopes
			w.Header().Set("WWW-Authenticate", wwwAuthenticateHeader)
			http.Error(w, "Forbidden: insufficient scopes", http.StatusForbidden)
		}
		return http.HandlerFunc(fn)
	}
}
