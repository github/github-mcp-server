package github

import (
	"context"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
)

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
		return scopes.HasRequiredScopes(tokenScopes, tool.AcceptedScopes), nil
	}
}
