package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithRequestConfig_PreservesProvidedRequestID(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(headers.RequestIDHeader, "client-request-id")

	var requestID string
	handler := WithRequestConfig(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		var ok bool
		requestID, ok = ghcontext.RequestID(r.Context())
		require.True(t, ok)
	}))

	handler.ServeHTTP(recorder, request)

	assert.Equal(t, "client-request-id", requestID)
	assert.Equal(t, "client-request-id", recorder.Header().Get(headers.RequestIDHeader))
}

func TestWithRequestConfig_GeneratesRequestIDWhenMissing(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	var requestID string
	handler := WithRequestConfig(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		var ok bool
		requestID, ok = ghcontext.RequestID(r.Context())
		require.True(t, ok)
	}))

	handler.ServeHTTP(recorder, request)

	assert.NotEmpty(t, requestID)
	assert.Equal(t, requestID, recorder.Header().Get(headers.RequestIDHeader))
	assert.Regexp(t, `^req_[0-9a-f]+$`, requestID)
}
