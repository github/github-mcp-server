package github

import (
	"context"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/permissions"
)

// GetToolPermissionMapFromInventory builds a tool permission map from an
// inventory, extracting each tool's RequiredPermissions. Tools with a
// zero-value (empty) requirement are omitted, since an empty requirement means
// "no gate". This mirrors scopes.GetToolScopeMapFromInventory.
func GetToolPermissionMapFromInventory(inv *inventory.Inventory) permissions.ToolPermissionMap {
	result := make(permissions.ToolPermissionMap)
	allTools := inv.AllTools()
	for i := range allTools {
		tool := &allTools[i]
		if !tool.RequiredPermissions.IsZero() {
			result[tool.Tool.Name] = tool.RequiredPermissions
		}
	}
	return result
}

// SetToolPermissionMapFromInventory builds and stores the global tool
// permission map from an inventory, so middleware and other consumers can look
// up requirements. Mirrors scopes.SetToolScopeMapFromInventory.
func SetToolPermissionMapFromInventory(inv *inventory.Inventory) {
	permissions.SetGlobalToolPermissionMap(GetToolPermissionMapFromInventory(inv))
}

// UnionPermissions returns the sorted, de-duplicated set of fine-grained
// permissions referenced by every tool in the inventory.
func UnionPermissions(inv *inventory.Inventory) []permissions.Permission {
	return GetToolPermissionMapFromInventory(inv).UniquePermissions()
}

// CreateToolPermissionFilter creates an inventory.ToolFilter that hides tools
// whose fine-grained permission requirement is not satisfied by the granted
// permissions.
//
// The filter FAILS OPEN: if granted is nil (no permission source available, as
// is the case in the OSS server today) the filter includes every tool. Tools
// with a zero-value requirement are always included. This means the OSS server
// ships this subsystem dormant — there is no granted-permission source in OSS,
// so no tools are hidden. A consumer such as the remote server supplies the
// granted map to activate filtering.
func CreateToolPermissionFilter(granted map[permissions.Permission]permissions.Level) inventory.ToolFilter {
	return func(_ context.Context, tool *inventory.ServerTool) (bool, error) {
		// Fail open when there is no granted-permission source.
		if granted == nil {
			return true, nil
		}
		return tool.RequiredPermissions.SatisfiedBy(granted), nil
	}
}
