package github

import (
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
