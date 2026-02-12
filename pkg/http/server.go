package http

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/http/oauth"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/go-chi/chi/v5"
)

// knownFeatureFlags are the feature flags that can be enabled via X-MCP-Features header.
// Only these flags are accepted from headers.
var knownFeatureFlags = []string{}

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

	// Content window size
	ContentWindowSize int

	// LockdownMode indicates if we should enable lockdown mode
	LockdownMode bool

	// RepoAccessCacheTTL overrides the default TTL for repository access cache entries.
	RepoAccessCacheTTL *time.Duration

	// ScopeChallenge indicates if we should return OAuth scope challenges, and if we should perform
	// tool filtering based on token scopes.
	ScopeChallenge bool
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
	logger.Info("starting server", "version", cfg.Version, "host", cfg.Host, "lockdownEnabled", cfg.LockdownMode)

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

	deps := github.NewRequestDeps(
		apiHost,
		cfg.Version,
		cfg.LockdownMode,
		repoAccessOpts,
		t,
		cfg.ContentWindowSize,
		featureChecker,
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
	oauthHandler, err := oauth.NewAuthHandler(oauthCfg)
	if err != nil {
		return fmt.Errorf("failed to create OAuth handler: %w", err)
	}

	r.Group(func(r chi.Router) {
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

// createHTTPFeatureChecker creates a feature checker that reads header features from context
// and validates them against the knownFeatureFlags whitelist
func createHTTPFeatureChecker() inventory.FeatureFlagChecker {
	// Pre-compute whitelist as set for O(1) lookup
	knownSet := make(map[string]bool, len(knownFeatureFlags))
	for _, f := range knownFeatureFlags {
		knownSet[f] = true
	}

	return func(ctx context.Context, flag string) (bool, error) {
		if knownSet[flag] && slices.Contains(ghcontext.GetHeaderFeatures(ctx), flag) {
			return true, nil
		}
		return false, nil
	}
}
