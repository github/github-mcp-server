package servercard

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

				etag := res.Header.Get("ETag")
				assert.True(t, strings.HasPrefix(etag, `"`) && strings.HasSuffix(etag, `"`), "ETag must be a quoted strong tag, got %q", etag)
				assert.Len(t, etag, 66, "ETag should wrap a 64-char hex SHA-256 in quotes")
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

	t.Run("GET serves the card at the canonical path", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, Path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Equal(t, MediaType, res.Header.Get(headers.ContentTypeHeader))
	})

	t.Run("POST owned by card handler", func(t *testing.T) {
		t.Parallel()

		// The handler is registered for all methods so non-GET requests are
		// answered here (405) rather than falling through to another route.
		req := httptest.NewRequest(http.MethodPost, Path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		assert.Equal(t, http.StatusMethodNotAllowed, res.StatusCode)
		assert.Equal(t, "*", res.Header.Get("Access-Control-Allow-Origin"))
	})

	t.Run("card is served at exactly one path", func(t *testing.T) {
		t.Parallel()

		// The card must be discoverable at a single canonical location only;
		// no alternate path (e.g. /mcp/server-card) is registered.
		req := httptest.NewRequest(http.MethodGet, "/mcp"+Path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		assert.Equal(t, http.StatusNotFound, res.StatusCode)
	})
}

// TestHandlerRegisterRoutesNotShadowedByCatchAll mirrors the production wiring
// (pkg/http/server.go), where the streamable MCP endpoint is mounted as a
// catch-all at "/" (pkg/http/handler.go: r.Mount("/", h)). The card's static
// route must take precedence over that wildcard mount so the card — and not the
// auth-gated MCP endpoint — answers GET and non-GET requests at the card path.
func TestHandlerRegisterRoutesNotShadowedByCatchAll(t *testing.T) {
	t.Parallel()

	const mcpStatus = http.StatusUnauthorized // sentinel for the auth-gated MCP endpoint

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Mount("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(mcpStatus)
		}))
	})
	r.Group(func(r chi.Router) {
		NewHandler(Config{}).RegisterRoutes(r)
	})

	t.Run("GET is owned by the card handler", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, Path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Equal(t, MediaType, res.Header.Get(headers.ContentTypeHeader))
	})

	t.Run("non-GET is owned by the card handler, not the catch-all", func(t *testing.T) {
		t.Parallel()

		// A POST must get the card handler's 405, not the MCP catch-all's
		// sentinel status — proving the static route is not shadowed.
		req := httptest.NewRequest(http.MethodPost, Path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		assert.Equal(t, http.StatusMethodNotAllowed, res.StatusCode)
		assert.NotEqual(t, mcpStatus, res.StatusCode)
	})
}

