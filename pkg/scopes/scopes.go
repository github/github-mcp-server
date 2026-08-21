package scopes

import (
	"github.com/github/github-mcp-server/pkg/inventory"
)

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

	// DeleteRepo grants permission to delete repositories
	DeleteRepo Scope = "delete_repo"

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

	// Workflow grants permission to update GitHub Actions workflow files
	Workflow Scope = "workflow"

	// Codespace grants full control of codespaces
	Codespace Scope = "codespace"
)

type oauthScopeDefinition struct {
	scope     Scope
	byDefault bool
}

var oauthScopeDefinitions = []oauthScopeDefinition{
	{scope: Repo, byDefault: true},
	{scope: DeleteRepo},
	{scope: ReadOrg, byDefault: true},
	{scope: ReadUser, byDefault: true},
	{scope: UserEmail, byDefault: true},
	{scope: ReadPackages, byDefault: true},
	{scope: WritePackages, byDefault: true},
	{scope: ReadProject, byDefault: true},
	{scope: Project, byDefault: true},
	{scope: Gist, byDefault: true},
	{scope: Notifications, byDefault: true},
	{scope: Workflow},
	{scope: Codespace},
}

// SupportedOAuthScopes returns every OAuth scope the server may request.
func SupportedOAuthScopes() []string {
	return oauthScopes(false)
}

// DefaultOAuthScopes returns the lower-risk scopes requested by default.
func DefaultOAuthScopes() []string {
	return oauthScopes(true)
}

func oauthScopes(defaultOnly bool) []string {
	result := make([]string, 0, len(oauthScopeDefinitions))
	for _, definition := range oauthScopeDefinitions {
		if !defaultOnly || definition.byDefault {
			result = append(result, string(definition.scope))
		}
	}
	return result
}

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

// RequireAll creates scope checks for a tool that always needs the given scopes.
func RequireAll(required ...Scope) inventory.ScopeAccess {
	scopes := scopeStrings(required)
	return inventory.ScopeAccess{
		Scopes: scopes,
		Visible: func(activeScopes []string) bool {
			return HasAll(activeScopes, required...)
		},
		Challenge: func(_ map[string]any, activeScopes []string) []string {
			if HasAll(activeScopes, required...) {
				return nil
			}
			return append([]string(nil), scopes...)
		},
	}
}

// PublicRead creates checks for a read-only operation that may target public data.
func PublicRead(required ...Scope) inventory.ScopeAccess {
	access := RequireAll(required...)
	access.Visible = func([]string) bool { return true }
	return access
}

// NoScopes creates scope checks for a tool that does not need OAuth scopes.
func NoScopes() inventory.ScopeAccess {
	return inventory.ScopeAccess{}
}

// HasAll reports whether a token grants every requested scope.
func HasAll(activeScopes []string, required ...Scope) bool {
	granted := expandScopeSet(activeScopes)
	for _, scope := range required {
		if !granted[string(scope)] {
			return false
		}
	}
	return true
}

// ChallengeAll returns the complete scope set for an operation, or nil when
// the active token already grants every scope.
func ChallengeAll(activeScopes []string, required ...Scope) []string {
	if HasAll(activeScopes, required...) {
		return nil
	}
	return scopeStrings(required)
}

func scopeStrings(scopes []Scope) []string {
	result := make([]string, len(scopes))
	for i, scope := range scopes {
		result[i] = string(scope)
	}
	return result
}

// expandScopeSet returns a set of all scopes granted by the given scopes,
// including child scopes from the hierarchy.
// For example, if "repo" is provided, the result includes "repo", "public_repo",
// and "security_events" since "repo" grants access to those child scopes.
func expandScopeSet(scopes []string) map[string]bool {
	expanded := make(map[string]bool, len(scopes))
	queue := append([]string(nil), scopes...)
	for len(queue) > 0 {
		scope := queue[0]
		queue = queue[1:]
		if expanded[scope] {
			continue
		}
		expanded[scope] = true
		for _, child := range ScopeHierarchy[Scope(scope)] {
			if !expanded[string(child)] {
				queue = append(queue, string(child))
			}
		}
	}
	return expanded
}
