package scopes

import (
	"sort"

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

// ExpandScopes takes a list of required scopes and returns all accepted scopes
// including parent scopes from the hierarchy.
// For example, if "public_repo" is required, "repo" is also accepted since
// having the "repo" scope grants access to "public_repo".
// The returned slice is sorted for deterministic output.
func ExpandScopes(required ...Scope) []string {
	if len(required) == 0 {
		return nil
	}

	accepted := make(map[string]bool)

	// Add required scopes
	for _, scope := range required {
		accepted[string(scope)] = true
	}

	// Add every ancestor scope that grants access to a required scope. Iterate
	// to a fixed point so provider hierarchies can be arbitrary DAGs.
	for changed := true; changed; {
		changed = false
		for parent, children := range ScopeHierarchy {
			for _, child := range children {
				if accepted[string(child)] && !accepted[string(parent)] {
					accepted[string(parent)] = true
					changed = true
				}
			}
		}
	}

	// Convert to slice and sort for deterministic output
	result := make([]string, 0, len(accepted))
	for scope := range accepted {
		result = append(result, scope)
	}
	sort.Strings(result)
	return result
}

// NewScopeRequirement creates one capability requirement. The challenge scope
// and each explicitly accepted alternative are expanded to include ancestors.
func NewScopeRequirement(challenge Scope, alternatives ...Scope) inventory.ScopeRequirement {
	alternatives = append([]Scope{challenge}, alternatives...)
	return inventory.ScopeRequirement{
		ChallengeScope: string(challenge),
		AnyOf:          ExpandScopes(alternatives...),
	}
}

// AnyOfScopePolicy creates a policy where any one supplied scope is sufficient.
func AnyOfScopePolicy(required ...Scope) inventory.ScopePolicy {
	if len(required) == 0 {
		return UnscopedScopePolicy()
	}
	paths := make([]inventory.ScopePath, 0, len(required))
	for _, scope := range required {
		paths = append(paths, inventory.ScopePath{
			AllOf: []inventory.ScopeRequirement{NewScopeRequirement(scope)},
		})
	}
	return inventory.ScopePolicy{AnyOf: paths}
}

// AllOfScopePolicy creates a policy where every supplied scope is required.
func AllOfScopePolicy(required ...Scope) inventory.ScopePolicy {
	requirements := make([]inventory.ScopeRequirement, 0, len(required))
	for _, scope := range required {
		requirements = append(requirements, NewScopeRequirement(scope))
	}
	if len(requirements) == 0 {
		return UnscopedScopePolicy()
	}
	return inventory.ScopePolicy{
		AnyOf: []inventory.ScopePath{{AllOf: requirements}},
	}
}

// UnscopedScopePolicy creates an explicit policy that requires no OAuth scope.
func UnscopedScopePolicy() inventory.ScopePolicy {
	return inventory.ScopePolicy{
		AnyOf: []inventory.ScopePath{{}},
	}
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

// ScopePolicySatisfied reports whether the token satisfies at least one
// complete authorization alternative.
func ScopePolicySatisfied(tokenScopes []string, policy inventory.ScopePolicy) bool {
	if len(policy.AnyOf) == 0 {
		return true
	}
	granted := expandScopeSet(tokenScopes)
	for _, path := range policy.AnyOf {
		satisfied := true
		for _, requirement := range path.AllOf {
			if !scopeRequirementSatisfied(granted, requirement) {
				satisfied = false
				break
			}
		}
		if satisfied {
			return true
		}
	}
	return false
}

// ChallengeScopesForPolicy returns the preferred challenge scopes for the first
// declared authorization alternative. Alternative order is policy, not an
// inferred least-privilege ranking.
func ChallengeScopesForPolicy(tokenScopes []string, policy inventory.ScopePolicy) []string {
	if len(policy.AnyOf) == 0 {
		return nil
	}
	if ScopePolicySatisfied(tokenScopes, policy) {
		return nil
	}
	granted := expandScopeSet(tokenScopes)
	path := policy.AnyOf[0]
	missing := make([]string, 0, len(path.AllOf))
	seen := make(map[string]bool)
	for _, requirement := range path.AllOf {
		if scopeRequirementSatisfied(granted, requirement) || requirement.ChallengeScope == "" || seen[requirement.ChallengeScope] {
			continue
		}
		seen[requirement.ChallengeScope] = true
		missing = append(missing, requirement.ChallengeScope)
	}
	return missing
}

func scopeRequirementSatisfied(granted map[string]bool, requirement inventory.ScopeRequirement) bool {
	if requirement.ChallengeScope == "" {
		return true
	}
	if granted[requirement.ChallengeScope] {
		return true
	}
	if len(requirement.AnyOf) == 0 {
		return false
	}
	for _, scope := range requirement.AnyOf {
		if granted[scope] {
			return true
		}
	}
	return false
}
