package utils //nolint:revive //TODO: figure out a better name for this package

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
//
// A stable Meta.viewUUID is included so MCP Apps can persist view state (e.g.
// a completed success card) across host remounts via localStorage, per the
// MCP Apps "Persisting view state" pattern.
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
		Meta: mcp.Meta{
			"viewUUID": newViewUUID(),
		},
	}
}

// newViewUUID returns a random UUID v4 string without pulling in an extra
// dependency. Used as the MCP Apps view persistence key.
func newViewUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Extremely unlikely. Do not return a constant key (shared storage key →
		// cross-view leakage); make it unique enough for a view persistence key.
		return fmt.Sprintf("view-%d", time.Now().UnixNano())
	}
	// UUID version 4 + RFC 4122 variant bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
