package scopes

import "github.com/github/github-mcp-server/pkg/inventory"

// ToolScopeMap maps tool names to their scope requirements.
type ToolScopeMap map[string]*ToolScopeInfo

// ToolScopeInfo contains scope information for a single tool.
type ToolScopeInfo struct {
	// RequiredScopes contains the scopes that are directly required by this tool.
	RequiredScopes []string

	// AcceptedScopes contains all scopes that satisfy the requirements (including parent scopes).
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

// HasAcceptedScope checks if any of the provided user scopes satisfy the tool's requirements.
func (t *ToolScopeInfo) HasAcceptedScope(userScopes ...string) bool {
	if t == nil || len(t.AcceptedScopes) == 0 {
		return true // No scopes required
	}

	userScopeSet := make(map[string]bool)
	for _, scope := range userScopes {
		userScopeSet[scope] = true
	}

	for _, scope := range t.AcceptedScopes {
		if userScopeSet[scope] {
			return true
		}
	}
	return false
}

// MissingScopes returns the required scopes that are not present in the user's scopes.
func (t *ToolScopeInfo) MissingScopes(userScopes ...string) []string {
	if t == nil || len(t.RequiredScopes) == 0 {
		return nil
	}

	// Create a set of user scopes for O(1) lookup
	userScopeSet := make(map[string]bool, len(userScopes))
	for _, s := range userScopes {
		userScopeSet[s] = true
	}

	// Check if any accepted scope is present
	hasAccepted := false
	for _, scope := range t.AcceptedScopes {
		if userScopeSet[scope] {
			hasAccepted = true
			break
		}
	}

	if hasAccepted {
		return nil // User has sufficient scopes
	}

	// Return required scopes as the minimum needed
	missing := make([]string, len(t.RequiredScopes))
	copy(missing, t.RequiredScopes)
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
