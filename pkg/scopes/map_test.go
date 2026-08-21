package scopes

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetToolScopeMapFromInventory(t *testing.T) {
	inv, err := inventory.NewBuilder().
		SetTools([]inventory.ServerTool{
			{Tool: testTool("scoped"), Toolset: testToolset(), ScopePolicy: AnyOfScopePolicy(ReadOrg)},
			{Tool: testTool("unscoped"), Toolset: testToolset()},
		}).
		WithToolsets([]string{"test"}).
		Build()
	require.NoError(t, err)

	scopeMap := GetToolScopeMapFromInventory(inv)
	require.Contains(t, scopeMap, "scoped")
	assert.Equal(t, AnyOfScopePolicy(ReadOrg), scopeMap["scoped"].ScopePolicy)
	assert.NotContains(t, scopeMap, "unscoped")
}

func TestToolScopeInfo(t *testing.T) {
	info := &ToolScopeInfo{ScopePolicy: AllOfScopePolicy(Repo, Workflow)}

	assert.True(t, info.Satisfies("repo", "workflow"))
	assert.False(t, info.Satisfies("repo"))
	assert.Equal(t, []string{"workflow"}, info.ChallengeScopes("repo"))
	assert.True(t, (*ToolScopeInfo)(nil).Satisfies())
	assert.Nil(t, (*ToolScopeInfo)(nil).ChallengeScopes())
}

func TestToolScopeInfoResolve(t *testing.T) {
	base := &ToolScopeInfo{
		ScopePolicy: AnyOfScopePolicy(Repo, ReadOrg),
		ScopeResolver: func(arguments map[string]any) inventory.ScopePolicy {
			if arguments["workflow"] == true {
				return AllOfScopePolicy(Repo, Workflow)
			}
			return AllOfScopePolicy(Repo)
		},
	}

	resolved := base.Resolve(map[string]any{"workflow": true})
	require.NotSame(t, base, resolved)
	assert.True(t, resolved.Satisfies("repo", "workflow"))
	assert.False(t, resolved.Satisfies("repo"))
	assert.Equal(t, []string{"workflow"}, resolved.ChallengeScopes("repo"))

	assert.True(t, base.Satisfies("read:org"))
	regular := base.Resolve(map[string]any{"workflow": false})
	assert.True(t, regular.Satisfies("repo"))
	assert.False(t, regular.Satisfies("read:org"))
}

func testTool(name string) mcp.Tool {
	return mcp.Tool{Name: name}
}

func testToolset() inventory.ToolsetMetadata {
	return inventory.ToolsetMetadata{ID: "test"}
}
