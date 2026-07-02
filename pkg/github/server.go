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
	// Version of the server
	Version string

	// GitHub Host to target for API requests (e.g. github.com or github.enterprise.com)
	Host string

	// GitHub Token to authenticate with the GitHub API
	Token string

	// EnabledToolsets is a list of toolsets to enable
	// See: https://github.com/github/github-mcp-server?tab=readme-ov-file#tool-configuration
	EnabledToolsets []string

	// EnabledTools is a list of specific tools to enable (additive to toolsets)
	// When specified, these tools are registered in addition to any specified toolset tools
	EnabledTools []string

	// EnabledFeatures is a list of feature flags that are enabled
	// Items with FeatureFlagEnable matching an entry in this list will be available
	EnabledFeatures []string

	// ReadOnly indicates if we should only offer read-only tools
	ReadOnly bool

	// Translator provides translated text for the server tooling
	Translator translations.TranslationHelperFunc

	// Content window size
	ContentWindowSize int

	// LockdownMode indicates if we should enable lockdown mode
	LockdownMode bool

	// InsidersMode expands to the curated set of feature flags enabled for insiders.
	InsidersMode bool

	// Logger is used for logging within the server
	Logger *slog.Logger
	// RepoAccessTTL overrides the default TTL for repository access cache entries.
	RepoAccessTTL *time.Duration

	// ExcludeTools is a list of tool names that should be disabled regardless of
	// other configuration. These tools will be excluded even if their toolset is enabled
	// or they are explicitly listed in EnabledTools.
	ExcludeTools []string

	// TokenScopes contains the OAuth scopes available to the token.
	// When non-nil, tools requiring scopes not in this list will be hidden.
	// This is used for PAT scope filtering where we can't issue scope challenges.
	TokenScopes []string

	// TokenProvider, when non-nil, supplies the GitHub token for each API
	// request instead of the static Token. It backs OAuth login, where the
	// token is obtained lazily on first use and refreshed thereafter.
	TokenProvider func() string

	// Additional server options to apply
	ServerOptions []MCPServerOption
}

type MCPServerOption func(*mcp.ServerOptions)

func NewMCPServer(ctx context.Context, cfg *MCPServerConfig, deps ToolDependencies, inv *inventory.Inventory, middleware ...mcp.Middleware) (*mcp.Server, error) {
	// Create the MCP server
	serverOpts := &mcp.ServerOptions{
		Instructions:      inv.Instructions(),
		Logger:            cfg.Logger,
		CompletionHandler: CompletionsHandler(deps.GetClient),
	}

	// Apply any additional server options
	for _, o := range cfg.ServerOptions {
		o(serverOpts)
	}

	ghServer := NewServer(cfg.Version, cfg.Translator("SERVER_NAME", "github"), cfg.Translator("SERVER_TITLE", "GitHub MCP Server"), serverOpts)

	// Add middlewares. Order matters - for example, the error context middleware should be applied last so that it runs FIRST (closest to the handler) to ensure all errors are captured,
	// and any middleware that needs to read or modify the context should be before it.
	ghServer.AddReceivingMiddleware(middleware...)
	ghServer.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	ghServer.AddReceivingMiddleware(addGitHubAPIErrorToContext)

	if unrecognized := inv.UnrecognizedToolsets(); len(unrecognized) > 0 {
		cfg.Logger.Warn("Warning: unrecognized toolsets ignored", "toolsets", strings.Join(unrecognized, ", "))
	}

	// Register GitHub tools/resources/prompts from the inventory.
	inv.RegisterAll(ctx, ghServer, deps)

	// Register MCP App UI resources whenever the embedded UI assets are
	// available. The resources are static HTML and are only referenced by
	// tools when the remote_mcp_ui_apps feature flag is enabled for the
	// request (the inventory strips the _meta.ui block otherwise via
	// stripMCPAppsMetadata), so registering them unconditionally is safe.
	// Registering here — rather than in the stdio bootstrap — ensures the
	// remote/HTTP server also serves them, fixing the "-32002 Resource not
	// found" error clients hit after the tool returns a ui:// URI.
	if UIAssetsAvailable() {
		RegisterUIResources(ghServer, cfg.ReadOnly)
	}

	return ghServer, nil
}

// ResolvedEnabledToolsets determines which toolsets should be enabled based on config.
// Returns nil for "use defaults", empty slice for "none", or explicit list.
func ResolvedEnabledToolsets(enabledToolsets []string, enabledTools []string) []string {
	if enabledToolsets != nil {
		return enabledToolsets
	}
	if len(enabledTools) > 0 {
		// When specific tools are requested but no toolsets, don't use default toolsets
		// This matches the original behavior: --tools=X alone registers only X
		return []string{}
	}

	// nil means "use defaults" in WithToolsets
	return nil
}

func addGitHubAPIErrorToContext(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (result mcp.Result, err error) {
		// Ensure the context is cleared of any previous errors
		// as context isn't propagated through middleware
		ctx = gherrors.ContextWithGitHubErrors(ctx)
		return next(ctx, method, req)
	}
}

// NewServer creates a new GitHub MCP server with the given version, server
// name, display title, and options. If name or title are empty the defaults
// "github" and "GitHub MCP Server" are used.
//
// The "github" name aligns with the configuration key VS Code derives from
// the human-readable title, so tool prefixes like "github/*" resolve. It
// intentionally diverges from the MCP registry identifier
// (io.github.github/github-mcp-server in server.json), which uses the source
// repo's last segment per the registry naming convention.
func NewServer(version, name, title string, opts *mcp.ServerOptions) *mcp.Server {
	if opts == nil {
		opts = &mcp.ServerOptions{}
	}

	if name == "" {
		name = "github"
	}
	if title == "" {
		title = "GitHub MCP Server"
	}

	// Create a new MCP server
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
