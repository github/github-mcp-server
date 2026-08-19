package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/oauth"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithScopeChallenge_MaxBodySize covers the fallback body-parsing path,
// used when WithMCPParse has not already populated MCPMethodInfo in context.
func TestWithScopeChallenge_MaxBodySize(t *testing.T) {
	const limit = 64
	oauthCfg := &oauth.Config{}
	fetcher := &mockScopeFetcher{scopes: []string{"repo"}}

	newRequestWithBody := func(body io.Reader) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/mcp", body)
		ctx := ghcontext.WithTokenInfo(req.Context(), &ghcontext.TokenInfo{
			Token:     "******",
			TokenType: utils.TokenTypeOAuthAccessToken,
		})
		return req.WithContext(ctx)
	}

	newRequest := func(body string) *http.Request {
		return newRequestWithBody(strings.NewReader(body))
	}

	t.Run("oversized body is rejected before the fallback parse", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"` + strings.Repeat("x", limit) + `"}}`
		require.Greater(t, len(body), limit)

		var nextCalled bool
		next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			nextCalled = true
		})

		handler := WithMaxBodySize(limit)(WithScopeChallenge(oauthCfg, fetcher)(next))

		// An unknown length skips WithMaxBodySize's Content-Length fast path,
		// so the overflow surfaces from the fallback read.
		req := newRequestWithBody(unknownLengthBody(body))
		require.Equal(t, int64(-1), req.ContentLength, "test setup: Content-Length should be unknown")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.False(t, nextCalled, "downstream handler must not run for an oversized request")
		assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
		assert.Contains(t, rr.Body.String(), "request body too large")
	})

	t.Run("allowed body still reaches the fallback parse and next handler", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","method":"tools/list"}`
		require.LessOrEqual(t, len(body), limit)

		var nextCalled bool
		var capturedBody string
		next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			nextCalled = true
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			capturedBody = string(b)
		})

		handler := WithMaxBodySize(limit)(WithScopeChallenge(oauthCfg, fetcher)(next))

		req := newRequest(body)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.True(t, nextCalled, "downstream handler should run for an allowed request")
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, body, capturedBody, "body should be preserved for downstream handlers")
	})
}
