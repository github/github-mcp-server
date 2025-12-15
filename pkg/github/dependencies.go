package github

import (
	"context"

	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/registry"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v79/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// ToolDependencies defines the interface for dependencies that tool handlers need.
// This is an interface to allow different implementations:
//   - Local server: stores closures that create clients on demand
//   - Remote server: can store pre-created clients per-request for efficiency
//
// The toolsets package uses `any` for deps and tool handlers type-assert to this interface.
type ToolDependencies interface {
	// GetClient returns a GitHub REST API client
	GetClient(ctx context.Context) (*gogithub.Client, error)

	// GetGQLClient returns a GitHub GraphQL client
	GetGQLClient(ctx context.Context) (*githubv4.Client, error)

	// GetRawClient returns a raw content client for GitHub
	GetRawClient(ctx context.Context) (*raw.Client, error)

	// GetRepoAccessCache returns the lockdown mode repo access cache
	GetRepoAccessCache() *lockdown.RepoAccessCache

	// GetT returns the translation helper function
	GetT() translations.TranslationHelperFunc

	// GetFlags returns feature flags
	GetFlags() FeatureFlags

	// GetContentWindowSize returns the content window size for log truncation
	GetContentWindowSize() int
}

// BaseDeps is the standard implementation of ToolDependencies for the local server.
// It stores pre-created clients. The remote server can create its own struct
// implementing ToolDependencies with different client creation strategies.
type BaseDeps struct {
	// Pre-created clients
	Client    *gogithub.Client
	GQLClient *githubv4.Client
	RawClient *raw.Client

	// Static dependencies
	RepoAccessCache   *lockdown.RepoAccessCache
	T                 translations.TranslationHelperFunc
	Flags             FeatureFlags
	ContentWindowSize int
}

// NewBaseDeps creates a BaseDeps with the provided clients and configuration.
func NewBaseDeps(
	client *gogithub.Client,
	gqlClient *githubv4.Client,
	rawClient *raw.Client,
	repoAccessCache *lockdown.RepoAccessCache,
	t translations.TranslationHelperFunc,
	flags FeatureFlags,
	contentWindowSize int,
) *BaseDeps {
	return &BaseDeps{
		Client:            client,
		GQLClient:         gqlClient,
		RawClient:         rawClient,
		RepoAccessCache:   repoAccessCache,
		T:                 t,
		Flags:             flags,
		ContentWindowSize: contentWindowSize,
	}
}

// GetClient implements ToolDependencies.
func (d BaseDeps) GetClient(_ context.Context) (*gogithub.Client, error) {
	return d.Client, nil
}

// GetGQLClient implements ToolDependencies.
func (d BaseDeps) GetGQLClient(_ context.Context) (*githubv4.Client, error) {
	return d.GQLClient, nil
}

// GetRawClient implements ToolDependencies.
func (d BaseDeps) GetRawClient(_ context.Context) (*raw.Client, error) {
	return d.RawClient, nil
}

// GetRepoAccessCache implements ToolDependencies.
func (d BaseDeps) GetRepoAccessCache() *lockdown.RepoAccessCache { return d.RepoAccessCache }

// GetT implements ToolDependencies.
func (d BaseDeps) GetT() translations.TranslationHelperFunc { return d.T }

// GetFlags implements ToolDependencies.
func (d BaseDeps) GetFlags() FeatureFlags { return d.Flags }

// GetContentWindowSize implements ToolDependencies.
func (d BaseDeps) GetContentWindowSize() int { return d.ContentWindowSize }

// NewTool creates a ServerTool with fully-typed ToolDependencies and toolset metadata.
// This helper isolates the type assertion from `any` to `ToolDependencies`,
// so tool implementations remain fully typed without assertions scattered throughout.
func NewTool[In, Out any](toolset registry.ToolsetMetadata, tool mcp.Tool, handler func(deps ToolDependencies) mcp.ToolHandlerFor[In, Out]) registry.ServerTool {
	return registry.NewServerTool(tool, toolset, func(d any) mcp.ToolHandlerFor[In, Out] {
		return handler(d.(ToolDependencies))
	})
}

// NewToolFromHandler creates a ServerTool with fully-typed ToolDependencies and toolset metadata
// for handlers that conform to mcp.ToolHandler directly.
func NewToolFromHandler(toolset registry.ToolsetMetadata, tool mcp.Tool, handler func(deps ToolDependencies) mcp.ToolHandler) registry.ServerTool {
	return registry.NewServerToolFromHandler(tool, toolset, func(d any) mcp.ToolHandler {
		return handler(d.(ToolDependencies))
	})
}
