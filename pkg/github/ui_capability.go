package github

import (
	"context"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpAppsExtensionKey is the capability extension key that clients use to
// advertise MCP Apps UI support.
const mcpAppsExtensionKey = "io.modelcontextprotocol/ui"

// MCPAppMIMEType is the MIME type for MCP App UI resources.
const MCPAppMIMEType = "text/html;profile=mcp-app"

// clientSupportsUI reports whether the MCP client that sent this request
// supports MCP Apps UI rendering.
// It checks the context first (set by HTTP/stateless servers from stored
// session capabilities), then falls back to the go-sdk Session (for stdio).
func clientSupportsUI(ctx context.Context, req *mcp.CallToolRequest) bool {
	// Check context first (works for HTTP/stateless servers)
	if supported, ok := ghcontext.HasUISupport(ctx); ok {
		return supported
	}
	// Fall back to go-sdk session (works for stdio/stateful servers)
	if req != nil && req.Session != nil {
		params := req.Session.InitializeParams()
		if params != nil && params.Capabilities != nil {
			_, hasUI := params.Capabilities.Extensions[mcpAppsExtensionKey]
			return hasUI
		}
	}
	return false
}
