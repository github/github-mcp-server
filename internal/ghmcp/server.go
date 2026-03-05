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
	"github.com/github/github-mcp-server/pkg/http/transport"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/lockdown"
	mcplog "github.com/github/github-mcp-server/pkg/log"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// githubClients holds all the GitHub API clients created for a server instance.
type githubClients struct {
	rest       *gogithub.Client
	gql        *githubv4.Client
	gqlHTTP    *http.Client // retained for middleware to modify transport
	raw        *raw.Client
	repoAccess *lockdown.RepoAccessCache
}

// createGitHubClients creates all the GitHub API clients needed by the server.
// If skipLockdown is true, the repo access cache is not initialized even when
// cfg.LockdownMode is set. This must be true when GitHub App multi-org auth is
// active: MultiOrgDeps creates per-installation caches via
// lockdown.NewRepoAccessCache (not the singleton GetInstance), and initializing
// the singleton here with the PAT-based GQL client would conflict.
func createGitHubClients(cfg github.MCPServerConfig, apiHost utils.APIHostResolver, skipLockdown bool) (*githubClients, error) {
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

	// Construct REST client
	restClient := gogithub.NewClient(nil).WithAuthToken(cfg.Token)
	restClient.UserAgent = fmt.Sprintf("github-mcp-server/%s", cfg.Version)
	restClient.BaseURL = restURL
	restClient.UploadURL = uploadURL

	// Construct GraphQL client
	// We use NewEnterpriseClient unconditionally since we already parsed the API host
	gqlHTTPClient := &http.Client{
		Transport: &transport.BearerAuthTransport{
			Transport: &transport.GraphQLFeaturesTransport{
				Transport: http.DefaultTransport,
			},
			Token: cfg.Token,
		},
	}

	gqlClient := githubv4.NewEnterpriseClient(graphQLURL.String(), gqlHTTPClient)

	// Create raw content client (shares REST client's HTTP transport)
	rawClient := raw.NewClient(restClient, rawURL)

	// Set up repo access cache for lockdown mode.
	// Skipped when skipLockdown is true (multi-org app auth): the singleton must
	// not be initialized with this PAT client — MultiOrgDeps creates its own
	// per-installation cache via lockdown.NewRepoAccessCache (non-singleton).
	var repoAccessCache *lockdown.RepoAccessCache
	if cfg.LockdownMode && !skipLockdown {
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

func NewStdioMCPServer(ctx context.Context, cfg github.MCPServerConfig) (*mcp.Server, error) {
	apiHost, err := utils.NewAPIHost(cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse API host: %w", err)
	}

	// Skip lockdown singleton init when GitHub App multi-org auth is active.
	// MultiOrgDeps creates its own per-org lockdown cache; initializing the
	// singleton here with the PAT client would corrupt it for all org requests.
	//
	// App auth is active when AppID is set AND at least one installation is
	// configured. The Installations map includes "_default" from
	// GITHUB_INSTALLATION_ID (populated by parseOrgInstallations in main.go),
	// so setting just AppID + InstallationID + PrivateKey is sufficient.
	appAuthActive := cfg.AppID != 0 && len(cfg.Installations) > 0
	clients, err := createGitHubClients(cfg, apiHost, appAuthActive)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub clients: %w", err)
	}

	// Create feature checker
	featureChecker := createFeatureChecker(cfg.EnabledFeatures)

	flags := github.FeatureFlags{
		LockdownMode: cfg.LockdownMode,
		InsidersMode: cfg.InsidersMode,
	}

	// Determine which deps to use: MultiOrgDeps for GitHub App multi-org, BaseDeps otherwise.
	var deps github.ToolDependencies
	var multiOrgFactory *github.MultiOrgClientFactory // non-nil when app auth active
	if appAuthActive {
		// GitHub App auth with multi-org support.
		rawURL, err := apiHost.RawURL(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get raw URL for multi-org factory: %w", err)
		}
		multiOrgFactory = github.NewMultiOrgClientFactory(
			cfg.AppID,
			cfg.PrivateKey,
			cfg.Installations,
			apiHost,
			rawURL,
			cfg.Version,
		)
		// Build repoAccessOpts to pass to MultiOrgDeps (mirrors createGitHubClients logic).
		var repoAccessOpts []lockdown.RepoAccessOption
		if cfg.LockdownMode {
			repoAccessOpts = append(repoAccessOpts,
				lockdown.WithLogger(cfg.Logger.With("component", "lockdown")),
			)
			if cfg.RepoAccessTTL != nil {
				repoAccessOpts = append(repoAccessOpts, lockdown.WithTTL(*cfg.RepoAccessTTL))
			}
		}
		deps = github.NewMultiOrgDeps(
			multiOrgFactory,
			cfg.Translator,
			flags,
			cfg.ContentWindowSize,
			featureChecker,
			cfg.LockdownMode,
			repoAccessOpts,
		)
	} else {
		// Standard PAT auth — use existing BaseDeps path.
		deps = github.NewBaseDeps(
			clients.rest,
			clients.gql,
			clients.raw,
			clients.repoAccess,
			cfg.Translator,
			flags,
			cfg.ContentWindowSize,
			featureChecker,
		)
	}

	// Parse toolset modes (e.g., "repos:rw,issues:ro") from config.
	// This splits the name:mode suffix from the toolset name and builds a read-only map.
	// "all:ro" is expanded to all known toolset IDs so every toolset becomes read-only.
	allKnownToolsets := github.AllToolsetIDs()
	toolsetNames, readOnlyToolsets := github.ParseToolsetModes(cfg.EnabledToolsets, allKnownToolsets)

	// Build and register the tool/resource/prompt inventory
	inventoryBuilder := github.NewInventory(cfg.Translator).
		WithDeprecatedAliases(github.DeprecatedToolAliases).
		WithReadOnly(cfg.ReadOnly).
		WithToolsets(github.ResolvedEnabledToolsets(cfg.DynamicToolsets, toolsetNames, cfg.EnabledTools)).
		WithToolsetModes(readOnlyToolsets).
		WithTools(github.CleanTools(cfg.EnabledTools)).
		WithExcludeTools(cfg.ExcludeTools).
		WithServerInstructions().
		WithFeatureChecker(featureChecker).
		WithInsidersMode(cfg.InsidersMode)

	// Apply token scope filtering if scopes are known (for PAT filtering)
	if cfg.TokenScopes != nil {
		inventoryBuilder = inventoryBuilder.WithFilter(github.CreateToolScopeFilter(cfg.TokenScopes))
	}

	inv, err := inventoryBuilder.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build inventory: %w", err)
	}

	// Build denylist once here so it can be passed to both NewMCPServer (for
	// completions and resource handlers) and the middleware section below.
	denylist := github.NewRepoDenylist(cfg.RepoDenylistEntries)
	cfg.Denylist = denylist

	ghServer, err := github.NewMCPServer(ctx, &cfg, deps, inv)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub MCP server: %w", err)
	}

	// Register MCP App UI resources if available (requires running script/build-ui).
	// We check availability to allow Insiders mode to work for non-UI features
	// even when UI assets haven't been built.
	if cfg.InsidersMode && github.UIAssetsAvailable() {
		github.RegisterUIResources(ghServer)
	}

	// Register guard middleware BEFORE the user-agent middleware.
	// Use single-call form so the first arg is outermost (runs first per request).
	//
	// Execution order (outermost first):
	//   UA → denylist → searchDenylist → ownerExtract → writeGuard
	//     → addGitHubAPIError → InjectDeps → handler
	//
	// UA is outermost because it's registered last (wraps everything).
	// addGitHubAPIError and InjectDeps are innermost — registered last in NewMCPServer
	// (last-registered = innermost = closest to the handler).
	//
	// Denylist runs before any GitHub API call (pure in-memory lookup).
	// Owner extract populates context owner for MultiOrgDeps routing so that
	// subsequent middleware can use deps.GetClient(ctx) with the correct
	// org-scoped client. Write guard calls deps.GetClient(ctx) which uses
	// OwnerFromContext — must run AFTER owner extract.
	var guardMiddleware []mcp.Middleware

	// Denylist guard (outermost — pure in-memory, no API calls).
	// Reuses the denylist built above — no second NewRepoDenylist call.
	if !denylist.IsEmpty() {
		guardMiddleware = append(guardMiddleware,
			github.RepoDenylistMiddleware(denylist),
			github.SearchDenylistMiddleware(denylist),
		)
	}

	// Owner extraction middleware (must run BEFORE write guard for correct
	// multi-org routing: WritePrivateOnlyMiddleware calls deps.GetClient(ctx)
	// which uses OwnerFromContext to select the org-scoped client).
	//
	// Only registered for GitHub App auth: PAT auth uses BaseDeps whose
	// GetClient ignores the context owner, so extraction is unnecessary.
	// If a new ToolDependencies implementation is added that requires owner
	// context, this condition must be updated.
	if appAuthActive {
		guardMiddleware = append(guardMiddleware, github.OwnerExtractMiddleware())
	}

	// Write guard (after owner extract — needs owner in context for correct client).
	if cfg.WritePrivateOnly {
		if cfg.ReadOnly {
			cfg.Logger.Warn("GITHUB_WRITE_PRIVATE_ONLY has no effect when --read-only is active")
		} else {
			guardMiddleware = append(guardMiddleware,
				github.WritePrivateOnlyMiddleware(deps, inv),
			)
		}
	}

	// Register all guard middleware in a single call to preserve ordering.
	// AddReceivingMiddleware(m1, m2, m3) → m1 is outermost (runs first).
	if len(guardMiddleware) > 0 {
		ghServer.AddReceivingMiddleware(guardMiddleware...)
	}

	// Existing user-agent middleware (must come AFTER guards).
	// Pass multiOrgFactory so the initialize handshake propagates the user agent
	// to all per-org clients created by the factory. nil when PAT auth is used.
	ghServer.AddReceivingMiddleware(addUserAgentsMiddleware(cfg, clients.rest, clients.gqlHTTP, multiOrgFactory))

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

	// InsidersMode indicates if we should enable experimental features
	InsidersMode bool

	// ExcludeTools is a list of tool names to disable regardless of other settings.
	// These tools will be excluded even if their toolset is enabled or they are
	// explicitly listed in EnabledTools.
	ExcludeTools []string

	// RepoAccessCacheTTL overrides the default TTL for repository access cache entries.
	RepoAccessCacheTTL *time.Duration

	// GitHub App authentication
	AppID          int64
	InstallationID int64
	PrivateKeyPath string
	PrivateKey     string

	// Multi-org installations (org name → installation ID)
	Installations map[string]int64

	// Write guard
	WritePrivateOnly bool

	// Repo denylist
	RepoDenylist []string
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

	// Fetch token scopes for scope-based tool filtering (PAT tokens only)
	// Only classic PATs (ghp_ prefix) return OAuth scopes via X-OAuth-Scopes header.
	// Fine-grained PATs and other token types don't support this, so we skip filtering.
	var tokenScopes []string
	if strings.HasPrefix(cfg.Token, "ghp_") {
		fetchedScopes, err := fetchTokenScopesForHost(ctx, cfg.Token, cfg.Host)
		if err != nil {
			logger.Warn("failed to fetch token scopes, continuing without scope filtering", "error", err)
		} else {
			tokenScopes = fetchedScopes
			logger.Info("token scopes fetched for filtering", "scopes", tokenScopes)
		}
	} else {
		logger.Debug("skipping scope filtering for non-PAT token")
	}

	// Resolve private key (from file path or inline content) for GitHub App auth.
	var privateKey []byte
	if cfg.PrivateKeyPath != "" || cfg.PrivateKey != "" {
		var err error
		privateKey, err = github.ResolvePrivateKey([]byte(cfg.PrivateKey), cfg.PrivateKeyPath)
		if err != nil {
			return fmt.Errorf("failed to resolve private key: %w", err)
		}
	}

	ghServer, err := NewStdioMCPServer(ctx, github.MCPServerConfig{
		Version:             cfg.Version,
		Host:                cfg.Host,
		Token:               cfg.Token,
		EnabledToolsets:     cfg.EnabledToolsets,
		EnabledTools:        cfg.EnabledTools,
		EnabledFeatures:     cfg.EnabledFeatures,
		DynamicToolsets:     cfg.DynamicToolsets,
		ReadOnly:            cfg.ReadOnly,
		Translator:          t,
		ContentWindowSize:   cfg.ContentWindowSize,
		LockdownMode:        cfg.LockdownMode,
		InsidersMode:        cfg.InsidersMode,
		ExcludeTools:        cfg.ExcludeTools,
		Logger:              logger,
		RepoAccessTTL:       cfg.RepoAccessCacheTTL,
		TokenScopes:         tokenScopes,
		AppID:               cfg.AppID,
		InstallationID:      cfg.InstallationID,
		PrivateKey:          privateKey,
		Installations:       cfg.Installations,
		WritePrivateOnly:    cfg.WritePrivateOnly,
		RepoDenylistEntries: cfg.RepoDenylist,
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

// addUserAgentsMiddleware returns middleware that sets the user agent on all
// GitHub API clients after the MCP initialize handshake provides client info.
//
// The optional multiOrgFactory parameter propagates the user agent to clients
// created by MultiOrgClientFactory (GitHub App multi-org auth). When nil, only
// the PAT-based REST and GQL clients are updated.
func addUserAgentsMiddleware(cfg github.MCPServerConfig, restClient *gogithub.Client, gqlHTTPClient *http.Client, multiOrgFactory *github.MultiOrgClientFactory) func(next mcp.MethodHandler) mcp.MethodHandler {
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

			restClient.UserAgent = userAgent

			gqlHTTPClient.Transport = &transport.UserAgentTransport{
				Transport: gqlHTTPClient.Transport,
				Agent:     userAgent,
			}

			// Propagate user agent to multi-org factory so all per-org clients
			// created after this point use the correct user agent string.
			if multiOrgFactory != nil {
				multiOrgFactory.SetUserAgent(userAgent)
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
