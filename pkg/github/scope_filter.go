package github

import (
	"context"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
)

// repoScopesSet contains scopes that grant access to repository content.
// Tools requiring only these scopes work on public repos without any token scope,
// so we don't filter them out even if the token lacks repo/public_repo.
var repoScopesSet = map[string]bool{
	string(scopes.Repo):       true,
	string(scopes.PublicRepo): true,
}

// hasRepoOnlyScopeAlternative returns true if at least one authorization path
// uses only repo-related scopes. Such a path may work on a public repository
// without any token scope.
func hasRepoOnlyScopeAlternative(policy inventory.ScopePolicy) bool {
	for _, alternative := range policy.AnyOf {
		if len(alternative.AllOf) == 0 {
			continue
		}
		repoOnly := true
		for _, requirement := range alternative.AllOf {
			accepted := requirement.AnyOf
			if len(accepted) == 0 {
				accepted = []string{requirement.ChallengeScope}
			}
			for _, scope := range accepted {
				if !repoScopesSet[scope] {
					repoOnly = false
					break
				}
			}
			if !repoOnly {
				break
			}
		}
		if repoOnly {
			return true
		}
	}
	return false
}

// CreateToolScopeFilter creates an inventory.ToolFilter that filters tools
// based on the token's OAuth scopes.
//
// For PATs (Personal Access Tokens), we cannot issue OAuth scope challenges
// like we can with OAuth apps. Instead, we hide tools that require scopes
// the token doesn't have.
//
// This is the recommended way to filter tools for stdio servers where the
// token is known at startup and won't change during the session.
//
// The filter returns true (include tool) if:
//   - The tool has an unscoped authorization path
//   - The tool is read-only and only requires repo/public_repo scopes (works on public repos)
//   - The token satisfies every requirement in any authorization path
//
// Example usage:
//
//	tokenScopes, err := scopes.FetchTokenScopes(ctx, token)
//	if err != nil {
//	    // Handle error - maybe skip filtering
//	}
//	filter := github.CreateToolScopeFilter(tokenScopes)
//	inventory := github.NewInventory(t).WithFilter(filter).Build()
func CreateToolScopeFilter(tokenScopes []string) inventory.ToolFilter {
	return func(_ context.Context, tool *inventory.ServerTool) (bool, error) {
		policy := tool.ScopePolicy
		// Read-only tools requiring only repo/public_repo work on public repos without any scope
		if tool.Tool.Annotations != nil && tool.Tool.Annotations.ReadOnlyHint && hasRepoOnlyScopeAlternative(policy) {
			return true, nil
		}
		return scopes.ScopePolicySatisfied(tokenScopes, policy), nil
	}
}
