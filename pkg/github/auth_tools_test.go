package github

import (
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthLogin(t *testing.T) {
	t.Parallel()

	// Verify tool definition
	serverTool := AuthLogin(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "auth_login", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "auth_login tool should be read-only")
}

func TestAuthTools(t *testing.T) {
	t.Parallel()

	tools := AuthTools(translations.NullTranslationHelper)
	require.Len(t, tools, 1)

	assert.Equal(t, "auth_login", tools[0].Tool.Name)
}
