package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/github/github-mcp-server/pkg/octicons"
	"github.com/google/jsonschema-go/jsonschema"
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
	// InstructionsFunc optionally returns instructions for this toolset.
	// It receives the inventory so it can check what other toolsets are enabled.
	InstructionsFunc func(inv *Inventory) string
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

	// FeatureFlagDisable specifies feature flags that, when any is enabled, cause this
	// tool to be omitted. Used to disable tools when a feature flag is on.
	FeatureFlagDisable []string

	// Enabled is an optional function called at build/filter time to determine
	// if this tool should be available. If nil, the tool is considered enabled
	// (subject to FeatureFlagEnable/FeatureFlagDisable checks).
	// The context carries request-scoped information for the consumer to use.
	// Returns (enabled, error). On error, the tool should be treated as disabled.
	Enabled func(ctx context.Context) (bool, error)

	// RequiredScopes specifies the minimum OAuth scopes required for this tool.
	// These are the scopes that must be present for the tool to function.
	RequiredScopes []string

	// AcceptedScopes specifies all OAuth scopes that can be used with this tool.
	// This includes the required scopes plus any higher-level scopes that provide
	// the necessary permissions due to scope hierarchy.
	AcceptedScopes []string
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
	// Project routing-relevant params to standard MCP-Param-* headers (SEP-2243)
	// so a remote proxy can read owner/repo from headers instead of re-parsing the
	// JSON-RPC body. No-op for tools without these params.
	AnnotateHeaderParams(&toolCopy)
	s.AddTool(&toolCopy, handler)
}

// HeaderParams maps tool input properties to the MCP-Param-* header name a
// header-aware proxy reads, avoiding a second parse of the request body. New
// routing-relevant params should be added here so projection stays automatic
// for every tool; the enforcement test in pkg/github guards full coverage.
var HeaderParams = map[string]string{"owner": "owner", "repo": "repo"}

// AnnotateHeaderParams returns a copy of tool whose routing-relevant input
// properties (per HeaderParams) carry an "x-mcp-header" annotation, which the
// SDK projects onto Mcp-Param-{name} request headers. It never mutates the
// input tool's schema or any map shared with the original tool definition:
// callers shallow-copy ServerTool.Tool, so the *jsonschema.Schema (and its
// per-property Extra maps) are shared, and per-request registration must not
// race on them. Only the schema, its Properties map, and the specific property
// schemas/Extra maps that gain an annotation are cloned.
func AnnotateHeaderParams(tool *mcp.Tool) {
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	if !ok || schema == nil {
		return
	}

	// Collect params that actually need an annotation, so a tool without
	// owner/repo (or already annotated) is left untouched and unCloned.
	var toAnnotate []string
	for prop := range HeaderParams {
		if ps := schema.Properties[prop]; ps != nil {
			if _, exists := ps.Extra["x-mcp-header"]; !exists {
				toAnnotate = append(toAnnotate, prop)
			}
		}
	}
	if len(toAnnotate) == 0 {
		return
	}

	// Clone only what we mutate: a fresh schema value, a fresh Properties map,
	// and fresh property schemas with fresh Extra maps. The original schema and
	// its maps are never written to, so concurrent per-request registration is
	// race-free and deterministic.
	schemaCopy := *schema
	schemaCopy.Properties = maps.Clone(schema.Properties)
	for _, prop := range toAnnotate {
		propCopy := *schemaCopy.Properties[prop]
		extra := make(map[string]any, len(propCopy.Extra)+1)
		maps.Copy(extra, propCopy.Extra)
		extra["x-mcp-header"] = HeaderParams[prop]
		propCopy.Extra = extra
		schemaCopy.Properties[prop] = &propCopy
	}
	tool.InputSchema = &schemaCopy
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
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							&mcp.TextContent{Text: fmt.Sprintf("invalid arguments: %s", err)},
						},
						IsError: true,
					}, nil
				}
				resp, _, err := handler(ctx, req, arguments)
				return resp, err
			}
		},
	}
}

// NewServerTool creates a ServerTool with a raw handler that receives deps via context.
// This is the preferred constructor for tools that use mcp.ToolHandler directly because
// it doesn't create closures at registration time, which is critical for performance in
// servers that create a new instance per request.
//
// The handler function is stored directly without wrapping in a deps closure.
// Dependencies should be injected into context before calling tool handlers.
func NewServerTool(tool mcp.Tool, toolset ToolsetMetadata, handler mcp.ToolHandler) ServerTool {
	return ServerTool{
		Tool:    tool,
		Toolset: toolset,
		// HandlerFunc ignores deps - deps are retrieved from context at call time
		HandlerFunc: func(_ any) mcp.ToolHandler {
			return handler
		},
	}
}
