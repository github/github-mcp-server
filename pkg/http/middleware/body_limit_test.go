package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unknownLengthBody hides Len from httptest.NewRequest so ContentLength is -1,
// as it is for a chunked or HTTP/2 request.
func unknownLengthBody(s string) io.Reader {
	return io.NopCloser(strings.NewReader(s))
}

func TestWithMaxBodySize(t *testing.T) {
	const limit = 16

	t.Run("allowed request under the limit passes through", func(t *testing.T) {
		var nextCalled bool
		var readBody string
		var readErr error

		next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			nextCalled = true
			b, err := io.ReadAll(r.Body)
			readBody, readErr = string(b), err
		})

		handler := WithMaxBodySize(limit)(next)

		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader("short"))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.True(t, nextCalled, "next handler should be called for an allowed request")
		require.NoError(t, readErr)
		assert.Equal(t, "short", readBody)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("boundary size exactly at the limit is allowed", func(t *testing.T) {
		body := strings.Repeat("a", limit)

		var nextCalled bool
		var readBody string
		var readErr error

		next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			nextCalled = true
			b, err := io.ReadAll(r.Body)
			readBody, readErr = string(b), err
		})

		handler := WithMaxBodySize(limit)(next)

		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.True(t, nextCalled, "next handler should be called when the body is exactly at the limit")
		require.NoError(t, readErr, "reading exactly maxBytes should not error")
		assert.Equal(t, body, readBody, "the full boundary-size body should be readable")
	})

	t.Run("oversized request with known Content-Length is rejected before next runs", func(t *testing.T) {
		body := strings.Repeat("a", limit+1)

		var nextCalled bool
		next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			nextCalled = true
		})

		handler := WithMaxBodySize(limit)(next)

		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
		require.Equal(t, int64(limit+1), req.ContentLength, "test setup: Content-Length should be known")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.False(t, nextCalled, "next handler must not run for an oversized request")
		assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
		assert.Contains(t, rr.Body.String(), "request body too large")
	})

	t.Run("oversized request with unknown length fails on downstream read", func(t *testing.T) {
		body := strings.Repeat("a", limit+1)

		var nextCalled bool
		var readErr error

		next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			nextCalled = true
			_, readErr = io.ReadAll(r.Body)
		})

		handler := WithMaxBodySize(limit)(next)

		req := httptest.NewRequest(http.MethodPost, "/mcp", unknownLengthBody(body))
		require.Equal(t, int64(-1), req.ContentLength, "test setup: Content-Length should be unknown")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.True(t, nextCalled, "next handler still runs; the limit is enforced on read")
		require.Error(t, readErr)
		assert.True(t, isMaxBytesError(readErr), "expected a *http.MaxBytesError, got %v", readErr)
	})
}
