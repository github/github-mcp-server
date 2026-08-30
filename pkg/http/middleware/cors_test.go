package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/http/middleware"
	"github.com/stretchr/testify/assert"
)

func TestSetCorsHeaders(t *testing.T) {
	innerCalled := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		innerCalled = true
		w.Header().Add("Access-Control-Expose-Headers", "X-Existing-Response")
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware.SetCorsHeaders(inner)

	t.Run("OPTIONS preflight returns 200 with CORS headers", func(t *testing.T) {
		innerCalled = false
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "https://confer.to")
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)
		req.Header.Set("Access-Control-Request-Headers", "content-type")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.False(t, innerCalled)
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
		assert.Empty(t, rr.Header().Get("Access-Control-Allow-Credentials"))
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Methods"), "POST")
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "Authorization")
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "Mcp-Session-Id")
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "X-MCP-Lockdown")
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "X-MCP-Insiders")
		assert.Contains(t, rr.Header().Get("Access-Control-Expose-Headers"), "Mcp-Session-Id")
		assert.Contains(t, rr.Header().Get("Access-Control-Expose-Headers"), "WWW-Authenticate")
	})

	t.Run("POST request includes CORS headers without replacing existing exposed headers", func(t *testing.T) {
		innerCalled = false
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("Origin", "https://confer.to")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.True(t, innerCalled)
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
		assert.Empty(t, rr.Header().Get("Access-Control-Allow-Credentials"))
		exposedHeaders := strings.Join(rr.Header().Values("Access-Control-Expose-Headers"), ", ")
		assert.Contains(t, exposedHeaders, "Mcp-Session-Id")
		assert.Contains(t, exposedHeaders, "WWW-Authenticate")
		assert.Contains(t, exposedHeaders, "X-Existing-Response")
	})
}
