package github

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/utils"
)

// Restriction messages returned when lockdown mode withholds content from a read tool.
const (
	lockdownPullRequestRestrictedMessage = "access to pull request is restricted by lockdown mode"
	lockdownIssueRestrictedMessage       = "access to issue details is restricted by lockdown mode"
)

// authorLockdownResult returns a restricted tool result when content authored by
// authorLogin cannot be surfaced for owner/repo under lockdown mode, and (nil, nil)
// when access is permitted. It should only be called when lockdown mode is enabled.
// It fails closed: a missing cache, an empty author, or a lookup error denies access.
func authorLockdownResult(ctx context.Context, cache *lockdown.RepoAccessCache, owner, repo, authorLogin, restrictedMessage string) (*mcp.CallToolResult, error) {
	if cache == nil {
		return nil, fmt.Errorf("lockdown cache is not configured")
	}
	if authorLogin == "" {
		return utils.NewToolResultError(restrictedMessage), nil
	}
	isSafeContent, err := cache.IsSafeContent(ctx, authorLogin, owner, repo)
	if err != nil {
		return utils.NewToolResultError(fmt.Sprintf("failed to check lockdown mode: %v", err)), nil
	}
	if !isSafeContent {
		return utils.NewToolResultError(restrictedMessage), nil
	}
	return nil, nil
}
