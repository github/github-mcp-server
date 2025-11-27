package ghmcp

import (
	"context"
	"fmt"
	"io"
	"log"
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
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v79/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
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

	// RepoAccessTTL overrides the default TTL for repository access cache entries.
	RepoAccessTTL *time.Duration
}

const stdioServerLogPrefix = "stdioserver"

func NewMCPServer(cfg MCPServerConfig, logger *slog.Logger) (*server.MCPServer, error) {
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

	repoAccessLogger := logger.With("component", "lockdown")
	repoAccessOpts = append(repoAccessOpts, lockdown.WithLogger(repoAccessLogger))
	var repoAccessCache *lockdown.RepoAccessCache
	if cfg.LockdownMode {
		repoAccessCache = lockdown.GetInstance(gqlClient, repoAccessOpts...)
	}

	// When a client send an initialize request, update the user agent to include the client info.
	beforeInit := func(_ context.Context, _ any, message *mcp.InitializeRequest) {
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
	}

	hooks := &server.Hooks{
		OnBeforeInitialize: []server.OnBeforeInitializeFunc{beforeInit},
		OnBeforeAny: []server.BeforeAnyHookFunc{
			func(ctx context.Context, _ any, _ mcp.MCPMethod, _ any) {
				// Ensure the context is cleared of any previous errors
				// as context isn't propagated through middleware
				errors.ContextWithGitHubErrors(ctx)
			},
		},
	}

	enabledToolsets := cfg.EnabledToolsets

	// If dynamic toolsets are enabled, remove "all" from the enabled toolsets
	if cfg.DynamicToolsets {
		enabledToolsets = github.RemoveToolset(enabledToolsets, github.ToolsetMetadataAll.ID)
	}

	// Clean up the passed toolsets
	enabledToolsets, invalidToolsets := github.CleanToolsets(enabledToolsets)

	// If "all" is present, override all other toolsets
	if github.ContainsToolset(enabledToolsets, github.ToolsetMetadataAll.ID) {
		enabledToolsets = []string{github.ToolsetMetadataAll.ID}
	}
	// If "default" is present, expand to real toolset IDs
	if github.ContainsToolset(enabledToolsets, github.ToolsetMetadataDefault.ID) {
		enabledToolsets = github.AddDefaultToolset(enabledToolsets)
	}

	if len(invalidToolsets) > 0 {
		fmt.Fprintf(os.Stderr, "Invalid toolsets ignored: %s\n", strings.Join(invalidToolsets, ", "))
	}

	// Generate instructions based on enabled toolsets
	instructions := github.GenerateInstructions(enabledToolsets)

	ghServer := github.NewServer(cfg.Version,
		server.WithInstructions(instructions),
		server.WithHooks(hooks),
	)

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

	// Create default toolsets
	tsg := github.DefaultToolsetGroup(
		cfg.ReadOnly,
		getClient,
		getGQLClient,
		getRawClient,
		cfg.Translator,
		cfg.ContentWindowSize,
		github.FeatureFlags{LockdownMode: cfg.LockdownMode},
		repoAccessCache,
	)

	// Enable and register toolsets if configured
	// This always happens if toolsets are specified, regardless of whether tools are also specified
	if len(enabledToolsets) > 0 {
		err = tsg.EnableToolsets(enabledToolsets, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to enable toolsets: %w", err)
		}

		// Register all mcp functionality with the server
		tsg.RegisterAll(ghServer)
	}

	// Register specific tools if configured
	if len(cfg.EnabledTools) > 0 {
		// Clean and validate tool names
		enabledTools := github.CleanTools(cfg.EnabledTools)

		// Register the specified tools (additive to any toolsets already enabled)
		err = tsg.RegisterSpecificTools(ghServer, enabledTools, cfg.ReadOnly)
		if err != nil {
			return nil, fmt.Errorf("failed to register tools: %w", err)
		}
	}

	// Register dynamic toolsets if configured (additive to toolsets and tools)
	if cfg.DynamicToolsets {
		dynamic := github.InitDynamicToolset(ghServer, tsg, cfg.Translator)
		dynamic.RegisterTools(ghServer)
	}

	return ghServer, nil
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
	stdLogger := log.New(logOutput, stdioServerLogPrefix, 0)

	ghServer, err := NewMCPServer(MCPServerConfig{
		Version:           cfg.Version,
		Host:              cfg.Host,
		Token:             cfg.Token,
		EnabledToolsets:   cfg.EnabledToolsets,
		EnabledTools:      cfg.EnabledTools,
		DynamicToolsets:   cfg.DynamicToolsets,
		ReadOnly:          cfg.ReadOnly,
		Translator:        t,
		ContentWindowSize: cfg.ContentWindowSize,
		LockdownMode:      cfg.LockdownMode,
		RepoAccessTTL:     cfg.RepoAccessCacheTTL,
	}, logger)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	stdioServer := server.NewStdioServer(ghServer)
	stdioServer.SetErrorLogger(stdLogger)

	if cfg.ExportTranslations {
		// Once server is initialized, all translations are loaded
		dumpTranslations()
	}

	// Start listening for messages
	errC := make(chan error, 1)
	go func() {
		in, out := io.Reader(os.Stdin), io.Writer(os.Stdout)

		if cfg.EnableCommandLogging {
			loggedIO := mcplog.NewIOLogger(in, out, logger)
			in, out = loggedIO, loggedIO
		}
		// enable GitHub errors in the context
		ctx := errors.ContextWithGitHubErrors(ctx)
		errC <- stdioServer.Listen(ctx, in, out)
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
