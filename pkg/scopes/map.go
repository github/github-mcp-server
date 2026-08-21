package scopes

import "github.com/github/github-mcp-server/pkg/inventory"

// ToolScopeMap maps tool names to their per-call scope checks.
type ToolScopeMap map[string]inventory.ScopeChallenge

var globalToolScopeMap ToolScopeMap

// SetToolScopeMapFromInventory builds and stores the scope checks from an inventory.
func SetToolScopeMapFromInventory(inv *inventory.Inventory) {
	globalToolScopeMap = GetToolScopeMapFromInventory(inv)
}

// SetGlobalToolScopeMap sets the scope map directly.
func SetGlobalToolScopeMap(m ToolScopeMap) {
	globalToolScopeMap = m
}

// GetToolScopeChallenge returns the scope check for a tool.
func GetToolScopeChallenge(toolName string) inventory.ScopeChallenge {
	return globalToolScopeMap[toolName]
}

// GetToolScopeMapFromInventory builds a scope map from an inventory.
func GetToolScopeMapFromInventory(inv *inventory.Inventory) ToolScopeMap {
	result := make(ToolScopeMap)
	for _, tool := range inv.AllTools() {
		if tool.ScopeAccess.Challenge != nil {
			result[tool.Tool.Name] = tool.ScopeAccess.Challenge
		}
	}
	return result
}
