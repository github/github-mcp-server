package ghmcp

import (
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewMCPServer_CreatesSuccessfully verifies that the server can be created
// with the deps injection middleware properly configured.
func TestNewMCPServer_CreatesSuccessfully(t *testing.T) {
	t.Parallel()

	// Create a minimal server configuration
	cfg := MCPServerConfig{
		Version:           "test",
		Host:              "", // defaults to github.com
		Token:             "test-token",
		EnabledToolsets:   []string{"context"},
		ReadOnly:          false,
		Translator:        translations.NullTranslationHelper,
		ContentWindowSize: 5000,
		LockdownMode:      false,
		InsiderMode:       false,
	}

	// Create the server
	server, err := NewMCPServer(cfg)
	require.NoError(t, err, "expected server creation to succeed")
	require.NotNil(t, server, "expected server to be non-nil")

	// The fact that the server was created successfully indicates that:
	// 1. The deps injection middleware is properly added
	// 2. Tools can be registered without panicking
	//
	// If the middleware wasn't properly added, tool calls would panic with
	// "ToolDependencies not found in context" when executed.
	//
	// The actual middleware functionality and tool execution with ContextWithDeps
	// is already tested in pkg/github/*_test.go.
}

// TestResolveEnabledToolsets verifies the toolset resolution logic.
func TestResolveEnabledToolsets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		cfg            MCPServerConfig
		expectedResult []string
	}{
		{
			name: "nil toolsets without dynamic mode and no tools - use defaults",
			cfg: MCPServerConfig{
				EnabledToolsets: nil,
				DynamicToolsets: false,
				EnabledTools:    nil,
			},
			expectedResult: nil, // nil means "use defaults"
		},
		{
			name: "nil toolsets with dynamic mode - start empty",
			cfg: MCPServerConfig{
				EnabledToolsets: nil,
				DynamicToolsets: true,
				EnabledTools:    nil,
			},
			expectedResult: []string{}, // empty slice means no toolsets
		},
		{
			name: "explicit toolsets",
			cfg: MCPServerConfig{
				EnabledToolsets: []string{"repos", "issues"},
				DynamicToolsets: false,
			},
			expectedResult: []string{"repos", "issues"},
		},
		{
			name: "empty toolsets - disable all",
			cfg: MCPServerConfig{
				EnabledToolsets: []string{},
				DynamicToolsets: false,
			},
			expectedResult: []string{}, // empty slice means no toolsets
		},
		{
			name: "specific tools without toolsets - no default toolsets",
			cfg: MCPServerConfig{
				EnabledToolsets: nil,
				DynamicToolsets: false,
				EnabledTools:    []string{"get_me"},
			},
			expectedResult: []string{}, // empty slice when tools specified but no toolsets
		},
		{
			name: "dynamic mode with explicit toolsets removes all and default",
			cfg: MCPServerConfig{
				EnabledToolsets: []string{"all", "repos"},
				DynamicToolsets: true,
			},
			expectedResult: []string{"repos"}, // "all" is removed in dynamic mode
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := resolveEnabledToolsets(tc.cfg)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func Test_extractTokenFromAuthHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		req  *http.Request
		want string
	}{
		{
			name: "valid bearer token",
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"Bearer ghp_1234567890abcdef"},
				},
			},
			want: "ghp_1234567890abcdef",
		},
		{
			name: "bearer token with extra whitespace",
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"  Bearer   ghp_token123  "},
				},
			},
			want: "ghp_token123",
		},
		{
			name: "case insensitive bearer",
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"bearer ghp_lowercase"},
				},
			},
			want: "ghp_lowercase",
		},
		{
			name: "mixed case bearer",
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"BeArEr ghp_mixedcase"},
				},
			},
			want: "ghp_mixedcase",
		},
		{
			name: "no authorization header",
			req: &http.Request{
				Header: http.Header{},
			},
			want: "",
		},
		{
			name: "empty authorization header",
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{""},
				},
			},
			want: "",
		},
		{
			name: "whitespace only authorization header",
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"   "},
				},
			},
			want: "",
		},
		{
			name: "missing token after bearer",
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"Bearer"},
				},
			},
			want: "",
		},
		{
			name: "bearer with only whitespace token",
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"Bearer   "},
				},
			},
			want: "",
		},
		{
			name: "non-bearer scheme",
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"Basic dXNlcjpwYXNz"},
				},
			},
			want: "",
		},
		{
			name: "no space between bearer and token",
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"Bearerghp_token"},
				},
			},
			want: "",
		},
		{
			name: "token only without scheme",
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"ghp_token_only"},
				},
			},
			want: "",
		},
		{
			name: "multiple spaces between bearer and token",
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"Bearer     ghp_multispace"},
				},
			},
			want: "ghp_multispace",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTokenFromAuthHeader(tt.req)
			assert.Equal(t, tt.want, got)
		})
	}
}
