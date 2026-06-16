package oauth

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serveCallback drives the callback handler with the given query string and
// returns the recorded response and the single reported result.
func serveCallback(t *testing.T, expectedState, query string) (*httptest.ResponseRecorder, callbackResult) {
	t.Helper()
	cs := &callbackServer{results: make(chan callbackResult, 1)}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/callback?"+query, nil)

	cs.handler(expectedState).ServeHTTP(rec, req)

	select {
	case res := <-cs.results:
		return rec, res
	default:
		t.Fatal("handler did not report a result")
		return nil, callbackResult{}
	}
}

func TestCallbackHandlerSuccess(t *testing.T) {
	rec, res := serveCallback(t, "state123", "code=the-code&state=state123")

	require.NoError(t, res.err)
	assert.Equal(t, "the-code", res.code)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Authorization Successful")
}

func TestCallbackHandlerStateMismatch(t *testing.T) {
	rec, res := serveCallback(t, "expected", "code=the-code&state=attacker")

	require.Error(t, res.err)
	assert.Empty(t, res.code)
	assert.Contains(t, res.err.Error(), "state mismatch")
	assert.Contains(t, rec.Body.String(), "state mismatch")
}

func TestCallbackHandlerMissingCode(t *testing.T) {
	_, res := serveCallback(t, "state123", "state=state123")

	require.Error(t, res.err)
	assert.Contains(t, res.err.Error(), "no authorization code")
}

func TestCallbackHandlerOAuthError(t *testing.T) {
	_, res := serveCallback(t, "state123", "error=access_denied&error_description=user+said+no")

	require.Error(t, res.err)
	assert.Contains(t, res.err.Error(), "access_denied")
	assert.Contains(t, res.err.Error(), "user said no")
}

func TestCallbackHandlerEscapesError(t *testing.T) {
	rec, _ := serveCallback(t, "state123", "error=evil&error_description=%3Cscript%3Ealert(1)%3C%2Fscript%3E")

	body := rec.Body.String()
	assert.NotContains(t, body, "<script>", "error message must be HTML-escaped")
	assert.Contains(t, body, "&lt;script&gt;")
}

func TestListenCallbackRandomPortIsLoopback(t *testing.T) {
	listener, err := listenCallback(0)
	require.NoError(t, err)
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	assert.True(t, addr.IP.IsLoopback(), "random port must bind loopback only, got %s", addr.IP)
	assert.NotZero(t, addr.Port)
}
