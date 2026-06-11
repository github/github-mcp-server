package ghmcp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/github/appauth"
	"github.com/github/github-mcp-server/pkg/http/transport"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/lockdown"
	mcplog "github.com/github/github-mcp-server/pkg/log"
	"github.com/github/github-mcp-server/pkg/observability"
	"github.com/github/github-mcp-server/pkg/observability/metrics"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	gogithub "github.com/google/go-github/v87/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// githubClients holds all the GitHub API clients created for a server instance.
type githubClients struct {
	rest         *gogithub.Client
	restUATransp *transport.UserAgentTransport
	gql          *githubv4.Client
	gqlHTTP      *http.Client // retained for middleware to modify transport
	raw          *raw.Client
	repoAccess   *lockdown.RepoAccessCache
}

// createGitHubClients creates all the GitHub API clients needed by the server.
// If authTransport is non-nil, it is used for authentication instead of cfg.Token.
func createGitHubClients(cfg github.MCPServerConfig, apiHost utils.APIHostResolver, authTransport http.RoundTripper) (*githubClients, error) {
	restURL, err := apiHost.BaseRESTURL(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get base REST URL: %w", err)
	}

	uploadURL, err := apiHost.UploadURL(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get upload URL: %w", err)
	}

	graphQLURL, err := apiHost.GraphqlURL(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get GraphQL URL: %w", err)
	}

	rawURL, err := apiHost.RawURL(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get Raw URL: %w", err)
	}

	// Determine the base transport for REST and GraphQL clients
	baseTransport := http.RoundTripper(http.DefaultTransport)
	if authTransport != nil {
		baseTransport = authTransport
	}

	// Construct REST client
	restUATransport := &transport.UserAgentTransport{
		Transport: baseTransport,
		Agent:     fmt.Sprintf("github-mcp-server/%s", cfg.Version),
	}
	var restClient *gogithub.Client
	if authTransport != nil {
		restClient, err = gogithub.NewClient(
			gogithub.WithHTTPClient(&http.Client{Transport: restUATransport}),
			gogithub.WithEnterpriseURLs(restURL.String(), uploadURL.String()),
		)
	} else {
		restClient, err = gogithub.NewClient(
			gogithub.WithHTTPClient(&http.Client{Transport: restUATransport}),
			gogithub.WithAuthToken(cfg.Token),
			gogithub.WithEnterpriseURLs(restURL.String(), uploadURL.String()),
		)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create REST client: %w", err)
	}

	// Construct GraphQL client
	// We use NewEnterpriseClient unconditionally since we already parsed the API host
	var gqlTransport http.RoundTripper
	if authTransport != nil {
		// Auth transport already sets the Authorization header.
		// Wrap with UserAgentTransport for consistency with the REST path.
		gqlTransport = &transport.UserAgentTransport{
			Transport: &transport.GraphQLFeaturesTransport{
				Transport: authTransport,
			},
			Agent: fmt.Sprintf("github-mcp-server/%s", cfg.Version),
		}
	} else {
		gqlTransport = &transport.BearerAuthTransport{
			Transport: &transport.GraphQLFeaturesTransport{
				Transport: http.DefaultTransport,
			},
			Token: cfg.Token,
		}
	}
	gqlHTTPClient := &http.Client{Transport: gqlTransport}

	gqlClient := githubv4.NewEnterpriseClient(graphQLURL.String(), gqlHTTPClient)

	// Create raw content client (shares REST client's HTTP transport)
	rawClient, err := raw.NewClient(restClient, rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create raw client: %w", err)
	}

	// Set up repo access cache for lockdown mode
	var repoAccessCache *lockdown.RepoAccessCache
	if cfg.LockdownMode {
		opts := []lockdown.RepoAccessOption{
			lockdown.WithLogger(cfg.Logger.With("component", "lockdown")),
		}
		if cfg.RepoAccessTTL != nil {
			opts = append(opts, lockdown.WithTTL(*cfg.RepoAccessTTL))
		}
		repoAccessCache = lockdown.NewRepoAccessCache(gqlClient, restClient, opts...)
	}

	return &githubClients{
		rest:         restClient,
		restUATransp: restUATransport,
		gql:          gqlClient,
		gqlHTTP:      gqlHTTPClient,
		raw:          rawClient,
		repoAccess:   repoAccessCache,
	}, nil
}

