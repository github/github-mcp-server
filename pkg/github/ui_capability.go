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

// uiSubmitted reports whether the call is itself an MCP App form submission.
// The form re-invokes its tool with _ui_submitted=true; such calls must execute
// rather than re-render the form.
func uiSubmitted(args map[string]any) bool {
	submitted, _ := OptionalParam[bool](args, "_ui_submitted")
	return submitted
}

// hasNonFormParams reports whether the call carries any parameter the tool's MCP
// App form cannot represent (anything outside formParams). Such calls must
// bypass the form and execute directly so the supplied values aren't silently
// dropped. formParams is the set of parameters the form collects and re-sends
// on submit.
func hasNonFormParams(args map[string]any, formParams map[string]struct{}) bool {
	for key, value := range args {
		if value == nil {
			continue
		}
		if _, ok := formParams[key]; !ok {
			return true
		}
	}
	return false
}

// shouldDeferToForm is the single source of truth for the show/defer decision
// shared by the form-backed write tools (create_pull_request,
// update_pull_request, issue_write). It reports whether a call should be handed
// off to its MCP App form instead of executing now: defer only when MCP Apps
// are enabled, the client can render UI, the call is not itself a form
// submission, and every supplied parameter can be represented by the form
// (formParams is the tool's form-parameter allowlist). When it returns false
// the handler executes directly; the host may still render the tool's view,
// which renders the result rather than an input form.
func shouldDeferToForm(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, args map[string]any, formParams map[string]struct{}) bool {
	return deps.IsFeatureEnabled(ctx, MCPAppsFeatureFlag) &&
		clientSupportsUI(ctx, req) &&
		!uiSubmitted(args) &&
		!hasNonFormParams(args, formParams)
}
