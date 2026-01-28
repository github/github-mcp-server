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

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/lockdown"
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

	featureChecker := CreateHTTPFeatureChecker(nil)

	deps := github.NewRequestDeps(
		apiHost,
		cfg.Version,
		cfg.LockdownMode,
		repoAccessOpts,
		t,
		cfg.ContentWindowSize,
		featureChecker,
	)

	r := chi.NewRouter()

	handler := NewHTTPMcpHandler(ctx, &cfg, deps, t, logger, WithHandlerFeatureChecker(featureChecker))
	handler.RegisterRoutes(r)

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
