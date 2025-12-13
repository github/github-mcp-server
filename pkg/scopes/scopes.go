// Package scopes provides OAuth scope definitions and utilities for the GitHub MCP Server.
// These scopes correspond to GitHub OAuth app scopes as documented at:
// https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps
package scopes

// Scope represents a GitHub OAuth scope.
type Scope string

// OAuth scope constants based on GitHub's OAuth app scopes.
// See: https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps
const (
	// NoScope grants read-only access to public information (including user profile info,
	// repository info, and gists).
	NoScope Scope = ""

	// Repo grants full access to public and private repositories including read and write
	// access to code, commit statuses, repository invitations, collaborators, deployment
	// statuses, and repository webhooks.
	Repo Scope = "repo"

	// RepoStatus grants read/write access to commit statuses in public and private repositories.
	RepoStatus Scope = "repo:status"

	// RepoDeployment grants access to deployment statuses for public and private repositories.
	RepoDeployment Scope = "repo_deployment"

	// PublicRepo limits access to public repositories. That includes read/write access to code,
	// commit statuses, repository projects, collaborators, and deployment statuses for public
	// repositories and organizations.
	PublicRepo Scope = "public_repo"

	// RepoInvite grants accept/decline abilities for invitations to collaborate on a repository.
	RepoInvite Scope = "repo:invite"

	// SecurityEvents grants read and write access to security events in the code scanning API.
	SecurityEvents Scope = "security_events"

	// AdminRepoHook grants read, write, ping, and delete access to repository hooks.
	AdminRepoHook Scope = "admin:repo_hook"

	// WriteRepoHook grants read, write, and ping access to hooks in public or private repositories.
	WriteRepoHook Scope = "write:repo_hook"

	// ReadRepoHook grants read and ping access to hooks in public or private repositories.
	ReadRepoHook Scope = "read:repo_hook"

	// AdminOrg grants full management of the organization and its teams, projects, and memberships.
	AdminOrg Scope = "admin:org"

	// WriteOrg grants read and write access to organization membership and organization projects.
	WriteOrg Scope = "write:org"

	// ReadOrg grants read-only access to organization membership, organization projects, and team membership.
	ReadOrg Scope = "read:org"

	// AdminPublicKey grants full management of public keys.
	AdminPublicKey Scope = "admin:public_key"

	// WritePublicKey grants create, list, and view details for public keys.
	WritePublicKey Scope = "write:public_key"

	// ReadPublicKey grants list and view details for public keys.
	ReadPublicKey Scope = "read:public_key"

	// AdminOrgHook grants read, write, ping, and delete access to organization hooks.
	AdminOrgHook Scope = "admin:org_hook"

	// Gist grants write access to gists.
	Gist Scope = "gist"

	// Notifications grants read access to a user's notifications, mark as read access to threads,
	// watch and unwatch access to a repository, and read, write, and delete access to thread subscriptions.
	Notifications Scope = "notifications"

	// User grants read/write access to profile info only.
	User Scope = "user"

	// ReadUser grants access to read a user's profile data.
	ReadUser Scope = "read:user"

	// UserEmail grants read access to a user's email addresses.
	UserEmail Scope = "user:email"

	// UserFollow grants access to follow or unfollow other users.
	UserFollow Scope = "user:follow"

	// Project grants read/write access to user and organization projects.
	Project Scope = "project"

	// ReadProject grants read only access to user and organization projects.
	ReadProject Scope = "read:project"

	// DeleteRepo grants access to delete adminable repositories.
	DeleteRepo Scope = "delete_repo"

	// WritePackages grants access to upload or publish a package in GitHub Packages.
	WritePackages Scope = "write:packages"

	// ReadPackages grants access to download or install packages from GitHub Packages.
	ReadPackages Scope = "read:packages"

	// DeletePackages grants access to delete packages from GitHub Packages.
	DeletePackages Scope = "delete:packages"

	// AdminGPGKey grants full management of GPG keys.
	AdminGPGKey Scope = "admin:gpg_key"

	// WriteGPGKey grants create, list, and view details for GPG keys.
	WriteGPGKey Scope = "write:gpg_key"

	// ReadGPGKey grants list and view details for GPG keys.
	ReadGPGKey Scope = "read:gpg_key"

	// Codespace grants the ability to create and manage codespaces.
	Codespace Scope = "codespace"

	// Workflow grants the ability to add and update GitHub Actions workflow files.
	Workflow Scope = "workflow"

	// ReadAuditLog grants read access to audit log data.
	ReadAuditLog Scope = "read:audit_log"
)