func TestHandlerETagConditionalRequests(t *testing.T) {
	t.Parallel()

	handler := NewHandler(Config{Version: "1.2.3"})

	get := func(t *testing.T, ifNoneMatch string) *http.Response {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, Path, nil)
		if ifNoneMatch != "" {
			req.Header.Set("If-None-Match", ifNoneMatch)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Result()
	}

	// Baseline GET yields a quoted strong ETag.
	res := get(t, "")
	etag := res.Header.Get("ETag")
	res.Body.Close()
	require.NotEmpty(t, etag)
	assert.True(t, strings.HasPrefix(etag, `"`) && strings.HasSuffix(etag, `"`))
	assert.NotContains(t, etag, "W/", "served ETag must be strong")

	t.Run("ETag is stable across calls", func(t *testing.T) {
		t.Parallel()
		second := get(t, "")
		defer second.Body.Close()
		assert.Equal(t, etag, second.Header.Get("ETag"))
	})

	tests := []struct {
		name           string
		ifNoneMatch    string
		expectedStatus int
		expectBody     bool
	}{
		{name: "matching strong tag", ifNoneMatch: etag, expectedStatus: http.StatusNotModified, expectBody: false},
		{name: "matching weak form", ifNoneMatch: "W/" + etag, expectedStatus: http.StatusNotModified, expectBody: false},
		{name: "wildcard", ifNoneMatch: "*", expectedStatus: http.StatusNotModified, expectBody: false},
		{name: "within a list", ifNoneMatch: `"other", ` + etag, expectedStatus: http.StatusNotModified, expectBody: false},
		{name: "non-matching tag", ifNoneMatch: `"deadbeef"`, expectedStatus: http.StatusOK, expectBody: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res := get(t, tc.ifNoneMatch)
			defer res.Body.Close()

			assert.Equal(t, tc.expectedStatus, res.StatusCode)
			// ETag and Cache-Control accompany both 200 and 304 responses.
			assert.Equal(t, etag, res.Header.Get("ETag"))
			assert.Equal(t, "public, max-age=3600", res.Header.Get("Cache-Control"))

			body, err := io.ReadAll(res.Body)
			require.NoError(t, err)
			if tc.expectBody {
				assert.NotEmpty(t, body)
				assert.Equal(t, MediaType, res.Header.Get(headers.ContentTypeHeader))
			} else {
				assert.Empty(t, body, "304 must have an empty body")
				assert.Empty(t, res.Header.Get(headers.ContentTypeHeader), "304 should not carry Content-Type")
			}
		})
	}
}

func TestHandlerRemoteURLFunc(t *testing.T) {
	t.Parallel()

	// Simulate a multi-tenant deployment deriving the remote URL per request.
	handler := NewHandler(Config{
		Version: "1.2.3",
		RemoteURLFunc: func(r *http.Request) string {
			return "https://" + r.Host + "/mcp/"
		},
	})

	makeReq := func(host string) *http.Response {
		req := httptest.NewRequest(http.MethodGet, Path, nil)
		req.Host = host
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Result()
	}

	resA := makeReq("tenant-a.example.test")
	var cardA ServerCard
	require.NoError(t, json.NewDecoder(resA.Body).Decode(&cardA))
	etagA := resA.Header.Get("ETag")
	resA.Body.Close()

	resB := makeReq("tenant-b.example.test")
	var cardB ServerCard
	require.NoError(t, json.NewDecoder(resB.Body).Decode(&cardB))
	etagB := resB.Header.Get("ETag")
	resB.Body.Close()

	require.Len(t, cardA.Remotes, 1)
	require.Len(t, cardB.Remotes, 1)
	assert.Equal(t, "https://tenant-a.example.test/mcp/", cardA.Remotes[0].URL)
	assert.Equal(t, "https://tenant-b.example.test/mcp/", cardB.Remotes[0].URL)
	assert.NotEqual(t, etagA, etagB, "different per-tenant bodies must yield different ETags")
}

func TestServeCardWritesCanonicalResponse(t *testing.T) {
	t.Parallel()

	// ServeCard is the reusable writer remotes call with a pre-built card.
	card := NewServerCard(Config{Version: "9.9.9", RemoteURL: "https://api.example.test/mcp/"})

	req := httptest.NewRequest(http.MethodGet, Path, nil)
	rec := httptest.NewRecorder()
	ServeCard(rec, req, card)

	res := rec.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, MediaType, res.Header.Get(headers.ContentTypeHeader))
	assert.Equal(t, "public, max-age=3600", res.Header.Get("Cache-Control"))
	assert.Equal(t, "*", res.Header.Get("Access-Control-Allow-Origin"))
	assert.NotEmpty(t, res.Header.Get("ETag"))

	var decoded ServerCard
	require.NoError(t, json.NewDecoder(res.Body).Decode(&decoded))
	assert.Equal(t, "9.9.9", decoded.Version)
	require.Len(t, decoded.Remotes, 1)
	assert.Equal(t, "https://api.example.test/mcp/", decoded.Remotes[0].URL)
}
