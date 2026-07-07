package utils //nolint:revive //TODO: figure out a better name for this package

import "github.com/modelcontextprotocol/go-sdk/mcp"

func NewToolResultText(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: message,
			},
		},
	}
}

func NewToolResultError(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: message,
			},
		},
		IsError: true,
	}
}

func NewToolResultErrorFromErr(message string, err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: message + ": " + err.Error(),
			},
		},
		IsError: true,
	}
}

func NewToolResultResource(message string, contents *mcp.ResourceContents) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: message,
			},
			&mcp.EmbeddedResource{
				Resource: contents,
			},
		},
		IsError: false,
	}
}

func NewToolResultResourceLink(message string, link *mcp.ResourceLink) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: message,
			},
			link,
		},
		IsError: false,
	}
}

// NewToolResultAwaitingFormSubmission signals to the agent that a tool call
// has been intercepted to show an MCP App form to the user and has NOT
// performed the requested operation. The agent must stop, not chain dependent
// tool calls, and not claim the operation succeeded. The result is marked
// IsError=true so agents that bail on error don't proceed; the host still
// renders the UI because rendering is keyed off the tool's _meta.ui, not the
// result. The MCP App form will submit the operation directly when the user
// clicks submit, after which a ui/update-model-context call delivers the real
// outcome to the agent.
func NewToolResultAwaitingFormSubmission(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: message,
			},
		},
		StructuredContent: map[string]any{
			"status": "awaiting_user_submission",
			"reason": "An interactive form is being shown to the user. The operation has not been performed.",
		},
		IsError: true,
	}
}
