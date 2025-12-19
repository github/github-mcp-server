package ghmcp

import (
	"context"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v79/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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

// TestSessionInfoMiddleware_AddsMetadataToInitializeResult verifies that the
// session info middleware enriches the InitializeResult with session metadata.
func TestSessionInfoMiddleware_AddsMetadataToInitializeResult(t *testing.T) {
	t.Parallel()

	// Create test cases for different configurations
	tests := []struct {
		name                  string
		cfg                   MCPServerConfig
		enabledToolsets       []string
		instructionToolsets   []string
		expectedReadOnly      bool
		expectedLockdown      bool
		expectedDynamic       bool
		expectedToolsetsMode  string
		expectedToolsetsCount int
		expectedTools         []string
	}{
		{
			name: "default configuration",
			cfg: MCPServerConfig{
				ReadOnly:        false,
				LockdownMode:    false,
				EnabledTools:    nil,
				EnabledToolsets: nil,
			},
			enabledToolsets:       nil,
			instructionToolsets:   []string{"default"},
			expectedReadOnly:      false,
			expectedLockdown:      false,
			expectedDynamic:       false,
			expectedToolsetsMode:  "default",
			expectedToolsetsCount: 1,
			expectedTools:         nil,
		},
		{
			name: "read-only with lockdown",
			cfg: MCPServerConfig{
				ReadOnly:        true,
				LockdownMode:    true,
				EnabledTools:    []string{"get_me", "list_repos"},
				EnabledToolsets: []string{"repos", "issues"},
			},
			enabledToolsets:       []string{"repos", "issues"},
			instructionToolsets:   []string{"repos", "issues"},
			expectedReadOnly:      true,
			expectedLockdown:      true,
			expectedDynamic:       false,
			expectedToolsetsMode:  "explicit",
			expectedToolsetsCount: 2,
			expectedTools:         []string{"get_me", "list_repos"},
		},
		{
			name: "dynamic toolsets mode",
			cfg: MCPServerConfig{
				DynamicToolsets: true,
				ReadOnly:        false,
				LockdownMode:    false,
				EnabledToolsets: []string{},
			},
			enabledToolsets:       []string{},
			instructionToolsets:   []string{},
			expectedReadOnly:      false,
			expectedLockdown:      false,
			expectedDynamic:       true,
			expectedToolsetsMode:  "none",
			expectedToolsetsCount: 0,
			expectedTools:         nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new mock client for each test case
			mockClient := mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetUser,
					gogithub.User{
						Login:     gogithub.Ptr("testuser"),
						ID:        gogithub.Ptr(int64(12345)),
						HTMLURL:   gogithub.Ptr("https://github.com/testuser"),
						AvatarURL: gogithub.Ptr("https://avatars.githubusercontent.com/u/12345"),
						Name:      gogithub.Ptr("Test User"),
						Email:     gogithub.Ptr("test@example.com"),
						Bio:       gogithub.Ptr("Test bio"),
						Company:   gogithub.Ptr("Test Company"),
						Location:  gogithub.Ptr("Test Location"),
					},
				),
			)

			// Create a GitHub client with the mock
			restClient := gogithub.NewClient(mockClient).WithAuthToken("test-token")
			// Create middleware
			middleware := addSessionInfoMiddleware(tc.cfg, restClient, tc.enabledToolsets, tc.instructionToolsets)

			// Create a mock handler that returns a valid InitializeResult
			mockHandler := func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
				return &mcp.InitializeResult{
					ProtocolVersion: "2024-11-05",
					Capabilities:    &mcp.ServerCapabilities{},
					ServerInfo: &mcp.Implementation{
						Name:    "test-server",
						Version: "1.0.0",
					},
				}, nil
			}

			// Wrap with middleware
			handler := middleware(mockHandler)

			// Call with initialize method
			result, err := handler(context.Background(), "initialize", &mcp.InitializeRequest{})
			require.NoError(t, err)
			require.NotNil(t, result)

			// Cast to InitializeResult
			initResult, ok := result.(*mcp.InitializeResult)
			require.True(t, ok, "result should be InitializeResult")

			// Verify metadata exists
			require.NotNil(t, initResult.Meta)
			sessionInfo, exists := initResult.Meta["sessionInfo"]
			require.True(t, exists, "sessionInfo should exist in metadata")

			// Cast sessionInfo to map
			sessionInfoMap, ok := sessionInfo.(map[string]any)
			require.True(t, ok, "sessionInfo should be a map")

			// Verify configuration flags
			assert.Equal(t, tc.expectedReadOnly, sessionInfoMap["readOnlyMode"])
			assert.Equal(t, tc.expectedLockdown, sessionInfoMap["lockdownMode"])
			assert.Equal(t, tc.expectedDynamic, sessionInfoMap["dynamicToolsets"])
			assert.Equal(t, tc.expectedToolsetsMode, sessionInfoMap["toolsetsMode"])

			// Verify toolsets
			enabledToolsets, ok := sessionInfoMap["enabledToolsets"].([]string)
			require.True(t, ok, "enabledToolsets should be a string slice")
			assert.Len(t, enabledToolsets, tc.expectedToolsetsCount)

			// Verify enabled tools if specified
			if tc.expectedTools != nil {
				tools, ok := sessionInfoMap["enabledTools"].([]string)
				require.True(t, ok, "enabledTools should be a string slice")
				assert.Equal(t, tc.expectedTools, tools)
			}

			// Verify user info exists (since we mocked a successful API call)
			userInfo, exists := sessionInfoMap["user"]
			require.True(t, exists, "user info should exist")
			userInfoMap, ok := userInfo.(map[string]any)
			require.True(t, ok, "user info should be a map")
			assert.Equal(t, "testuser", userInfoMap["login"])
			assert.Equal(t, int64(12345), userInfoMap["id"])
			assert.Equal(t, "https://github.com/testuser", userInfoMap["profileURL"])
			assert.Equal(t, "Test User", userInfoMap["name"])
			assert.Equal(t, "test@example.com", userInfoMap["email"])
		})
	}
}

