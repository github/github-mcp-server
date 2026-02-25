package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/transport"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// depsContextKey is the context key for ToolDependencies.
// Using a private type prevents collisions with other packages.
type depsContextKey struct{}

// ErrDepsNotInContext is returned when ToolDependencies is not found in context.
var ErrDepsNotInContext = errors.New("ToolDependencies not found in context; use ContextWithDeps to inject")

func InjectDepsMiddleware(deps ToolDependencies) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (result mcp.Result, err error) {
			return next(ContextWithDeps(ctx, deps), method, req)
		}
	}
}

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
	GetRepoAccessCache(ctx context.Context) (*lockdown.RepoAccessCache, error)

	// GetT returns the translation helper function
	GetT() translations.TranslationHelperFunc

	// GetFlags returns feature flags
	GetFlags(ctx context.Context) FeatureFlags

	// GetContentWindowSize returns the content window size for log truncation
	GetContentWindowSize() int

	// IsFeatureEnabled checks if a feature flag is enabled.
	IsFeatureEnabled(ctx context.Context, flagName string) bool
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

	// Feature flag checker for runtime checks
	featureChecker inventory.FeatureFlagChecker
}

// Compile-time assertion to verify that BaseDeps implements the ToolDependencies interface.
var _ ToolDependencies = (*BaseDeps)(nil)

// NewBaseDeps creates a BaseDeps with the provided clients and configuration.
func NewBaseDeps(
	client *gogithub.Client,
	gqlClient *githubv4.Client,
	rawClient *raw.Client,
	repoAccessCache *lockdown.RepoAccessCache,
	t translations.TranslationHelperFunc,
	flags FeatureFlags,
	contentWindowSize int,
	featureChecker inventory.FeatureFlagChecker,
) *BaseDeps {
	return &BaseDeps{
		Client:            client,
		GQLClient:         gqlClient,
		RawClient:         rawClient,
		RepoAccessCache:   repoAccessCache,
		T:                 t,
		Flags:             flags,
		ContentWindowSize: contentWindowSize,
		featureChecker:    featureChecker,
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
func (d BaseDeps) GetRepoAccessCache(_ context.Context) (*lockdown.RepoAccessCache, error) {
	return d.RepoAccessCache, nil
}

// GetT implements ToolDependencies.
func (d BaseDeps) GetT() translations.TranslationHelperFunc { return d.T }

// GetFlags implements ToolDependencies.
func (d BaseDeps) GetFlags(_ context.Context) FeatureFlags { return d.Flags }

// GetContentWindowSize implements ToolDependencies.
func (d BaseDeps) GetContentWindowSize() int { return d.ContentWindowSize }

// IsFeatureEnabled checks if a feature flag is enabled.
// Returns false if the feature checker is nil, flag name is empty, or an error occurs.
// This allows tools to conditionally change behavior based on feature flags.
func (d BaseDeps) IsFeatureEnabled(ctx context.Context, flagName string) bool {
	if d.featureChecker == nil || flagName == "" {
		return false
	}

	enabled, err := d.featureChecker(ctx, flagName)
	if err != nil {
		// Log error but don't fail the tool - treat as disabled
		fmt.Fprintf(os.Stderr, "Feature flag check error for %q: %v\n", flagName, err)
		return false
	}

	return enabled
}

// NewTool creates a ServerTool that retrieves ToolDependencies from context at call time.
// This avoids creating closures at registration time, which is important for performance
// in servers that create a new server instance per request (like the remote server).
//
// The handler function receives deps extracted from context via MustDepsFromContext.
// Ensure ContextWithDeps is called to inject deps before any tool handlers are invoked.
//
// requiredScopes specifies the minimum OAuth scopes needed for this tool.
// AcceptedScopes are automatically derived using the scope hierarchy (e.g., if
// public_repo is required, repo is also accepted since repo grants public_repo).
func NewTool[In, Out any](
	toolset inventory.ToolsetMetadata,
	tool mcp.Tool,
	requiredScopes []scopes.Scope,
	handler func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, args In) (*mcp.CallToolResult, Out, error),
) inventory.ServerTool {
	st := inventory.NewServerToolWithContextHandler(tool, toolset, func(ctx context.Context, req *mcp.CallToolRequest, args In) (*mcp.CallToolResult, Out, error) {
		deps := MustDepsFromContext(ctx)
		return handler(ctx, deps, req, args)
	})
	st.RequiredScopes = scopes.ToStringSlice(requiredScopes...)
	st.AcceptedScopes = scopes.ExpandScopes(requiredScopes...)
	return st
}

// NewToolFromHandler creates a ServerTool that retrieves ToolDependencies from context at call time.
// Use this when you have a handler that conforms to mcp.ToolHandler directly.
//
// The handler function receives deps extracted from context via MustDepsFromContext.
// Ensure ContextWithDeps is called to inject deps before any tool handlers are invoked.
//
// requiredScopes specifies the minimum OAuth scopes needed for this tool.
// AcceptedScopes are automatically derived using the scope hierarchy.
func NewToolFromHandler(
	toolset inventory.ToolsetMetadata,
	tool mcp.Tool,
	requiredScopes []scopes.Scope,
	handler func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest) (*mcp.CallToolResult, error),
) inventory.ServerTool {
	st := inventory.NewServerToolWithRawContextHandler(tool, toolset, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		deps := MustDepsFromContext(ctx)
		return handler(ctx, deps, req)
	})
	st.RequiredScopes = scopes.ToStringSlice(requiredScopes...)
	st.AcceptedScopes = scopes.ExpandScopes(requiredScopes...)
	return st
}

