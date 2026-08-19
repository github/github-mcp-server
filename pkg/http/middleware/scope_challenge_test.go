package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/oauth"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestWithScopeChallengeResolvesScopesFromParsedArguments(t *testing.T) {
	setDynamicScopeTestMap(t)

	tests := []struct {
		name       string
		arguments  map[string]any
		wantStatus int
		wantNext   bool
	}{
		{
			name:       "regular file only requires repo",
			arguments:  map[string]any{"path": "README.md"},
			wantStatus: http.StatusNoContent,
			wantNext:   true,
		},
		{
			name:       "non-ASCII workflow path without header requires workflow",
			arguments:  map[string]any{"path": ".github/workflows/构建.yml"},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "workflow in array requires workflow",
			arguments: map[string]any{"files": []any{
				map[string]any{"path": "README.md"},
				map[string]any{"path": ".github/workflows/ci.yml"},
			}},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusNoContent)
			})
			handler := WithScopeChallenge(&oauth.Config{}, &mockScopeFetcher{})(next)

			request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			assert.Empty(t, request.Header.Get("Mcp-Param-path"))
			ctx := scopeChallengeContext(request.Context())
			ctx = ghcontext.WithMCPMethodInfo(ctx, &ghcontext.MCPMethodInfo{
				Method:    "tools/call",
				ItemName:  "write_file",
				Arguments: tt.arguments,
			})
			request = request.WithContext(ctx)

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			assert.Equal(t, tt.wantStatus, response.Code)
			assert.Equal(t, tt.wantNext, nextCalled)
			if tt.wantStatus == http.StatusForbidden {
				challenge := response.Header().Get("WWW-Authenticate")
				assert.Contains(t, challenge, `scope="repo workflow"`)
				assert.Contains(t, challenge, "Additional scopes required: workflow")
			}
		})
	}
}

func TestWithScopeChallengeResolvesScopesFromFallbackBody(t *testing.T) {
	setDynamicScopeTestMap(t)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	handler := WithScopeChallenge(&oauth.Config{}, &mockScopeFetcher{})(next)

	body := []byte(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"write_file","arguments":{"path":".github/workflows/ci.yml"}}}`)
	request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	request = request.WithContext(scopeChallengeContext(request.Context()))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusForbidden, response.Code)
	assert.False(t, nextCalled)
	assert.Contains(t, response.Header().Get("WWW-Authenticate"), "Additional scopes required: workflow")
}

func setDynamicScopeTestMap(t *testing.T) {
	t.Helper()
	scopes.SetGlobalToolScopeMap(scopes.ToolScopeMap{
		"write_file": {
			RequiredScopes: []string{"repo"},
			AcceptedScopes: []string{"repo"},
			ScopeResolver: func(arguments map[string]any) []string {
				if path, _ := arguments["path"].(string); strings.HasPrefix(path, ".github/workflows/") {
					return []string{"workflow"}
				}
				files, _ := arguments["files"].([]any)
				for _, file := range files {
					fileMap, _ := file.(map[string]any)
					if path, _ := fileMap["path"].(string); strings.HasPrefix(path, ".github/workflows/") {
						return []string{"workflow"}
					}
				}
				return nil
			},
		},
	})
	t.Cleanup(func() {
		scopes.SetGlobalToolScopeMap(nil)
	})
}

func scopeChallengeContext(ctx context.Context) context.Context {
	ctx = ghcontext.WithTokenInfo(ctx, &ghcontext.TokenInfo{
		Token:     "oauth-token",
		TokenType: utils.TokenTypeOAuthAccessToken,
	})
	ctx = ghcontext.WithTokenScopes(ctx, []string{"repo"})
	return ctx
}