func NewStdioMCPServer(ctx context.Context, cfg github.MCPServerConfig, authTransport http.RoundTripper) (*mcp.Server, error) {
	apiHost, err := utils.NewAPIHost(cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse API host: %w", err)
	}

	clients, err := createGitHubClients(cfg, apiHost, authTransport)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub clients: %w", err)
	}

	// Create feature checker — resolves explicit features + insiders expansion
	featureChecker := createFeatureChecker(cfg.EnabledFeatures, cfg.InsidersMode)

	// Create dependencies for tool handlers
	obs, err := observability.NewExporters(cfg.Logger, metrics.NewNoopMetrics())
	if err != nil {
		return nil, fmt.Errorf("failed to create observability exporters: %w", err)
	}
	deps := github.NewBaseDeps(
		clients.rest,
		clients.gql,
		clients.raw,
		clients.repoAccess,
		cfg.Translator,
		github.FeatureFlags{
			LockdownMode: cfg.LockdownMode,
		},
		cfg.ContentWindowSize,
		featureChecker,
		obs,
	)
	// Build and register the tool/resource/prompt inventory
	inventoryBuilder := github.NewInventory(cfg.Translator).
		WithDeprecatedAliases(github.DeprecatedToolAliases).
		WithReadOnly(cfg.ReadOnly).
		WithToolsets(github.ResolvedEnabledToolsets(cfg.EnabledToolsets, cfg.EnabledTools)).
		WithTools(github.CleanTools(cfg.EnabledTools)).
		WithExcludeTools(cfg.ExcludeTools).
		WithServerInstructions().
		WithFeatureChecker(featureChecker)

	// Apply token scope filtering if scopes are known (for PAT filtering)
	if cfg.TokenScopes != nil {
		inventoryBuilder = inventoryBuilder.WithFilter(github.CreateToolScopeFilter(cfg.TokenScopes))
	}

	inventory, err := inventoryBuilder.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build inventory: %w", err)
	}

	ghServer, err := github.NewMCPServer(ctx, &cfg, deps, inventory)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub MCP server: %w", err)
	}

	ghServer.AddReceivingMiddleware(addUserAgentsMiddleware(cfg, clients.restUATransp, clients.gqlHTTP))

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

	// EnabledFeatures is a list of feature flags that are enabled
	// Items with FeatureFlagEnable matching an entry in this list will be available
	EnabledFeatures []string

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

	// InsidersMode expands to the curated set of feature flags enabled for insiders.
	InsidersMode bool

	// ExcludeTools is a list of tool names to disable regardless of other settings.
	// These tools will be excluded even if their toolset is enabled or they are
	// explicitly listed in EnabledTools.
	ExcludeTools []string

	// RepoAccessCacheTTL overrides the default TTL for repository access cache entries.
	RepoAccessCacheTTL *time.Duration

	// GitHub App authentication (alternative to Token)
	// When AppID, PrivateKey, and InstallationID are all set, the server
	// authenticates as a GitHub App installation instead of using a PAT.
	AppID          int64
	PrivateKey     []byte
	InstallationID int64
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
	logger.Info("starting server", "version", cfg.Version, "host", cfg.Host, "readOnly", cfg.ReadOnly, "lockdownEnabled", cfg.LockdownMode)

	// Set up GitHub App authentication transport if configured
	var appAuthTransport http.RoundTripper
	if cfg.AppID != 0 && len(cfg.PrivateKey) > 0 && cfg.InstallationID != 0 {
		apiHost, err := utils.NewAPIHost(cfg.Host)
		if err != nil {
			return fmt.Errorf("failed to parse API host for app auth: %w", err)
		}
		baseURL, err := apiHost.BaseRESTURL(ctx)
		if err != nil {
			return fmt.Errorf("failed to get base REST URL for app auth: %w", err)
		}
		tr, err := appauth.NewTransport(http.DefaultTransport, appauth.Config{
			AppID:          cfg.AppID,
			PrivateKey:     cfg.PrivateKey,
			InstallationID: cfg.InstallationID,
			BaseURL:        baseURL.String(),
		})
		if err != nil {
			return fmt.Errorf("failed to create GitHub App auth transport: %w", err)
		}
		appAuthTransport = tr
		logger.Info("using GitHub App authentication", "appID", cfg.AppID, "installationID", cfg.InstallationID)
	}

	// Fetch token scopes for scope-based tool filtering (PAT tokens only)
	// Only classic PATs (ghp_ prefix) return OAuth scopes via X-OAuth-Scopes header.
	// Fine-grained PATs and other token types don't support this, so we skip filtering.
	var tokenScopes []string
	if appAuthTransport == nil && strings.HasPrefix(cfg.Token, "ghp_") {
		fetchedScopes, err := fetchTokenScopesForHost(ctx, cfg.Token, cfg.Host)
		if err != nil {
			logger.Warn("failed to fetch token scopes, continuing without scope filtering", "error", err)
		} else {
			tokenScopes = fetchedScopes
			logger.Info("token scopes fetched for filtering", "scopes", tokenScopes)
		}
	} else if appAuthTransport == nil {
		logger.Debug("skipping scope filtering for non-PAT token")
	}

	ghServer, err := NewStdioMCPServer(ctx, github.MCPServerConfig{
		Version:           cfg.Version,
		Host:              cfg.Host,
		Token:             cfg.Token,
		EnabledToolsets:   cfg.EnabledToolsets,
		EnabledTools:      cfg.EnabledTools,
		EnabledFeatures:   cfg.EnabledFeatures,
		ReadOnly:          cfg.ReadOnly,
		Translator:        t,
		ContentWindowSize: cfg.ContentWindowSize,
		LockdownMode:      cfg.LockdownMode,
		InsidersMode:      cfg.InsidersMode,
		ExcludeTools:      cfg.ExcludeTools,
		Logger:            logger,
		RepoAccessTTL:     cfg.RepoAccessCacheTTL,
		TokenScopes:       tokenScopes,
	}, appAuthTransport)
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

