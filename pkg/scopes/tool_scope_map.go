package scopes

// ToolScopeMap represents the scope lookup data for fast access in production.
// Keys are tool names, values contain required and accepted scopes as maps for O(1) lookup.
type ToolScopeMap map[string]*ToolScopeInfo

// AllRequiredScopes returns all unique required scopes across all tools.
func (m ToolScopeMap) AllRequiredScopes() ScopeSet {
	result := make(ScopeSet)
	for _, info := range m {
		for scope := range info.RequiredScopes {
			result[scope] = true
		}
	}
	return result
}

// AllAcceptedScopes returns all unique accepted scopes across all tools.
func (m ToolScopeMap) AllAcceptedScopes() ScopeSet {
	result := make(ScopeSet)
	for _, info := range m {
		for scope := range info.AcceptedScopes {
			result[scope] = true
		}
	}
	return result
}

// ToolsRequiringScope returns all tool names that require the given scope.
func (m ToolScopeMap) ToolsRequiringScope(scope string) []string {
	var tools []string
	for name, info := range m {
		if info.RequiredScopes.Contains(scope) {
			tools = append(tools, name)
		}
	}
	return tools
}

// ToolsAcceptingScope returns all tool names that accept the given scope.
func (m ToolScopeMap) ToolsAcceptingScope(scope string) []string {
	var tools []string
	for name, info := range m {
		if info.AcceptedScopes.Contains(scope) {
			tools = append(tools, name)
		}
	}
	return tools
}

// ToolScopeInfo contains scope information for a single tool optimized for fast lookup.
type ToolScopeInfo struct {
	// RequiredScopes contains the scopes that are directly required by this tool.
	// Map values are always true for O(1) lookup.
	RequiredScopes ScopeSet `json:"required_scopes"`

	// AcceptedScopes contains all scopes that satisfy the requirements (including required scopes).
	// This includes parent scopes that implicitly grant access through the hierarchy.
	// Map values are always true for O(1) lookup.
	AcceptedScopes ScopeSet `json:"accepted_scopes"`
}

// ScopeSet is a set of scopes for fast O(1) lookup.
// All values are true.
type ScopeSet map[string]bool

// Contains checks if the scope set contains the given scope.
func (s ScopeSet) Contains(scope string) bool {
	return s[scope]
}

// ContainsAny checks if the scope set contains any of the given scopes.
func (s ScopeSet) ContainsAny(scopes ...string) bool {
	for _, scope := range scopes {
		if s[scope] {
			return true
		}
	}
	return false
}

// ToSlice converts the scope set to a slice of scope strings.
func (s ScopeSet) ToSlice() []string {
	result := make([]string, 0, len(s))
	for scope := range s {
		result = append(result, scope)
	}
	return result
}

// NewScopeSet creates a new ScopeSet from a slice of strings.
func NewScopeSet(scopes []string) ScopeSet {
	set := make(ScopeSet, len(scopes))
	for _, s := range scopes {
		set[s] = true
	}
	return set
}

// NewToolScopeInfo creates a ToolScopeInfo from required scopes.
// It automatically calculates accepted scopes based on the scope hierarchy.
func NewToolScopeInfo(required []Scope) *ToolScopeInfo {
	// Build required scopes set
	requiredSet := make(ScopeSet, len(required))
	for _, s := range required {
		requiredSet[s.String()] = true
	}

	// Build accepted scopes set (includes required + parent scopes from hierarchy)
	acceptedSet := make(ScopeSet)
	for _, reqScope := range required {
		// Add the required scope itself
		acceptedSet[reqScope.String()] = true
		// Add all parent scopes that satisfy this requirement
		accepted := GetAcceptedScopes(reqScope)
		for _, accScope := range accepted {
			acceptedSet[accScope.String()] = true
		}
	}

	return &ToolScopeInfo{
		RequiredScopes: requiredSet,
		AcceptedScopes: acceptedSet,
	}
}

// HasAcceptedScope checks if any of the provided scopes satisfy this tool's requirements.
// This is the primary method for checking if a user's token has sufficient permissions.
func (t *ToolScopeInfo) HasAcceptedScope(userScopes ...string) bool {
	// If the tool requires no scopes, any token is acceptable
	if len(t.RequiredScopes) == 0 {
		return true
	}

	// Check if any of the user's scopes are in the accepted set
	for _, scope := range userScopes {
		if t.AcceptedScopes[scope] {
			return true
		}
	}
	return false
}

// MissingScopes returns the required scopes that are not satisfied by the given user scopes.
// Returns nil if all requirements are satisfied.
func (t *ToolScopeInfo) MissingScopes(userScopes ...string) []string {
	if len(t.RequiredScopes) == 0 {
		return nil
	}

	// Build a set of user scopes for fast lookup
	userSet := NewScopeSet(userScopes)

	// Check each required scope
	var missing []string
	for requiredScope := range t.RequiredScopes {
		// Check if any accepted scope for this requirement is present
		found := false
		accepted := GetAcceptedScopes(Scope(requiredScope))
		for _, accScope := range accepted {
			if userSet[accScope.String()] {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, requiredScope)
		}
	}

	return missing
}

// BuildToolScopeMap builds a ToolScopeMap from a slice of tool definitions.
// This is the primary function for library users to build the scope lookup map.
//
// Example usage:
//
//	// Get tools from your toolset group
//	tools := []struct{ Name string; Meta map[string]any }{...}
//
//	// Build the map
//	scopeMap := scopes.BuildToolScopeMapFromMeta(tools)
//
//	// Check if a user's token has access to a tool
//	if info, ok := scopeMap["create_issue"]; ok {
//	    if info.HasAcceptedScope(userScopes...) {
//	        // User can use this tool
//	    }
//	}
func BuildToolScopeMapFromMeta(tools []ToolMeta) ToolScopeMap {
	result := make(ToolScopeMap, len(tools))
	for _, tool := range tools {
		required := GetScopesFromMeta(tool.Meta)
		result[tool.Name] = NewToolScopeInfo(required)
	}
	return result
}

// ToolMeta is a minimal interface for tools that have scope metadata.
type ToolMeta struct {
	Name string
	Meta map[string]any
}

// GetToolScopeInfo creates a ToolScopeInfo directly from a tool's Meta map.
func GetToolScopeInfo(meta map[string]any) *ToolScopeInfo {
	required := GetScopesFromMeta(meta)
	return NewToolScopeInfo(required)
}
