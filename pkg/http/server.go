package http

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/github/github-mcp-server/internal/requeststate"
	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/http/middleware"
	"github.com/github/github-mcp-server/pkg/http/oauth"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/observability"
	"github.com/github/github-mcp-server/pkg/observability/metrics"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/go-chi/chi/v5"
)

// MRTRStateKeyEnv is the environment variable used to configure HTTP request-state encryption.
const MRTRStateKeyEnv = "GITHUB_MCP_SERVER_MRTR_STATE_KEY"

type ServerConfig struct {
	// Version of the server
	Version string

	// GitHub Host to target for API requests (e.g. github.com or github.enterprise.com)
	Host string

	// Port to listen on (default: 8082).
	Port int

	// ListenHost is the host the HTTP server binds to (e.g. "127.0.0.1").
	// When empty, the server binds to all interfaces. Combined with Port.
	ListenHost string

	// BaseURL is the publicly accessible URL of this server for OAuth resource metadata.
	// If not set, the server will derive the URL from incoming request headers.
	BaseURL string

	// ResourcePath is the externally visible base path for this server (e.g., "/mcp").
	// This is used to restore the original path when a proxy strips a base path before forwarding.
	ResourcePath string

	// AuthorizationServer overrides the authorization server URL advertised in the
	// OAuth Protected Resource Metadata (/.well-known/oauth-protected-resource).
	// When set, this URL is used instead of the one derived from the GitHub host.
	// Useful when deploying behind an OAuth proxy (e.g. for GHES, which does not
	// natively support RFC 8414 / RFC 7591 / PKCE).
	AuthorizationServer string

	// TrustProxyHeaders indicates whether X-Forwarded-Host and X-Forwarded-Proto
	// should be honored when constructing OAuth resource metadata URLs. Only
	// enable this when the server is deployed behind a trusted proxy that sets
	// these headers. When BaseURL is set, it always wins and this setting has
	// no effect.
	TrustProxyHeaders bool

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

	// ScopeChallenge indicates if we should return OAuth scope challenges, and if we should perform
	// tool filtering based on token scopes.
	ScopeChallenge bool

	// ReadOnly indicates if we should only register read-only tools.
	// When set via CLI flag, this acts as an upper bound — per-request headers
	// cannot re-enable write tools.
	ReadOnly bool

	// EnabledToolsets is a list of toolsets to enable.
	// When set via CLI flag, per-request headers can only narrow within these toolsets.
	EnabledToolsets []string

	// EnabledTools is a list of specific tools to enable (additive to toolsets).
	EnabledTools []string

	// ExcludeTools is a list of tool names to disable regardless of other settings.
	// When set via CLI flag, per-request headers cannot re-include these tools.
	ExcludeTools []string

	// EnabledFeatures is a list of feature flags that are enabled.
	EnabledFeatures []string

	// InsidersMode expands to the curated set of feature flags enabled for insiders.
	InsidersMode bool

	// MRTRStateKey is a Base64-encoded 32-byte key used to protect multi-round-trip request state.
	MRTRStateKey string

	// MaxRequestBodyBytes bounds the size of HTTP request bodies accepted by
	// the MCP endpoints. When zero, middleware.DefaultMaxRequestBodyBytes is used.
	MaxRequestBodyBytes int64

	disableDeleteRepository bool
}

