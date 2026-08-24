package http

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/github/github-mcp-server/pkg/http/middleware"
	"github.com/github/github-mcp-server/pkg/http/oauth"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCORSTestServer builds the same top-level router layout as RunHTTPServer
// (MCP routes + OAuth metadata routes under a shared CORS middleware) without
// binding a real port, so tests can assert on CORS behavior for every response
// class: MCP auth challenges (401), OAuth metadata (200), and unmatched paths
// (404).
func newCORSTestServer(t *testing.T) http.Handler {
	t.Helper()

	dotcomHost, err := utils.NewAPIHost("https://api.github.com")
	require.NoError(t, err)

	handler := NewHTTPMcpHandler(
		context.Background(),
		&ServerConfig{Version: "test"},
		nil, // deps not exercised by these routes
		translations.NullTranslationHelper,
		slog.Default(),
		dotcomHost,
	)

	oauthHandler, err := oauth.NewAuthHandler(&oauth.Config{
		BaseURL: "https://api.example.com",
	}, dotcomHost)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(middleware.SetCorsHeaders)

	r.Group(func(r chi.Router) {
		handler.RegisterMiddleware(r)
		handler.RegisterRoutes(r)
	})
	r.Group(func(r chi.Router) {
		oauthHandler.RegisterRoutes(r)
	})
	return r
}

func TestTopLevelCORSHeadersOnAllResponses(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		path         string
		wantStatus   int
		wantACAO     string // Access-Control-Allow-Origin
		wantExposeWW bool   // WWW-Authenticate must be exposed via Access-Control-Expose-Headers
	}{
		{
			name:         "MCP endpoint without token returns 401 challenge with CORS headers",
			method:       http.MethodPost,
			path:         "/mcp",
			wantStatus:   http.StatusUnauthorized,
			wantACAO:     "*",
			wantExposeWW: true,
		},
		{
			name:         "OAuth protected resource metadata returns 200 with CORS headers",
			method:       http.MethodGet,
			path:         "/.well-known/oauth-protected-resource",
			wantStatus:   http.StatusOK,
			wantACAO:     "*",
			wantExposeWW: false,
		},
		{
			name:         "unmatched path falls through to root-mounted MCP handler's 401 with CORS headers",
			method:       http.MethodGet,
			path:         "/definitely-not-a-route",
			wantStatus:   http.StatusUnauthorized,
			wantACAO:     "*",
			wantExposeWW: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newCORSTestServer(t)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()
			srv.ServeHTTP(rr, req)

			resp := rr.Result()
			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			assert.Equal(t, tt.wantACAO, resp.Header.Get("Access-Control-Allow-Origin"))
			if tt.wantExposeWW {
				assert.NotEmpty(t, resp.Header.Get("WWW-Authenticate"),
					"401 challenge should carry WWW-Authenticate per MCP spec")
				assert.Contains(t, resp.Header.Get("Access-Control-Expose-Headers"), "WWW-Authenticate",
					"WWW-Authenticate must be readable cross-origin")
			}
		})
	}
}

func TestTopLevelCORSPreflightOnMetadataRoute(t *testing.T) {
	srv := newCORSTestServer(t)

	req := httptest.NewRequest(http.MethodOptions, "/.well-known/oauth-protected-resource", nil)
	req.Header.Set("Origin", "https://client.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	resp := rr.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"preflight against the metadata route should short-circuit with 200, not fall through to 405/404")
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	assert.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "GET")
}
