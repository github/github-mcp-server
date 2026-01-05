package scopes

// Scope represents a GitHub OAuth scope.
// These constants define all OAuth scopes used by the GitHub MCP server tools.
// See https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps
type Scope string

const (
	// NoScope indicates no scope is required (public access).
	NoScope Scope = ""

	// Repo grants full control of private repositories
	Repo Scope = "repo"

	// PublicRepo grants access to public repositories
	PublicRepo Scope = "public_repo"

	// ReadOrg grants read-only access to organization membership, teams, and projects
	ReadOrg Scope = "read:org"

	// WriteOrg grants write access to organization membership and teams
	WriteOrg Scope = "write:org"

	// AdminOrg grants full control of organizations and teams
	AdminOrg Scope = "admin:org"

	// Gist grants write access to gists
	Gist Scope = "gist"

	// Notifications grants access to notifications
	Notifications Scope = "notifications"

	// ReadProject grants read-only access to projects
	ReadProject Scope = "read:project"

	// Project grants full control of projects
	Project Scope = "project"

	// SecurityEvents grants read and write access to security events
	SecurityEvents Scope = "security_events"

	// User grants read/write access to profile info
	User Scope = "user"

	// ReadUser grants read-only access to profile info
	ReadUser Scope = "read:user"

	// UserEmail grants read access to user email addresses
	UserEmail Scope = "user:email"

	// ReadPackages grants read access to packages
	ReadPackages Scope = "read:packages"

	// WritePackages grants write access to packages
	WritePackages Scope = "write:packages"
)

// ScopeHierarchy defines parent-child relationships between scopes.
// A parent scope implicitly grants access to all child scopes.
// For example, "repo" grants access to "public_repo" and "security_events".
var ScopeHierarchy = map[Scope][]Scope{
	Repo:          {PublicRepo, SecurityEvents},
	AdminOrg:      {WriteOrg, ReadOrg},
	WriteOrg:      {ReadOrg},
	Project:       {ReadProject},
	WritePackages: {ReadPackages},
	User:          {ReadUser, UserEmail},
}

// ScopeSet represents a set of OAuth scopes.
type ScopeSet map[Scope]bool

// NewScopeSet creates a new ScopeSet from the given scopes.
func NewScopeSet(scopes ...Scope) ScopeSet {
	set := make(ScopeSet)
	for _, scope := range scopes {
		set[scope] = true
	}
	return set
}

// ToSlice converts a ScopeSet to a slice of Scope values.
func (s ScopeSet) ToSlice() []Scope {
	scopes := make([]Scope, 0, len(s))
	for scope := range s {
		scopes = append(scopes, scope)
	}
	return scopes
}

// ToStringSlice converts a ScopeSet to a slice of string values.
func (s ScopeSet) ToStringSlice() []string {
	scopes := make([]string, 0, len(s))
	for scope := range s {
		scopes = append(scopes, string(scope))
	}
	return scopes
}

// ToStringSlice converts a slice of Scopes to a slice of strings.
func ToStringSlice(scopes ...Scope) []string {
	result := make([]string, len(scopes))
	for i, scope := range scopes {
		result[i] = string(scope)
	}
	return result
}

// ExpandScopes takes a list of required scopes and returns all accepted scopes
// including parent scopes from the hierarchy.
// For example, if "public_repo" is required, "repo" is also accepted since
// having the "repo" scope grants access to "public_repo".
func ExpandScopes(required ...Scope) []string {
	if len(required) == 0 {
		return nil
	}

	accepted := make(map[string]bool)

	// Add required scopes
	for _, scope := range required {
		accepted[string(scope)] = true
	}

	// Add parent scopes that grant access to required scopes
	for parent, children := range ScopeHierarchy {
		for _, child := range children {
			if accepted[string(child)] {
				accepted[string(parent)] = true
			}
		}
	}

	// Convert to slice
	result := make([]string, 0, len(accepted))
	for scope := range accepted {
		result = append(result, scope)
	}
	return result
}
