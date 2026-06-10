package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/oauth"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type scopeChallengeFetcher struct {
	scopes []string
	err    error
}

func (m *scopeChallengeFetcher) FetchTokenScopes(_ context.Context, _ string) ([]string, error) {
	return m.scopes, m.err
}

func TestWithScopeChallenge_ReturnsMachineReadableInsufficientScopeCode(t *testing.T) {
	scopes.SetGlobalToolScopeMap(scopes.ToolScopeMap{
		"create_or_update_file": {
			RequiredScopes: []string{"repo"},
			AcceptedScopes: []string{"repo"},
		},
	})
	t.Cleanup(func() {
		scopes.SetGlobalToolScopeMap(nil)
	})

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := WithScopeChallenge(
		&oauth.Config{BaseURL: "https://example.com"},
		&scopeChallengeFetcher{scopes: []string{}},
	)(nextHandler)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req = req.WithContext(ghcontext.WithTokenInfo(req.Context(), &ghcontext.TokenInfo{
		Token:     "gho_test",
		TokenType: utils.TokenTypeOAuthAccessToken,
	}))
	req = req.WithContext(ghcontext.WithMCPMethodInfo(req.Context(), &ghcontext.MCPMethodInfo{
		Method:   "tools/call",
		ItemName: "create_or_update_file",
	}))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Header().Get("WWW-Authenticate"), `error="insufficient_scope"`)

	var body struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "insufficient_scope", body.Code)
}
