package github

import (
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
)

// AllPrompts 返回 所有提示 with their embedded 工具集 元数据.
// Prompt 函数s 返回 ServerPrompt directly with 工具集 info.
func AllPrompts(t translations.TranslationHelperFunc) []inventory.ServerPrompt {
	return []inventory.ServerPrompt{
		// Issue 提示
		AssignCodingAgentPrompt(t),
		IssueToFixWorkflowPrompt(t),
	}
}
