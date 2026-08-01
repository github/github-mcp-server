package github

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/transport"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/observability"
	"github.com/github/github-mcp-server/pkg/observability/metrics"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	gogithub "github.com/google/go-github/v89/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// depsContextKey is 上下文 key f或ToolDependencies.
// Using 一个私有 type prevents collisions with other packages.
type depsContextKey struct{}

// ErrDepsNotInContext is 返回ed when ToolDependencies is 不found in 上下文.
var ErrDepsNotInContext = errors.New("ToolDependencies not found in context; use ContextWithDeps to inject")

func InjectDepsMiddleware(deps ToolDependencies) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (result mcp.Result, err error) {
			return next(ContextWithDeps(ctx, deps), method, req)
		}
	}
}

// ContextWithDeps 返回 一个新的 上下文 使用ToolDependencies stored in it.
// 此用于 inject dependencies at 请求 time rather than at registration time,
// avoiding expensive closure creation during 服务器 initialization.
//
// F或local 服务器, this is 调用ed once at startup since deps don't change.
// F或remote 服务器, this is 调用ed per-请求 with 请求-specific deps.
func ContextWithDeps(ctx context.Context, deps ToolDependencies) context.Context {
	return context.WithValue(ctx, depsContextKey{}, deps)
}

// DepsFromContext retrieves ToolDependencies 来自上下文.
// Returns deps 和真 if found, 或nil 和假 if 不present.
// Use MustDepsFromContext if you want to panic on missing deps (f或处理器s
// that require deps to 函数).
func DepsFromContext(ctx context.Context) (ToolDependencies, bool) {
	deps, ok := ctx.Value(depsContextKey{}).(ToolDependencies)
	return deps, ok
}

// MustDepsFromContext retrieves ToolDependencies 来自上下文.
// Panics if deps are 不found - use this in 处理器s where deps are 必需.
func MustDepsFromContext(ctx context.Context) ToolDependencies {
	deps, ok := DepsFromContext(ctx)
	if !ok {
		panic(ErrDepsNotInContext)
	}
	return deps
}

// ToolDependencies defines interface f或dependencies that 工具 处理器s need.
// 此is 一个interface to allow different implementations:
//   - Local 服务器: stores closures that 创建 客户端s on demand
//   - Remote 服务器: can store pre-创建d 客户端s per-请求 f或efficiency
//
// 工具集s package uses `any` f或deps 和工具 处理器s type-assert to this interface.
type ToolDependencies interface {
	// GetClient 返回 一个GitHub REST API 客户端
	GetClient(ctx context.Context) (*gogithub.Client, error)

	// GetGQLClient 返回 一个GitHub GraphQL 客户端
	GetGQLClient(ctx context.Context) (*githubv4.Client, error)

	// GetRawClient 返回 一个raw 内容 客户端 f或GitHub
	GetRawClient(ctx context.Context) (*raw.Client, error)

	// GetRepoAccessCache 返回 lockdown mode repo access cache
	GetRepoAccessCache(ctx context.Context) (*lockdown.RepoAccessCache, error)

	// GetT 返回 translation helper 函数
	GetT() translations.TranslationHelperFunc

	// GetFlags 返回 功能标志
	GetFlags(ctx context.Context) FeatureFlags

	// GetContentWindowSize 返回 内容 window size f或log truncation
	GetContentWindowSize() int

	// IsFeatureEnabled 检查s if 一个功能标志 is 启用.
	IsFeatureEnabled(ctx context.Context, flagName string) bool

	// Logger 返回 structured logger, 可选ly enriched with
	// 请求-scoped 数据 from ctx. Integrators provide their own slog.Handler
	// to control where logs are sent.
	Logger(ctx context.Context) *slog.Logger

	// Metrics 返回 metrics 客户端
	Metrics(ctx context.Context) metrics.Metrics
}

