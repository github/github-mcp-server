package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	ghcontext "github.com/github/github-mcp-server/v2/pkg/context"
	"github.com/github/github-mcp-server/v2/pkg/http/oauth"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithInvalidTokenChallenge(t *testing.T) {
	oauthCfg := &oauth.Config{BaseURL: "https://mcp.example.com", ResourcePath: "/mcp"}

	// toolsCall builds a request whose context carries a ghu_ token and a
	// tools/call method, the common shape the middleware actually probes.
	toolsCall := func(tokenType utils.TokenType, token string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		ctx := ghcontext.WithTokenInfo(req.Context(), &ghcontext.TokenInfo{
			Token:     token,
			TokenType: tokenType,
		})
		ctx = ghcontext.WithMCPMethodInfo(ctx, &ghcontext.MCPMethodInfo{
			Method:   "tools/call",
			ItemName: "get_me",
		})
		return req.WithContext(ctx)
	}

	tests := []struct {
		name          string
		request       func() *http.Request
		fetcher       *mockScopeFetcher
		expectNext    bool
		expectStatus  int
		expectInvalid bool // WWW-Authenticate carries error="invalid_token"
	}{
		{
			name:          "invalid ghu_ token on tools/call returns 401 invalid_token",
			request:       func() *http.Request { return toolsCall(utils.TokenTypeUserToServerGitHubAppToken, "ghu_expired") },
			fetcher:       &mockScopeFetcher{err: scopes.ErrInvalidToken},
			expectNext:    false,
			expectStatus:  http.StatusUnauthorized,
			expectInvalid: true,
		},
		{
			name:         "valid ghu_ token passes through",
			request:      func() *http.Request { return toolsCall(utils.TokenTypeUserToServerGitHubAppToken, "ghu_good") },
			fetcher:      &mockScopeFetcher{scopes: []string{}},
			expectNext:   true,
			expectStatus: http.StatusOK,
		},
		{
			name:         "network error is not turned into a 401",
			request:      func() *http.Request { return toolsCall(utils.TokenTypeUserToServerGitHubAppToken, "ghu_good") },
			fetcher:      &mockScopeFetcher{err: errors.New("dial tcp: connection refused")},
			expectNext:   true,
			expectStatus: http.StatusOK,
		},
		{
			name: "classic PAT with scopes already in context is not probed again",
			request: func() *http.Request {
				req := toolsCall(utils.TokenTypePersonalAccessToken, "ghp_good")
				ctx := ghcontext.WithTokenScopes(req.Context(), []string{"repo"})
				return req.WithContext(ctx)
			},
			// err would fire if the middleware probed; it must not.
			fetcher:      &mockScopeFetcher{err: scopes.ErrInvalidToken},
			expectNext:   true,
			expectStatus: http.StatusOK,
		},
		{
			name: "non tools/call method passes through without probing",
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
				ctx := ghcontext.WithTokenInfo(req.Context(), &ghcontext.TokenInfo{
					Token:     "ghu_whatever",
					TokenType: utils.TokenTypeUserToServerGitHubAppToken,
				})
				ctx = ghcontext.WithMCPMethodInfo(ctx, &ghcontext.MCPMethodInfo{Method: "tools/list"})
				return req.WithContext(ctx)
			},
			fetcher:      &mockScopeFetcher{err: scopes.ErrInvalidToken},
			expectNext:   true,
			expectStatus: http.StatusOK,
		},
		{
			name: "missing token info passes through",
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
				ctx := ghcontext.WithMCPMethodInfo(req.Context(), &ghcontext.MCPMethodInfo{Method: "tools/call"})
				return req.WithContext(ctx)
			},
			fetcher:      &mockScopeFetcher{err: scopes.ErrInvalidToken},
			expectNext:   true,
			expectStatus: http.StatusOK,
		},
		{
			name: "health check endpoint is never probed",
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/_ping", nil)
				ctx := ghcontext.WithTokenInfo(req.Context(), &ghcontext.TokenInfo{
					Token:     "ghu_whatever",
					TokenType: utils.TokenTypeUserToServerGitHubAppToken,
				})
				return req.WithContext(ctx)
			},
			fetcher:      &mockScopeFetcher{err: scopes.ErrInvalidToken},
			expectNext:   true,
			expectStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nextCalled bool
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := WithInvalidTokenChallenge(oauthCfg, tt.fetcher)(next)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, tt.request())

			assert.Equal(t, tt.expectNext, nextCalled, "next handler invocation")
			assert.Equal(t, tt.expectStatus, rr.Code)

			if tt.expectInvalid {
				wwwAuth := rr.Header().Get("WWW-Authenticate")
				require.NotEmpty(t, wwwAuth, "expected WWW-Authenticate header")
				assert.Contains(t, wwwAuth, `error="invalid_token"`)
				assert.Contains(t, wwwAuth, "resource_metadata=")
				assert.Contains(t, wwwAuth, "/.well-known/oauth-protected-resource")
			}
		})
	}
}
