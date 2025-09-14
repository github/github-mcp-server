package github

import (
	"os"
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
	if contains(enabledToolsets, "context") {
		instructions = append(instructions, "Always call 'get_me' first to understand current user permissions and context.")
	}
	
	// Individual toolset instructions
	for _, toolset := range enabledToolsets {
		if inst := getToolsetInstructions(toolset); inst != "" {
			instructions = append(instructions, inst)
		}
	}
	
	// Base instruction with context management
	baseInstruction := "The GitHub MCP Server provides GitHub API tools. Tool selection guidance: Use 'list_*' tools for broad, simple retrieval and pagination of all items of a type (e.g., all issues, all PRs, all branches) with basic filtering. Use 'search_*' tools for targeted queries with specific criteria, keywords, or complex filters (e.g., issues with certain text, PRs by author, code containing functions). Context management: 1) GitHub API responses can overflow context windows, 2) Process large datasets in batches of 5-10 items, 3) For summarization tasks, fetch minimal data first, then drill down into specifics."
	
	allInstructions := []string{baseInstruction}
	allInstructions = append(allInstructions, instructions...)
	
	return strings.Join(allInstructions, " ")
}

// getToolsetInstructions returns specific instructions for individual toolsets
func getToolsetInstructions(toolset string) string {
	switch toolset {
	case "pull_requests":
		return "PR review workflow: Always use 'create_pending_pull_request_review' → 'add_comment_to_pending_review' → 'submit_pending_pull_request_review' for complex reviews with line-specific comments."
	case "issues":
		return "Issue workflow: Check 'list_issue_types' first for organizations to use proper issue types. Use 'search_issues' before creating new issues to avoid duplicates. Always set 'state_reason' when closing issues."
	case "notifications":
		return "Notifications: Filter by 'participating' for issues/PRs you're involved in. Use 'mark_all_notifications_read' with repository filters to avoid marking unrelated notifications."
	case "discussions":
		return "Discussions: Use 'list_discussion_categories' to understand available categories before creating discussions. Filter by category for better organization."
	default:
		return ""
	}
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
