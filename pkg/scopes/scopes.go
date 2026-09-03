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
	required = append([]Scope(nil), required...)
	requiredScopes := scopeStrings(required)
	return inventory.ScopeAccess{
		Scopes: append([]string(nil), requiredScopes...),
		Visible: func(activeScopes []string) bool {
			return HasAll(activeScopes, required...)
		},
		Challenge: func(_ map[string]any, activeScopes []string) []string {
			if HasAll(activeScopes, required...) {
				return nil
			}
			return append([]string(nil), requiredScopes...)
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

// DynamicChallenge creates an argument-dependent scope policy. maxScopes must
// exhaustively list every scope challenge can return, allowing middleware to
// skip argument decoding and challenge evaluation when they are already granted.
func DynamicChallenge(maxScopes []Scope, visible inventory.ScopeVisibility, challenge inventory.ScopeChallenge) inventory.ScopeAccess {
	if len(maxScopes) == 0 {
		panic("dynamic scope challenge requires exhaustive maximum scopes")
	}
	if challenge == nil {
		panic("dynamic scope challenge requires a callback")
	}
	return inventory.ScopeAccess{
		Scopes:    scopeStrings(maxScopes),
		Visible:   visible,
		Challenge: challenge,
		Dynamic:   true,
	}
}

// HasAll reports whether a token grants every requested scope.
func HasAll(activeScopes []string, required ...Scope) bool {
	for _, requiredScope := range required {
		if !hasScope(activeScopes, requiredScope) {
			return false
		}
	}
	return true
}

// HasAllScopeNames reports whether a token grants every requested scope name.
// It avoids materializing an expanded scope set on the request hot path.
func HasAllScopeNames(activeScopes, requiredScopes []string) bool {
	for _, requiredScope := range requiredScopes {
		if !hasScope(activeScopes, Scope(requiredScope)) {
			return false
		}
	}
	return true
}

func hasScope(activeScopes []string, required Scope) bool {
	for _, activeScope := range activeScopes {
		if scopeImplies(Scope(activeScope), required, len(ScopeHierarchy)+1) {
			return true
		}
	}
	return false
}

func scopeImplies(granted, required Scope, remaining int) bool {
	if granted == required {
		return true
	}
	if remaining == 0 {
		return false
	}
	for _, child := range ScopeHierarchy[granted] {
		if scopeImplies(child, required, remaining-1) {
			return true
		}
	}
	return false
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
