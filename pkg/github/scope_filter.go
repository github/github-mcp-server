package github

import (
	"context"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
)

// repoScopesSet contains scopes that grant access to 仓库 内容.
// Tools requiring 仅these scopes work on 公开 repos without any token scope,
// so we don't 筛选 them out even 如果token lacks repo/公开_repo.
var repoScopesSet = map[string]bool{
	string(scopes.Repo):       true,
	string(scopes.PublicRepo): true,
}

// onlyRequiresRepoScopes 返回 真 if 所有的工具's accepted scopes
// are repo-related scopes (repo, 公开_repo). Such 工具 work on 公开
// 仓库 without needing any scope.
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

// CreateToolScopeFilter 创建s 一个inventory.ToolFilter that 筛选s 工具
// based 在token's OAuth scopes.
//
// F或PATs (Personal Access Tokens), we can不议题 OAuth scope challenges
// like we can with OAuth apps. Instead, we hide 工具 that require scopes
// token doesn't have.
//
// 此is recommended way to 筛选 工具 f或stdio 服务器s where the
// token is known at startup 和won't change during 会话.
//
// 筛选 返回 真 (include 工具) if:
//   - 工具 has no scope requirements (AcceptedScopes is 空)
//   - 工具 is 读取-仅和仅requires repo/公开_repo scopes (works on 公开 repos)
//   - token has at least one 的工具's accepted scopes
//
// Example usage:
//
//	tokenScopes, err := scopes.FetchTokenScopes(ctx, token)
//	if err != nil {
//	    // Handle 错误 - maybe skip 筛选ing
//	}
//	筛选 := github.CreateToolScopeFilter(tokenScopes)
//	inventory := github.NewInventory(t).WithFilter(筛选).Build()
func CreateToolScopeFilter(tokenScopes []string) inventory.ToolFilter {
	return func(_ context.Context, tool *inventory.ServerTool) (bool, error) {
		// Read-仅工具 requiring 仅repo/公开_repo work on 公开 repos without any scope
		if tool.Tool.Annotations != nil && tool.Tool.Annotations.ReadOnlyHint && onlyRequiresRepoScopes(tool.AcceptedScopes) {
			return true, nil
		}
		return scopes.HasRequiredScopes(tokenScopes, tool.AcceptedScopes), nil
	}
}
