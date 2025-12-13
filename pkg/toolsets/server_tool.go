package toolsets

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandlerFunc is a function that takes dependencies and returns an MCP tool handler.
// This allows tools to be defined statically while their handlers are generated
// on-demand with the appropriate dependencies.
type HandlerFunc func(deps ToolDependencies) mcp.ToolHandler

// ToolDependencies contains all dependencies that tool handlers might need.
// Fields are pointers/interfaces so they can be nil when not needed by a specific tool.
type ToolDependencies struct {
	// GetClient returns a GitHub REST API client
	GetClient any // func(context.Context) (*github.Client, error)

	// GetGQLClient returns a GitHub GraphQL client
	GetGQLClient any // func(context.Context) (*githubv4.Client, error)

	// GetRawClient returns a raw HTTP client for GitHub
	GetRawClient any // raw.GetRawClientFn

	// RepoAccessCache is the lockdown mode repo access cache
	RepoAccessCache any // *lockdown.RepoAccessCache

	// T is the translation helper function
	T any // translations.TranslationHelperFunc

	// Flags are feature flags
	Flags any // FeatureFlags

	// ContentWindowSize is the size of the content window for log truncation
	ContentWindowSize int
}

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
func (st *ServerTool) Handler(deps ToolDependencies) mcp.ToolHandler {
	if st.HandlerFunc == nil {
		return nil
	}
	return st.HandlerFunc(deps)
}

// RegisterFunc registers the tool with the server using the provided dependencies.
func (st *ServerTool) RegisterFunc(s *mcp.Server, deps ToolDependencies) {
	handler := st.Handler(deps)
	s.AddTool(&st.Tool, handler)
}

// NewServerTool creates a ServerTool from a tool definition and a typed handler function.
// The handler function takes dependencies and returns a typed handler.
func NewServerTool[In any, Out any](tool mcp.Tool, handlerFn func(deps ToolDependencies) mcp.ToolHandlerFor[In, Out]) ServerTool {
	return ServerTool{
		Tool: tool,
		HandlerFunc: func(deps ToolDependencies) mcp.ToolHandler {
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
func NewServerToolFromHandler(tool mcp.Tool, handlerFn func(deps ToolDependencies) mcp.ToolHandler) ServerTool {
	return ServerTool{Tool: tool, HandlerFunc: handlerFn}
}

// NewServerToolLegacy creates a ServerTool from a tool definition and an already-bound typed handler.
// This is for backward compatibility during the refactor - the handler doesn't use ToolDependencies.
// Deprecated: Use NewServerTool instead for new code.
func NewServerToolLegacy[In any, Out any](tool mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) ServerTool {
	return ServerTool{
		Tool: tool,
		HandlerFunc: func(_ ToolDependencies) mcp.ToolHandler {
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
// This is for backward compatibility during the refactor - the handler doesn't use ToolDependencies.
// Deprecated: Use NewServerToolFromHandler instead for new code.
func NewServerToolFromHandlerLegacy(tool mcp.Tool, handler mcp.ToolHandler) ServerTool {
	return ServerTool{
		Tool: tool,
		HandlerFunc: func(_ ToolDependencies) mcp.ToolHandler {
			return handler
		},
	}
}
