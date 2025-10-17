package github

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// GenerateInstructions creates server instructions based on enabled toolsets
func GenerateInstructions(enabledToolsets []string) string {
	// For testing - add a flag to disable instructions
	if os.Getenv("DISABLE_INSTRUCTIONS") == "true" {
		return "" // Baseline mode
	}

	var instructions []string

	// Core instruction - always included if context toolset enabled
	if slices.Contains(enabledToolsets, "context") {
		instructions = append(instructions, "Always call 'get_me' first to understand current user permissions and context.")
	}

	generalInstructions := getGeneralInstructions(enabledToolsets)
	if generalInstructions != "" {
		instructions = append(instructions, "Here are common scenarios you may encounter followed by name and description of the steps to follow:", generalInstructions)
	}

	// Individual toolset instructions
	for _, toolset := range enabledToolsets {
		if inst := getToolsetInstructions(toolset); inst != "" {
			instructions = append(instructions, inst)
		}
	}

	// Base instruction with context management
	baseInstruction := `The GitHub MCP Server provides tools to interact with GitHub platform.

Tool selection guidance:
	1. Use 'list_*' tools for broad, simple retrieval and pagination of all items of a type (e.g., all issues, all PRs, all branches) with basic filtering.
	2. Use 'search_*' tools for targeted queries with specific criteria, keywords, or complex filters (e.g., issues with certain text, PRs by author, code containing functions).

Context management:
	1. Use pagination whenever possible with batches of 5-10 items.
	2. Use minimal_output parameter set to true if the full information is not needed to accomplish a task.

Tool usage guidance:
	1. For 'search_*' tools: Use separate 'sort' and 'order' parameters if available for sorting results - do not include 'sort:' syntax in query strings. Query strings should contain only search criteria (e.g., 'org:google language:python'), not sorting instructions.`

	allInstructions := []string{baseInstruction}
	allInstructions = append(allInstructions, instructions...)

	return strings.Join(allInstructions, " ")
}

// scenarioDefinition defines a scenario with its instruction text and required toolsets
type scenarioDefinition struct {
	instruction      string
	requiredToolsets []string
}

// getGeneralInstructions returns scenario-based guidance for common development tasks
func getGeneralInstructions(enabledToolsets []string) string {
	enabledSet := make(map[string]bool)
	for _, ts := range enabledToolsets {
		enabledSet[ts] = true
	}

	scenarios := map[string]scenarioDefinition{
		"Triaging Security Alerts": {
			instruction:      "Use list_dependabot_alerts, list_code_scanning_alerts, or list_secret_scanning_alerts with state='open' to find active security issues. Use get_*_alert for detailed information about specific alerts. For broader context, search list_global_security_advisories or get_global_security_advisory for known CVEs affecting your dependencies.",
			requiredToolsets: []string{},
		},
		"Manage Workflow": {
			instruction:      "Call list_notifications regularly to see what needs attention. Use get_notification_details for full context on important items. Mark items as done with dismiss_notification or mark_all_notifications_read to keep your inbox organized. Use manage_notification_subscription to adjust notification preferences for threads or manage_repository_notification_subscription for entire repositories.",
			requiredToolsets: []string{"notifications"},
		},
		"Investigating Bugs": {
			instruction:      "Use search_code to find relevant code patterns or function definitions across repositories. Use search_issues to check if similar bugs were reported before. Once you find relevant files, use get_file_contents to read them. Review get_commit and list_commits to understand recent changes that might have introduced the issue.",
			requiredToolsets: []string{"repos"},
		},
	}

	var parts []string
	parts = append(parts, "When helping with development tasks, consider these common scenarios and appropriate tool choices:")

	// Filter scenarios based on enabled toolsets
	for scenarioName, scenario := range scenarios {
		if len(scenario.requiredToolsets) == 0 {
			parts = append(parts, fmt.Sprintf("%s: %s", scenarioName, scenario.instruction))
			continue
		}

		hasAllRequiredToolsets := true
		for _, required := range scenario.requiredToolsets {
			if !enabledSet[required] {
				hasAllRequiredToolsets = false
				break
			}
		}

		if hasAllRequiredToolsets {
			parts = append(parts, fmt.Sprintf("%s: %s", scenarioName, scenario.instruction))
		}
	}

	return strings.Join(parts, " ")
}

// getToolsetInstructions returns specific instructions for individual toolsets
func getToolsetInstructions(toolset string) string {
	switch toolset {
	case "pull_requests":
		return `## Pull Requests

PR review workflow: Always use 'pull_request_review_write' with method 'create' to create a pending review, then 'add_comment_to_pending_review' to add comments, and finally 'pull_request_review_write' with method 'submit_pending' to submit the review for complex reviews with line-specific comments.`
	case "issues":
		return `## Issues

Check 'list_issue_types' first for organizations to use proper issue types. Use 'search_issues' before creating new issues to avoid duplicates. Always set 'state_reason' when closing issues.`
	case "discussions":
		return `## Discussions
		
Use 'list_discussion_categories' to understand available categories before creating discussions. Filter by category for better organization.`
	default:
		return ""
	}
}
