package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/octicons"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	gogithub "github.com/google/go-github/v79/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
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

	// Logger is used for logging within the server
	Logger *slog.Logger
	// RepoAccessTTL overrides the default TTL for repository access cache entries.
	RepoAccessTTL *time.Duration

	// TokenScopes contains the OAuth scopes available to the token.
	// When non-nil, tools requiring scopes not in this list will be hidden.
	// This is used for PAT scope filtering where we can't issue scope challenges.
	TokenScopes []string
}

// githubClients holds all the GitHub API clients created for a server instance.
type githubClients struct {
	rest       *gogithub.Client
	gql        *githubv4.Client
	gqlHTTP    *http.Client // retained for middleware to modify transport
	raw        *raw.Client
	repoAccess *lockdown.RepoAccessCache
}

// createGitHubClients creates all the GitHub API clients needed by the server.
func createGitHubClients(cfg MCPServerConfig, apiHost apiHost) (*githubClients, error) {
	// Construct REST client
	restClient := gogithub.NewClient(nil).WithAuthToken(cfg.Token)
	restClient.UserAgent = fmt.Sprintf("github-mcp-server/%s", cfg.Version)
	restClient.BaseURL = apiHost.baseRESTURL
	restClient.UploadURL = apiHost.uploadURL

	// Construct GraphQL client
	// We use NewEnterpriseClient unconditionally since we already parsed the API host
	gqlHTTPClient := &http.Client{
		Transport: &bearerAuthTransport{
			transport: http.DefaultTransport,
			token:     cfg.Token,
		},
	}
	gqlClient := githubv4.NewEnterpriseClient(apiHost.graphqlURL.String(), gqlHTTPClient)

	// Create raw content client (shares REST client's HTTP transport)
	rawClient := raw.NewClient(restClient, apiHost.rawURL)

	// Set up repo access cache for lockdown mode
	var repoAccessCache *lockdown.RepoAccessCache
	if cfg.LockdownMode {
		opts := []lockdown.RepoAccessOption{
			lockdown.WithLogger(cfg.Logger.With("component", "lockdown")),
		}
		if cfg.RepoAccessTTL != nil {
			opts = append(opts, lockdown.WithTTL(*cfg.RepoAccessTTL))
		}
		repoAccessCache = lockdown.GetInstance(gqlClient, opts...)
	}

	return &githubClients{
		rest:       restClient,
		gql:        gqlClient,
		gqlHTTP:    gqlHTTPClient,
		raw:        rawClient,
		repoAccess: repoAccessCache,
	}, nil
}

// resolveEnabledToolsets determines which toolsets should be enabled based on config.
// Returns nil for "use defaults", empty slice for "none", or explicit list.
func resolveEnabledToolsets(cfg MCPServerConfig) []string {
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

func NewMCPServer(cfg MCPServerConfig) (*mcp.Server, error) {
	apiHost, err := ParseAPIHost(cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse API host: %w", err)
	}

	clients, err := createGitHubClients(cfg, apiHost)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub clients: %w", err)
	}

	enabledToolsets := resolveEnabledToolsets(cfg)

	// For instruction generation, we need actual toolset names (not nil).
	// nil means "use defaults" in inventory, so expand it for instructions.
	instructionToolsets := enabledToolsets
	if instructionToolsets == nil {
		instructionToolsets = GetDefaultToolsetIDs()
	}

	// Create the MCP server
	serverOpts := &mcp.ServerOptions{
		Instructions: GenerateInstructions(instructionToolsets),
		Logger:       cfg.Logger,
		CompletionHandler: CompletionsHandler(func(_ context.Context) (*gogithub.Client, error) {
			return clients.rest, nil
		}),
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
	ghServer.AddReceivingMiddleware(addUserAgentsMiddleware(cfg, clients.rest, clients.gqlHTTP))

	// Create dependencies for tool handlers
	deps := NewBaseDeps(
		clients.rest,
		clients.gql,
		clients.raw,
		clients.repoAccess,
		cfg.Translator,
		FeatureFlags{LockdownMode: cfg.LockdownMode},
		cfg.ContentWindowSize,
	)

	// Inject dependencies into context for all tool handlers
	ghServer.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			return next(ContextWithDeps(ctx, deps), method, req)
		}
	})

	// Build and register the tool/resource/prompt inventory
	inventoryBuilder := NewInventory(cfg.Translator).
		WithDeprecatedAliases(DeprecatedToolAliases).
		WithReadOnly(cfg.ReadOnly).
		WithToolsets(enabledToolsets).
		WithTools(CleanTools(cfg.EnabledTools)).
		WithFeatureChecker(createFeatureChecker(cfg.EnabledFeatures))

	// Apply token scope filtering if scopes are known (for PAT filtering)
	if cfg.TokenScopes != nil {
		inventoryBuilder = inventoryBuilder.WithFilter(CreateToolScopeFilter(cfg.TokenScopes))
	}

	inventory := inventoryBuilder.Build()

	if unrecognized := inventory.UnrecognizedToolsets(); len(unrecognized) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: unrecognized toolsets ignored: %s\n", strings.Join(unrecognized, ", "))
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
func registerDynamicTools(server *mcp.Server, inventory *inventory.Inventory, deps *BaseDeps, t translations.TranslationHelperFunc) {
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

type userAgentTransport struct {
	transport http.RoundTripper
	agent     string
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", t.agent)
	return t.transport.RoundTrip(req)
}

type bearerAuthTransport struct {
	transport http.RoundTripper
	token     string
}

func (t *bearerAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.transport.RoundTrip(req)
}

func addGitHubAPIErrorToContext(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (result mcp.Result, err error) {
		// Ensure the context is cleared of any previous errors
		// as context isn't propagated through middleware
		ctx = ghErrors.ContextWithGitHubErrors(ctx)
		return next(ctx, method, req)
	}
}

func addUserAgentsMiddleware(cfg MCPServerConfig, restClient *gogithub.Client, gqlHTTPClient *http.Client) func(next mcp.MethodHandler) mcp.MethodHandler {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (result mcp.Result, err error) {
			if method != "initialize" {
				return next(ctx, method, request)
			}

			initializeRequest, ok := request.(*mcp.InitializeRequest)
			if !ok {
				return next(ctx, method, request)
			}

			message := initializeRequest
			userAgent := fmt.Sprintf(
				"github-mcp-server/%s (%s/%s)",
				cfg.Version,
				message.Params.ClientInfo.Name,
				message.Params.ClientInfo.Version,
			)

			restClient.UserAgent = userAgent

			gqlHTTPClient.Transport = &userAgentTransport{
				transport: gqlHTTPClient.Transport,
				agent:     userAgent,
			}

			return next(ctx, method, request)
		}
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

// isAcceptedError checks if the error is an accepted error.
func isAcceptedError(err error) bool {
	var acceptedError *github.AcceptedError
	return errors.As(err, &acceptedError)
}

func MarshalledTextResult(v any) *mcp.CallToolResult {
	data, err := json.Marshal(v)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to marshal text result to json", err)
	}

	return utils.NewToolResultText(string(data))
}
