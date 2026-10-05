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

// assertCanonicalCORS asserts the CORS and cache-negotiation headers the
// discovery spec mandates on every card response (errors, preflight, and 304s).
func assertCanonicalCORS(t *testing.T, res *http.Response) {
	t.Helper()
	h := res.Header
	assert.Equal(t, "*", h.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET", h.Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, If-None-Match", h.Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "ETag", h.Get("Access-Control-Expose-Headers"))
	assert.Equal(t, "Accept, X-Forwarded-Host", h.Get("Vary"))
}

// assertStrongETag asserts a quoted, strong SHA-256 ETag and returns it.
func assertStrongETag(t *testing.T, res *http.Response) string {
	t.Helper()
	etag := res.Header.Get("ETag")
	assert.True(t, strings.HasPrefix(etag, `"`) && strings.HasSuffix(etag, `"`), "ETag must be a quoted strong tag, got %q", etag)
	assert.Len(t, etag, 66, "ETag should wrap a 64-char hex SHA-256 in quotes")
	return etag
}

func TestHandlerServeHTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		method         string
		accept         string
		expectedStatus int
		expectBody     bool
	}{
		{
			name:           "GET returns the card",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			expectBody:     true,
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
			assertCanonicalCORS(t, res)

			if tc.expectedStatus == http.StatusOK && tc.method != http.MethodOptions {
				assert.Equal(t, MediaType, res.Header.Get(headers.ContentTypeHeader))
				assertStrongETag(t, res)
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

// TestHandlerMultiLineAccept guards the list-valued-header fix: the handler joins
// Accept's repeated field-lines and negotiates them as one list (Header.Get would
// read only the first line and 406).
func TestHandlerMultiLineAccept(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, Path, nil)
	req.Header.Add(headers.AcceptHeader, "text/html")
	req.Header.Add(headers.AcceptHeader, MediaType)
	rec := httptest.NewRecorder()

	NewHandler(Config{}).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Result().StatusCode)
}

// TestAcceptsCard exercises RFC 9110 Accept negotiation directly, covering quoted
// commas/escapes, pre-q media parameters, post-q extensions, and specificity/q=0.
func TestAcceptsCard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		accept string
		want   bool
	}{
		{name: "empty accepts anything", accept: "", want: true},
		{name: "exact type", accept: MediaType, want: true},
		{name: "exact type refused", accept: MediaType + ";q=0", want: false},
		{name: "full wildcard", accept: "*/*", want: true},
		{name: "full wildcard refused", accept: "*/*;q=0", want: false},
		{name: "application wildcard", accept: "application/*", want: true},
		{name: "application wildcard refused", accept: "application/*;q=0", want: false},
		{name: "unrelated type", accept: "text/html", want: false},
		{name: "quoted comma in refused param", accept: `application/mcp-server-card+json;note="a,b";q=0`, want: false},
		{name: "escaped quote in refused param", accept: `application/mcp-server-card+json;note="a\",b";q=0`, want: false},
		{name: "pre-q param falls through to wildcard", accept: "application/mcp-server-card+json;profile=x;q=0, */*;q=1", want: true},
		{name: "pre-q param without fallback does not match", accept: "application/mcp-server-card+json;profile=x", want: false},
		{name: "post-q extension is ignored", accept: "application/mcp-server-card+json;q=1;ext=foo", want: true},
		{name: "post-q extension on a refused range", accept: "application/mcp-server-card+json;q=0;ext=foo", want: false},
		{name: "most specific refusal wins over broad accept", accept: "application/mcp-server-card+json;q=0, */*;q=1", want: false},
		{name: "specific accept wins over broad refusal", accept: "application/mcp-server-card+json, */*;q=0", want: true},
		{name: "quoted wildcard param does not spoof a match", accept: `application/mcp-server-card+json;x="*/*";q=0`, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, acceptsCard(tc.accept))
		})
	}
}

// TestHandlerVaryComposition asserts the handler appends Accept to Vary without
// discarding values a deployment's middleware set earlier in the chain.
func TestHandlerVaryComposition(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, Path, nil)
	rec := httptest.NewRecorder()
	rec.Header().Set("Vary", "Origin")

	NewHandler(Config{}).ServeHTTP(rec, req)

	assert.Equal(t, []string{"Origin", "Accept, X-Forwarded-Host"}, rec.Result().Header.Values("Vary"))
}

func TestHandlerRegisterRoutes(t *testing.T) {
	t.Parallel()

	// The remote mounts the MCP endpoint as a catch-all at "/" (mirroring
	// pkg/http/handler.go: r.Mount("/", h)). The card's static route must take
	// precedence so the card — not the auth-gated MCP endpoint — answers its
	// single canonical path, with no alternate (e.g. /mcp/server-card).
	const mcpStatus = http.StatusUnauthorized // sentinel for the auth-gated MCP catch-all

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Mount("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(mcpStatus)
		}))
	})
	r.Group(func(r chi.Router) {
		NewHandler(Config{}).RegisterRoutes(r)
	})

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantCard   bool
	}{
		{name: "canonical GET is served by the card, not the catch-all", method: http.MethodGet, path: Path, wantStatus: http.StatusOK, wantCard: true},
		{name: "non-GET at the card path is the card's 405, not the catch-all", method: http.MethodPost, path: Path, wantStatus: http.StatusMethodNotAllowed},
		{name: "no alternate path is registered", method: http.MethodGet, path: "/mcp" + Path, wantStatus: mcpStatus},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			res := rec.Result()
			defer res.Body.Close()

			assert.Equal(t, tc.wantStatus, res.StatusCode)
			if tc.wantCard {
				assert.Equal(t, MediaType, res.Header.Get(headers.ContentTypeHeader))
			}
		})
	}
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

	// Baseline GET yields a quoted strong ETag; every table case re-asserts it, proving stability.
	res := get(t, "")
	etag := assertStrongETag(t, res)
	res.Body.Close()

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
		{name: "list with backslash in tag", ifNoneMatch: `"o\", ` + etag, expectedStatus: http.StatusNotModified, expectBody: false},
		{name: "non-matching tag", ifNoneMatch: `"deadbeef"`, expectedStatus: http.StatusOK, expectBody: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res := get(t, tc.ifNoneMatch)
			defer res.Body.Close()

			assert.Equal(t, tc.expectedStatus, res.StatusCode)
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

	fetch := func(host string) (string, string) {
		req := httptest.NewRequest(http.MethodGet, Path, nil)
		req.Host = host
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		res := rec.Result()
		defer res.Body.Close()
		var card ServerCard
		require.NoError(t, json.NewDecoder(res.Body).Decode(&card))
		require.Len(t, card.Remotes, 1)
		return card.Remotes[0].URL, res.Header.Get("ETag")
	}

	urlA, etagA := fetch("tenant-a.example.test")
	urlB, etagB := fetch("tenant-b.example.test")

	assert.Equal(t, "https://tenant-a.example.test/mcp/", urlA)
	assert.Equal(t, "https://tenant-b.example.test/mcp/", urlB)
	assert.NotEqual(t, etagA, etagB, "different per-tenant bodies must yield different ETags")
}
