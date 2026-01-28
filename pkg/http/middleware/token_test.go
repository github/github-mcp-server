package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/http/oauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractUserToken(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name                string
		authHeader          string
		validateTokenFormat bool
		expectedStatus      int
		expectWWWAuth       bool
	}{
		{
			name:                "valid PAT token passes through",
			authHeader:          "Bearer ghp_test1234567890123456789012345678901234",
			validateTokenFormat: false,
			expectedStatus:      http.StatusOK,
			expectWWWAuth:       false,
		},
		{
			name:                "valid fine-grained PAT passes through",
			authHeader:          "Bearer github_pat_test1234567890123456789012345678901234",
			validateTokenFormat: false,
			expectedStatus:      http.StatusOK,
			expectWWWAuth:       false,
		},
		{
			name:                "valid OAuth token passes through",
			authHeader:          "Bearer gho_test1234567890123456789012345678901234",
			validateTokenFormat: false,
			expectedStatus:      http.StatusOK,
			expectWWWAuth:       false,
		},
		{
			name:                "valid old-style token passes through",
			authHeader:          "Bearer 1234567890abcdef1234567890abcdef12345678",
			validateTokenFormat: false,
			expectedStatus:      http.StatusOK,
			expectWWWAuth:       false,
		},
		{
			name:                "IDE token passes through",
			authHeader:          "Bearer tid=1;exp=25145314523;chat=1:hmac",
			validateTokenFormat: false,
			expectedStatus:      http.StatusOK,
			expectWWWAuth:       false,
		},
		{
			name:                "missing auth header returns 401 with WWW-Authenticate",
			authHeader:          "",
			validateTokenFormat: false,
			expectedStatus:      http.StatusUnauthorized,
			expectWWWAuth:       true,
		},
		{
			name:                "invalid token format returns 400 when ValidateTokenFormat is false",
			authHeader:          "Bearer invalid_token",
			validateTokenFormat: false,
			expectedStatus:      http.StatusBadRequest,
			expectWWWAuth:       false,
		},
		{
			name:                "invalid token format returns 401 with WWW-Authenticate when ValidateTokenFormat is true",
			authHeader:          "Bearer invalid_token",
			validateTokenFormat: true,
			expectedStatus:      http.StatusUnauthorized,
			expectWWWAuth:       true,
		},
		{
			name:                "GitHub-Bearer prefix returns 400 when ValidateTokenFormat is false",
			authHeader:          "GitHub-Bearer some_token",
			validateTokenFormat: false,
			expectedStatus:      http.StatusBadRequest,
			expectWWWAuth:       false,
		},
		{
			name:                "GitHub-Bearer prefix returns 401 when ValidateTokenFormat is true",
			authHeader:          "GitHub-Bearer some_token",
			validateTokenFormat: true,
			expectedStatus:      http.StatusUnauthorized,
			expectWWWAuth:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oauthCfg := &oauth.Config{
				BaseURL:             "https://api.github.com",
				ValidateTokenFormat: tc.validateTokenFormat,
			}

			middleware := ExtractUserToken(oauthCfg)
			handler := middleware(dummyHandler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)

			if tc.expectWWWAuth {
				wwwAuth := rr.Header().Get("WWW-Authenticate")
				require.NotEmpty(t, wwwAuth, "expected WWW-Authenticate header to be set")
				assert.True(t, strings.HasPrefix(wwwAuth, "Bearer resource_metadata="),
					"WWW-Authenticate header should have Bearer resource_metadata format")
			} else {
				assert.Empty(t, rr.Header().Get("WWW-Authenticate"),
					"WWW-Authenticate header should not be set")
			}
		})
	}
}

func TestParseAuthorizationHeader(t *testing.T) {
	tests := []struct {
		name         string
		authHeader   string
		expectedAuth authType
		expectError  bool
	}{
		{
			name:         "classic PAT",
			authHeader:   "Bearer ghp_abcdef1234567890",
			expectedAuth: authTypeGhToken,
			expectError:  false,
		},
		{
			name:         "fine-grained PAT",
			authHeader:   "Bearer github_pat_abcdef1234567890",
			expectedAuth: authTypeGhToken,
			expectError:  false,
		},
		{
			name:         "OAuth token",
			authHeader:   "Bearer gho_abcdef1234567890",
			expectedAuth: authTypeGhToken,
			expectError:  false,
		},
		{
			name:         "user access token",
			authHeader:   "Bearer ghu_abcdef1234567890",
			expectedAuth: authTypeGhToken,
			expectError:  false,
		},
		{
			name:         "installation token",
			authHeader:   "Bearer ghs_abcdef1234567890",
			expectedAuth: authTypeGhToken,
			expectError:  false,
		},
		{
			name:         "old-style 40 char hex token",
			authHeader:   "Bearer 1234567890abcdef1234567890abcdef12345678",
			expectedAuth: authTypeGhToken,
			expectError:  false,
		},
		{
			name:         "IDE token with colon",
			authHeader:   "Bearer tid=1;exp=25145314523;chat=1:hmac",
			expectedAuth: authTypeIDE,
			expectError:  false,
		},
		{
			name:         "lowercase bearer",
			authHeader:   "bearer ghp_abcdef1234567890",
			expectedAuth: authTypeGhToken,
			expectError:  false,
		},
		{
			name:         "missing header",
			authHeader:   "",
			expectedAuth: 0,
			expectError:  true,
		},
		{
			name:         "unsupported prefix",
			authHeader:   "GitHub-Bearer token",
			expectedAuth: 0,
			expectError:  true,
		},
		{
			name:         "invalid token format",
			authHeader:   "Bearer invalid",
			expectedAuth: 0,
			expectError:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			authType, _, err := parseAuthorizationHeader(req)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedAuth, authType)
			}
		})
	}
}
