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
		"Reviewing Pull Requests": {
			instruction:      "Use get_pull_request to fetch PR details and get_pull_request_files to see changes. Use create_pending_pull_request_review to start a review, add_comment_to_pending_review for line-specific feedback, then submit_pending_pull_request_review to publish. Check get_pull_request_status to verify CI/CD checks before approving.",
			requiredToolsets: []string{"pull_requests"},
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
