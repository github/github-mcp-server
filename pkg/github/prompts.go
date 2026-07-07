package github

import (
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
)

// AllPrompts returns all prompts with their embedded toolset metadata.
// Prompt functions return ServerPrompt directly with toolset info.
func AllPrompts(t translations.TranslationHelperFunc) []inventory.ServerPrompt {
	return []inventory.ServerPrompt{
		// Issue prompts
		AssignCodingAgentPrompt(t),
		IssueToFixWorkflowPrompt(t),
	}
}
