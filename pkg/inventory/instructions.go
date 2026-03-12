package inventory

import (
	"os"
	"strings"
)

// generateInstructions creates server instructions based on enabled toolsets
func generateInstructions(inv *Inventory) string {
	// For testing - add a flag to disable instructions
	if os.Getenv("DISABLE_INSTRUCTIONS") == "true" {
		return "" // Baseline mode
	}

	var instructions []string

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

	instructions = append(instructions, baseInstruction)

	// Collect instructions from each enabled toolset
	for _, toolset := range inv.EnabledToolsets() {
		if toolset.InstructionsFunc != nil {
			if toolsetInstructions := toolset.InstructionsFunc(inv); toolsetInstructions != "" {
				instructions = append(instructions, toolsetInstructions)
			}
		}
	}

	return strings.Join(instructions, " ")
}