type RequestDeps struct {
	// Static dependencies
	apiHosts          utils.APIHostResolver
	version           string
	lockdownMode      bool
	RepoAccessOpts    []lockdown.RepoAccessOption
	T                 translations.TranslationHelperFunc
	ContentWindowSize int

	// Feature flag checker for runtime checks
	featureChecker inventory.FeatureFlagChecker
}

// NewRequestDeps creates a RequestDeps with the provided clients and configuration.
func NewRequestDeps(
	apiHosts utils.APIHostResolver,
	version string,
	lockdownMode bool,
	repoAccessOpts []lockdown.RepoAccessOption,
	t translations.TranslationHelperFunc,
	contentWindowSize int,
	featureChecker inventory.FeatureFlagChecker,
) *RequestDeps {
	return &RequestDeps{
		apiHosts:          apiHosts,
		version:           version,
		lockdownMode:      lockdownMode,
		RepoAccessOpts:    repoAccessOpts,
		T:                 t,
		ContentWindowSize: contentWindowSize,
		featureChecker:    featureChecker,
	}
}

// GetClient implements ToolDependencies.
func (d *RequestDeps) GetClient(ctx context.Context) (*gogithub.Client, error) {
	// extract the token from the context
	tokenInfo, ok := ghcontext.GetTokenInfo(ctx)
	if !ok {
		return nil, fmt.Errorf("no token info in context")
	}
	token := tokenInfo.Token

	baseRestURL, err := d.apiHosts.BaseRESTURL(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get base REST URL: %w", err)
	}
	uploadURL, err := d.apiHosts.UploadURL(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get upload URL: %w", err)
	}

	// Construct REST client
	restClient := gogithub.NewClient(nil).WithAuthToken(token)
	restClient.UserAgent = fmt.Sprintf("github-mcp-server/%s", d.version)
	restClient.BaseURL = baseRestURL
	restClient.UploadURL = uploadURL
	return restClient, nil
}

// GetGQLClient implements ToolDependencies.
func (d *RequestDeps) GetGQLClient(ctx context.Context) (*githubv4.Client, error) {
	// extract the token from the context
	tokenInfo, ok := ghcontext.GetTokenInfo(ctx)
	if !ok {
		return nil, fmt.Errorf("no token info in context")
	}
	token := tokenInfo.Token

	// Construct GraphQL client
	// We use NewEnterpriseClient unconditionally since we already parsed the API host
	// Wrap transport with GraphQLFeaturesTransport to inject feature flags from context,
	// matching the transport chain used by the remote server.
	gqlHTTPClient := &http.Client{
		Transport: &transport.BearerAuthTransport{
			Transport: &transport.GraphQLFeaturesTransport{
				Transport: http.DefaultTransport,
			},
			Token: token,
		},
	}

	graphqlURL, err := d.apiHosts.GraphqlURL(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get GraphQL URL: %w", err)
	}

	gqlClient := githubv4.NewEnterpriseClient(graphqlURL.String(), gqlHTTPClient)
	return gqlClient, nil
}

// GetRawClient implements ToolDependencies.
func (d *RequestDeps) GetRawClient(ctx context.Context) (*raw.Client, error) {
	client, err := d.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	rawURL, err := d.apiHosts.RawURL(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Raw URL: %w", err)
	}

	rawClient := raw.NewClient(client, rawURL)

	return rawClient, nil
}

// GetRepoAccessCache implements ToolDependencies.
func (d *RequestDeps) GetRepoAccessCache(ctx context.Context) (*lockdown.RepoAccessCache, error) {
	if !d.lockdownMode {
		return nil, nil
	}

	gqlClient, err := d.GetGQLClient(ctx)
	if err != nil {
		return nil, err
	}

	// Create repo access cache
	instance := lockdown.GetInstance(gqlClient, d.RepoAccessOpts...)
	return instance, nil
}

// GetT implements ToolDependencies.
func (d *RequestDeps) GetT() translations.TranslationHelperFunc { return d.T }

// GetFlags implements ToolDependencies.
func (d *RequestDeps) GetFlags(ctx context.Context) FeatureFlags {
	return FeatureFlags{
		LockdownMode: d.lockdownMode && ghcontext.IsLockdownMode(ctx),
		InsidersMode: ghcontext.IsInsidersMode(ctx),
	}
}

// GetContentWindowSize implements ToolDependencies.
func (d *RequestDeps) GetContentWindowSize() int { return d.ContentWindowSize }

// IsFeatureEnabled checks if a feature flag is enabled.
func (d *RequestDeps) IsFeatureEnabled(ctx context.Context, flagName string) bool {
	if d.featureChecker == nil || flagName == "" {
		return false
	}

	enabled, err := d.featureChecker(ctx, flagName)
	if err != nil {
		// Log error but don't fail the tool - treat as disabled
		fmt.Fprintf(os.Stderr, "Feature flag check error for %q: %v\n", flagName, err)
		return false
	}

	return enabled
}
