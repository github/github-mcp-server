package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/github/github-mcp-server/pkg/http/oauth"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractUserToken(t *testing.T) {
	oauthCfg := &oauth.Config{
		BaseURL:             "https://example.com",
		AuthorizationServer: "https://github.com/login/oauth",
	}

	tests := []struct {
		name               string
		authHeader         string
		expectedStatusCode int
		expectedTokenType  utils.TokenType
		expectedToken      string
		expectTokenInfo    bool
		expectWWWAuth      bool
	}{
		// Missing authorization header
		{
			name:               "missing Authorization header returns 401 with WWW-Authenticate",
			authHeader:         "",
			expectedStatusCode: http.StatusUnauthorized,
			expectTokenInfo:    false,
			expectWWWAuth:      true,
		},
		// Personal Access Token (classic) - ghp_ prefix
		{
			name:               "personal access token (classic) with Bearer prefix",
			authHeader:         "Bearer ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectedStatusCode: http.StatusOK,
			expectedTokenType:  utils.TokenTypePersonalAccessToken,
			expectedToken:      "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectTokenInfo:    true,
		},
		{
			name:               "personal access token (classic) with bearer lowercase",
			authHeader:         "bearer ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectedStatusCode: http.StatusOK,
			expectedTokenType:  utils.TokenTypePersonalAccessToken,
			expectedToken:      "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectTokenInfo:    true,
		},
		{
			name:               "personal access token (classic) without Bearer prefix",
			authHeader:         "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectedStatusCode: http.StatusOK,
			expectedTokenType:  utils.TokenTypePersonalAccessToken,
			expectedToken:      "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectTokenInfo:    true,
		},
		// Fine-grained Personal Access Token - github_pat_ prefix
		{
			name:               "fine-grained personal access token with Bearer prefix",
			authHeader:         "Bearer github_pat_xxxxxxxxxxxxxxxxxxxxxxx",
			expectedStatusCode: http.StatusOK,
			expectedTokenType:  utils.TokenTypeFineGrainedPersonalAccessToken,
			expectedToken:      "github_pat_xxxxxxxxxxxxxxxxxxxxxxx",
			expectTokenInfo:    true,
		},
		{
			name:               "fine-grained personal access token without Bearer prefix",
			authHeader:         "github_pat_xxxxxxxxxxxxxxxxxxxxxxx",
			expectedStatusCode: http.StatusOK,
			expectedTokenType:  utils.TokenTypeFineGrainedPersonalAccessToken,
			expectedToken:      "github_pat_xxxxxxxxxxxxxxxxxxxxxxx",
			expectTokenInfo:    true,
		},
		// OAuth Access Token - gho_ prefix
		{
			name:               "OAuth access token with Bearer prefix",
			authHeader:         "Bearer gho_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectedStatusCode: http.StatusOK,
			expectedTokenType:  utils.TokenTypeOAuthAccessToken,
			expectedToken:      "gho_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectTokenInfo:    true,
		},
		{
			name:               "OAuth access token without Bearer prefix",
			authHeader:         "gho_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectedStatusCode: http.StatusOK,
			expectedTokenType:  utils.TokenTypeOAuthAccessToken,
			expectedToken:      "gho_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectTokenInfo:    true,
		},
		// User-to-Server GitHub App Token - ghu_ prefix
		{
			name:               "user-to-server GitHub App token with Bearer prefix",
			authHeader:         "Bearer ghu_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectedStatusCode: http.StatusOK,
			expectedTokenType:  utils.TokenTypeUserToServerGitHubAppToken,
			expectedToken:      "ghu_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectTokenInfo:    true,
		},
		{
			name:               "user-to-server GitHub App token without Bearer prefix",
			authHeader:         "ghu_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectedStatusCode: http.StatusOK,
			expectedTokenType:  utils.TokenTypeUserToServerGitHubAppToken,
			expectedToken:      "ghu_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectTokenInfo:    true,
		},
		// Server-to-Server GitHub App Token (installation token) - ghs_ prefix
		{
			name:               "server-to-server GitHub App token with Bearer prefix",
			authHeader:         "Bearer ghs_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectedStatusCode: http.StatusOK,
			expectedTokenType:  utils.TokenTypeServerToServerGitHubAppToken,
			expectedToken:      "ghs_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectTokenInfo:    true,
		},
		{
			name:               "server-to-server GitHub App token without Bearer prefix",
			authHeader:         "ghs_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectedStatusCode: http.StatusOK,
			expectedTokenType:  utils.TokenTypeServerToServerGitHubAppToken,
			expectedToken:      "ghs_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectTokenInfo:    true,
		},
		// Old-style Personal Access Token (40 hex characters, pre-2021)
		{
			name:               "old-style personal access token (40 hex chars) with Bearer prefix",
			authHeader:         "Bearer 0123456789abcdef0123456789abcdef01234567",
			expectedStatusCode: http.StatusOK,
			expectedTokenType:  utils.TokenTypePersonalAccessToken,
			expectedToken:      "0123456789abcdef0123456789abcdef01234567",
			expectTokenInfo:    true,
		},
		{
			name:               "old-style personal access token (40 hex chars) without Bearer prefix",
			authHeader:         "0123456789abcdef0123456789abcdef01234567",
			expectedStatusCode: http.StatusOK,
			expectedTokenType:  utils.TokenTypePersonalAccessToken,
			expectedToken:      "0123456789abcdef0123456789abcdef01234567",
			expectTokenInfo:    true,
		},
		// Error cases
		{
			name:               "unsupported GitHub-Bearer header returns 400",
			authHeader:         "GitHub-Bearer some_encrypted_token",
			expectedStatusCode: http.StatusBadRequest,
			expectTokenInfo:    false,
		},
		{
			name:               "invalid token format returns 400",
			authHeader:         "Bearer invalid_token_format",
			expectedStatusCode: http.StatusBadRequest,
			expectTokenInfo:    false,
		},
		{
			name:               "unrecognized prefix returns 400",
			authHeader:         "Bearer xyz_notavalidprefix",
			expectedStatusCode: http.StatusBadRequest,
			expectTokenInfo:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedTokenInfo *ghcontext.TokenInfo
			var tokenInfoCaptured bool

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedTokenInfo, tokenInfoCaptured = ghcontext.GetTokenInfo(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			middleware := ExtractUserToken(oauthCfg)
			handler := middleware(nextHandler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set(headers.AuthorizationHeader, tt.authHeader)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code)

			if tt.expectWWWAuth {
				wwwAuth := rr.Header().Get("WWW-Authenticate")
				assert.NotEmpty(t, wwwAuth, "expected WWW-Authenticate header")
				assert.Contains(t, wwwAuth, "Bearer resource_metadata=")
			}

			if tt.expectTokenInfo {
				require.True(t, tokenInfoCaptured, "expected TokenInfo to be present in context")
				require.NotNil(t, capturedTokenInfo)
				assert.Equal(t, tt.expectedTokenType, capturedTokenInfo.TokenType)
				assert.Equal(t, tt.expectedToken, capturedTokenInfo.Token)
			} else {
				assert.False(t, tokenInfoCaptured, "expected no TokenInfo in context")
			}
		})
	}
}

