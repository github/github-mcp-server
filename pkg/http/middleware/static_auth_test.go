package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/middleware"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestStaticAuthBrowserGuard(t *testing.T) {
	tests := []struct {
		name               string
		method             string
		origin             string
		authorization      string
		preflightHeaders   string
		withUpstreamToken  bool
		expectedStatus     int
		expectInnerCalled  bool
		expectWildcardCORS bool
	}{
		{
			name:               "non-browser request is allowed",
			method:             http.MethodPost,
			expectedStatus:     http.StatusNoContent,
			expectInnerCalled:  true,
			expectWildcardCORS: true,
		},
		{
			name:           "headerless browser request is rejected before CORS",
			method:         http.MethodPost,
			origin:         "https://example.com",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:               "browser request with authorization is allowed",
			method:             http.MethodPost,
			origin:             "https://example.com",
			authorization:      "Bearer github_pat_requesttoken",
			expectedStatus:     http.StatusNoContent,
			expectInnerCalled:  true,
			expectWildcardCORS: true,
		},
		{
			name:               "upstream token context is allowed",
			method:             http.MethodPost,
			origin:             "https://example.com",
			withUpstreamToken:  true,
			expectedStatus:     http.StatusNoContent,
			expectInnerCalled:  true,
			expectWildcardCORS: true,
		},
		{
			name:             "headerless static auth preflight is rejected",
			method:           http.MethodOptions,
			origin:           "https://example.com",
			preflightHeaders: "content-type",
			expectedStatus:   http.StatusForbidden,
		},
		{
			name:               "preflight requesting authorization is allowed",
			method:             http.MethodOptions,
			origin:             "https://example.com",
			preflightHeaders:   "content-type, AUTHORIZATION",
			expectedStatus:     http.StatusOK,
			expectWildcardCORS: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			innerCalled := false
			inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				innerCalled = true
				w.WriteHeader(http.StatusNoContent)
			})
			handler := middleware.StaticAuthBrowserGuard(middleware.SetCorsHeaders(inner))

			req := httptest.NewRequest(tt.method, "/", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			if tt.preflightHeaders != "" {
				req.Header.Set("Access-Control-Request-Headers", tt.preflightHeaders)
			}
			if tt.withUpstreamToken {
				req = req.WithContext(ghcontext.WithTokenInfo(req.Context(), &ghcontext.TokenInfo{
					Token:     "gho_upstreamtoken",
					TokenType: utils.TokenTypeOAuthAccessToken,
				}))
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			assert.Equal(t, tt.expectInnerCalled, innerCalled)
			if tt.expectWildcardCORS {
				assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}
