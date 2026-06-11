package github

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/permissions"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateToolPermissionFilter(t *testing.T) {
	toolNoReq := &inventory.ServerTool{Tool: mcp.Tool{Name: "ungated"}}
	toolIssuesWrite := &inventory.ServerTool{
		Tool:                mcp.Tool{Name: "create_issue"},
		RequiredPermissions: permissions.Require(permissions.Issues.Write()),
	}

	t.Run("fails open when granted is nil", func(t *testing.T) {
		filter := CreateToolPermissionFilter(nil)
		for _, tool := range []*inventory.ServerTool{toolNoReq, toolIssuesWrite} {
			ok, err := filter(context.Background(), tool)
			require.NoError(t, err)
			assert.True(t, ok, "nil granted must include every tool")
		}
	})

	t.Run("ungated tools always included", func(t *testing.T) {
		filter := CreateToolPermissionFilter(map[permissions.Permission]permissions.Level{})
		ok, err := filter(context.Background(), toolNoReq)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("gated tool hidden without sufficient grant", func(t *testing.T) {
		filter := CreateToolPermissionFilter(map[permissions.Permission]permissions.Level{
			permissions.Issues: permissions.LevelRead,
		})
		ok, err := filter(context.Background(), toolIssuesWrite)
		require.NoError(t, err)
		assert.False(t, ok, "issues:read must not satisfy issues:write")
	})

	t.Run("gated tool shown with sufficient grant", func(t *testing.T) {
		filter := CreateToolPermissionFilter(map[permissions.Permission]permissions.Level{
			permissions.Issues: permissions.LevelWrite,
		})
		ok, err := filter(context.Background(), toolIssuesWrite)
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

func TestGetToolPermissionMapFromInventory(t *testing.T) {
	inv, err := inventory.NewBuilder().SetTools([]inventory.ServerTool{
		{Tool: mcp.Tool{Name: "ungated"}},
		{
			Tool:                mcp.Tool{Name: "create_issue"},
			RequiredPermissions: permissions.Require(permissions.Issues.Write()),
		},
	}).Build()
	require.NoError(t, err)

	m := GetToolPermissionMapFromInventory(inv)
	_, hasUngated := m["ungated"]
	assert.False(t, hasUngated, "tools with zero requirement are omitted")
	req, hasGated := m["create_issue"]
	require.True(t, hasGated)
	assert.Equal(t, "issues:write", req.String())

	union := UnionPermissions(inv)
	assert.Equal(t, []permissions.Permission{permissions.Issues}, union)
}
