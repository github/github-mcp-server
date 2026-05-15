package http

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

type ServerConfig struct {
	// Version of the server
	Version string

	// GitHub Host to target for API requests (e.g. github.com or github.enterprise.com)
	Host string

	// Port to listen on (default: 8082)
	Port int

	// BaseURL is the publicly accessible URL of this server for OAuth resource metadata.
	// If not set, the server will derive the URL from incoming request headers.
	BaseURL string

	// ResourcePath is the externally visible base path for this server (e.g., "/mcp").
	// This is used to restore the original path when a proxy strips a base path before forwarding.
	ResourcePath string

	// ExportTranslations indicates if we should export translations
	// See: https://github.com/github/github-mcp-server?tab=readme-ov-file#i18n--overriding-descriptions
	ExportTranslations bool

	// EnableCommandLogging indicates if we should log commands
	EnableCommandLogging bool

	// Path to the log file if not stderr
	LogFilePath string

	// LogLevel overrides the default log level ("debug", "info", "warn", "error").
	// When empty, defaults depend on LogFilePath: Debug when writing to a file,
	// Info when writing to stderr.
	LogLevel string

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

	// DynamicToolsets enables dynamic toolset discovery mode.
	DynamicToolsets bool

	// ExcludeTools is a list of tool names to disable regardless of other settings.
	// When set via CLI flag, per-request headers cannot re-include these tools.
	ExcludeTools []string

	// InsidersMode indicates if we should enable experimental features.
	InsidersMode bool
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
	} else {
		logOutput = os.Stderr
	}
	level, err := observability.ParseLogLevel(cfg.LogLevel, defaultHTTPLogLevel(cfg.LogFilePath))
	if err != nil {
		return err
	}
	slogHandler = slog.NewTextHandler(logOutput, &slog.HandlerOptions{Level: level})
	logger := slog.New(slogHandler)
	logger.Info("starting server", "version", cfg.Version, "host", cfg.Host, "lockdownEnabled", cfg.LockdownMode, "readOnly", cfg.ReadOnly, "insidersMode", cfg.InsidersMode)

	apiHost, err := utils.NewAPIHost(cfg.Host)
	if err != nil {
		return fmt.Errorf("failed to parse API host: %w", err)
	}

	repoAccessOpts := []lockdown.RepoAccessOption{
		lockdown.WithLogger(logger.With("component", "lockdown")),
	}
	if cfg.RepoAccessCacheTTL != nil {
		repoAccessOpts = append(repoAccessOpts, lockdown.WithTTL(*cfg.RepoAccessCacheTTL))
	}

	featureChecker := createHTTPFeatureChecker()

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

	// Initialize the global tool scope map
	err = initGlobalToolScopeMap(t)
	if err != nil {
		return fmt.Errorf("failed to initialize tool scope map: %w", err)
	}

	// Register OAuth protected resource metadata endpoints
	oauthCfg := &oauth.Config{
		BaseURL:      cfg.BaseURL,
		ResourcePath: cfg.ResourcePath,
	}

	serverOptions := []HandlerOption{}
	if cfg.ScopeChallenge {
		scopeFetcher := scopes.NewFetcher(apiHost, scopes.FetcherOptions{})
		serverOptions = append(serverOptions, WithScopeFetcher(scopeFetcher))
	}

	r := chi.NewRouter()
	handler := NewHTTPMcpHandler(ctx, &cfg, deps, t, logger, apiHost, append(serverOptions, WithFeatureChecker(featureChecker), WithOAuthConfig(oauthCfg))...)
	oauthHandler, err := oauth.NewAuthHandler(oauthCfg, apiHost)
	if err != nil {
		return fmt.Errorf("failed to create OAuth handler: %w", err)
	}

	r.Group(func(r chi.Router) {
		r.Use(middleware.SetCorsHeaders)

		// Register Middleware First, needs to be before route registration
		handler.RegisterMiddleware(r)

		// Register MCP server routes
		handler.RegisterRoutes(r)
	})
	logger.Info("MCP endpoints registered", "baseURL", cfg.BaseURL)

	r.Group(func(r chi.Router) {
		// Register OAuth protected resource metadata endpoints
		oauthHandler.RegisterRoutes(r)
	})
	logger.Info("OAuth protected resource endpoints registered", "baseURL", cfg.BaseURL)

	addr := fmt.Sprintf(":%d", cfg.Port)
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

// defaultHTTPLogLevel mirrors the ghmcp stdio default: Debug when a log
// file is configured, Info when writing to stderr.
func defaultHTTPLogLevel(logFilePath string) slog.Level {
	if logFilePath != "" {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}

func initGlobalToolScopeMap(t translations.TranslationHelperFunc) error {
	// Build inventory with all tools to extract scope information
	inv, err := inventory.NewBuilder().
		SetTools(github.AllTools(t)).
		Build()

	if err != nil {
		return fmt.Errorf("failed to build inventory for tool scope map: %w", err)
	}

	// Initialize the global scope map
	scopes.SetToolScopeMapFromInventory(inv)

	return nil
}

// createHTTPFeatureChecker creates a feature checker that resolves features
// per-request by reading header features and insiders mode from context,
// then validating against the centralized AllowedFeatureFlags allowlist.
func createHTTPFeatureChecker() inventory.FeatureFlagChecker {
	return func(ctx context.Context, flag string) (bool, error) {
		headerFeatures := ghcontext.GetHeaderFeatures(ctx)
		insidersMode := ghcontext.IsInsidersMode(ctx)
		effective := github.ResolveFeatureFlags(headerFeatures, insidersMode)
		return effective[flag], nil
	}
}
