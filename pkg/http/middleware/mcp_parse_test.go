package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithMCPParse(t *testing.T) {
	tests := []struct {
		name            string
		method          string
		path            string
		body            string
		expectInfo      bool
		expectedMethod  string
		expectedItem    string
		expectedRaw     string
		expectedArgs    map[string]any
		expectArgsError bool
	}{
		{
			name:       "health check path is skipped",
			method:     http.MethodPost,
			path:       "/_ping",
			body:       `{"jsonrpc":"2.0","method":"tools/list"}`,
			expectInfo: false,
		},
		{
			name:       "GET request is skipped",
			method:     http.MethodGet,
			path:       "/mcp",
			body:       `{"jsonrpc":"2.0","method":"tools/list"}`,
			expectInfo: false,
		},
		{
			name:       "empty body is skipped",
			method:     http.MethodPost,
			path:       "/mcp",
			body:       "",
			expectInfo: false,
		},
		{
			name:       "invalid JSON is skipped",
			method:     http.MethodPost,
			path:       "/mcp",
			body:       "not valid json",
			expectInfo: false,
		},
		{
			name:       "non-JSON-RPC 2.0 is skipped",
			method:     http.MethodPost,
			path:       "/mcp",
			body:       `{"jsonrpc":"1.0","method":"tools/list"}`,
			expectInfo: false,
		},
		{
			name:       "empty method is skipped",
			method:     http.MethodPost,
			path:       "/mcp",
			body:       `{"jsonrpc":"2.0","method":""}`,
			expectInfo: false,
		},
		{
			name:           "tools/list parses method only",
			method:         http.MethodPost,
			path:           "/mcp",
			body:           `{"jsonrpc":"2.0","method":"tools/list"}`,
			expectInfo:     true,
			expectedMethod: "tools/list",
		},
		{
			name:           "tools/call parses name",
			method:         http.MethodPost,
			path:           "/mcp",
			body:           `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_file_contents"}}`,
			expectInfo:     true,
			expectedMethod: "tools/call",
			expectedItem:   "get_file_contents",
		},
		{
			name:           "tools/call parses owner and repo from arguments",
			method:         http.MethodPost,
			path:           "/mcp",
			body:           `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_file_contents","arguments":{"owner":"github","repo":"github-mcp-server","path":"README.md"}}}`,
			expectInfo:     true,
			expectedMethod: "tools/call",
			expectedItem:   "get_file_contents",
			expectedRaw:    `{"owner":"github","repo":"github-mcp-server","path":"README.md"}`,
			expectedArgs:   map[string]any{"owner": "github", "repo": "github-mcp-server", "path": "README.md"},
		},
		{
			name:            "tools/call with invalid arguments JSON continues without args",
			method:          http.MethodPost,
			path:            "/mcp",
			body:            `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_file_contents","arguments":"not an object"}}`,
			expectInfo:      true,
			expectedMethod:  "tools/call",
			expectedItem:    "get_file_contents",
			expectedRaw:     `"not an object"`,
			expectArgsError: true,
		},
		{
			name:           "prompts/get parses name",
			method:         http.MethodPost,
			path:           "/mcp",
			body:           `{"jsonrpc":"2.0","method":"prompts/get","params":{"name":"my_prompt"}}`,
			expectInfo:     true,
			expectedMethod: "prompts/get",
			expectedItem:   "my_prompt",
		},
		{
			name:           "resources/read parses URI as item name",
			method:         http.MethodPost,
			path:           "/mcp",
			body:           `{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"repo://github/github-mcp-server"}}`,
			expectInfo:     true,
			expectedMethod: "resources/read",
			expectedItem:   "repo://github/github-mcp-server",
		},
		{
			name:           "initialize method parses correctly",
			method:         http.MethodPost,
			path:           "/mcp",
			body:           `{"jsonrpc":"2.0","method":"initialize","params":{"capabilities":{}}}`,
			expectInfo:     true,
			expectedMethod: "initialize",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedInfo *ghcontext.MCPMethodInfo
			var infoCaptured bool

			// Create a handler that captures the MCPMethodInfo from context
			nextHandler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				capturedInfo, infoCaptured = ghcontext.MCPMethod(r.Context())
			})

			middleware := WithMCPParse()
			handler := middleware(nextHandler)

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if tt.expectInfo {
				require.True(t, infoCaptured, "MCPMethodInfo should be present in context")
				require.NotNil(t, capturedInfo)
				assert.Equal(t, tt.expectedMethod, capturedInfo.Method)
				assert.Equal(t, tt.expectedItem, capturedInfo.ItemName)
				if tt.expectedRaw != "" {
					assert.JSONEq(t, tt.expectedRaw, string(capturedInfo.RawArguments))
				}
				decodedArgs, err := capturedInfo.DecodeArguments()
				if tt.expectArgsError {
					assert.Error(t, err)
				} else {
					require.NoError(t, err)
				}
				if tt.expectedArgs != nil {
					assert.Equal(t, tt.expectedArgs, decodedArgs)
				}
			} else {
				assert.False(t, infoCaptured, "MCPMethodInfo should not be present in context")
			}
		})
	}
}

