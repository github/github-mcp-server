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

	// Whether to enable dynamic toolsets
	// See: https://github.com/github/github-mcp-server?tab=readme-ov-file#dynamic-tool-discovery
	DynamicToolsets bool

	// ReadOnly indicates if we should only offer read-only tools
	ReadOnly bool

	// Translator provides translated text for the server tooling
	Translator translations.TranslationHelperFunc

	// Content window size
	ContentWindowSize int

	// LockdownMode indicates if we should enable lockdown mode
	LockdownMode bool

	// Insider indicates if we should enable experimental features
	InsiderMode bool

	// Logger is used for logging within the server
	Logger *slog.Logger
	// RepoAccessTTL overrides the default TTL for repository access cache entries.
	RepoAccessTTL *time.Duration

	// TokenScopes contains the OAuth scopes available to the token.
	// When non-nil, tools requiring scopes not in this list will be hidden.
	// This is used for PAT scope filtering where we can't issue scope challenges.
	TokenScopes []string

	// Additional server options to apply
	ServerOptions []MCPServerOption
}

type MCPServerOption func(*mcp.ServerOptions)

func NewMCPServer(cfg *MCPServerConfig, deps ToolDependencies, inventory *inventory.Inventory) (*mcp.Server, error) {
	// Create the MCP server
	serverOpts := &mcp.ServerOptions{
		Instructions:      inventory.Instructions(),
		Logger:            cfg.Logger,
		CompletionHandler: CompletionsHandler(deps.GetClient),
	}

	// Apply any additional server options
	for _, o := range cfg.ServerOptions {
		o(serverOpts)
	}

	// In dynamic mode, explicitly advertise capabilities since tools/resources/prompts
	// may be enabled at runtime even if none are registered initially.
	if cfg.DynamicToolsets {
		serverOpts.Capabilities = &mcp.ServerCapabilities{
			Tools:     &mcp.ToolCapabilities{},
			Resources: &mcp.ResourceCapabilities{},
			Prompts:   &mcp.PromptCapabilities{},
		}
	}

	ghServer := NewServer(cfg.Version, serverOpts)

	// Add middlewares
	ghServer.AddReceivingMiddleware(addGitHubAPIErrorToContext)
	ghServer.AddReceivingMiddleware(InjectDepsMiddleware(deps))

	if unrecognized := inventory.UnrecognizedToolsets(); len(unrecognized) > 0 {
		cfg.Logger.Warn("Warning: unrecognized toolsets ignored", "toolsets", strings.Join(unrecognized, ", "))
	}

	// Register GitHub tools/resources/prompts from the inventory.
	// In dynamic mode with no explicit toolsets, this is a no-op since enabledToolsets
	// is empty - users enable toolsets at runtime via the dynamic tools below (but can
	// enable toolsets or tools explicitly that do need registration).
	inventory.RegisterAll(context.Background(), ghServer, deps)

	// Register dynamic toolset management tools (enable/disable) - these are separate
	// meta-tools that control the inventory, not part of the inventory itself
	if cfg.DynamicToolsets {
		registerDynamicTools(ghServer, inventory, deps, cfg.Translator)
	}

	return ghServer, nil
}

// registerDynamicTools adds the dynamic toolset enable/disable tools to the server.
func registerDynamicTools(server *mcp.Server, inventory *inventory.Inventory, deps ToolDependencies, t translations.TranslationHelperFunc) {
	dynamicDeps := DynamicToolDependencies{
		Server:    server,
		Inventory: inventory,
		ToolDeps:  deps,
		T:         t,
	}
	for _, tool := range DynamicTools(inventory) {
		tool.RegisterFunc(server, dynamicDeps)
	}
}

// createFeatureChecker returns a FeatureFlagChecker that checks if a flag name
// is present in the provided list of enabled features. For the local server,
// this is populated from the --features CLI flag.
func createFeatureChecker(enabledFeatures []string) inventory.FeatureFlagChecker {
	// Build a set for O(1) lookup
	featureSet := make(map[string]bool, len(enabledFeatures))
	for _, f := range enabledFeatures {
		featureSet[f] = true
	}
	return func(_ context.Context, flagName string) (bool, error) {
		return featureSet[flagName], nil
	}
}

// resolveEnabledToolsets determines which toolsets should be enabled based on config.
// Returns nil for "use defaults", empty slice for "none", or explicit list.
func resolveEnabledToolsets(cfg *MCPServerConfig) []string {
	enabledToolsets := cfg.EnabledToolsets

	// In dynamic mode, remove "all" and "default" since users enable toolsets on demand
	if cfg.DynamicToolsets && enabledToolsets != nil {
		enabledToolsets = RemoveToolset(enabledToolsets, string(ToolsetMetadataAll.ID))
		enabledToolsets = RemoveToolset(enabledToolsets, string(ToolsetMetadataDefault.ID))
	}

	if enabledToolsets != nil {
		return enabledToolsets
	}
	if cfg.DynamicToolsets {
		// Dynamic mode with no toolsets specified: start empty so users enable on demand
		return []string{}
	}
	if len(cfg.EnabledTools) > 0 {
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

// NewServer creates a new GitHub MCP server with the specified GH client and logger.
func NewServer(version string, opts *mcp.ServerOptions) *mcp.Server {
	if opts == nil {
		opts = &mcp.ServerOptions{}
	}

	// Create a new MCP server
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "github-mcp-server",
		Title:   "GitHub MCP Server",
		Version: version,
		Icons:   octicons.Icons("mark-github"),
	}, opts)

	return s
}

func CompletionsHandler(getClient GetClientFn) func(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	return func(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
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
