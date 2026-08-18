package transport

import (
	"bytes"
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type countingReadCloser struct {
	reader    io.Reader
	bytesRead int
}

func (c *countingReadCloser) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	c.bytesRead += n
	return n, err
}

func (c *countingReadCloser) Close() error {
	return nil
}

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

// TestETagTransport_SkipsBodiesOverEntryByteBudget verifies a response whose
// body exceeds the per-entry byte budget is served but never cached, so a
// subsequent request is not revalidated against a stored ETag.
func TestETagTransport_SkipsBodiesOverEntryByteBudget(t *testing.T) {
	t.Parallel()

	const etag = `"big"`
	body := make([]byte, 64)

	var lastIfNoneMatch string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastIfNoneMatch = r.Header.Get(headers.IfNoneMatchHeader)
		w.Header().Set(headers.ETagHeader, etag)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	rt := &ETagTransport{Transport: http.DefaultTransport, MaxEntryBytes: 16}

	do := func() []byte {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
		require.NoError(t, err)
		resp, err := rt.RoundTrip(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return data
	}

	assert.Equal(t, body, do())
	assert.Equal(t, body, do(), "oversized body is served in full each time")
	assert.Empty(t, lastIfNoneMatch, "an oversized response must not be cached or revalidated")
}

func TestETagTransport_StreamsOversizedUnknownLengthBody(t *testing.T) {
	t.Parallel()

	const maxEntry = 16
	body := []byte("0123456789abcdef-streamed-remainder")
	stream := &countingReadCloser{reader: bytes.NewReader(body)}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		assert.Empty(t, req.Header.Get(headers.IfNoneMatchHeader), "an oversized response must not be cached or revalidated")
		return &http.Response{
			StatusCode:    http.StatusOK,
			Status:        "200 OK",
			Header:        http.Header{headers.ETagHeader: []string{`"streamed"`}},
			Body:          stream,
			ContentLength: -1,
			Request:       req,
		}, nil
	})
	rt := &ETagTransport{Transport: transport, MaxEntryBytes: maxEntry}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/stream", nil)
	require.NoError(t, err)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	require.LessOrEqual(t, stream.bytesRead, maxEntry+1, "RoundTrip must not read the entire oversized body before returning")

	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, body, data, "caller receives the buffered prefix plus the streamed remainder")

	stream = &countingReadCloser{reader: bytes.NewReader(body)}
	resp, err = rt.RoundTrip(req)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	require.NoError(t, resp.Body.Close())
}

// TestETagTransport_EvictsByTotalByteBudget verifies that inserting a second
// entry that pushes the cache over its total-byte budget evicts the
// least-recently-used entry, which is then re-fetched in full.
func TestETagTransport_EvictsByTotalByteBudget(t *testing.T) {
	t.Parallel()

	body := make([]byte, 10)
	ifNoneMatch := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ifNoneMatch[r.URL.Path] = r.Header.Get(headers.IfNoneMatchHeader)
		w.Header().Set(headers.ETagHeader, `"`+r.URL.Path+`"`)
		if r.Header.Get(headers.IfNoneMatchHeader) != "" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	// Budget holds a single 10-byte entry; a second entry evicts the first.
	rt := &ETagTransport{Transport: http.DefaultTransport, MaxTotalBytes: 15}

	get := func(path string) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+path, nil)
		require.NoError(t, err)
		resp, err := rt.RoundTrip(req)
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	get("/a") // cache A
	get("/b") // cache B, evicts A
	get("/a") // A was evicted -> full GET, no conditional header

	assert.Empty(t, ifNoneMatch["/a"], "A must be re-fetched in full after eviction")
}

// TestETagTransport_DoesNotCacheNoStoreResponse verifies a response marked
// Cache-Control: no-store is never retained.
func TestETagTransport_DoesNotCacheNoStoreResponse(t *testing.T) {
	t.Parallel()

	var lastIfNoneMatch string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastIfNoneMatch = r.Header.Get(headers.IfNoneMatchHeader)
		w.Header().Set(headers.ETagHeader, `"ns"`)
		w.Header().Set(headers.CacheControlHeader, "private, no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "secret")
	}))
	defer server.Close()

	rt := &ETagTransport{Transport: http.DefaultTransport}

	do := func() {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
		require.NoError(t, err)
		resp, err := rt.RoundTrip(req)
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	do()
	do()
	assert.Empty(t, lastIfNoneMatch, "a no-store response must not be cached or revalidated")
}

// TestETagTransport_DoesNotCacheVaryStar verifies a response with Vary: * is
// never retained.
func TestETagTransport_DoesNotCacheVaryStar(t *testing.T) {
	t.Parallel()

	var lastIfNoneMatch string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastIfNoneMatch = r.Header.Get(headers.IfNoneMatchHeader)
		w.Header().Set(headers.ETagHeader, `"vary"`)
		w.Header().Set(headers.VaryHeader, "*")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "body")
	}))
	defer server.Close()

	rt := &ETagTransport{Transport: http.DefaultTransport}

	do := func() {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
		require.NoError(t, err)
		resp, err := rt.RoundTrip(req)
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	do()
	do()
	assert.Empty(t, lastIfNoneMatch, "a Vary:* response must not be cached or revalidated")
}

// TestETagTransport_RequestNoStoreBypassesCache verifies a client request that
// sends Cache-Control: no-store neither reads from nor revalidates against the
// cache, even after a prior response was cached.
func TestETagTransport_RequestNoStoreBypassesCache(t *testing.T) {
	t.Parallel()

	var lastIfNoneMatch string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastIfNoneMatch = r.Header.Get(headers.IfNoneMatchHeader)
		w.Header().Set(headers.ETagHeader, `"x"`)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "body")
	}))
	defer server.Close()

	rt := &ETagTransport{Transport: http.DefaultTransport}

	// Prime the cache with a normal GET.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	// A no-store request must not send If-None-Match.
	req2, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	req2.Header.Set(headers.CacheControlHeader, "no-store")
	resp2, err := rt.RoundTrip(req2)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp2.Body)
	resp2.Body.Close()

	assert.Empty(t, lastIfNoneMatch, "a no-store request must bypass the cache")
}
