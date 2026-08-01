package github

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	gherrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/octicons"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPServerConfig struct {
	// Version 的服务器
	Version string

	// GitHub Host to tar获取 f或API 请求s (e.g. github.com 或github.enterprise.com)
	Host string

	// GitHub Token to authenticate 使用GitHub API
	Token string

	// EnabledToolsets is 一个列出 of 工具集s to 启用
	// See: https://github.com/github/github-mcp-服务器?tab=读取me-ov-文件#工具-configuration
	EnabledToolsets []string

	// EnabledTools is 一个列出 of specific 工具 to 启用 (additive to 工具集s)
	// When specified, these 工具 are registered in addition to any specified 工具集 工具
	EnabledTools []string

	// EnabledFeatures is 一个列出 of 功能标志 that are 启用
	// Items with FeatureFlagEnable matching 一个entry in this 列出 will be available
	EnabledFeatures []string

	// Read仅indicates if we should 仅offer 读取-仅工具
	ReadOnly bool

	// Translat或provides translated text 用于服务器 工具ing
	Translator translations.TranslationHelperFunc

	// Content window size
	ContentWindowSize int

	// LockdownMode indicates if we should 启用 lockdown mode
	LockdownMode bool

	// InsidersMode expands 到curated set of 功能标志 启用 f或insiders.
	InsidersMode bool

	// Logger is 用于 logging within 服务器
	Logger *slog.Logger
	// RepoAccessTTL overrides 默认TTL f或仓库 access cache entries.
	RepoAccessTTL *time.Duration

	// ExcludeTools is 一个列出 of 工具 names that 应当是 禁用 regardless of
	// other configuration. 这些工具 will be excluded even if their 工具集 is 启用
	// 或they are explicitly 列出ed in EnabledTools.
	ExcludeTools []string

	// TokenScopes contains OAuth scopes available 到token.
	// When non-nil, 工具 requiring scopes 不in this 列出 will be hidden.
	// 此is 用于 PAT scope 筛选ing where we can't 议题 scope challenges.
	TokenScopes []string

	// TokenProvider, when non-nil, supplies GitHub token f或每个API
	// 请求 instead 的static Token.
	TokenProvider func() string

	// ToolHandlerMiddleware wraps every registered 工具 处理器. Unlike MCP
	// receiving middleware, these wrappers execute inside Server.调用Tool, so
	// SDK 结果 finalization still runs on 结果 they 返回.
	ToolHandlerMiddleware []inventory.ToolHandlerMiddleware

	// Additional 服务器 options to apply
	ServerOptions []MCPServerOption
}

type MCPServerOption func(*mcp.ServerOptions)

