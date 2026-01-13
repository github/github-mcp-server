package ghmcp

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
	"github.com/github/github-mcp-server/pkg/http/middleware"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type HTTPServerConfig struct {
	// Version of the server
	Version string

	// GitHub Host to target for API requests (e.g. github.com or github.enterprise.com)
	Host string

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

func RunHTTPServer(cfg HTTPServerConfig) error {
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

	// Set up repo access cache for lockdown mode
	var opts []lockdown.RepoAccessOption
	if cfg.LockdownMode {
		opts = []lockdown.RepoAccessOption{
			lockdown.WithLogger(logger.With("component", "lockdown")),
		}
		if cfg.RepoAccessCacheTTL != nil {
			opts = append(opts, lockdown.WithTTL(*cfg.RepoAccessCacheTTL))
		}
	}

	apiHost, err := utils.ParseAPIHost(cfg.Host)
	if err != nil {
		return fmt.Errorf("failed to parse API host: %w", err)
	}

	deps := github.NewRequestDeps(
		&apiHost,
		cfg.Version,
		cfg.LockdownMode,
		opts,
		t,
		github.FeatureFlags{
			LockdownMode: cfg.LockdownMode,
		},
		cfg.ContentWindowSize,
	)

	ghServer, err := github.NewMcpServer(&github.MCPServerConfig{
		Version:           cfg.Version,
		Host:              cfg.Host,
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
	}, deps)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	handler := NewHttpMcpHandler(&cfg, ghServer)

	httpSvr := http.Server{
		Addr:    ":8082",
		Handler: handler,
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

	logger.Info("HTTP server listening on :8082")
	if err := httpSvr.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}

	logger.Info("server stopped gracefully")
	return nil
}

type HttpMcpHandler struct {
	config   *HTTPServerConfig
	ghServer *mcp.Server
}

func NewHttpMcpHandler(cfg *HTTPServerConfig, mcpServer *mcp.Server) *HttpMcpHandler {
	return &HttpMcpHandler{
		config:   cfg,
		ghServer: mcpServer,
	}
}

func (s *HttpMcpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.ghServer
	}, &mcp.StreamableHTTPOptions{
		Stateless: true,
	})

	middleware.ExtractUserToken()(mcpHandler).ServeHTTP(w, r)
}
