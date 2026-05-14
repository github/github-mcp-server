package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/github/github-mcp-server/pkg/http/middleware"
	"github.com/stretchr/testify/assert"
)

func TestSetCorsHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware.SetCorsHeaders(inner)

	t.Run("OPTIONS preflight returns 200 with CORS headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "http://localhost:6274")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Methods"), "POST")
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "Authorization")
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "Mcp-Session-Id")
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "X-MCP-Lockdown")
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "X-MCP-Insiders")
		assert.Contains(t, rr.Header().Get("Access-Control-Expose-Headers"), "Mcp-Session-Id")
		assert.Contains(t, rr.Header().Get("Access-Control-Expose-Headers"), "WWW-Authenticate")
	})

	t.Run("POST request includes CORS headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("Origin", "http://localhost:6274")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
	})
}
