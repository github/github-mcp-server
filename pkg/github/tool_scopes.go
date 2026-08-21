package github

import (
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
)

func repositoryOrOrganizationScopePolicy(arguments map[string]any) inventory.ScopePolicy {
	if owner, ok := arguments["owner"].(string); !ok || owner == "" {
		return scopes.UnscopedScopePolicy()
	}
	repoValue, hasRepo := arguments["repo"]
	if !hasRepo || repoValue == nil || repoValue == "" {
		return scopes.AllOfScopePolicy(scopes.ReadOrg)
	}
	repo, ok := repoValue.(string)
	if !ok {
		return scopes.UnscopedScopePolicy()
	}
	if repo != "" {
		return scopes.AllOfScopePolicy(scopes.Repo)
	}
	return scopes.AllOfScopePolicy(scopes.ReadOrg)
}

func uiGetScopePolicy(arguments map[string]any) inventory.ScopePolicy {
	if owner, ok := arguments["owner"].(string); !ok || owner == "" {
		return scopes.UnscopedScopePolicy()
	}
	method, ok := arguments["method"].(string)
	if !ok {
		return scopes.UnscopedScopePolicy()
	}
	if method == "issue_types" {
		return scopes.AllOfScopePolicy(scopes.ReadOrg)
	}
	switch method {
	case "labels", "assignees", "milestones", "branches", "issue_fields", "reviewers":
		if repo, ok := arguments["repo"].(string); ok && repo != "" {
			return scopes.AllOfScopePolicy(scopes.Repo)
		}
	}
	return scopes.UnscopedScopePolicy()
}
