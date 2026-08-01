package github

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/utils"
)

// Restriction messages 返回ed when lockdown mode withholds 内容 from 一个读取 工具.
const (
	lockdownPullRequestRestrictedMessage = "access to pull request is restricted by lockdown mode"
	lockdownIssueRestrictedMessage       = "access to issue details is restricted by lockdown mode"
)

// authorLockdownResult 返回 一个restricted 工具 结果 when 内容 authored by
// authorLogin can不be surfaced f或owner/repo under lockdown mode, 和(nil, nil)
// when access is permitted. It should 仅be 调用ed when lockdown mode is 启用.
// It fails closed: 一个missing cache, 一个空 author, 或一个lookup 错误 denies access.
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
