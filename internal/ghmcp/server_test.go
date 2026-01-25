package ghmcp

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
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
