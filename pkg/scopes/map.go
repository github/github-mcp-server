package scopes

import "github.com/github/github-mcp-server/pkg/inventory"

// ToolScopeMap maps tool names to their complete scope access policies.
type ToolScopeMap map[string]inventory.ScopeAccess

// ToolScopeAccess is the immutable request-time subset of a tool scope policy.
// Its maximum scopes stay private so callers cannot mutate the global lookup.
type ToolScopeAccess struct {
	maxScopes []string
	challenge inventory.ScopeChallenge
}

var globalToolScopeMap map[string]ToolScopeAccess

// SetToolScopeMapFromInventory builds and stores the scope checks from an inventory.
func SetToolScopeMapFromInventory(inv *inventory.Inventory) {
	SetGlobalToolScopeMap(GetToolScopeMapFromInventory(inv))
}

// SetGlobalToolScopeMap sets the scope map directly.
func SetGlobalToolScopeMap(m ToolScopeMap) {
	if m == nil {
		globalToolScopeMap = nil
		return
	}
	globalToolScopeMap = make(map[string]ToolScopeAccess, len(m))
	for name, access := range m {
		if access.Challenge == nil {
			continue
		}
		if access.Dynamic && len(access.Scopes) == 0 {
			panic("dynamic scope challenge requires exhaustive maximum scopes")
		}
		globalToolScopeMap[name] = ToolScopeAccess{
			maxScopes: append([]string(nil), access.Scopes...),
			challenge: access.Challenge,
		}
	}
}

// GetToolScopeAccess returns the immutable request-time scope policy for a tool.
func GetToolScopeAccess(toolName string) (ToolScopeAccess, bool) {
	access, ok := globalToolScopeMap[toolName]
	return access, ok
}

// MaximumScopesSatisfied reports whether activeScopes grants the exhaustive
// upper bound for this policy.
func (access ToolScopeAccess) MaximumScopesSatisfied(activeScopes []string) bool {
	return len(access.maxScopes) > 0 && HasAllScopeNames(activeScopes, access.maxScopes)
}

// ResolveChallenge evaluates the call-specific policy.
func (access ToolScopeAccess) ResolveChallenge(arguments map[string]any, activeScopes []string) []string {
	return access.challenge(arguments, activeScopes)
}

// MaximumScopes returns a copy of the exhaustive upper bound.
func (access ToolScopeAccess) MaximumScopes() []string {
	return append([]string(nil), access.maxScopes...)
}

// GetToolScopeMapFromInventory builds a scope map from an inventory.
func GetToolScopeMapFromInventory(inv *inventory.Inventory) ToolScopeMap {
	result := make(ToolScopeMap)
	for _, tool := range inv.AllTools() {
		if tool.ScopeAccess.Challenge != nil {
			access := tool.ScopeAccess
			access.Scopes = append([]string(nil), access.Scopes...)
			result[tool.Tool.Name] = access
		}
	}
	return result
}