func TestWithMCPParseRetainsLargeArgumentsWithoutMaterializingThem(t *testing.T) {
	nested := map[string]any{
		"items": []any{
			map[string]any{"payload": strings.Repeat("x", 32*1024)},
			[]any{1.0, 2.0, 3.0},
		},
	}
	rawArguments, err := json.Marshal(nested)
	require.NoError(t, err)
	body := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"test_tool","arguments":` + string(rawArguments) + `}}`

	var capturedInfo *ghcontext.MCPMethodInfo
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		capturedInfo, _ = ghcontext.MCPMethod(r.Context())
	})
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	WithMCPParse()(next).ServeHTTP(httptest.NewRecorder(), request)

	require.NotNil(t, capturedInfo)
	assert.Equal(t, json.RawMessage(rawArguments), capturedInfo.RawArguments)
}

func TestWithMCPParse_BodyRestoration(t *testing.T) {
	originalBody := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"test_tool"}}`

	var capturedBody string

	nextHandler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		capturedBody = string(body)
	})

	middleware := WithMCPParse()
	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(originalBody))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, originalBody, capturedBody, "body should be restored for downstream handlers")
}

// TestWithMCPParse_WithMaxBodySize mirrors the production middleware ordering,
// where WithMaxBodySize runs ahead of WithMCPParse.
func TestWithMCPParse_WithMaxBodySize(t *testing.T) {
	const limit = 128

	buildBody := func(size int) string {
		payload := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"test_tool","arguments":{"pad":"PADDING"}}}`
		if len(payload) >= size {
			return payload
		}
		pad := strings.Repeat("x", size-len(payload))
		return strings.Replace(payload, "PADDING", "PADDING"+pad, 1)
	}

	t.Run("oversized body is rejected before parsing", func(t *testing.T) {
		body := buildBody(limit + 1)
		require.Greater(t, len(body), limit)

		var nextCalled bool
		nextHandler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			nextCalled = true
		})

		handler := WithMaxBodySize(limit)(WithMCPParse()(nextHandler))

		// An unknown length skips WithMaxBodySize's Content-Length fast path,
		// so the overflow surfaces from WithMCPParse's own read.
		req := httptest.NewRequest(http.MethodPost, "/mcp", unknownLengthBody(body))
		require.Equal(t, int64(-1), req.ContentLength, "test setup: Content-Length should be unknown")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.False(t, nextCalled, "downstream handler must not run for an oversized request")
		assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
		assert.Contains(t, rr.Body.String(), "request body too large")
	})

	t.Run("boundary-size body is parsed and preserved", func(t *testing.T) {
		body := buildBody(limit)
		require.Len(t, body, limit)

		var capturedInfo *ghcontext.MCPMethodInfo
		var capturedBody string
		nextHandler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			capturedInfo, _ = ghcontext.MCPMethod(r.Context())
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			capturedBody = string(b)
		})

		handler := WithMaxBodySize(limit)(WithMCPParse()(nextHandler))

		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		require.NotNil(t, capturedInfo, "MCPMethodInfo should be parsed for an allowed request")
		assert.Equal(t, "tools/call", capturedInfo.Method)
		assert.Equal(t, "test_tool", capturedInfo.ItemName)
		assert.Equal(t, body, capturedBody, "body should be preserved for downstream handlers")
	})
}