func TestExtractUserToken_NilOAuthConfig(t *testing.T) {
	var capturedTokenInfo *ghcontext.TokenInfo
	var tokenInfoCaptured bool

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTokenInfo, tokenInfoCaptured = ghcontext.GetTokenInfo(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	middleware := ExtractUserToken(nil)
	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(headers.AuthorizationHeader, "Bearer ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.True(t, tokenInfoCaptured)
	require.NotNil(t, capturedTokenInfo)
	assert.Equal(t, utils.TokenTypePersonalAccessToken, capturedTokenInfo.TokenType)
}

func TestExtractUserToken_MissingAuthHeader_WWWAuthenticateFormat(t *testing.T) {
	oauthCfg := &oauth.Config{
		BaseURL:             "https://api.example.com",
		AuthorizationServer: "https://github.com/login/oauth",
		ResourcePath:        "/mcp",
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := ExtractUserToken(oauthCfg)
	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No Authorization header
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	wwwAuth := rr.Header().Get("WWW-Authenticate")
	assert.NotEmpty(t, wwwAuth)
	assert.Contains(t, wwwAuth, "Bearer")
	assert.Contains(t, wwwAuth, "resource_metadata=")
	assert.Contains(t, wwwAuth, "/.well-known/oauth-protected-resource")
}

func TestSendAuthChallenge(t *testing.T) {
	tests := []struct {
		name             string
		oauthCfg         *oauth.Config
		requestPath      string
		expectedContains []string
	}{
		{
			name: "with base URL configured",
			oauthCfg: &oauth.Config{
				BaseURL: "https://mcp.example.com",
			},
			requestPath: "/api/test",
			expectedContains: []string{
				"Bearer",
				"resource_metadata=",
				"https://mcp.example.com/.well-known/oauth-protected-resource",
			},
		},
		{
			name:        "with nil config uses request host",
			oauthCfg:    nil,
			requestPath: "/api/test",
			expectedContains: []string{
				"Bearer",
				"resource_metadata=",
				"/.well-known/oauth-protected-resource",
			},
		},
		{
			name: "with resource path configured",
			oauthCfg: &oauth.Config{
				BaseURL:      "https://mcp.example.com",
				ResourcePath: "/mcp",
			},
			requestPath: "/api/test",
			expectedContains: []string{
				"Bearer",
				"resource_metadata=",
				"/mcp",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)

			sendAuthChallenge(rr, req, tt.oauthCfg)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
			wwwAuth := rr.Header().Get("WWW-Authenticate")
			for _, expected := range tt.expectedContains {
				assert.Contains(t, wwwAuth, expected)
			}
		})
	}
}
