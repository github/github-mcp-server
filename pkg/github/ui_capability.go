package github

import "github.com/modelcontextprotocol/go-sdk/mcp"

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
// supports MCP Apps UI rendering, based on its ClientInfo.Name.
func clientSupportsUI(req *mcp.CallToolRequest) bool {
	if req == nil || req.Session == nil {
		return false
	}
	params := req.Session.InitializeParams()
	if params == nil || params.ClientInfo == nil {
		return false
	}
	return uiSupportedClients[params.ClientInfo.Name]
}