// String returns the string representation of the scope.
func (s Scope) String() string {
	return string(s)
}

// ScopeHierarchy defines which scopes include other scopes.
// For example, "repo" includes "repo:status", "repo_deployment", "public_repo", and "repo:invite".
// When a user has "repo" scope, they automatically have access to all included scopes.
var ScopeHierarchy = map[Scope][]Scope{
	Repo: {RepoStatus, RepoDeployment, PublicRepo, RepoInvite, SecurityEvents},
	User: {ReadUser, UserEmail, UserFollow},

	AdminRepoHook: {WriteRepoHook, ReadRepoHook},
	WriteRepoHook: {ReadRepoHook},

	AdminOrg: {WriteOrg, ReadOrg},
	WriteOrg: {ReadOrg},

	AdminPublicKey: {WritePublicKey, ReadPublicKey},
	WritePublicKey: {ReadPublicKey},

	AdminGPGKey: {WriteGPGKey, ReadGPGKey},
	WriteGPGKey: {ReadGPGKey},

	Project: {ReadProject},

	WritePackages: {ReadPackages},
}

// GetAcceptedScopes returns all scopes that satisfy the given required scope.
// This includes the scope itself plus any parent scopes that include it.
func GetAcceptedScopes(required Scope) []Scope {
	accepted := []Scope{required}
	seen := make(map[Scope]bool)
	seen[required] = true

	// Recursively find all parent scopes
	var findParents func(Scope)
	findParents = func(child Scope) {
		for parent, children := range ScopeHierarchy {
			for _, c := range children {
				if c == child && !seen[parent] {
					seen[parent] = true
					accepted = append(accepted, parent)
					findParents(parent) // Recursively find parents of this parent
				}
			}
		}
	}
	findParents(required)

	return accepted
}

// ScopeIncludes checks if a scope includes another scope (directly or through hierarchy).
func ScopeIncludes(have, need Scope) bool {
	if have == need {
		return true
	}

	// Check if 'have' directly includes 'need'
	if children, ok := ScopeHierarchy[have]; ok {
		for _, child := range children {
			if child == need {
				return true
			}
			// Recursively check
			if ScopeIncludes(child, need) {
				return true
			}
		}
	}

	return false
}

// HasRequiredScopes checks if the given scopes satisfy all required scopes.
func HasRequiredScopes(have []Scope, required []Scope) bool {
	for _, req := range required {
		found := false
		for _, h := range have {
			if ScopeIncludes(h, req) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// ScopeStrings converts a slice of Scope to a slice of strings.
func ScopeStrings(scopes []Scope) []string {
	result := make([]string, len(scopes))
	for i, s := range scopes {
		result[i] = s.String()
	}
	return result
}

// ParseScopes converts a slice of strings to a slice of Scope.
func ParseScopes(strs []string) []Scope {
	result := make([]Scope, len(strs))
	for i, s := range strs {
		result[i] = Scope(s)
	}
	return result
}

// MetaKey is the key used to store OAuth scopes in the mcp.Tool.Meta field.
const MetaKey = "requiredOAuthScopes"

// WithScopes returns a Meta map containing the required OAuth scopes.
// This is used when defining an mcp.Tool to specify the required scopes.
//
// Example usage:
//
//	tool := mcp.Tool{
//	    Name: "get_issue",
//	    Meta: scopes.WithScopes(scopes.Repo),
//	    ...
//	}
func WithScopes(requiredScopes ...Scope) map[string]any {
	scopeStrings := make([]string, len(requiredScopes))
	for i, s := range requiredScopes {
		scopeStrings[i] = s.String()
	}
	return map[string]any{
		MetaKey: scopeStrings,
	}
}

// GetScopesFromMeta extracts the required OAuth scopes from an mcp.Tool.Meta field.
// Returns nil if no scopes are defined.
func GetScopesFromMeta(meta map[string]any) []Scope {
	if meta == nil {
		return nil
	}

	scopesVal, ok := meta[MetaKey]
	if !ok {
		return nil
	}

	// Handle both []string and []any (from JSON unmarshaling)
	switch v := scopesVal.(type) {
	case []string:
		return ParseScopes(v)
	case []any:
		strs := make([]string, len(v))
		for i, s := range v {
			if str, ok := s.(string); ok {
				strs[i] = str
			}
		}
		return ParseScopes(strs)
	default:
		return nil
	}
}
