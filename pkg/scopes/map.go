package scopes

import (
	"github.com/github/github-mcp-server/pkg/inventory"
)

// ToolScopeMap maps tool names to their scope requirements.
type ToolScopeMap map[string]*ToolScopeInfo

// ToolScopeInfo contains scope information for a single tool.
type ToolScopeInfo struct {
	// Policy describes every supported authorization path for the tool.
	ScopePolicy inventory.ScopePolicy

	// ScopeResolver returns the exact policy for a call.
	ScopeResolver inventory.ScopeResolver
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
func GetToolScopeMapFromInventory(inv *inventory.Inventory) ToolScopeMap {
	result := make(ToolScopeMap)

	// Get all tools from the inventory (both enabled and disabled)
	// We need all tools for scope checking purposes
	allTools := inv.AllTools()
	for i := range allTools {
		tool := &allTools[i]
		if len(tool.ScopePolicy.AnyOf) > 0 || tool.ScopeResolver != nil {
			result[tool.Tool.Name] = &ToolScopeInfo{
				ScopePolicy:   tool.ScopePolicy,
				ScopeResolver: tool.ScopeResolver,
			}
		}
	}

	return result
}

// Resolve returns the scope requirements for a specific call.
func (t *ToolScopeInfo) Resolve(arguments map[string]any) *ToolScopeInfo {
	if t == nil || t.ScopeResolver == nil {
		return t
	}
	resolved := *t
	resolved.ScopePolicy = t.ScopeResolver(arguments)
	return &resolved
}

// Satisfies checks whether the provided scopes satisfy the tool policy.
func (t *ToolScopeInfo) Satisfies(userScopes ...string) bool {
	if t == nil {
		return true
	}
	return ScopePolicySatisfied(userScopes, t.ScopePolicy)
}

// ChallengeScopes returns the preferred scopes needed for the first declared
// authorization alternative.
func (t *ToolScopeInfo) ChallengeScopes(userScopes ...string) []string {
	if t == nil {
		return nil
	}
	return ChallengeScopesForPolicy(userScopes, t.ScopePolicy)
}
