package toolsets

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandlerFunc is a function that takes dependencies and returns an MCP tool handler.
// This allows tools to be defined statically while their handlers are generated
// on-demand with the appropriate dependencies.
// The deps parameter is typed as `any` to avoid circular dependencies - callers
// should define their own typed dependencies struct and type-assert as needed.
type HandlerFunc func(deps any) mcp.ToolHandler

// ServerTool represents an MCP tool with a handler generator function.
// The tool definition is static, while the handler is generated on-demand
// when the tool is registered with a server.
type ServerTool struct {
	// Tool is the MCP tool definition containing name, description, schema, etc.
	Tool mcp.Tool

	// HandlerFunc generates the handler when given dependencies.
	// This allows tools to be passed around without handlers being set up,
	// and handlers are only created when needed.
	HandlerFunc HandlerFunc
}

// Handler returns a tool handler by calling HandlerFunc with the given dependencies.
func (st *ServerTool) Handler(deps any) mcp.ToolHandler {
	if st.HandlerFunc == nil {
		return nil
	}
	return st.HandlerFunc(deps)
}

// RegisterFunc registers the tool with the server using the provided dependencies.
func (st *ServerTool) RegisterFunc(s *mcp.Server, deps any) {
	handler := st.Handler(deps)
	s.AddTool(&st.Tool, handler)
}

// NewServerTool creates a ServerTool from a tool definition and a typed handler function.
// The handler function takes dependencies (as any) and returns a typed handler.
// Callers should type-assert deps to their typed dependencies struct.
func NewServerTool[In any, Out any](tool mcp.Tool, handlerFn func(deps any) mcp.ToolHandlerFor[In, Out]) ServerTool {
	return ServerTool{
		Tool: tool,
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

// NewServerToolFromHandler creates a ServerTool from a tool definition and a raw handler function.
// Use this when you have a handler that already conforms to mcp.ToolHandler.
func NewServerToolFromHandler(tool mcp.Tool, handlerFn func(deps any) mcp.ToolHandler) ServerTool {
	return ServerTool{Tool: tool, HandlerFunc: handlerFn}
}

// NewServerToolLegacy creates a ServerTool from a tool definition and an already-bound typed handler.
// This is for backward compatibility during the refactor - the handler doesn't use dependencies.
// Deprecated: Use NewServerTool instead for new code.
func NewServerToolLegacy[In any, Out any](tool mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) ServerTool {
	return ServerTool{
		Tool: tool,
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

// NewServerToolFromHandlerLegacy creates a ServerTool from a tool definition and an already-bound raw handler.
// This is for backward compatibility during the refactor - the handler doesn't use dependencies.
// Deprecated: Use NewServerToolFromHandler instead for new code.
func NewServerToolFromHandlerLegacy(tool mcp.Tool, handler mcp.ToolHandler) ServerTool {
	return ServerTool{
		Tool: tool,
		HandlerFunc: func(_ any) mcp.ToolHandler {
			return handler
		},
	}
}
