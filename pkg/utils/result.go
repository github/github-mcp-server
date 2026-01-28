// Package utils is deprecated: use pkg/mcpresult instead
package utils //nolint:revive // vague package name

import (
	"github.com/github/github-mcp-server/pkg/mcpresult"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Deprecated: use [mcpresult.NewText] instead.
func NewToolResultText(message string) *mcp.CallToolResult {
	return mcpresult.NewText(message)
}

// Deprecated: use [mcpresult.NewError] instead.
func NewToolResultError(message string) *mcp.CallToolResult {
	return mcpresult.NewError(message)
}

// Deprecated: use [mcpresult.NewErrorFromErr] instead.
func NewToolResultErrorFromErr(message string, err error) *mcp.CallToolResult {
	return mcpresult.NewErrorFromErr(message, err)
}

// Deprecated: use [mcpresult.NewResource] instead.
func NewToolResultResource(message string, contents *mcp.ResourceContents) *mcp.CallToolResult {
	return mcpresult.NewResource(message, contents)
}
