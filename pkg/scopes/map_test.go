package scopes

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetToolScopeMapFromInventory(t *testing.T) {
	access := RequireAll(ReadOrg)
	inv, err := inventory.NewBuilder().
		SetTools([]inventory.ServerTool{
			{
				Tool:        mcp.Tool{Name: "scoped"},
				Toolset:     inventory.ToolsetMetadata{ID: "test"},
				ScopeAccess: access,
			},
			{
				Tool:    mcp.Tool{Name: "unscoped"},
				Toolset: inventory.ToolsetMetadata{ID: "test"},
			},
		}).
		WithToolsets([]string{"test"}).
		Build()
	require.NoError(t, err)

	scopeMap := GetToolScopeMapFromInventory(inv)
	require.Contains(t, scopeMap, "scoped")
	assert.Empty(t, scopeMap["scoped"](nil, []string{"admin:org"}))
	assert.Equal(t, []string{"read:org"}, scopeMap["scoped"](nil, []string{"repo"}))
	assert.NotContains(t, scopeMap, "unscoped")
}

func TestGlobalToolScopeMap(t *testing.T) {
	check := RequireAll(Repo).Challenge
	SetGlobalToolScopeMap(ToolScopeMap{"repo_tool": check})
	t.Cleanup(func() { SetGlobalToolScopeMap(nil) })

	assert.Equal(t, []string{"repo"}, GetToolScopeChallenge("repo_tool")(nil, nil))
	assert.Nil(t, GetToolScopeChallenge("missing"))
}
