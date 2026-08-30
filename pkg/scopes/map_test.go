package scopes

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetToolScopeMapFromInventory(t *testing.T) {
	access := DynamicChallenge(
		[]Scope{Repo, ReadOrg},
		func([]string) bool { return true },
		func(_ map[string]any, activeScopes []string) []string {
			return ChallengeAll(activeScopes, ReadOrg)
		},
	)
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
	assert.Equal(t, []string{"repo", "read:org"}, scopeMap["scoped"].Scopes)
	assert.True(t, scopeMap["scoped"].Dynamic)
	assert.True(t, scopeMap["scoped"].Visible(nil))
	assert.Empty(t, scopeMap["scoped"].Challenge(nil, []string{"admin:org"}))
	assert.Equal(t, []string{"read:org"}, scopeMap["scoped"].Challenge(nil, []string{"repo"}))
	assert.NotContains(t, scopeMap, "unscoped")
}

func TestGlobalToolScopeMap(t *testing.T) {
	access := RequireAll(Repo)
	SetGlobalToolScopeMap(ToolScopeMap{"repo_tool": access})
	t.Cleanup(func() { SetGlobalToolScopeMap(nil) })

	access.Scopes[0] = "mutated"
	stored, ok := GetToolScopeAccess("repo_tool")
	require.True(t, ok)
	assert.Equal(t, []string{"repo"}, stored.MaximumScopes())
	assert.Equal(t, []string{"repo"}, stored.ResolveChallenge(nil, nil))

	maxScopes := stored.MaximumScopes()
	maxScopes[0] = "mutated again"
	stored, ok = GetToolScopeAccess("repo_tool")
	require.True(t, ok)
	assert.Equal(t, []string{"repo"}, stored.MaximumScopes())

	_, ok = GetToolScopeAccess("missing")
	assert.False(t, ok)

	assert.Panics(t, func() {
		SetGlobalToolScopeMap(ToolScopeMap{
			"invalid": {
				Dynamic:   true,
				Challenge: func(map[string]any, []string) []string { return nil },
			},
		})
	})
}
