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
	"github.com/github/github-mcp-server/pkg/lockdown"
	mcplog "github.com/github/github-mcp-server/pkg/log"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/toolsets"
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

func NewMCPServer(cfg MCPServerConfig) (*mcp.Server, error) {
	apiHost, err := parseAPIHost(cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse API host: %w", err)
	}

	// Construct our REST client
	restClient := gogithub.NewClient(nil).WithAuthToken(cfg.Token)
	restClient.UserAgent = fmt.Sprintf("github-mcp-server/%s", cfg.Version)
	restClient.BaseURL = apiHost.baseRESTURL
	restClient.UploadURL = apiHost.uploadURL

	// Construct our GraphQL client
	// We're using NewEnterpriseClient here unconditionally as opposed to NewClient because we already
	// did the necessary API host parsing so that github.com will return the correct URL anyway.
	gqlHTTPClient := &http.Client{
		Transport: &bearerAuthTransport{
			transport: http.DefaultTransport,
			token:     cfg.Token,
		},
	} // We're going to wrap the Transport later in beforeInit
	gqlClient := githubv4.NewEnterpriseClient(apiHost.graphqlURL.String(), gqlHTTPClient)
	repoAccessOpts := []lockdown.RepoAccessOption{}
	if cfg.RepoAccessTTL != nil {
		repoAccessOpts = append(repoAccessOpts, lockdown.WithTTL(*cfg.RepoAccessTTL))
	}

	repoAccessLogger := cfg.Logger.With("component", "lockdown")
	repoAccessOpts = append(repoAccessOpts, lockdown.WithLogger(repoAccessLogger))
	var repoAccessCache *lockdown.RepoAccessCache
	if cfg.LockdownMode {
		repoAccessCache = lockdown.GetInstance(gqlClient, repoAccessOpts...)
	}

	enabledToolsets := cfg.EnabledToolsets

	// Clean up the passed toolsets (removes duplicates, whitespace)
	enabledToolsets, invalidToolsets := github.CleanToolsets(enabledToolsets)

	if len(invalidToolsets) > 0 {
		fmt.Fprintf(os.Stderr, "Invalid toolsets ignored: %s\n", strings.Join(invalidToolsets, ", "))
	}

	// Generate instructions based on enabled toolsets
	instructions := github.GenerateInstructions(enabledToolsets)

	getClient := func(_ context.Context) (*gogithub.Client, error) {
		return restClient, nil // closing over client
	}

	getGQLClient := func(_ context.Context) (*githubv4.Client, error) {
		return gqlClient, nil // closing over client
	}

	getRawClient := func(ctx context.Context) (*raw.Client, error) {
		client, err := getClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get GitHub client: %w", err)
		}
		return raw.NewClient(client, apiHost.rawURL), nil // closing over client
	}

	ghServer := github.NewServer(cfg.Version, &mcp.ServerOptions{
		Instructions:      instructions,
		Logger:            cfg.Logger,
		CompletionHandler: github.CompletionsHandler(getClient),
	})

	// Add middlewares
	ghServer.AddReceivingMiddleware(addGitHubAPIErrorToContext)
	ghServer.AddReceivingMiddleware(addUserAgentsMiddleware(cfg, restClient, gqlHTTPClient))

	// Create the dependencies struct for tool handlers
	deps := github.ToolDependencies{
		GetClient:         getClient,
		GetGQLClient:      getGQLClient,
		GetRawClient:      getRawClient,
		RepoAccessCache:   repoAccessCache,
		T:                 cfg.Translator,
		Flags:             github.FeatureFlags{LockdownMode: cfg.LockdownMode},
		ContentWindowSize: cfg.ContentWindowSize,
	}

	// Create toolset group with all tools, resources, and prompts
	tsg := github.NewToolsetGroup(cfg.Translator, getClient, getRawClient)

	// Add deprecated tool aliases for backward compatibility
	// See docs/deprecated-tool-aliases.md for the full list of renames
	tsg.AddDeprecatedToolAliases(github.DeprecatedToolAliases)

	// Clean tool names (WithTools will resolve any deprecated aliases)
	enabledTools := github.CleanTools(cfg.EnabledTools)

	// For dynamic toolsets mode:
	// - If toolsets are explicitly provided (including "default"), honor them
	// - If no toolsets are specified (nil), start with no toolsets enabled (empty slice)
	//   so users can enable them on demand via the dynamic tools
	if cfg.DynamicToolsets && cfg.EnabledToolsets == nil {
		enabledToolsets = []string{}
	}

	// Apply filters based on configuration
	// - WithReadOnly: filters out write tools when true
	// - WithToolsets: nil=defaults, empty=none, handles "all"/"default" keywords
	// - WithTools: additional tools that bypass toolset filtering (additive, resolves aliases)
	// - WithFeatureChecker: filters based on feature flags
	filteredTsg := tsg.
		WithReadOnly(cfg.ReadOnly).
		WithToolsets(enabledToolsets).
		WithTools(enabledTools).
		WithFeatureChecker(createFeatureChecker(cfg.EnabledFeatures))

	// Register all mcp functionality with the server
	// Use background context for local server (no per-request actor context)
	filteredTsg.RegisterAll(context.Background(), ghServer, deps)

	// Register dynamic toolset management if configured
	// Dynamic tools get access to the filtered toolset group which tracks enabled state.
	// ToolsForToolset() returns all tools for a toolset regardless of enabled status,
	// so dynamic tools can enable any toolset at runtime.
	if cfg.DynamicToolsets {
		dynamicDeps := github.DynamicToolDependencies{
			Server:       ghServer,
			ToolsetGroup: filteredTsg,
			ToolDeps:     deps,
			T:            cfg.Translator,
		}
		dynamicTools := github.DynamicTools()
		for _, tool := range dynamicTools {
			tool.RegisterFunc(ghServer, dynamicDeps)
		}
	}

	return ghServer, nil
}

// createFeatureChecker returns a FeatureFlagChecker that checks if a flag name
// is present in the provided list of enabled features. For the local server,
// this is populated from the --features CLI flag.
func createFeatureChecker(enabledFeatures []string) toolsets.FeatureFlagChecker {
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
