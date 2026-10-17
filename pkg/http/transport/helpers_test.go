package transport

import (
	"net/http"
	"testing"
)

// newIsolatedTransport returns an http.Transport owned by a single test.
//
// Sharing http.DefaultTransport across parallel tests is unsafe: closing one
// test's httptest.Server also closes DefaultTransport's idle connections,
// breaking other tests still using it. Tests asserting DefaultTransport
// fallback behavior specifically must use it directly and not run in parallel.
func newIsolatedTransport(t *testing.T) *http.Transport {
	t.Helper()

	transport := &http.Transport{}
	t.Cleanup(transport.CloseIdleConnections)
	return transport
}
