package transport

import (
	"net/http"
	"testing"
)

// newIsolatedTransport returns an http.Transport owned by a single test.
//
// httptest.Server.Close closes the idle connections of the process-global
// http.DefaultTransport, regardless of which server is being shut down. Tests
// here run in parallel and each shut down a server, so sharing
// http.DefaultTransport lets one test's cleanup break another test's request
// with "http: CloseIdleConnections called". Giving every test its own
// transport keeps that global side effect out of reach.
//
// Use this wherever a test just needs a working transport. Tests that assert
// behaviour specific to http.DefaultTransport must use it directly and must
// not run in parallel.
func newIsolatedTransport(t *testing.T) *http.Transport {
	t.Helper()

	transport := &http.Transport{}
	t.Cleanup(transport.CloseIdleConnections)
	return transport
}
