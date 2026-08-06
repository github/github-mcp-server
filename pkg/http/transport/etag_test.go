package transport

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/github/github-mcp-server/pkg/http/headers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestETagTransport_ServesCachedBodyOn304 verifies the core conditional-request
// flow: the first GET carries no If-None-Match and is cached with its ETag; the
// second GET sends the cached ETag and, on a 304 Not Modified, is served the
// cached body instead of the empty 304 body.
func TestETagTransport_ServesCachedBodyOn304(t *testing.T) {
	t.Parallel()

	const etag = `"abc123"`
	const body = `{"number":1}`

	var requests int32
	var lastIfNoneMatch string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requests, 1)
		lastIfNoneMatch = r.Header.Get(headers.IfNoneMatchHeader)
		w.Header().Set(headers.ETagHeader, etag)
		if n == 1 {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, body)
			return
		}
		// Second request revalidates and is unchanged.
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	rt := &ETagTransport{Transport: http.DefaultTransport}

	do := func() (int, string) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
		require.NoError(t, err)
		resp, err := rt.RoundTrip(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp.StatusCode, string(data)
	}

	status1, body1 := do()
	assert.Equal(t, http.StatusOK, status1)
	assert.Equal(t, body, body1)
	assert.Empty(t, lastIfNoneMatch, "first request must not send If-None-Match")

	status2, body2 := do()
	assert.Equal(t, http.StatusOK, status2, "304 is translated to the cached 200")
	assert.Equal(t, body, body2, "cached body is served on 304")
	assert.Equal(t, etag, lastIfNoneMatch, "second request sends the cached ETag")
	assert.Equal(t, int32(2), atomic.LoadInt32(&requests), "every request still reaches the server")
}

// TestETagTransport_UpdatesRateLimitHeadersFrom304 verifies that a cache-served
// response surfaces the live 304 response's rate-limit headers rather than the
// stale headers captured with the cached body.
func TestETagTransport_UpdatesRateLimitHeadersFrom304(t *testing.T) {
	t.Parallel()

	const etag = `"v1"`

	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&requests, 1)
		w.Header().Set(headers.ETagHeader, etag)
		if n == 1 {
			w.Header().Set("X-RateLimit-Remaining", "100")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "cached")
			return
		}
		w.Header().Set("X-RateLimit-Remaining", "99")
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	rt := &ETagTransport{Transport: http.DefaultTransport}

	do := func() http.Header {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
		require.NoError(t, err)
		resp, err := rt.RoundTrip(req)
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return resp.Header
	}

	h1 := do()
	assert.Equal(t, "100", h1.Get("X-RateLimit-Remaining"))

	h2 := do()
	assert.Equal(t, "99", h2.Get("X-RateLimit-Remaining"), "rate-limit headers come from the live 304")
}

// TestETagTransport_ScopesCacheByAuthorization verifies cached bodies are not
// shared across tokens.
func TestETagTransport_ScopesCacheByAuthorization(t *testing.T) {
	t.Parallel()

	const etag = `"shared-url"`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(headers.ETagHeader, etag)
		if r.Header.Get(headers.IfNoneMatchHeader) != "" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, r.Header.Get(headers.AuthorizationHeader))
	}))
	defer server.Close()

	rt := &ETagTransport{Transport: http.DefaultTransport}

	get := func(auth string) string {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
		require.NoError(t, err)
		req.Header.Set(headers.AuthorizationHeader, auth)
		resp, err := rt.RoundTrip(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return string(data)
	}

	assert.Equal(t, "Bearer a", get("Bearer a"))
	assert.Equal(t, "Bearer b", get("Bearer b"), "a different token must not receive the other token's cached body")
	assert.Equal(t, "Bearer a", get("Bearer a"), "the first token's cached body is served on revalidation")
}

// TestETagTransport_OnlyCachesGET verifies non-GET requests bypass the cache and
// are never sent a conditional header.
func TestETagTransport_OnlyCachesGET(t *testing.T) {
	t.Parallel()

	var sawIfNoneMatch bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(headers.IfNoneMatchHeader) != "" {
			sawIfNoneMatch = true
		}
		w.Header().Set(headers.ETagHeader, `"x"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	rt := &ETagTransport{Transport: http.DefaultTransport}

	do := func() {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL, nil)
		require.NoError(t, err)
		resp, err := rt.RoundTrip(req)
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	do()
	do()
	assert.False(t, sawIfNoneMatch, "POST requests must not be revalidated")
}

// TestETagTransport_DoesNotOverrideCallerConditional verifies a caller-supplied
// If-None-Match header is preserved and not replaced by the cache.
func TestETagTransport_DoesNotOverrideCallerConditional(t *testing.T) {
	t.Parallel()

	var gotIfNoneMatch string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIfNoneMatch = r.Header.Get(headers.IfNoneMatchHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	rt := &ETagTransport{Transport: http.DefaultTransport}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	req.Header.Set(headers.IfNoneMatchHeader, `"caller"`)

	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, `"caller"`, gotIfNoneMatch)
}
