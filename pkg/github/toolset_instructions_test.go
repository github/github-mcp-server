package github

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inventoryWithToolset builds an inventory containing a single tool that belongs
// to the given toolset, so HasToolset(toolsetID) reports true.
func inventoryWithToolset(t *testing.T, toolsetID string) *inventory.Inventory {
	t.Helper()

	tool := inventory.NewServerToolFromHandler(
		mcp.Tool{
			Name:        "sample_tool",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		inventory.ToolsetMetadata{ID: inventory.ToolsetID(toolsetID), Description: "test toolset"},
		func(_ any) mcp.ToolHandler {
			return func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return nil, nil
			}
		},
	)

	inv, err := inventory.NewBuilder().
		SetTools([]inventory.ServerTool{tool}).
		WithToolsets([]string{"all"}).
		Build()
	require.NoError(t, err)
	return inv
}

func TestGenerateToolsetInstructions_ContentSpecific(t *testing.T) {
	assert.Contains(t, generateContextToolsetInstructions(nil), "get_me")
	assert.Contains(t, generateIssuesToolsetInstructions(nil), "## Issues")
	assert.Contains(t, generateDiscussionsToolsetInstructions(nil), "## Discussions")

	projects := generateProjectsToolsetInstructions(nil)
	assert.Contains(t, projects, "## Projects")
	assert.Contains(t, projects, "Pagination")
}

func TestGeneratePullRequestsToolsetInstructions(t *testing.T) {
	t.Run("without repos toolset omits template guidance", func(t *testing.T) {
		inv := inventoryWithToolset(t, "pull_requests")
		require.False(t, inv.HasToolset("repos"))

		instructions := generatePullRequestsToolsetInstructions(inv)
		assert.Contains(t, instructions, "## Pull Requests")
		assert.NotContains(t, instructions, "pull_request_template")
	})

	t.Run("with repos toolset includes template guidance", func(t *testing.T) {
		inv := inventoryWithToolset(t, "repos")
		require.True(t, inv.HasToolset("repos"))

		instructions := generatePullRequestsToolsetInstructions(inv)
		assert.Contains(t, instructions, "## Pull Requests")
		assert.Contains(t, instructions, "pull_request_template")
	})
}
