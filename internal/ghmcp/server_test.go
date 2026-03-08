package ghmcp

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/require"
)

func TestNewStdioMCPServer_StrictToolsetValidation(t *testing.T) {
	t.Parallel()

	_, err := NewStdioMCPServer(context.Background(), testMCPServerConfig([]string{"repos", "typo"}, true))
	require.Error(t, err)
	require.ErrorIs(t, err, inventory.ErrUnknownToolsets)
	require.Contains(t, err.Error(), "typo")
}

func TestNewStdioMCPServer_AllowsUnknownToolsetsWhenNotStrict(t *testing.T) {
	t.Parallel()

	server, err := NewStdioMCPServer(context.Background(), testMCPServerConfig([]string{"repos", "typo"}, false))
	require.NoError(t, err)
	require.NotNil(t, server)
}

func testMCPServerConfig(toolsets []string, strict bool) github.MCPServerConfig {
	return github.MCPServerConfig{
		Version:                 "test",
		Token:                   "test-token",
		EnabledToolsets:         toolsets,
		StrictToolsetValidation: strict,
		Translator:              translations.NullTranslationHelper,
		ContentWindowSize:       5000,
		Logger:                  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}
