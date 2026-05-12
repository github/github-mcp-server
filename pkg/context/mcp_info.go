package context

import "context"

type mcpMethodInfoCtx string

var mcpMethodInfoCtxKey mcpMethodInfoCtx = "mcpmethodinfo"

// MCPMethodInfo contains pre-parsed MCP method information extracted from the JSON-RPC request.
// This is populated early in the request lifecycle to enable:
//   - Inventory filtering via ForMCPRequest (only register needed tools/resources/prompts)
//   - Avoiding duplicate JSON parsing in middlewares (secret-scanning, scope-challenge)
//   - Performance optimization for per-request server creation
type MCPMethodInfo struct {
	// Method is the MCP method being called (e.g., "tools/call", "tools/list", "initialize")
	Method string
	// ItemName is the name of the specific item being accessed (tool name, resource URI, prompt name)
	// Only populated for call/get methods (tools/call, prompts/get, resources/read)
	ItemName string
	// Owner is the repository owner from tool call arguments, if present
	Owner string
	// Repo is the repository name from tool call arguments, if present
	Repo string
	// Arguments contains the raw tool arguments for tools/call requests
	Arguments map[string]any
}

// WithMCPMethodInfo stores the MCPMethodInfo in the context.
func WithMCPMethodInfo(ctx context.Context, info *MCPMethodInfo) context.Context {
	return context.WithValue(ctx, mcpMethodInfoCtxKey, info)
}

// MCPMethod retrieves the MCPMethodInfo from the context.
func MCPMethod(ctx context.Context) (*MCPMethodInfo, bool) {
	if info, ok := ctx.Value(mcpMethodInfoCtxKey).(*MCPMethodInfo); ok {
		return info, true
	}
	return nil, false
}
