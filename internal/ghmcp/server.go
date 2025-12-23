package ghmcp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/lockdown"
	mcplog "github.com/github/github-mcp-server/pkg/log"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/translations"
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
		enabledToolsets = github.RemoveToolset(enabledToolsets, string(github.ToolsetMetadataAll.ID))
		enabledToolsets = github.RemoveToolset(enabledToolsets, string(github.ToolsetMetadataDefault.ID))
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
	apiHost, err := parseAPIHost(cfg.Host)
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
		instructionToolsets = github.GetDefaultToolsetIDs()
	}

	// Create the MCP server
	serverOpts := &mcp.ServerOptions{
		Instructions: github.GenerateInstructions(instructionToolsets),
		Logger:       cfg.Logger,
		CompletionHandler: github.CompletionsHandler(func(_ context.Context) (*gogithub.Client, error) {
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

	ghServer := github.NewServer(cfg.Version, serverOpts)

	// Add middlewares
	ghServer.AddReceivingMiddleware(addGitHubAPIErrorToContext)
	ghServer.AddReceivingMiddleware(addUserAgentsMiddleware(cfg, clients.rest, clients.gqlHTTP))

	// Create dependencies for tool handlers
	deps := github.NewBaseDeps(
		clients.rest,
		clients.gql,
		clients.raw,
		clients.repoAccess,
		cfg.Translator,
		github.FeatureFlags{LockdownMode: cfg.LockdownMode},
		cfg.ContentWindowSize,
	)

	// Inject dependencies into context for all tool handlers
	ghServer.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			return next(github.ContextWithDeps(ctx, deps), method, req)
		}
	})

	// Build and register the tool/resource/prompt inventory
	inventory := github.NewInventory(cfg.Translator).
		WithDeprecatedAliases(github.DeprecatedToolAliases).
		WithReadOnly(cfg.ReadOnly).
		WithToolsets(enabledToolsets).
		WithTools(github.CleanTools(cfg.EnabledTools)).
		WithFeatureChecker(createFeatureChecker(cfg.EnabledFeatures)).
		Build()

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
func registerDynamicTools(server *mcp.Server, inventory *inventory.Inventory, deps *github.BaseDeps, t translations.TranslationHelperFunc) {
	dynamicDeps := github.DynamicToolDependencies{
		Server:    server,
		Inventory: inventory,
		ToolDeps:  deps,
		T:         t,
	}
	for _, tool := range github.DynamicTools(inventory) {
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

type StdioServerConfig struct {
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

	// ReadOnly indicates if we should only register read-only tools
	ReadOnly bool

	// ExportTranslations indicates if we should export translations
	// See: https://github.com/github/github-mcp-server?tab=readme-ov-file#i18n--overriding-descriptions
	ExportTranslations bool

	// EnableCommandLogging indicates if we should log commands
	EnableCommandLogging bool

	// Path to the log file if not stderr
	LogFilePath string

	// Content window size
	ContentWindowSize int

	// LockdownMode indicates if we should enable lockdown mode
	LockdownMode bool

	// RepoAccessCacheTTL overrides the default TTL for repository access cache entries.
	RepoAccessCacheTTL *time.Duration
}

// RunStdioServer is not concurrent safe.
func RunStdioServer(cfg StdioServerConfig) error {
	// Create app context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	t, dumpTranslations := translations.TranslationHelper()

	var slogHandler slog.Handler
	var logOutput io.Writer
	if cfg.LogFilePath != "" {
		file, err := os.OpenFile(cfg.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		logOutput = file
		slogHandler = slog.NewTextHandler(logOutput, &slog.HandlerOptions{Level: slog.LevelDebug})
	} else {
		logOutput = os.Stderr
		slogHandler = slog.NewTextHandler(logOutput, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	logger := slog.New(slogHandler)
	logger.Info("starting server", "version", cfg.Version, "host", cfg.Host, "dynamicToolsets", cfg.DynamicToolsets, "readOnly", cfg.ReadOnly, "lockdownEnabled", cfg.LockdownMode)

	ghServer, err := NewMCPServer(MCPServerConfig{
		Version:           cfg.Version,
		Host:              cfg.Host,
		Token:             cfg.Token,
		EnabledToolsets:   cfg.EnabledToolsets,
		EnabledTools:      cfg.EnabledTools,
		EnabledFeatures:   cfg.EnabledFeatures,
		DynamicToolsets:   cfg.DynamicToolsets,
		ReadOnly:          cfg.ReadOnly,
		Translator:        t,
		ContentWindowSize: cfg.ContentWindowSize,
		LockdownMode:      cfg.LockdownMode,
		Logger:            logger,
		RepoAccessTTL:     cfg.RepoAccessCacheTTL,
	})
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	if cfg.ExportTranslations {
		// Once server is initialized, all translations are loaded
		dumpTranslations()
	}

	// Start listening for messages
	errC := make(chan error, 1)
	go func() {
		var in io.ReadCloser
		var out io.WriteCloser

		in = os.Stdin
		out = os.Stdout

		if cfg.EnableCommandLogging {
			loggedIO := mcplog.NewIOLogger(in, out, logger)
			in, out = loggedIO, loggedIO
		}

		// enable GitHub errors in the context
		ctx := errors.ContextWithGitHubErrors(ctx)
		errC <- ghServer.Run(ctx, &mcp.IOTransport{Reader: in, Writer: out})
	}()

	// Output github-mcp-server string
	_, _ = fmt.Fprintf(os.Stderr, "GitHub MCP Server running on stdio\n")

	// Wait for shutdown signal
	select {
	case <-ctx.Done():
		logger.Info("shutting down server", "signal", "context done")
	case err := <-errC:
		if err != nil {
			logger.Error("error running server", "error", err)
			return fmt.Errorf("error running server: %w", err)
		}
	}

	return nil
}

type apiHost struct {
	baseRESTURL *url.URL
	graphqlURL  *url.URL
	uploadURL   *url.URL
	rawURL      *url.URL
}

func newDotcomHost() (apiHost, error) {
	baseRestURL, err := url.Parse("https://api.github.com/")
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse dotcom REST URL: %w", err)
	}

	gqlURL, err := url.Parse("https://api.github.com/graphql")
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse dotcom GraphQL URL: %w", err)
	}

	uploadURL, err := url.Parse("https://uploads.github.com")
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse dotcom Upload URL: %w", err)
	}

	rawURL, err := url.Parse("https://raw.githubusercontent.com/")
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse dotcom Raw URL: %w", err)
	}

	return apiHost{
		baseRESTURL: baseRestURL,
		graphqlURL:  gqlURL,
		uploadURL:   uploadURL,
		rawURL:      rawURL,
	}, nil
}

func newGHECHost(hostname string) (apiHost, error) {
	u, err := url.Parse(hostname)
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHEC URL: %w", err)
	}

	// Unsecured GHEC would be an error
	if u.Scheme == "http" {
		return apiHost{}, fmt.Errorf("GHEC URL must be HTTPS")
	}

	restURL, err := url.Parse(fmt.Sprintf("https://api.%s/", u.Hostname()))
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHEC REST URL: %w", err)
	}

	gqlURL, err := url.Parse(fmt.Sprintf("https://api.%s/graphql", u.Hostname()))
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHEC GraphQL URL: %w", err)
	}

	uploadURL, err := url.Parse(fmt.Sprintf("https://uploads.%s", u.Hostname()))
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHEC Upload URL: %w", err)
	}

	rawURL, err := url.Parse(fmt.Sprintf("https://raw.%s/", u.Hostname()))
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHEC Raw URL: %w", err)
	}

	return apiHost{
		baseRESTURL: restURL,
		graphqlURL:  gqlURL,
		uploadURL:   uploadURL,
		rawURL:      rawURL,
	}, nil
}

