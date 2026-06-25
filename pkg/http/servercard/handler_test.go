package servercard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerServeHTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		method              string
		accept              string
		expectedStatus      int
		expectedContentType string
		expectBody          bool
	}{
		{
			name:                "GET returns the card",
			method:              http.MethodGet,
			expectedStatus:      http.StatusOK,
			expectedContentType: MediaType,
			expectBody:          true,
		},
		{
			name:                "GET with card media type Accept",
			method:              http.MethodGet,
			accept:              MediaType,
			expectedStatus:      http.StatusOK,
			expectedContentType: MediaType,
			expectBody:          true,
		},
		{
			name:                "GET with wildcard Accept",
			method:              http.MethodGet,
			accept:              "*/*",
			expectedStatus:      http.StatusOK,
			expectedContentType: MediaType,
			expectBody:          true,
		},
		{
			name:                "GET with Accept list including the card type",
			method:              http.MethodGet,
			accept:              "text/html, application/mcp-server-card+json;q=0.9",
			expectedStatus:      http.StatusOK,
			expectedContentType: MediaType,
			expectBody:          true,
		},
		{
			name:           "GET with incompatible Accept is rejected",
			method:         http.MethodGet,
			accept:         "text/html",
			expectedStatus: http.StatusNotAcceptable,
		},
		{
			name:           "HEAD returns headers without body",
			method:         http.MethodHead,
			expectedStatus: http.StatusOK,
			expectBody:     false,
		},
		{
			name:           "OPTIONS preflight",
			method:         http.MethodOptions,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST is not allowed",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := NewHandler(Config{Version: "1.2.3"})
			req := httptest.NewRequest(tc.method, Path, nil)
			if tc.accept != "" {
				req.Header.Set(headers.AcceptHeader, tc.accept)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)
			res := rec.Result()
			defer res.Body.Close()

			assert.Equal(t, tc.expectedStatus, res.StatusCode)

			// CORS headers are always present, even on errors and preflight.
			assert.Equal(t, "*", res.Header.Get("Access-Control-Allow-Origin"))
			assert.Equal(t, "GET", res.Header.Get("Access-Control-Allow-Methods"))
			assert.Equal(t, "Content-Type", res.Header.Get("Access-Control-Allow-Headers"))

			if tc.expectedStatus == http.StatusOK && tc.method != http.MethodOptions {
				assert.Equal(t, MediaType, res.Header.Get(headers.ContentTypeHeader))
				assert.Equal(t, "public, max-age=3600", res.Header.Get("Cache-Control"))
			}

			if tc.expectBody {
				var card ServerCard
				require.NoError(t, json.NewDecoder(res.Body).Decode(&card))
				assert.Equal(t, SchemaURL, card.Schema)
				assert.Equal(t, "1.2.3", card.Version)
				require.Len(t, card.Remotes, 1)
				assert.Equal(t, DefaultRemoteURL, card.Remotes[0].URL)
			}
		})
	}
}

func TestHandlerRegisterRoutes(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()
	NewHandler(Config{}).RegisterRoutes(r)

	for _, path := range []string{Path, "/mcp" + Path} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			assert.Equal(t, http.StatusOK, res.StatusCode)
			assert.Equal(t, MediaType, res.Header.Get(headers.ContentTypeHeader))
		})

		t.Run(path+" POST owned by card handler", func(t *testing.T) {
			t.Parallel()

			// The handler is registered for all methods so non-GET requests are
			// answered here (405) rather than falling through to another route.
			req := httptest.NewRequest(http.MethodPost, path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			assert.Equal(t, http.StatusMethodNotAllowed, res.StatusCode)
			assert.Equal(t, "*", res.Header.Get("Access-Control-Allow-Origin"))
		})
	}
}
