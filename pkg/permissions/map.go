package permissions

import "slices"

// ToolPermissionMap maps tool names to their fine-grained permission
// requirement. It mirrors scopes.ToolScopeMap. Tools with a zero-value
// (empty) requirement are omitted, since an empty requirement means "no gate".
type ToolPermissionMap map[string]Requirement

// globalToolPermissionMap is populated from the inventory by the github package
// (github.SetToolPermissionMapFromInventory) so that middleware and other
// consumers can look up requirements without re-deriving them.
var globalToolPermissionMap ToolPermissionMap

// SetGlobalToolPermissionMap stores the global tool permission map.
func SetGlobalToolPermissionMap(m ToolPermissionMap) {
	globalToolPermissionMap = m
}

// GetToolPermissionMap returns the global tool permission map, or an empty map
// if it has not been populated yet.
func GetToolPermissionMap() ToolPermissionMap {
	if globalToolPermissionMap == nil {
		return make(ToolPermissionMap)
	}
	return globalToolPermissionMap
}

// GetToolRequirement returns the requirement for a single tool from the global
// map. The zero-value Requirement (no gate) is returned when the tool is
// unknown or ungated.
func GetToolRequirement(toolName string) Requirement {
	return GetToolPermissionMap()[toolName]
}

// UniquePermissions returns the sorted, de-duplicated set of permissions used
// across all tools in the map.
func (m ToolPermissionMap) UniquePermissions() []Permission {
	seen := make(map[Permission]struct{})
	for _, req := range m {
		for _, p := range req.Permissions() {
			seen[p] = struct{}{}
		}
	}
	out := make([]Permission, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}
