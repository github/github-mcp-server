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

// onlyRequiresRepoScopes returns true if all of the tool's accepted scopes
// are repo-related scopes (repo, public_repo). Such tools work on public
// repositories without needing any scope.
func onlyRequiresRepoScopes(acceptedScopes []string) bool {
	if len(acceptedScopes) == 0 {
		return false
	}
	for _, scope := range acceptedScopes {
		if !repoScopesSet[scope] {
			return false
		}
	}
	return true
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
//   - The tool has no scope requirements (AcceptedScopes is empty)
//   - The tool is read-only and only requires repo/public_repo scopes (works on public repos)
//   - The token has at least one of the tool's accepted scopes
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
		// Read-only tools requiring only repo/public_repo work on public repos without any scope
		if tool.Tool.Annotations != nil && tool.Tool.Annotations.ReadOnlyHint && onlyRequiresRepoScopes(tool.AcceptedScopes) {
			return true, nil
		}
		return scopes.HasRequiredScopes(tokenScopes, tool.AcceptedScopes), nil
	}
}
