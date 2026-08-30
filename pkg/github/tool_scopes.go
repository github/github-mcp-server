package github

import (
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
)

func repositoryOrOrganizationScopeAccess() inventory.ScopeAccess {
	return scopes.DynamicChallenge(
		[]scopes.Scope{scopes.Repo, scopes.ReadOrg},
		func([]string) bool {
			// Repository reads may target public repositories.
			return true
		},
		func(arguments map[string]any, activeScopes []string) []string {
			if owner, ok := arguments["owner"].(string); !ok || owner == "" {
				return nil
			}
			repoValue, hasRepo := arguments["repo"]
			if !hasRepo || repoValue == nil || repoValue == "" {
				return scopes.ChallengeAll(activeScopes, scopes.ReadOrg)
			}
			if _, ok := repoValue.(string); !ok {
				return nil
			}
			return scopes.ChallengeAll(activeScopes, scopes.Repo)
		},
	)
}

func uiGetScopeAccess() inventory.ScopeAccess {
	return scopes.DynamicChallenge(
		[]scopes.Scope{scopes.Repo, scopes.ReadOrg},
		func([]string) bool {
			return true
		},
		func(arguments map[string]any, activeScopes []string) []string {
			if owner, ok := arguments["owner"].(string); !ok || owner == "" {
				return nil
			}
			method, ok := arguments["method"].(string)
			if !ok {
				return nil
			}
			if method == "issue_types" {
				return scopes.ChallengeAll(activeScopes, scopes.ReadOrg)
			}
			if method == "reviewers" {
				if repo, ok := arguments["repo"].(string); ok && repo != "" {
					return scopes.ChallengeAll(activeScopes, scopes.Repo, scopes.ReadOrg)
				}
				return nil
			}
			switch method {
			case "labels", "assignees", "milestones", "branches", "issue_fields":
				if repo, ok := arguments["repo"].(string); ok && repo != "" {
					return scopes.ChallengeAll(activeScopes, scopes.Repo)
				}
			}
			return nil
		},
	)
}
