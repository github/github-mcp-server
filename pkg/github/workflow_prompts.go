package github

import (
	"context"
	"fmt"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// IssueToFixWorkflowPrompt provides a guided workflow for creating an issue and then generating a PR to fix it
func IssueToFixWorkflowPrompt(t translations.TranslationHelperFunc) inventory.ServerPrompt {
	return inventory.NewServerPrompt(
		ToolsetMetadataIssues,
		mcp.Prompt{
			Name:        "issue_to_fix_workflow",
			Description: t("PROMPT_ISSUE_TO_FIX_WORKFLOW_DESCRIPTION", "Create an issue for a problem and then generate a pull request to fix it"),
			Arguments: []*mcp.PromptArgument{
				{
					Name:        "owner",
					Description: "Repository owner",
					Required:    true,
				},
				{
					Name:        "repo",
					Description: "Repository name",
					Required:    true,
				},
				{
					Name:        "title",
					Description: "Issue title",
					Required:    true,
				},
				{
					Name:        "description",
					Description: "Issue description",
					Required:    true,
				},
				{
					Name:        "labels",
					Description: "Comma-separated list of labels to apply (optional)",
					Required:    false,
				},
				{
					Name:        "assignees",
					Description: "Comma-separated list of assignees (optional)",
					Required:    false,
				},
			},
		},
		func(_ context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			owner := request.Params.Arguments["owner"]
			repo := request.Params.Arguments["repo"]
			title := request.Params.Arguments["title"]
			description := request.Params.Arguments["description"]

			labels := ""
			if l, exists := request.Params.Arguments["labels"]; exists {
				labels = fmt.Sprintf("%v", l)
			}

			assignees := ""
			if a, exists := request.Params.Arguments["assignees"]; exists {
				assignees = fmt.Sprintf("%v", a)
			}

			messages := []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: "You are a development workflow assistant helping to create GitHub issues and generate corresponding pull requests to fix them. You should: 1) Create a well-structured issue with clear problem description, 2) Assign it to Copilot coding agent to generate a solution, and 3) Monitor the PR creation process.",
					},
				},
				{
					Role: "user",
					Content: &mcp.TextContent{Text: fmt.Sprintf("I need to create an issue titled '%s' in %s/%s and then have a PR generated to fix it. The issue description is: %s%s%s",
						title, owner, repo, description,
						func() string {
							if labels != "" {
								return fmt.Sprintf("\n\nLabels to apply: %s", labels)
							}
							return ""
						}(),
						func() string {
							if assignees != "" {
								return fmt.Sprintf("\nAssignees: %s", assignees)
							}
							return ""
						}())},
				},
				{
					Role:    "assistant",
					Content: &mcp.TextContent{Text: fmt.Sprintf("I'll help you create the issue '%s' in %s/%s and then coordinate with Copilot to generate a fix. Let me start by creating the issue with the provided details.", title, owner, repo)},
				},
				{
					Role:    "user",
					Content: &mcp.TextContent{Text: "Perfect! Please:\n1. Create the issue with the title, description, labels, and assignees\n2. Once created, assign it to Copilot coding agent to generate a solution\n3. Monitor the process and let me know when the PR is ready for review"},
				},
				{
					Role:    "assistant",
					Content: &mcp.TextContent{Text: "Excellent plan! Here's what I'll do:\n\n1. ✅ Create the issue with all specified details\n2. 🤖 Assign to Copilot coding agent for automated fix\n3. 📋 Monitor progress and notify when PR is created\n4. 🔍 Provide PR details for your review\n\nLet me start by creating the issue."},
				},
			}
			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		},
	)
}
