package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockScopeFetcher is a mock implementation of scopes.FetcherInterface
type mockScopeFetcher struct {
	scopes []string
	err    error
}

func (m *mockScopeFetcher) FetchTokenScopes(_ context.Context, _ string) ([]string, error) {
	return m.scopes, m.err
}

func TestWithPATScopes(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name                    string
		tokenInfo               *ghcontext.TokenInfo
		fetcherScopes           []string
		fetcherErr              error
		expectScopesFetched     bool
		expectedScopes          []string
		expectNextHandlerCalled bool
	}{
		{
			name:                    "no token info in context calls next handler",
			tokenInfo:               nil,
			expectScopesFetched:     false,
			expectedScopes:          nil,
			expectNextHandlerCalled: true,
		},
		{
			name: "non-PAT token type skips scope fetching",
			tokenInfo: &ghcontext.TokenInfo{
				Token:     "gho_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
				TokenType: utils.TokenTypeOAuthAccessToken,
			},
			expectScopesFetched:     false,
			expectedScopes:          nil,
			expectNextHandlerCalled: true,
		},
		{
			name: "fine-grained PAT skips scope fetching",
			tokenInfo: &ghcontext.TokenInfo{
				Token:     "github_pat_xxxxxxxxxxxxxxxxxxxxxxx",
				TokenType: utils.TokenTypeFineGrainedPersonalAccessToken,
			},
			expectScopesFetched:     false,
			expectedScopes:          nil,
			expectNextHandlerCalled: true,
		},
		{
			name: "classic PAT fetches and stores scopes",
			tokenInfo: &ghcontext.TokenInfo{
				Token:     "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
				TokenType: utils.TokenTypePersonalAccessToken,
			},
			fetcherScopes:           []string{"repo", "user", "read:org"},
			expectScopesFetched:     true,
			expectedScopes:          []string{"repo", "user", "read:org"},
			expectNextHandlerCalled: true,
		},
		{
			name: "classic PAT with empty scopes",
			tokenInfo: &ghcontext.TokenInfo{
				Token:     "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
				TokenType: utils.TokenTypePersonalAccessToken,
			},
			fetcherScopes:           []string{},
			expectScopesFetched:     true,
			expectedScopes:          []string{},
			expectNextHandlerCalled: true,
		},
		{
			name: "fetcher error calls next handler without scopes",
			tokenInfo: &ghcontext.TokenInfo{
				Token:     "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
				TokenType: utils.TokenTypePersonalAccessToken,
			},
			fetcherErr:              errors.New("network error"),
			expectScopesFetched:     false,
			expectedScopes:          nil,
			expectNextHandlerCalled: true,
		},
		{
			name: "old-style PAT (40 hex chars) fetches scopes",
			tokenInfo: &ghcontext.TokenInfo{
				Token:     "0123456789abcdef0123456789abcdef01234567",
				TokenType: utils.TokenTypePersonalAccessToken,
			},
			fetcherScopes:           []string{"repo"},
			expectScopesFetched:     true,
			expectedScopes:          []string{"repo"},
			expectNextHandlerCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedScopes []string
			var scopesFound bool
			var nextHandlerCalled bool

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextHandlerCalled = true
				capturedScopes, scopesFound = ghcontext.GetTokenScopes(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			fetcher := &mockScopeFetcher{
				scopes: tt.fetcherScopes,
				err:    tt.fetcherErr,
			}

			middleware := WithPATScopes(logger, fetcher)
			handler := middleware(nextHandler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)

			// Set up context with token info if provided
			if tt.tokenInfo != nil {
				ctx := ghcontext.WithTokenInfo(req.Context(), tt.tokenInfo)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectNextHandlerCalled, nextHandlerCalled, "next handler called mismatch")

			if tt.expectNextHandlerCalled {
				assert.Equal(t, tt.expectScopesFetched, scopesFound, "scopes found mismatch")
				assert.Equal(t, tt.expectedScopes, capturedScopes)
			}
		})
	}
}

func TestWithPATScopes_PreservesExistingTokenInfo(t *testing.T) {
	logger := slog.Default()

	var capturedTokenInfo *ghcontext.TokenInfo
	var capturedScopes []string
	var scopesFound bool

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTokenInfo, _ = ghcontext.GetTokenInfo(r.Context())
		capturedScopes, scopesFound = ghcontext.GetTokenScopes(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	fetcher := &mockScopeFetcher{
		scopes: []string{"repo", "user"},
	}

	originalTokenInfo := &ghcontext.TokenInfo{
		Token:     "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		TokenType: utils.TokenTypePersonalAccessToken,
	}

	middleware := WithPATScopes(logger, fetcher)
	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := ghcontext.WithTokenInfo(req.Context(), originalTokenInfo)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	require.NotNil(t, capturedTokenInfo)
	assert.Equal(t, originalTokenInfo.Token, capturedTokenInfo.Token)
	assert.Equal(t, originalTokenInfo.TokenType, capturedTokenInfo.TokenType)
	assert.True(t, scopesFound)
	assert.Equal(t, []string{"repo", "user"}, capturedScopes)
}
