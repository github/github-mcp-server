package github

import (
	"context"
	"fmt"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// PullRequestReviewWorkflowPrompt provides a guided workflow for comprehensive PR review
func PullRequestReviewWorkflowPrompt(t translations.TranslationHelperFunc) (tool mcp.Prompt, handler server.PromptHandlerFunc) {
	return mcp.NewPrompt("PRReviewWorkflow",
			mcp.WithPromptDescription(t("PROMPT_PR_REVIEW_WORKFLOW_DESCRIPTION", "Guide through comprehensive pull request review process using pending review workflow")),
			mcp.WithArgument("owner", mcp.ArgumentDescription("Repository owner"), mcp.RequiredArgument()),
			mcp.WithArgument("repo", mcp.ArgumentDescription("Repository name"), mcp.RequiredArgument()),
			mcp.WithArgument("pullNumber", mcp.ArgumentDescription("Pull request number to review"), mcp.RequiredArgument()),
		), func(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			owner := request.Params.Arguments["owner"]
			repo := request.Params.Arguments["repo"]
			pullNumber := request.Params.Arguments["pullNumber"]

			messages := []mcp.PromptMessage{
				{
					Role:    "system",
					Content: mcp.NewTextContent("You are a code review assistant helping with a comprehensive GitHub pull request review. You should use the pending review workflow to provide thorough, professional feedback. This involves: 1) Creating a pending review, 2) Adding multiple specific comments, and 3) Submitting the complete review with overall feedback."),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent(fmt.Sprintf("I need to review pull request #%s in %s/%s. Please help me conduct a thorough review using the pending review workflow.", pullNumber, owner, repo)),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent(fmt.Sprintf("I'll help you conduct a comprehensive review of PR #%s in %s/%s using the pending review workflow. Let me start by getting the PR details and creating a pending review.", pullNumber, owner, repo)),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("Perfect! Please first get the PR details and files changed, then create a pending review. After that, I'll provide specific feedback for you to add as line comments before we submit the complete review."),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent("Absolutely! Here's my plan:\n\n1. First, I'll get the PR details and files changed\n2. Create a pending review\n3. Wait for your specific feedback to add as line comments\n4. Submit the complete review with overall assessment\n\nLet me start by examining the pull request."),
				},
			}
			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		}
}

// NotificationTriageWorkflowPrompt provides a guided workflow for processing notifications
func NotificationTriageWorkflowPrompt(t translations.TranslationHelperFunc) (tool mcp.Prompt, handler server.PromptHandlerFunc) {
	return mcp.NewPrompt("NotificationTriageWorkflow",
			mcp.WithPromptDescription(t("PROMPT_NOTIFICATION_TRIAGE_WORKFLOW_DESCRIPTION", "Systematically process and triage GitHub notifications")),
			mcp.WithArgument("filter", mcp.ArgumentDescription("Notification filter (default, include_read_notifications, only_participating)")),
		), func(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			filter := "default"
			if f, exists := request.Params.Arguments["filter"]; exists {
				filter = fmt.Sprintf("%v", f)
			}

			messages := []mcp.PromptMessage{
				{
					Role:    "system",
					Content: mcp.NewTextContent("You are a notification management assistant helping to efficiently process GitHub notifications. You should help triage notifications by examining them and taking appropriate actions like dismissing, unsubscribing, or marking as read."),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent(fmt.Sprintf("I need to triage my GitHub notifications. Please help me process them systematically using filter '%s'.", filter)),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent(fmt.Sprintf("I'll help you efficiently triage your GitHub notifications using the '%s' filter. Let me start by listing your notifications and then we can examine each one to determine the appropriate action.", filter)),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("Great! For each notification, please show me the details and suggest what action to take - whether to dismiss it, unsubscribe from the thread, or take other action."),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent("Perfect! I'll examine each notification and provide recommendations. Let me start by getting your notification list and then we'll go through them systematically."),
				},
			}
			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		}
}

// IssueInvestigationWorkflowPrompt provides guided workflow for investigating and delegating issues
func IssueInvestigationWorkflowPrompt(t translations.TranslationHelperFunc) (tool mcp.Prompt, handler server.PromptHandlerFunc) {
	return mcp.NewPrompt("IssueInvestigationWorkflow",
			mcp.WithPromptDescription(t("PROMPT_ISSUE_INVESTIGATION_WORKFLOW_DESCRIPTION", "Investigate issues and delegate appropriate ones to Copilot coding agent")),
			mcp.WithArgument("owner", mcp.ArgumentDescription("Repository owner"), mcp.RequiredArgument()),
			mcp.WithArgument("repo", mcp.ArgumentDescription("Repository name"), mcp.RequiredArgument()),
			mcp.WithArgument("searchQuery", mcp.ArgumentDescription("Search query for issues (optional)")),
		), func(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			owner := request.Params.Arguments["owner"]
			repo := request.Params.Arguments["repo"]
			searchQuery := ""
			if q, exists := request.Params.Arguments["searchQuery"]; exists {
				searchQuery = fmt.Sprintf("%v", q)
			}

			messages := []mcp.PromptMessage{
				{
					Role:    "system",
					Content: mcp.NewTextContent("You are an issue management assistant helping to investigate GitHub issues and identify which ones are suitable for delegation to Copilot coding agent. You should examine issues for clarity, scope, and complexity to determine suitability for autonomous work."),
				},
				{
					Role: "user",
					Content: mcp.NewTextContent(fmt.Sprintf("I need to investigate issues in %s/%s and identify which ones can be assigned to Copilot. %s", owner, repo, func() string {
						if searchQuery != "" {
							return fmt.Sprintf("Please focus on issues matching: '%s'", searchQuery)
						}
						return "Please help me find suitable issues."
					}())),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent(fmt.Sprintf("I'll help you investigate issues in %s/%s and identify which ones are suitable for Copilot assignment. Let me search for relevant issues and examine them for clarity, scope, and complexity.", owner, repo)),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("Perfect! For each issue, please check if it has:\n- Clear problem description\n- Defined acceptance criteria\n- Appropriate scope (not too large/complex)\n- Technical feasibility for autonomous work\n\nThen assign suitable ones to Copilot."),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent("Excellent criteria! I'll evaluate each issue against these requirements:\n\n- Clear problem description\n- Defined acceptance criteria\n- Appropriate scope\n- Technical feasibility\n\nLet me start by finding and examining the issues."),
				},
			}
			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		}
}

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
					Role:    "system",
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
