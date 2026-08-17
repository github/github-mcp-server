package scopes

import "github.com/github/github-mcp-server/pkg/inventory"

// ToolScopeMap maps tool names to their scope requirements.
type ToolScopeMap map[string]*ToolScopeInfo

// ToolScopeInfo contains scope information for a single tool.
type ToolScopeInfo struct {
	// RequiredScopes contains the scopes that are directly required by this tool.
	// They are ALL required (AND-of-ORs) and are the single source of truth for
	// allow/challenge decisions.
	RequiredScopes []string

	// AcceptedScopes contains the required scopes plus higher scopes from the
	// hierarchy. It is display-only metadata and is NOT used to make any
	// allow/challenge/missing decision.
	AcceptedScopes []string
}

// globalToolScopeMap is populated from inventory when SetToolScopeMapFromInventory is called
var globalToolScopeMap ToolScopeMap

// SetToolScopeMapFromInventory builds and stores a tool scope map from an inventory.
// This should be called after building the inventory to make scopes available for middleware.
func SetToolScopeMapFromInventory(inv *inventory.Inventory) {
	globalToolScopeMap = GetToolScopeMapFromInventory(inv)
}

// SetGlobalToolScopeMap sets the global tool scope map directly.
// This is useful for testing when you don't have a full inventory.
func SetGlobalToolScopeMap(m ToolScopeMap) {
	globalToolScopeMap = m
}

// GetToolScopeMap returns the global tool scope map.
// Returns an empty map if SetToolScopeMapFromInventory hasn't been called yet.
func GetToolScopeMap() (ToolScopeMap, error) {
	if globalToolScopeMap == nil {
		return make(ToolScopeMap), nil
	}
	return globalToolScopeMap, nil
}

// GetToolScopeInfo returns scope information for a specific tool from the global scope map.
func GetToolScopeInfo(toolName string) (*ToolScopeInfo, error) {
	m, err := GetToolScopeMap()
	if err != nil {
		return nil, err
	}
	return m[toolName], nil
}

// GetToolScopeMapFromInventory builds a tool scope map from an inventory.
// This extracts scope information from ServerTool.RequiredScopes and ServerTool.AcceptedScopes.
func GetToolScopeMapFromInventory(inv *inventory.Inventory) ToolScopeMap {
	result := make(ToolScopeMap)

	// Get all tools from the inventory (both enabled and disabled)
	// We need all tools for scope checking purposes
	allTools := inv.AllTools()
	for i := range allTools {
		tool := &allTools[i]
		if len(tool.RequiredScopes) > 0 || len(tool.AcceptedScopes) > 0 {
			result[tool.Tool.Name] = &ToolScopeInfo{
				RequiredScopes: tool.RequiredScopes,
				AcceptedScopes: tool.AcceptedScopes,
			}
		}
	}

	return result
}

// Satisfies reports whether the provided user scopes satisfy ALL of the tool's
// required scopes (AND-of-ORs). Each required scope may be satisfied directly or
// by a higher scope in the hierarchy, so the user scopes are expanded downward
// (via expandScopeSet) before checking. A tool with no required scopes is always
// satisfied.
//
// AcceptedScopes is display-only metadata and is intentionally not consulted.
func (t *ToolScopeInfo) Satisfies(userScopes ...string) bool {
	if t == nil || len(t.RequiredScopes) == 0 {
		return true // No scopes required
	}

	granted := expandScopeSet(userScopes)
	for _, required := range t.RequiredScopes {
		if !granted[required] {
			return false
		}
	}
	return true
}

// MissingScopes returns the subset of the tool's required scopes that the user
// scopes do not satisfy, after expanding the user scopes through the scope
// hierarchy. It returns nil when all required scopes are satisfied (or none are
// required). The result is the precise set of additional scopes needed for an
// OAuth scope challenge.
func (t *ToolScopeInfo) MissingScopes(userScopes ...string) []string {
	if t == nil || len(t.RequiredScopes) == 0 {
		return nil
	}

	granted := expandScopeSet(userScopes)
	var missing []string
	for _, required := range t.RequiredScopes {
		if !granted[required] {
			missing = append(missing, required)
		}
	}
	return missing
}

// GetRequiredScopesSlice returns the required scopes as a slice of strings.
func (t *ToolScopeInfo) GetRequiredScopesSlice() []string {
	if t == nil {
		return nil
	}
	scopes := make([]string, len(t.RequiredScopes))
	copy(scopes, t.RequiredScopes)
	return scopes
}
