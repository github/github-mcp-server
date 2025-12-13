package github

import (
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolDependencies contains all dependencies that tool handlers might need.
// This is a properly-typed struct that lives in pkg/github to avoid circular
// dependencies. The toolsets package uses `any` for deps and tool handlers
// type-assert to this struct.
type ToolDependencies struct {
	// GetClient returns a GitHub REST API client
	GetClient GetClientFn

	// GetGQLClient returns a GitHub GraphQL client
	GetGQLClient GetGQLClientFn

	// GetRawClient returns a raw HTTP client for GitHub
	GetRawClient raw.GetRawClientFn

	// RepoAccessCache is the lockdown mode repo access cache
	RepoAccessCache *lockdown.RepoAccessCache

	// T is the translation helper function
	T translations.TranslationHelperFunc

	// Flags are feature flags
	Flags FeatureFlags

	// ContentWindowSize is the size of the content window for log truncation
	ContentWindowSize int
}

// NewTool creates a ServerTool with fully-typed ToolDependencies and toolset metadata.
// This helper isolates the type assertion from `any` to `ToolDependencies`,
// so tool implementations remain fully typed without assertions scattered throughout.
func NewTool[In, Out any](toolset toolsets.ToolsetMetadata, tool mcp.Tool, handler func(deps ToolDependencies) mcp.ToolHandlerFor[In, Out]) toolsets.ServerTool {
	return toolsets.NewServerTool(tool, toolset, func(d any) mcp.ToolHandlerFor[In, Out] {
		return handler(d.(ToolDependencies))
	})
}

// NewToolFromHandler creates a ServerTool with fully-typed ToolDependencies and toolset metadata
// for handlers that conform to mcp.ToolHandler directly.
func NewToolFromHandler(toolset toolsets.ToolsetMetadata, tool mcp.Tool, handler func(deps ToolDependencies) mcp.ToolHandler) toolsets.ServerTool {
	return toolsets.NewServerToolFromHandler(tool, toolset, func(d any) mcp.ToolHandler {
		return handler(d.(ToolDependencies))
	})
}
