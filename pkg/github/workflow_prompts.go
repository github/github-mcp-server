package github

import (
	"context"
	"fmt"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// IssueToFixWorkflowPrompt provides a guided workflow for creating an issue and then generating a PR to fix it
func IssueToFixWorkflowPrompt(t translations.TranslationHelperFunc) (tool mcp.Prompt, handler server.PromptHandlerFunc) {
	return mcp.NewPrompt("IssueToFixWorkflow",
			mcp.WithPromptDescription(t("PROMPT_ISSUE_TO_FIX_WORKFLOW_DESCRIPTION", "Create an issue for a problem and then generate a pull request to fix it")),
			mcp.WithArgument("owner", mcp.ArgumentDescription("Repository owner"), mcp.RequiredArgument()),
			mcp.WithArgument("repo", mcp.ArgumentDescription("Repository name"), mcp.RequiredArgument()),
			mcp.WithArgument("title", mcp.ArgumentDescription("Issue title"), mcp.RequiredArgument()),
			mcp.WithArgument("description", mcp.ArgumentDescription("Issue description"), mcp.RequiredArgument()),
			mcp.WithArgument("labels", mcp.ArgumentDescription("Comma-separated list of labels to apply (optional)")),
			mcp.WithArgument("assignees", mcp.ArgumentDescription("Comma-separated list of assignees (optional)")),
		), func(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
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

			messages := []mcp.PromptMessage{
				{
					Role:    "user",
					Content: mcp.NewTextContent("You are a development workflow assistant helping to create GitHub issues and generate corresponding pull requests to fix them. You should: 1) Create a well-structured issue with clear problem description, 2) Assign it to Copilot coding agent to generate a solution, and 3) Monitor the PR creation process."),
				},
				{
					Role: "user",
					Content: mcp.NewTextContent(fmt.Sprintf("I need to create an issue titled '%s' in %s/%s and then have a PR generated to fix it. The issue description is: %s%s%s",
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
						}())),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent(fmt.Sprintf("I'll help you create the issue '%s' in %s/%s and then coordinate with Copilot to generate a fix. Let me start by creating the issue with the provided details.", title, owner, repo)),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("Perfect! Please:\n1. Create the issue with the title, description, labels, and assignees\n2. Once created, assign it to Copilot coding agent to generate a solution\n3. Monitor the process and let me know when the PR is ready for review"),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent("Excellent plan! Here's what I'll do:\n\n1. ✅ Create the issue with all specified details\n2. 🤖 Assign to Copilot coding agent for automated fix\n3. 📋 Monitor progress and notify when PR is created\n4. 🔍 Provide PR details for your review\n\nLet me start by creating the issue."),
				},
			}
			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		}
}