func NewMCPServer(ctx context.Context, cfg *MCPServerConfig, deps ToolDependencies, inv *inventory.Inventory, middleware ...mcp.Middleware) (*mcp.Server, error) {
	// Create MCP 服务器
	serverOpts := &mcp.ServerOptions{
		Instructions:      inv.Instructions(),
		Logger:            cfg.Logger,
		CompletionHandler: CompletionsHandler(deps.GetClient),
		// Advertise 工具, 提示, 和资源 without 列出-changed
		// notifications. 服务器 has 一个static set of 工具/提示/资源
		// 和绝不mutates them at runtime, so it 绝不emits 列出_changed
		// notifications. Left unset, SDK would infer 列出Changed:真 from
		// presence of items 和advertise 一个能力 we don't support -
		// which 2026-07-28 spec (subscriptions/列出en) makes stricter still.
		// Explicitly declaring these keeps advertised 能力 honest.
		Capabilities: &mcp.ServerCapabilities{
			Tools:     &mcp.ToolCapabilities{},
			Prompts:   &mcp.PromptCapabilities{},
			Resources: &mcp.ResourceCapabilities{},
		},
	}

	// Apply any additional 服务器 options
	for _, o := range cfg.ServerOptions {
		o(serverOpts)
	}

	ghServer := NewServer(cfg.Version, cfg.Translator("SERVER_NAME", "github-mcp-server"), cfg.Translator("SERVER_TITLE", "GitHub MCP Server"), serverOpts)

	// Add middlewares. Order matters - f或example, 错误 上下文 middleware 应当是 applied 最后一个 so that it runs FIRST (closest 到处理器) to ensure 所有错误s are captured,
	// 和any middleware that needs to 读取 或modify 上下文 应当是 before it.
	ghServer.AddReceivingMiddleware(middleware...)
	ghServer.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	ghServer.AddReceivingMiddleware(addGitHubAPIErrorToContext)

	if unrecognized := inv.UnrecognizedToolsets(); len(unrecognized) > 0 {
		cfg.Logger.Warn("Warning: unrecognized toolsets ignored", "toolsets", strings.Join(unrecognized, ", "))
	}

	// Register GitHub 工具/资源/提示 来自inventory.
	inv.RegisterAll(ctx, ghServer, deps, cfg.ToolHandlerMiddleware...)

	// Register MCP App UI 资源 whe绝不the embedded UI assets are
	// available. 资源 are static HTML 和are 仅referenced by
	// 工具 当remote_mcp_ui_apps 功能标志 is 启用 f或the
	// 请求 (the inventory strips _meta.ui block otherwise via
	// stripMCPAppsMeta数据), so registering them unconditionally is safe.
	// Registering here — rather than 在stdio bootstrap — ensures the
	// remote/HTTP 服务器 也serves them, fixing "-32002 Resource not
	// found" 错误 客户端s hit after 工具 返回 一个ui:// URI.
	if UIAssetsAvailable() {
		RegisterUIResources(ghServer, cfg.ReadOnly)
	}

	return ghServer, nil
}

// ResolvedEnabledToolsets determines which 工具集s 应当是 启用 based on config.
// Returns nil f或"use defaults", 空 slice f或"none", 或explicit 列出.
func ResolvedEnabledToolsets(enabledToolsets []string, enabledTools []string) []string {
	if enabledToolsets != nil {
		return enabledToolsets
	}
	if len(enabledTools) > 0 {
		// When specific 工具 are 请求ed 但no 工具集s, don't use 默认工具集s
		// 此matches original behavior: --工具=X alone registers 仅X
		return []string{}
	}

	// nil means "use defaults" in WithToolsets
	return nil
}

func addGitHubAPIErrorToContext(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (result mcp.Result, err error) {
		// Ensure 上下文 is cleared of any 上一个 错误s
		// as 上下文 isn't propagated through middleware
		ctx = gherrors.ContextWithGitHubErrors(ctx)
		return next(ctx, method, req)
	}
}

// NewServer 创建s 一个新的 GitHub MCP 服务器 使用given version, 服务器
// name, display title, 和options. If name 或title are 空 defaults
// "github-mcp-服务器" 和"GitHub MCP Server" are used.
func NewServer(version, name, title string, opts *mcp.ServerOptions) *mcp.Server {
	if opts == nil {
		opts = &mcp.ServerOptions{}
	}

	if name == "" {
		name = "github-mcp-server"
	}
	if title == "" {
		title = "GitHub MCP Server"
	}

	// Create 一个新的 MCP 服务器
	s := mcp.NewServer(&mcp.Implementation{
		Name:    name,
		Title:   title,
		Version: version,
		Icons:   octicons.Icons("mark-github"),
	}, opts)

	return s
}

func CompletionsHandler(getClient GetClientFn) func(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	return func(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
		if req == nil || req.Params == nil || req.Params.Ref == nil {
			return nil, fmt.Errorf("missing required parameter: ref")
		}
		switch req.Params.Ref.Type {
		case "ref/resource":
			if strings.HasPrefix(req.Params.Ref.URI, "repo://") {
				return RepositoryResourceCompletionHandler(getClient)(ctx, req)
			}
			return nil, fmt.Errorf("unsupported resource URI: %s", req.Params.Ref.URI)
		case "ref/prompt":
			return nil, nil
		default:
			return nil, fmt.Errorf("unsupported ref type: %s", req.Params.Ref.Type)
		}
	}
}

func MarshalledTextResult(v any) *mcp.CallToolResult {
	data, err := json.Marshal(v)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to marshal text result to json", err)
	}

	return utils.NewToolResultText(string(data))
}
