package ghmcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/github/github-mcp-server/pkg/github"
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

// TestBearerAuthTransport_AddsGraphQLFeaturesHeader verifies that the bearerAuthTransport
// properly reads GraphQL features from context and adds them as a header.
func TestBearerAuthTransport_AddsGraphQLFeaturesHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		features            []string
		expectHeader        bool
		expectedHeaderValue string
	}{
		{
			name:                "single feature",
			features:            []string{"issues_copilot_assignment_api_support"},
			expectHeader:        true,
			expectedHeaderValue: "issues_copilot_assignment_api_support",
		},
		{
			name:                "multiple features",
			features:            []string{"feature1", "feature2", "feature3"},
			expectHeader:        true,
			expectedHeaderValue: "feature1, feature2, feature3",
		},
		{
			name:         "no features",
			features:     []string{},
			expectHeader: false,
		},
		{
			name:         "nil features",
			features:     nil,
			expectHeader: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test server that records the request
			var capturedRequest *http.Request
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				w.WriteHeader(http.StatusOK)
			}))
			defer testServer.Close()

			// Create the transport chain
			transport := &bearerAuthTransport{
				transport: http.DefaultTransport,
				token:     "test-token",
			}

			// Create an HTTP client with the transport
			client := &http.Client{Transport: transport}

			// Create a context with GraphQL features
			ctx := context.Background()
			if tc.features != nil {
				ctx = github.WithGraphQLFeatures(ctx, tc.features...)
			}

			// Make a request with the context
			req, err := http.NewRequestWithContext(ctx, "POST", testServer.URL, nil)
			require.NoError(t, err)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Verify the Authorization header is set
			assert.Equal(t, "Bearer test-token", capturedRequest.Header.Get("Authorization"))

			// Verify the GraphQL-Features header
			if tc.expectHeader {
				assert.Equal(t, tc.expectedHeaderValue, capturedRequest.Header.Get("GraphQL-Features"))
			} else {
				assert.Empty(t, capturedRequest.Header.Get("GraphQL-Features"))
			}
		})
	}
}

// TestUserAgentTransport_PreservesGraphQLFeatures verifies that the userAgentTransport
// doesn't interfere with GraphQL features set by bearerAuthTransport.
func TestUserAgentTransport_PreservesGraphQLFeatures(t *testing.T) {
	t.Parallel()

	// Create a test server that records the request
	var capturedRequest *http.Request
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequest = r
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// Create the transport chain (same as in production)
	// userAgentTransport -> bearerAuthTransport -> http.DefaultTransport
	transport := &userAgentTransport{
		transport: &bearerAuthTransport{
			transport: http.DefaultTransport,
			token:     "test-token",
		},
		agent: "test-agent/1.0.0",
	}

	// Create an HTTP client with the transport chain
	client := &http.Client{Transport: transport}

	// Create a context with GraphQL features
	ctx := github.WithGraphQLFeatures(context.Background(), "issues_copilot_assignment_api_support")

	// Make a request with the context
	req, err := http.NewRequestWithContext(ctx, "POST", testServer.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify all headers are set correctly
	assert.Equal(t, "test-agent/1.0.0", capturedRequest.Header.Get("User-Agent"))
	assert.Equal(t, "Bearer test-token", capturedRequest.Header.Get("Authorization"))
	assert.Equal(t, "issues_copilot_assignment_api_support", capturedRequest.Header.Get("GraphQL-Features"))
}