// createFeatureChecker returns a FeatureFlagChecker that resolves features
// using the centralized ResolveFeatureFlags function. For the local server,
// features are resolved once at startup from --features CLI flag and insiders mode.
func createFeatureChecker(enabledFeatures []string, insidersMode bool) inventory.FeatureFlagChecker {
	featureSet := github.ResolveFeatureFlags(enabledFeatures, insidersMode)
	return func(_ context.Context, flagName string) (bool, error) {
		return featureSet[flagName], nil
	}
}

func addUserAgentsMiddleware(cfg github.MCPServerConfig, restUATransp *transport.UserAgentTransport, gqlHTTPClient *http.Client) func(next mcp.MethodHandler) mcp.MethodHandler {
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
			if cfg.InsidersMode {
				userAgent += " (insiders)"
			}

			restUATransp.Agent = userAgent

			gqlHTTPClient.Transport = &transport.UserAgentTransport{
				Transport: gqlHTTPClient.Transport,
				Agent:     userAgent,
			}

			return next(ctx, method, request)
		}
	}
}

// fetchTokenScopesForHost fetches the OAuth scopes for a token from the GitHub API.
// It constructs the appropriate API host URL based on the configured host.
func fetchTokenScopesForHost(ctx context.Context, token, host string) ([]string, error) {
	apiHost, err := utils.NewAPIHost(host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse API host: %w", err)
	}

	fetcher := scopes.NewFetcher(apiHost, scopes.FetcherOptions{})

	return fetcher.FetchTokenScopes(ctx, token)
}
