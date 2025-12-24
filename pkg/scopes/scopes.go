package scopes

// Scope represents a GitHub OAuth scope.
// These constants define all OAuth scopes used by the GitHub MCP server tools.
// See https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps
type Scope string

const (
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
)

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