// TestSessionInfoMiddleware_OmitsUserOnAPIFailure verifies that when the
// get_me API call fails, the user info is omitted from session info.
func TestSessionInfoMiddleware_OmitsUserOnAPIFailure(t *testing.T) {
	t.Parallel()

	// Mock GitHub API to return an error
	mockClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.GetUser,
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			}),
		),
	)

	// Create a GitHub client with the mock
	restClient := gogithub.NewClient(mockClient).WithAuthToken("invalid-token")

	cfg := MCPServerConfig{
		ReadOnly:     false,
		LockdownMode: false,
	}

	// Create middleware
	middleware := addSessionInfoMiddleware(cfg, restClient, []string{"context"}, []string{"context"})

	// Create a mock handler
	mockHandler := func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		return &mcp.InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities:    &mcp.ServerCapabilities{},
			ServerInfo: &mcp.Implementation{
				Name:    "test-server",
				Version: "1.0.0",
			},
		}, nil
	}

	// Wrap with middleware
	handler := middleware(mockHandler)

	// Call with initialize method
	result, err := handler(context.Background(), "initialize", &mcp.InitializeRequest{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Cast to InitializeResult
	initResult, ok := result.(*mcp.InitializeResult)
	require.True(t, ok)

	// Verify metadata exists
	require.NotNil(t, initResult.Meta)
	sessionInfo, exists := initResult.Meta["sessionInfo"]
	require.True(t, exists)

	// Cast sessionInfo to map
	sessionInfoMap, ok := sessionInfo.(map[string]any)
	require.True(t, ok)

	// Verify user info does NOT exist (API call failed)
	_, exists = sessionInfoMap["user"]
	assert.False(t, exists, "user info should not exist when API call fails")

	// Verify other fields still exist
	assert.NotNil(t, sessionInfoMap["readOnlyMode"])
	assert.NotNil(t, sessionInfoMap["lockdownMode"])
	assert.NotNil(t, sessionInfoMap["enabledToolsets"])
}

// TestUnauthenticatedSessionInfoMiddleware verifies that the unauthenticated
// middleware adds session info without user data.
func TestUnauthenticatedSessionInfoMiddleware(t *testing.T) {
	t.Parallel()

	cfg := MCPServerConfig{
		ReadOnly:        true,
		LockdownMode:    false,
		DynamicToolsets: true,
		EnabledTools:    []string{"tool1", "tool2"},
	}

	// Create middleware
	middleware := addUnauthenticatedSessionInfoMiddleware(cfg, []string{}, []string{})

	// Create a mock handler
	mockHandler := func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		return &mcp.InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities:    &mcp.ServerCapabilities{},
			ServerInfo: &mcp.Implementation{
				Name:    "test-server",
				Version: "1.0.0",
			},
		}, nil
	}

	// Wrap with middleware
	handler := middleware(mockHandler)

	// Call with initialize method
	result, err := handler(context.Background(), "initialize", &mcp.InitializeRequest{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Cast to InitializeResult
	initResult, ok := result.(*mcp.InitializeResult)
	require.True(t, ok)

	// Verify metadata exists
	require.NotNil(t, initResult.Meta)
	sessionInfo, exists := initResult.Meta["sessionInfo"]
	require.True(t, exists)

	// Cast sessionInfo to map
	sessionInfoMap, ok := sessionInfo.(map[string]any)
	require.True(t, ok)

	// Verify configuration flags
	assert.Equal(t, true, sessionInfoMap["readOnlyMode"])
	assert.Equal(t, false, sessionInfoMap["lockdownMode"])
	assert.Equal(t, true, sessionInfoMap["dynamicToolsets"])
	assert.Equal(t, false, sessionInfoMap["authenticated"])

	// Verify enabled tools
	tools, ok := sessionInfoMap["enabledTools"].([]string)
	require.True(t, ok)
	assert.Equal(t, []string{"tool1", "tool2"}, tools)

	// Verify user info does NOT exist (unauthenticated mode)
	_, exists = sessionInfoMap["user"]
	assert.False(t, exists, "user info should not exist in unauthenticated mode")
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
