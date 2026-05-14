package middleware

import (
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
		name           string
		method         string
		path           string
		body           string
		expectInfo     bool
		expectedMethod string
		expectedItem   string
		expectedOwner  string
		expectedRepo   string
		expectedArgs   map[string]any
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
			expectedOwner:  "github",
			expectedRepo:   "github-mcp-server",
			expectedArgs:   map[string]any{"owner": "github", "repo": "github-mcp-server", "path": "README.md"},
		},
		{
			name:           "tools/call with invalid arguments JSON continues without args",
			method:         http.MethodPost,
			path:           "/mcp",
			body:           `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_file_contents","arguments":"not an object"}}`,
			expectInfo:     true,
			expectedMethod: "tools/call",
			expectedItem:   "get_file_contents",
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
				assert.Equal(t, tt.expectedOwner, capturedInfo.Owner)
				assert.Equal(t, tt.expectedRepo, capturedInfo.Repo)
				if tt.expectedArgs != nil {
					assert.Equal(t, tt.expectedArgs, capturedInfo.Arguments)
				}
			} else {
				assert.False(t, infoCaptured, "MCPMethodInfo should not be present in context")
			}
		})
	}
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