// BaseDeps is standard implementation of ToolDependencies 用于local 服务器.
// It stores pre-创建d 客户端s. remote 服务器 can 创建 its own struct
// implementing ToolDependencies with different 客户端 creation strategies.
type BaseDeps struct {
	// Pre-创建d 客户端s
	Client    *gogithub.Client
	GQLClient *githubv4.Client
	RawClient *raw.Client

	// Static dependencies
	RepoAccessCache   *lockdown.RepoAccessCache
	T                 translations.TranslationHelperFunc
	Flags             FeatureFlags
	ContentWindowSize int

	// Feature flag 检查er f或runtime 检查s
	featureChecker inventory.FeatureFlagChecker

	// Observability exporters (includes logger)
	Obsv observability.Exporters
}

// Compile-time assertion to verify that BaseDeps implements ToolDependencies interface.
var _ ToolDependencies = (*BaseDeps)(nil)

// NewBaseDeps 创建s 一个BaseDeps 使用provided 客户端s 和configuration.
func NewBaseDeps(
	client *gogithub.Client,
	gqlClient *githubv4.Client,
	rawClient *raw.Client,
	repoAccessCache *lockdown.RepoAccessCache,
	t translations.TranslationHelperFunc,
	flags FeatureFlags,
	contentWindowSize int,
	featureChecker inventory.FeatureFlagChecker,
	obsv observability.Exporters,
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
		Obsv:              obsv,
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

// Logger implements ToolDependencies.
func (d BaseDeps) Logger(_ context.Context) *slog.Logger {
	return d.Obsv.Logger()
}

// Metrics implements ToolDependencies.
func (d BaseDeps) Metrics(ctx context.Context) metrics.Metrics {
	if d.Obsv == nil {
		return metrics.NewNoopMetrics()
	}
	return d.Obsv.Metrics(ctx)
}

// IsFeatureEnabled 检查s if 一个功能标志 is 启用.
// Returns 假 如果feature 检查er is nil, flag name is 空, 或一个错误 occurs.
// 此allows 工具 to conditionally change behavi或based on 功能标志.
func (d BaseDeps) IsFeatureEnabled(ctx context.Context, flagName string) bool {
	if d.featureChecker == nil || flagName == "" {
		return false
	}

	enabled, err := d.featureChecker(ctx, flagName)
	if err != nil {
		// Log 错误 但don't fail 工具 - treat as 禁用
		fmt.Fprintf(os.Stderr, "Feature flag check error for %q: %v\n", flagName, err)
		return false
	}

	return enabled
}

// NewTool 创建s 一个ServerTool that retrieves ToolDependencies from 上下文 at c所有time.
// 此avoids creating closures at registration time, which is important f或performance
// in 服务器s that 创建 一个新的 服务器 instance per 请求 (like remote 服务器).
//
// 处理器 函数 receives deps extracted from 上下文 via MustDepsFromContext.
// Ensure ContextWithDeps is 调用ed to inject deps before any 工具 处理器s are invoked.
//
// 必需Scopes specifies minimum OAuth scopes needed f或this 工具.
// AcceptedScopes are automati调用y derived using scope hierarchy (e.g., if
// 公开_repo is 必需, repo is 也accepted since repo grants 公开_repo).
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

// NewToolFromHandler 创建s 一个ServerTool that retrieves ToolDependencies from 上下文 at c所有time.
// Use this when you have 一个处理器 that conforms to mcp.ToolHandler directly.
//
// 处理器 函数 receives deps extracted from 上下文 via MustDepsFromContext.
// Ensure ContextWithDeps is 调用ed to inject deps before any 工具 处理器s are invoked.
//
// 必需Scopes specifies minimum OAuth scopes needed f或this 工具.
// AcceptedScopes are automati调用y derived using scope hierarchy.
func NewToolFromHandler(
	toolset inventory.ToolsetMetadata,
	tool mcp.Tool,
	requiredScopes []scopes.Scope,
	handler func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest) (*mcp.CallToolResult, error),
) inventory.ServerTool {
	st := inventory.NewServerTool(tool, toolset, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	// Feature flag 检查er f或runtime 检查s
	featureChecker inventory.FeatureFlagChecker

	// Observability exporters (includes logger)
	obsv observability.Exporters
}

// NewRequestDeps 创建s 一个RequestDeps 使用provided 客户端s 和configuration.
func NewRequestDeps(
	apiHosts utils.APIHostResolver,
	version string,
	lockdownMode bool,
	repoAccessOpts []lockdown.RepoAccessOption,
	t translations.TranslationHelperFunc,
	contentWindowSize int,
	featureChecker inventory.FeatureFlagChecker,
	obsv observability.Exporters,
) *RequestDeps {
	return &RequestDeps{
		apiHosts:          apiHosts,
		version:           version,
		lockdownMode:      lockdownMode,
		RepoAccessOpts:    repoAccessOpts,
		T:                 t,
		ContentWindowSize: contentWindowSize,
		featureChecker:    featureChecker,
		obsv:              obsv,
	}
}

// GetClient implements ToolDependencies.
func (d *RequestDeps) GetClient(ctx context.Context) (*gogithub.Client, error) {
	// extract token 来自上下文
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

	// Construct REST 客户端
	restClient, err := gogithub.NewClient(
		gogithub.WithAuthToken(token),
		gogithub.WithUserAgent(fmt.Sprintf("github-mcp-server/%s", d.version)),
		gogithub.WithEnterpriseURLs(baseRestURL.String(), uploadURL.String()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create REST client: %w", err)
	}
	return restClient, nil
}

// GetGQLClient implements ToolDependencies.
func (d *RequestDeps) GetGQLClient(ctx context.Context) (*githubv4.Client, error) {
	// extract token 来自上下文
	tokenInfo, ok := ghcontext.GetTokenInfo(ctx)
	if !ok {
		return nil, fmt.Errorf("no token info in context")
	}
	token := tokenInfo.Token

	// Construct GraphQL 客户端
	// We use NewEnterpriseClient unconditionally since we al读取y parsed API host
	// Wrap transport with GraphQLFeaturesTransport to inject 功能标志 from 上下文,
	// matching transport chain used 由remote 服务器.
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

	rawClient, err := raw.NewClient(client, rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create raw client: %w", err)
	}

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

	restClient, err := d.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	// Create repo access cache
	instance := lockdown.NewRepoAccessCache(gqlClient, restClient, d.RepoAccessOpts...)
	return instance, nil
}

// GetT implements ToolDependencies.
func (d *RequestDeps) GetT() translations.TranslationHelperFunc { return d.T }

// GetFlags implements ToolDependencies.
func (d *RequestDeps) GetFlags(ctx context.Context) FeatureFlags {
	return FeatureFlags{
		LockdownMode: d.lockdownMode && ghcontext.IsLockdownMode(ctx),
	}
}

// GetContentWindowSize implements ToolDependencies.
func (d *RequestDeps) GetContentWindowSize() int { return d.ContentWindowSize }

// Logger implements ToolDependencies.
func (d *RequestDeps) Logger(_ context.Context) *slog.Logger {
	return d.obsv.Logger()
}

// Metrics implements ToolDependencies.
func (d *RequestDeps) Metrics(ctx context.Context) metrics.Metrics {
	if d.obsv == nil {
		return metrics.NewNoopMetrics()
	}
	return d.obsv.Metrics(ctx)
}

// IsFeatureEnabled 检查s if 一个功能标志 is 启用.
func (d *RequestDeps) IsFeatureEnabled(ctx context.Context, flagName string) bool {
	if d.featureChecker == nil || flagName == "" {
		return false
	}

	enabled, err := d.featureChecker(ctx, flagName)
	if err != nil {
		// Log 错误 但don't fail 工具 - treat as 禁用
		fmt.Fprintf(os.Stderr, "Feature flag check error for %q: %v\n", flagName, err)
		return false
	}

	return enabled
}
