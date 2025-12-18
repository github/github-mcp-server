package github

import (
	"context"
	"errors"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v79/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// depsContextKey is the context key for ToolDependencies.
// Using a private type prevents collisions with other packages.
type depsContextKey struct{}

// ErrDepsNotInContext is returned when ToolDependencies is not found in context.
var ErrDepsNotInContext = errors.New("ToolDependencies not found in context; use ContextWithDeps to inject")

// ContextWithDeps returns a new context with the ToolDependencies stored in it.
// This is used to inject dependencies at request time rather than at registration time,
// avoiding expensive closure creation during server initialization.
//
// For the local server, this is called once at startup since deps don't change.
// For the remote server, this is called per-request with request-specific deps.
func ContextWithDeps(ctx context.Context, deps ToolDependencies) context.Context {
	return context.WithValue(ctx, depsContextKey{}, deps)
}

// DepsFromContext retrieves ToolDependencies from the context.
// Returns the deps and true if found, or nil and false if not present.
// Use MustDepsFromContext if you want to panic on missing deps (for handlers
// that require deps to function).
func DepsFromContext(ctx context.Context) (ToolDependencies, bool) {
	deps, ok := ctx.Value(depsContextKey{}).(ToolDependencies)
	return deps, ok
}

// MustDepsFromContext retrieves ToolDependencies from the context.
// Panics if deps are not found - use this in handlers where deps are required.
func MustDepsFromContext(ctx context.Context) ToolDependencies {
	deps, ok := DepsFromContext(ctx)
	if !ok {
		panic(ErrDepsNotInContext)
	}
	return deps
}

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

// NewTool creates a ServerTool that retrieves ToolDependencies from context at call time.
// This avoids creating closures at registration time, which is important for performance
// in servers that create a new server instance per request (like the remote server).
//
// The handler function receives deps extracted from context via MustDepsFromContext.
// Ensure ContextWithDeps is called to inject deps before any tool handlers are invoked.
func NewTool[In, Out any](toolset inventory.ToolsetMetadata, tool mcp.Tool, handler func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, args In) (*mcp.CallToolResult, Out, error)) inventory.ServerTool {
	return inventory.NewServerToolWithContextHandler(tool, toolset, func(ctx context.Context, req *mcp.CallToolRequest, args In) (*mcp.CallToolResult, Out, error) {
		deps := MustDepsFromContext(ctx)
		return handler(ctx, deps, req, args)
	})
}

// NewToolFromHandler creates a ServerTool that retrieves ToolDependencies from context at call time.
// Use this when you have a handler that conforms to mcp.ToolHandler directly.
//
// The handler function receives deps extracted from context via MustDepsFromContext.
// Ensure ContextWithDeps is called to inject deps before any tool handlers are invoked.
func NewToolFromHandler(toolset inventory.ToolsetMetadata, tool mcp.Tool, handler func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest) (*mcp.CallToolResult, error)) inventory.ServerTool {
	return inventory.NewServerToolWithRawContextHandler(tool, toolset, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		deps := MustDepsFromContext(ctx)
		return handler(ctx, deps, req)
	})
}
