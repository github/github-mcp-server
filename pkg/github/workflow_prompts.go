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
					Content: mcp.NewTextContent("Excellent criteria! I'll evaluate each issue against these requirements:\n\n Clear problem description\n Defined acceptance criteria  \n Appropriate scope\n Technical feasibility\n\nLet me start by finding and examining the issues."),
				},
			}
			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		}
}

// SecurityAlertWorkflowPrompt provides guided workflow for managing security alerts
func SecurityAlertWorkflowPrompt(t translations.TranslationHelperFunc) (tool mcp.Prompt, handler server.PromptHandlerFunc) {
	return mcp.NewPrompt("SecurityAlertWorkflow",
			mcp.WithPromptDescription(t("PROMPT_SECURITY_ALERT_WORKFLOW_DESCRIPTION", "Convert security alerts into trackable issues and assign to appropriate resources")),
			mcp.WithArgument("owner", mcp.ArgumentDescription("Repository owner"), mcp.RequiredArgument()),
			mcp.WithArgument("repo", mcp.ArgumentDescription("Repository name"), mcp.RequiredArgument()),
			mcp.WithArgument("alertType", mcp.ArgumentDescription("Type of alerts to process: dependabot, code_scanning, or secret_scanning")),
		), func(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			owner := request.Params.Arguments["owner"]
			repo := request.Params.Arguments["repo"]
			alertType := "dependabot"
			if t, exists := request.Params.Arguments["alertType"]; exists {
				alertType = fmt.Sprintf("%v", t)
			}

			messages := []mcp.PromptMessage{
				{
					Role:    "system",
					Content: mcp.NewTextContent("You are a security management assistant helping to process security alerts and convert them into actionable work items. You should examine alerts, create tracking issues for important ones, and assign appropriate work to Copilot where suitable."),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent(fmt.Sprintf("I need to process %s security alerts for %s/%s. Please help me convert critical alerts into trackable issues and assign work appropriately.", alertType, owner, repo)),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent(fmt.Sprintf("I'll help you process %s alerts for %s/%s systematically. Let me examine the alerts, identify critical ones, and create appropriate tracking issues.", alertType, owner, repo)),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("Great! Please prioritize by severity and create issues for critical/high priority alerts. For straightforward dependency updates, consider assigning to Copilot. For complex security issues, create detailed issues for human review."),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent("Perfect approach! I'll:\n\n1. List and examine the security alerts\n2. Prioritize by severity (critical/high first)\n3. Create detailed tracking issues\n4. Assign simple dependency updates to Copilot\n5. Flag complex security issues for human review\n\nLet me start by examining the alerts."),
				},
			}
			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		}
}

// RepositorySetupWorkflowPrompt provides guided workflow for setting up new repositories
func RepositorySetupWorkflowPrompt(t translations.TranslationHelperFunc) (tool mcp.Prompt, handler server.PromptHandlerFunc) {
	return mcp.NewPrompt("RepositorySetupWorkflow",
			mcp.WithPromptDescription(t("PROMPT_REPOSITORY_SETUP_WORKFLOW_DESCRIPTION", "Guide through setting up a new repository with initial content and structure")),
			mcp.WithArgument("repoName", mcp.ArgumentDescription("Name for the new repository"), mcp.RequiredArgument()),
			mcp.WithArgument("description", mcp.ArgumentDescription("Repository description")),
			mcp.WithArgument("private", mcp.ArgumentDescription("Whether repository should be private (true/false)")),
		), func(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			repoName := request.Params.Arguments["repoName"]
			description := ""
			if d, exists := request.Params.Arguments["description"]; exists {
				description = fmt.Sprintf("%v", d)
			}
			private := "false"
			if p, exists := request.Params.Arguments["private"]; exists {
				private = fmt.Sprintf("%v", p)
			}

			messages := []mcp.PromptMessage{
				{
					Role:    "system",
					Content: mcp.NewTextContent("You are a repository setup assistant helping to create and configure new GitHub repositories with best practices. You should guide through creating the repository, setting up initial structure, and establishing good development workflows."),
				},
				{
					Role: "user",
					Content: mcp.NewTextContent(fmt.Sprintf("I want to create a new repository called '%s'%s%s. Please help me set it up with proper initial structure.", repoName, func() string {
						if description != "" {
							return fmt.Sprintf(" with description '%s'", description)
						}
						return ""
					}(), func() string {
						if private == "true" {
							return " (private repository)"
						}
						return ""
					}())),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent(fmt.Sprintf("I'll help you create and set up the '%s' repository with a proper initial structure. Let me guide you through the process step by step.", repoName)),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("Perfect! Please:\n1. Create the repository\n2. Set up a development branch\n3. Add essential files (README, .gitignore, etc.)\n4. Create an initial pull request to establish the workflow\n\nLet me know what type of project this is so you can suggest appropriate templates."),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent("Excellent plan! I'll create a well-structured repository with:\n\n Repository creation\n Development branch setup\n Essential files (README, .gitignore, etc.)\n Initial PR workflow\n\nWhat type of project is this? (e.g., JavaScript/Node.js, Python, Go, documentation, etc.) This will help me suggest the right templates and structure."),
				},
			}
			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		}
}