func newGHESHost(hostname string) (apiHost, error) {
	u, err := url.Parse(hostname)
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHES URL: %w", err)
	}

	restURL, err := url.Parse(fmt.Sprintf("%s://%s/api/v3/", u.Scheme, u.Hostname()))
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHES REST URL: %w", err)
	}

	gqlURL, err := url.Parse(fmt.Sprintf("%s://%s/api/graphql", u.Scheme, u.Hostname()))
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHES GraphQL URL: %w", err)
	}

	// Check if subdomain isolation is enabled
	// See https://docs.github.com/en/enterprise-server@3.17/admin/configuring-settings/hardening-security-for-your-enterprise/enabling-subdomain-isolation#about-subdomain-isolation
	hasSubdomainIsolation := checkSubdomainIsolation(u.Scheme, u.Hostname())

	var uploadURL *url.URL
	if hasSubdomainIsolation {
		// With subdomain isolation: https://uploads.hostname/
		uploadURL, err = url.Parse(fmt.Sprintf("%s://uploads.%s/", u.Scheme, u.Hostname()))
	} else {
		// Without subdomain isolation: https://hostname/api/uploads/
		uploadURL, err = url.Parse(fmt.Sprintf("%s://%s/api/uploads/", u.Scheme, u.Hostname()))
	}
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHES Upload URL: %w", err)
	}

	var rawURL *url.URL
	if hasSubdomainIsolation {
		// With subdomain isolation: https://raw.hostname/
		rawURL, err = url.Parse(fmt.Sprintf("%s://raw.%s/", u.Scheme, u.Hostname()))
	} else {
		// Without subdomain isolation: https://hostname/raw/
		rawURL, err = url.Parse(fmt.Sprintf("%s://%s/raw/", u.Scheme, u.Hostname()))
	}
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHES Raw URL: %w", err)
	}

	return apiHost{
		baseRESTURL: restURL,
		graphqlURL:  gqlURL,
		uploadURL:   uploadURL,
		rawURL:      rawURL,
	}, nil
}

// checkSubdomainIsolation detects if GitHub Enterprise Server has subdomain isolation enabled
// by attempting to ping the raw.<host>/_ping endpoint on the subdomain. The raw subdomain must always exist for subdomain isolation.
func checkSubdomainIsolation(scheme, hostname string) bool {
	subdomainURL := fmt.Sprintf("%s://raw.%s/_ping", scheme, hostname)

	client := &http.Client{
		Timeout: 5 * time.Second,
		// Don't follow redirects - we just want to check if the endpoint exists
		//nolint:revive // parameters are required by http.Client.CheckRedirect signature
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(subdomainURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Note that this does not handle ports yet, so development environments are out.
func parseAPIHost(s string) (apiHost, error) {
	if s == "" {
		return newDotcomHost()
	}

	u, err := url.Parse(s)
	if err != nil {
		return apiHost{}, fmt.Errorf("could not parse host as URL: %s", s)
	}

	if u.Scheme == "" {
		return apiHost{}, fmt.Errorf("host must have a scheme (http or https): %s", s)
	}

	if strings.HasSuffix(u.Hostname(), "github.com") {
		return newDotcomHost()
	}

	if strings.HasSuffix(u.Hostname(), "ghe.com") {
		return newGHECHost(s)
	}

	return newGHESHost(s)
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
		ctx = errors.ContextWithGitHubErrors(ctx)
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
