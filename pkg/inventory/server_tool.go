package inventory

import (
	"context"
	"encoding/json"

	"github.com/github/github-mcp-server/pkg/octicons"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandlerFunc is a function that takes dependencies and returns an MCP tool handler.
// This allows tools to be defined statically while their handlers are generated
// on-demand with the appropriate dependencies.
// The deps parameter is typed as `any` to avoid circular dependencies - callers
// should define their own typed dependencies struct and type-assert as needed.
type HandlerFunc func(deps any) mcp.ToolHandler

// ToolsetID is a unique identifier for a toolset.
// Using a distinct type provides compile-time type safety.
type ToolsetID string

// ToolsetMetadata contains metadata about the toolset a tool belongs to.
type ToolsetMetadata struct {
	// ID is the unique identifier for the toolset (e.g., "repos", "issues")
	ID ToolsetID
	// Description provides a human-readable description of the toolset
	Description string
	// Default indicates this toolset should be enabled by default
	Default bool
	// Icon is the name of the Octicon to use for tools in this toolset.
	// Use the base name without size suffix, e.g., "repo" not "repo-16".
	// See https://primer.style/foundations/icons for available icons.
	Icon string
}

// Icons returns MCP Icon objects for this toolset, or nil if no icon is set.
// Icons are provided in both 16x16 and 24x24 sizes.
func (tm ToolsetMetadata) Icons() []mcp.Icon {
	return octicons.Icons(tm.Icon)
}

// ServerTool represents an MCP tool with metadata and a handler generator function.
// The tool definition is static, while the handler is generated on-demand
// when the tool is registered with a server.
// Tools are now self-describing with their toolset membership and read-only status
// derived from the Tool.Annotations.ReadOnlyHint field.
type ServerTool struct {
	// Tool is the MCP tool definition containing name, description, schema, etc.
	Tool mcp.Tool

	// Toolset contains metadata about which toolset this tool belongs to.
	Toolset ToolsetMetadata

	// HandlerFunc generates the handler when given dependencies.
	// This allows tools to be passed around without handlers being set up,
	// and handlers are only created when needed.
	HandlerFunc HandlerFunc

	// FeatureFlagEnable specifies a feature flag that must be enabled for this tool
	// to be available. If set and the flag is not enabled, the tool is omitted.
	FeatureFlagEnable string

	// FeatureFlagDisable specifies a feature flag that, when enabled, causes this tool
	// to be omitted. Used to disable tools when a feature flag is on.
	FeatureFlagDisable string

	// Enabled is an optional function called at build/filter time to determine
	// if this tool should be available. If nil, the tool is considered enabled
	// (subject to FeatureFlagEnable/FeatureFlagDisable checks).
	// The context carries request-scoped information for the consumer to use.
	// Returns (enabled, error). On error, the tool should be treated as disabled.
	Enabled func(ctx context.Context) (bool, error)
}

// IsReadOnly returns true if this tool is marked as read-only via annotations.
func (st *ServerTool) IsReadOnly() bool {
	return st.Tool.Annotations != nil && st.Tool.Annotations.ReadOnlyHint
}

// HasHandler returns true if this tool has a handler function.
func (st *ServerTool) HasHandler() bool {
	return st.HandlerFunc != nil
}

// Handler returns a tool handler by calling HandlerFunc with the given dependencies.
// Panics if HandlerFunc is nil - all tools should have handlers.
func (st *ServerTool) Handler(deps any) mcp.ToolHandler {
	if st.HandlerFunc == nil {
		panic("HandlerFunc is nil for tool: " + st.Tool.Name)
	}
	return st.HandlerFunc(deps)
}

// RegisterFunc registers the tool with the server using the provided dependencies.
// Icons are automatically applied from the toolset metadata if not already set.
// A shallow copy of the tool is made to avoid mutating the original ServerTool.
// Panics if the tool has no handler - all tools should have handlers.
func (st *ServerTool) RegisterFunc(s *mcp.Server, deps any) {
	handler := st.Handler(deps) // This will panic if HandlerFunc is nil
	// Make a shallow copy of the tool to avoid mutating the original
	toolCopy := st.Tool
	// Apply icons from toolset metadata if tool doesn't have icons set
	if len(toolCopy.Icons) == 0 {
		toolCopy.Icons = st.Toolset.Icons()
	}
	s.AddTool(&toolCopy, handler)
}

// NewServerTool creates a ServerTool from a tool definition, toolset metadata, and a typed handler function.
// The handler function takes dependencies (as any) and returns a typed handler.
// Callers should type-assert deps to their typed dependencies struct.
//
// Deprecated: This creates closures at registration time. For better performance in
// per-request server scenarios, use NewServerToolWithContextHandler instead.
func NewServerTool[In any, Out any](tool mcp.Tool, toolset ToolsetMetadata, handlerFn func(deps any) mcp.ToolHandlerFor[In, Out]) ServerTool {
	return ServerTool{
		Tool:    tool,
		Toolset: toolset,
		HandlerFunc: func(deps any) mcp.ToolHandler {
			typedHandler := handlerFn(deps)
			return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				var arguments In
				if err := json.Unmarshal(req.Params.Arguments, &arguments); err != nil {
					return nil, err
				}
				resp, _, err := typedHandler(ctx, req, arguments)
				return resp, err
			}
		},
	}
}

// NewServerToolWithContextHandler creates a ServerTool with a handler that receives deps via context.
// This is the preferred approach for tools because it doesn't create closures at registration time,
// which is critical for performance in servers that create a new instance per request.
//
// The handler function is stored directly without wrapping in a deps closure.
// Dependencies should be injected into context before calling tool handlers.
func NewServerToolWithContextHandler[In any, Out any](tool mcp.Tool, toolset ToolsetMetadata, handler mcp.ToolHandlerFor[In, Out]) ServerTool {
	return ServerTool{
		Tool:    tool,
		Toolset: toolset,
		// HandlerFunc ignores deps - deps are retrieved from context at call time
		HandlerFunc: func(_ any) mcp.ToolHandler {
			return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				var arguments In
				if err := json.Unmarshal(req.Params.Arguments, &arguments); err != nil {
					return nil, err
				}
				resp, _, err := handler(ctx, req, arguments)
				return resp, err
			}
		},
	}
}

// NewServerToolFromHandler creates a ServerTool from a tool definition, toolset metadata, and a raw handler function.
// Use this when you have a handler that already conforms to mcp.ToolHandler.
//
// Deprecated: This creates closures at registration time. For better performance in
// per-request server scenarios, use NewServerToolWithRawContextHandler instead.
func NewServerToolFromHandler(tool mcp.Tool, toolset ToolsetMetadata, handlerFn func(deps any) mcp.ToolHandler) ServerTool {
	return ServerTool{Tool: tool, Toolset: toolset, HandlerFunc: handlerFn}
}

// NewServerToolWithRawContextHandler creates a ServerTool with a raw handler that receives deps via context.
// This is the preferred approach for tools that use mcp.ToolHandler directly because it doesn't
// create closures at registration time.
//
// The handler function is stored directly without wrapping in a deps closure.
// Dependencies should be injected into context before calling tool handlers.
func NewServerToolWithRawContextHandler(tool mcp.Tool, toolset ToolsetMetadata, handler mcp.ToolHandler) ServerTool {
	return ServerTool{
		Tool:    tool,
		Toolset: toolset,
		// HandlerFunc ignores deps - deps are retrieved from context at call time
		HandlerFunc: func(_ any) mcp.ToolHandler {
			return handler
		},
	}
}
