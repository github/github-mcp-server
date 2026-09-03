package context

import (
	"context"
	"encoding/json"
)

type mcpMethodInfoCtx string

var mcpMethodInfoCtxKey mcpMethodInfoCtx = "mcpmethodinfo"

// MCPMethodInfo contains pre-parsed MCP method information extracted from the JSON-RPC request.
// This is populated early in the request lifecycle to enable:
//   - Inventory filtering via ForMCPRequest (only register needed tools/resources/prompts)
//   - Avoiding duplicate JSON envelope parsing in downstream middleware
//   - Performance optimization for per-request server creation
type MCPMethodInfo struct {
	// Method is the MCP method being called (e.g., "tools/call", "tools/list", "initialize")
	Method string
	// ItemName is the name of the specific item being accessed (tool name, resource URI, prompt name)
	// Only populated for call/get methods (tools/call, prompts/get, resources/read)
	ItemName string
	// RawArguments contains the unmaterialized tool arguments for tools/call requests.
	RawArguments json.RawMessage
}

// DecodeArguments materializes tool arguments when request middleware needs
// call-specific values. Invalid argument shapes are returned to the caller so
// the request can continue to the tool handler's normal validation path.
func (info *MCPMethodInfo) DecodeArguments() (map[string]any, error) {
	if len(info.RawArguments) == 0 {
		return nil, nil
	}

	var arguments map[string]any
	if err := json.Unmarshal(info.RawArguments, &arguments); err != nil {
		return nil, err
	}
	return arguments, nil
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
