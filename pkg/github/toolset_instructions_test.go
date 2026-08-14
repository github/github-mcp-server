package github

import (
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryInstructionsExplainSymlinkWriteSemantics(t *testing.T) {
	t.Setenv("DISABLE_INSTRUCTIONS", "false")

	reposInventory, err := inventory.NewBuilder().
		SetTools([]inventory.ServerTool{{Toolset: ToolsetMetadataRepos}}).
		WithToolsets([]string{"repos"}).
		WithServerInstructions().
		Build()
	require.NoError(t, err)

	instructions := strings.ToLower(reposInventory.Instructions())
	assert.Contains(t, instructions, "## repository file writes")
	assert.Contains(t, instructions, "may return the target contents")
	assert.Contains(t, instructions, "do not follow symbolic links")

	defaultInventory, err := inventory.NewBuilder().
		SetTools([]inventory.ServerTool{
			{Toolset: ToolsetMetadataContext},
			{Toolset: ToolsetMetadataRepos},
		}).
		WithToolsets([]string{"default"}).
		WithServerInstructions().
		Build()
	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(defaultInventory.Instructions()), "## repository file writes")

	contextInventory, err := inventory.NewBuilder().
		SetTools([]inventory.ServerTool{{Toolset: ToolsetMetadataContext}}).
		WithToolsets([]string{"context"}).
		WithServerInstructions().
		Build()
	require.NoError(t, err)
	assert.NotContains(t, strings.ToLower(contextInventory.Instructions()), "## repository file writes")
}

func TestFileWritePathsExplainSymlinkWriteSemantics(t *testing.T) {
	createSchema, ok := CreateOrUpdateFile(translations.NullTranslationHelper).Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	pushSchema, ok := PushFiles(translations.NullTranslationHelper).Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	createPath := createSchema.Properties["path"]
	require.NotNil(t, createPath)
	pushFiles := pushSchema.Properties["files"]
	require.NotNil(t, pushFiles)
	require.NotNil(t, pushFiles.Items)
	pushPath := pushFiles.Items.Properties["path"]
	require.NotNil(t, pushPath)

	tools := []struct {
		name             string
		description      string
		expectedBehavior string
	}{
		{
			name:             "create_or_update_file",
			description:      createPath.Description,
			expectedBehavior: "rewrites the symbolic link's target path",
		},
		{
			name:             "push_files",
			description:      pushPath.Description,
			expectedBehavior: "replaces the link with a regular file",
		},
	}

	for _, tool := range tools {
		t.Run(tool.name, func(t *testing.T) {
			description := strings.ToLower(tool.description)
			assert.Contains(t, description, "exact git path")
			assert.Contains(t, description, tool.expectedBehavior)
		})
	}
}
