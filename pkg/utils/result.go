package utils //nolint:revive //TODO: figure out a better name for this package

import (
	"encoding/base64"

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

// NewToolResultResourceWithFlag returns a CallToolResult with either an embedded resource
// or regular content based on the disableEmbeddedResources flag.
// When disableEmbeddedResources is true, text content is returned as TextContent and
// binary content is returned as ImageContent, providing better client compatibility.
func NewToolResultResourceWithFlag(message string, contents *mcp.ResourceContents, disableEmbeddedResources bool) *mcp.CallToolResult {
	if !disableEmbeddedResources {
		// Default behavior - return as embedded resource
		return NewToolResultResource(message, contents)
	}

	// When flag is enabled, return as regular content
	var content mcp.Content
	switch {
	case contents.Text != "":
		// Text content - use TextContent with mime type in annotations
		content = &mcp.TextContent{
			Text: contents.Text,
			Annotations: &mcp.Annotations{
				Audience: []mcp.Role{"user"},
			},
			Meta: mcp.Meta{
				"mimeType": contents.MIMEType,
				"uri":      contents.URI,
			},
		}
	case len(contents.Blob) > 0:
		// Binary content - use ImageContent with base64 data
		content = &mcp.ImageContent{
			Data:     []byte(base64.StdEncoding.EncodeToString(contents.Blob)),
			MIMEType: contents.MIMEType,
			Meta: mcp.Meta{
				"uri": contents.URI,
			},
			Annotations: &mcp.Annotations{
				Audience: []mcp.Role{"user"},
			},
		}
	default:
		// Fallback to embedded resource if neither text nor blob
		return NewToolResultResource(message, contents)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: message,
			},
			content,
		},
		IsError: false,
	}
}
