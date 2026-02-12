package github

import "github.com/modelcontextprotocol/go-sdk/mcp"

// UIExtensionID is the MCP Apps extension identifier used for capability negotiation.
// Clients advertise MCP Apps support by including this key in their capabilities.
// See: https://github.com/modelcontextprotocol/ext-apps
const UIExtensionID = "io.modelcontextprotocol/ui"

// clientSupportsUI checks whether the client that sent this request supports
// MCP Apps UI rendering. It inspects the client's experimental capabilities
// for the MCP Apps extension identifier.
//
// When the client does not support MCP Apps, tools should skip any UI-gated
// flow (e.g., interactive forms) and execute the action directly.
func clientSupportsUI(req *mcp.CallToolRequest) bool {
	if req == nil || req.Session == nil {
		return false
	}
	params := req.Session.InitializeParams()
	if params == nil || params.Capabilities == nil {
		return false
	}
	_, ok := params.Capabilities.Experimental[UIExtensionID]
	return ok
}