func RunHTTPServer(cfg ServerConfig) error {
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
	logger.Info("starting server", "version", cfg.Version, "host", cfg.Host, "lockdownEnabled", cfg.LockdownMode, "readOnly", cfg.ReadOnly, "insidersMode", cfg.InsidersMode)

	stateSealer, err := configureRequestState(&cfg, logger)
	if err != nil {
		return err
	}

	apiHost, err := utils.NewAPIHost(cfg.Host)
	if err != nil {
		return fmt.Errorf("failed to parse API host: %w", err)
	}
	hostType, err := utils.ParseHostType(cfg.Host)
	if err != nil {
		return fmt.Errorf("failed to classify API host: %w", err)
	}

	repoAccessOpts := []lockdown.RepoAccessOption{
		lockdown.WithLogger(logger.With("component", "lockdown")),
	}
	if cfg.RepoAccessCacheTTL != nil {
		repoAccessOpts = append(repoAccessOpts, lockdown.WithTTL(*cfg.RepoAccessCacheTTL))
	}

	featureChecker := createHTTPFeatureChecker(cfg.EnabledFeatures, cfg.InsidersMode)
	scopeFetcher := scopes.NewFetcher(apiHost, scopes.FetcherOptions{})
	inventoryFactory, err := NewDefaultInventoryFactory(&cfg, t, featureChecker, scopeFetcher)
	if err != nil {
		return fmt.Errorf("failed to build inventory: %w", err)
	}

	obs, err := observability.NewExporters(logger, metrics.NewNoopMetrics())
	if err != nil {
		return fmt.Errorf("failed to create observability exporters: %w", err)
	}

	deps := github.NewRequestDeps(
		apiHost,
		cfg.Version,
		cfg.LockdownMode,
		repoAccessOpts,
		t,
		cfg.ContentWindowSize,
		featureChecker,
		obs,
	)
	deps.StateSealer = stateSealer

	// Initialize the global tool scope map
	err = initGlobalToolScopeMap(t, hostType)
	if err != nil {
		return fmt.Errorf("failed to initialize tool scope map: %w", err)
	}

	// Register OAuth protected resource metadata endpoints
	oauthCfg := newOAuthConfig(cfg)

	serverOptions := []HandlerOption{
		WithInventoryFactory(inventoryFactory),
		WithScopeFetcher(scopeFetcher),
	}

	handler := NewHTTPMcpHandler(ctx, &cfg, deps, t, logger, apiHost, append(serverOptions, WithFeatureChecker(featureChecker), WithOAuthConfig(oauthCfg))...)
	oauthHandler, err := oauth.NewAuthHandler(oauthCfg, apiHost)
	if err != nil {
		return fmt.Errorf("failed to create OAuth handler: %w", err)
	}

	r := newHTTPRouter(
		func(r chi.Router) {
			// Register Middleware First, needs to be before route registration
			handler.RegisterMiddleware(r)
			// Register MCP server routes
			handler.RegisterRoutes(r)
		},
		oauthHandler.RegisterRoutes,
	)
	logger.Info("MCP endpoints registered", "baseURL", cfg.BaseURL)
	logger.Info("OAuth protected resource endpoints registered", "baseURL", cfg.BaseURL)

	addr := resolveListenAddress(cfg.ListenHost, cfg.Port)
	httpSvr := http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		logger.Info("shutting down server")
		if err := httpSvr.Shutdown(shutdownCtx); err != nil {
			logger.Error("error during server shutdown", "error", err)
		}
	}()

	if cfg.ExportTranslations {
		// Once server is initialized, all translations are loaded
		dumpTranslations()
	}

	logger.Info("HTTP server listening", "addr", addr)
	if err := httpSvr.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}

	logger.Info("server stopped gracefully")
	return nil
}

func newHTTPRouter(registerMCPRoutes, registerOAuthRoutes func(chi.Router)) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.SetCorsHeaders)
	r.Group(registerMCPRoutes)
	r.Group(registerOAuthRoutes)
	return r
}

func newOAuthConfig(cfg ServerConfig) *oauth.Config {
	return &oauth.Config{
		BaseURL:             cfg.BaseURL,
		ResourcePath:        cfg.ResourcePath,
		TrustProxyHeaders:   cfg.TrustProxyHeaders,
		AuthorizationServer: cfg.AuthorizationServer,
	}
}

// resolveListenAddress returns the address string passed to http.Server.
// When host is empty the server binds to all interfaces on the given port;
// otherwise host and port are joined into a single address.
func resolveListenAddress(host string, port int) string {
	if host == "" {
		return fmt.Sprintf(":%d", port)
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func configureRequestState(cfg *ServerConfig, logger *slog.Logger) (github.RequestStateSealer, error) {
	if cfg.MRTRStateKey == "" {
		cfg.disableDeleteRepository = true
		logger.Warn("delete_repository disabled: request-state encryption key is not configured", "environmentVariable", MRTRStateKeyEnv)
		return nil, nil
	}
	sealer, err := requeststate.New(cfg.MRTRStateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", MRTRStateKeyEnv, err)
	}
	return sealer, nil
}

func initGlobalToolScopeMap(t translations.TranslationHelperFunc, hostType utils.HostType) error {
	// Build inventory with all tools to extract scope information
	inv, err := inventory.NewBuilder().
		SetTools(github.AllTools(t, github.WithHost(hostType))).
		Build()

	if err != nil {
		return fmt.Errorf("failed to build inventory for tool scope map: %w", err)
	}

	// Initialize the global scope map
	scopes.SetToolScopeMapFromInventory(inv)

	return nil
}

// createHTTPFeatureChecker creates a feature checker that resolves static CLI
// features plus per-request header features and insiders mode.
func createHTTPFeatureChecker(enabledFeatures []string, insidersMode bool) inventory.FeatureFlagChecker {
	return func(ctx context.Context, flag string) (bool, error) {
		headerFeatures := ghcontext.GetHeaderFeatures(ctx)
		features := make([]string, 0, len(enabledFeatures)+len(headerFeatures))
		features = append(features, enabledFeatures...)
		features = append(features, headerFeatures...)

		effective := github.ResolveFeatureFlags(features, insidersMode || ghcontext.IsInsidersMode(ctx))
		return effective[flag], nil
	}
}
