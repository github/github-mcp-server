package middleware_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/http/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixedAllowedRequestHeaders = "Content-Type, Mcp-Session-Id, Mcp-Protocol-Version, Mcp-Method, Mcp-Name, Last-Event-ID, Authorization, X-MCP-Readonly, X-MCP-Toolsets, X-MCP-Tools, X-MCP-Exclude-Tools, X-MCP-Features, X-MCP-Lockdown, X-MCP-Insiders, Mcp-Param-owner, Mcp-Param-repo"

func TestSetCorsHeadersPreflight(t *testing.T) {
	tests := []struct {
		name                   string
		requestedHeadersValues []string
		expectedAllowedHeaders string
	}{
		{
			name: "current MCP request headers",
			requestedHeadersValues: []string{
				"authorization, content-type, mcp-protocol-version, mcp-method, mcp-name, mcp-param-owner, mcp-param-repo",
			},
			expectedAllowedHeaders: fixedAllowedRequestHeaders,
		},
		{
			name:                   "future projected parameter",
			requestedHeadersValues: []string{"Mcp-Param-region"},
			expectedAllowedHeaders: fixedAllowedRequestHeaders + ", Mcp-Param-Region",
		},
		{
			name: "mixed case duplicates across values",
			requestedHeadersValues: []string{
				"mCp-PaRaM-ReGiOn, MCP-PARAM-ZONE",
				"MCP-PARAM-REGION, mcp-param-zone",
			},
			expectedAllowedHeaders: fixedAllowedRequestHeaders + ", Mcp-Param-Region, Mcp-Param-Zone",
		},
		{
			name: "invalid unrelated bare and lookalike names",
			requestedHeadersValues: []string{
				"Mcp-Param-, X-Evil, XMcp-Param-region, Mcp_Param-region, Mcp-Param-\x00region, Mcp-Param-bad name",
			},
			expectedAllowedHeaders: fixedAllowedRequestHeaders,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			innerCalled := false
			handler := middleware.SetCorsHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				innerCalled = true
			}))
			req := httptest.NewRequest(http.MethodOptions, "/", nil)
			req.Header.Set("Origin", "https://confer.to")
			req.Header.Set("Access-Control-Request-Method", http.MethodPost)
			for _, value := range tt.requestedHeadersValues {
				req.Header.Add("Access-Control-Request-Headers", value)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.False(t, innerCalled)
			assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
			assert.Empty(t, rr.Header().Get("Access-Control-Allow-Credentials"))
			assert.Equal(t, "GET, POST, DELETE, OPTIONS", rr.Header().Get("Access-Control-Allow-Methods"))
			assert.Equal(t, "86400", rr.Header().Get("Access-Control-Max-Age"))
			assert.Equal(t, tt.expectedAllowedHeaders, rr.Header().Get("Access-Control-Allow-Headers"))
			assert.Equal(t, "Mcp-Session-Id, WWW-Authenticate", rr.Header().Get("Access-Control-Expose-Headers"))
			assert.NotContains(t, rr.Header().Get("Access-Control-Expose-Headers"), "Mcp-Param-")
		})
	}
}

func TestSetCorsHeadersPreflightBoundsProjectedHeaders(t *testing.T) {
	const (
		maxProjectedHeaders = 64
		requestedHeaders    = 1024
	)

	requested := make([]string, 0, requestedHeaders*2)
	expected := strings.Split(fixedAllowedRequestHeaders, ", ")
	for i := range requestedHeaders {
		header := fmt.Sprintf("mcp-param-%04d", i)
		requested = append(requested, header, strings.ToUpper(header))
		if i < maxProjectedHeaders {
			expected = append(expected, http.CanonicalHeaderKey(header))
		}
	}
	requestedValue := strings.Join(requested, ", ")

	handler := middleware.SetCorsHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("preflight reached the inner handler")
	}))
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Access-Control-Request-Headers", requestedValue)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	allowedValue := rr.Header().Get("Access-Control-Allow-Headers")
	assert.Equal(t, expected, strings.Split(allowedValue, ", "))
	assert.NotContains(t, allowedValue, "Mcp-Param-0064")
	assert.Less(t, len(allowedValue), len(requestedValue))
}

func TestSetCorsHeadersPostPreservesBehavior(t *testing.T) {
	innerCalled := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		innerCalled = true
		w.Header().Add("Access-Control-Expose-Headers", "X-Existing-Response")
		w.WriteHeader(http.StatusCreated)
		_, err := w.Write([]byte("created"))
		require.NoError(t, err)
	})
	handler := middleware.SetCorsHeaders(inner)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Origin", "https://confer.to")
	req.Header.Set("Access-Control-Request-Headers", "Mcp-Param-region")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.True(t, innerCalled)
	assert.Equal(t, "created", rr.Body.String())
	assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, rr.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, fixedAllowedRequestHeaders, rr.Header().Get("Access-Control-Allow-Headers"))
	exposedHeaders := strings.Join(rr.Header().Values("Access-Control-Expose-Headers"), ", ")
	assert.Contains(t, exposedHeaders, "Mcp-Session-Id")
	assert.Contains(t, exposedHeaders, "WWW-Authenticate")
	assert.Contains(t, exposedHeaders, "X-Existing-Response")
	assert.NotContains(t, exposedHeaders, "Mcp-Param-")
}
