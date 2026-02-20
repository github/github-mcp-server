package github

import (
	"context"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// uiSupportedClients lists client names (from ClientInfo.Name) known to
// support MCP Apps UI rendering.
//
// This is a temporary workaround until the Go SDK adds an Extensions field
// to ClientCapabilities (see https://github.com/modelcontextprotocol/go-sdk/issues/777).
// Once that lands, detection should use capabilities.extensions instead.
var uiSupportedClients = map[string]bool{
	"Visual Studio Code - Insiders": true,
	"Visual Studio Code":            true,
}

// clientSupportsUI reports whether the MCP client that sent this request
// supports MCP Apps UI rendering.
// It checks the go-sdk Session first (for stdio/stateful servers), then
// falls back to the request context (for HTTP/stateless servers where
// the session may not persist InitializeParams across requests).
func clientSupportsUI(ctx context.Context, req *mcp.CallToolRequest) bool {
	// Try go-sdk session first (works for stdio/stateful servers)
	if req != nil && req.Session != nil {
		params := req.Session.InitializeParams()
		if params != nil && params.ClientInfo != nil {
			return uiSupportedClients[params.ClientInfo.Name]
		}
	}
	// Fall back to context (works for HTTP/stateless servers)
	if name, ok := ghcontext.GetClientName(ctx); ok {
		return uiSupportedClients[name]
	}
	return false
}
