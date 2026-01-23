package http

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/github/github-mcp-server/pkg/http/middleware"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type InventoryFactoryFunc func(r *http.Request) (*inventory.Inventory, error)
type GitHubMCPServerFactoryFunc func(ctx context.Context, r *http.Request, deps github.ToolDependencies, inventory *inventory.Inventory, cfg *github.MCPServerConfig) (*mcp.Server, error)

type HTTPMcpHandler struct {
	config                 *HTTPServerConfig
	deps                   github.ToolDependencies
	logger                 *slog.Logger
	t                      translations.TranslationHelperFunc
	githubMcpServerFactory GitHubMCPServerFactoryFunc
	inventoryFactoryFunc   InventoryFactoryFunc
}

type HTTPMcpHandlerOptions struct {
	GitHubMcpServerFactory GitHubMCPServerFactoryFunc
	InventoryFactory       InventoryFactoryFunc
}

type HTTPMcpHandlerOption func(*HTTPMcpHandlerOptions)

func WithGitHubMCPServerFactory(f GitHubMCPServerFactoryFunc) HTTPMcpHandlerOption {
	return func(o *HTTPMcpHandlerOptions) {
		o.GitHubMcpServerFactory = f
	}
}

func WithInventoryFactory(f InventoryFactoryFunc) HTTPMcpHandlerOption {
	return func(o *HTTPMcpHandlerOptions) {
		o.InventoryFactory = f
	}
}

func NewHTTPMcpHandler(cfg *HTTPServerConfig,
	deps github.ToolDependencies,
	t translations.TranslationHelperFunc,
	logger *slog.Logger,
	options ...HTTPMcpHandlerOption) *HTTPMcpHandler {
	opts := &HTTPMcpHandlerOptions{}
	for _, o := range options {
		o(opts)
	}

	githubMcpServerFactory := opts.GitHubMcpServerFactory
	if githubMcpServerFactory == nil {
		githubMcpServerFactory = DefaultGitHubMCPServerFactory
	}

	inventoryFactory := opts.InventoryFactory
	if inventoryFactory == nil {
		inventoryFactory = DefaultInventoryFactory(cfg, t, nil)
	}

	return &HTTPMcpHandler{
		config:                 cfg,
		deps:                   deps,
		logger:                 logger,
		t:                      t,
		githubMcpServerFactory: githubMcpServerFactory,
		inventoryFactoryFunc:   inventoryFactory,
	}
}

func (h *HTTPMcpHandler) RegisterRoutes(r chi.Router) {
	r.Mount("/", h)
}

func (h *HTTPMcpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	inventory, err := h.inventoryFactoryFunc(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ghServer, err := h.githubMcpServerFactory(r.Context(), r, h.deps, inventory, &github.MCPServerConfig{
		Version:           h.config.Version,
		Translator:        h.t,
		ContentWindowSize: h.config.ContentWindowSize,
		Logger:            h.logger,
		RepoAccessTTL:     h.config.RepoAccessCacheTTL,
	})

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}

	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return ghServer
	}, &mcp.StreamableHTTPOptions{
		Stateless: true,
	})

	middleware.ExtractUserToken()(mcpHandler).ServeHTTP(w, r)
}

func DefaultGitHubMCPServerFactory(ctx context.Context, _ *http.Request, deps github.ToolDependencies, inventory *inventory.Inventory, cfg *github.MCPServerConfig) (*mcp.Server, error) {
	return github.NewMCPServer(&github.MCPServerConfig{
		Version:           cfg.Version,
		Translator:        cfg.Translator,
		ContentWindowSize: cfg.ContentWindowSize,
		Logger:            cfg.Logger,
		RepoAccessTTL:     cfg.RepoAccessTTL,
	}, deps, inventory)
}

func DefaultInventoryFactory(cfg *HTTPServerConfig, t translations.TranslationHelperFunc, staticChecker inventory.FeatureFlagChecker) InventoryFactoryFunc {
	return func(r *http.Request) (*inventory.Inventory, error) {
		b := github.NewInventory(t).WithDeprecatedAliases(github.DeprecatedToolAliases)

		// Feature checker composition
		headerFeatures := parseCommaSeparatedHeader(r.Header.Get(headers.MCPFeaturesHeader))
		if checker := ComposeFeatureChecker(headerFeatures, staticChecker); checker != nil {
			b = b.WithFeatureChecker(checker)
		}

		b = InventoryFiltersForRequestHeaders(r, b)
		b.WithServerInstructions()

		return b.Build()
	}
}

// InventoryFiltersForRequestHeaders applies inventory filters based on HTTP request headers.
// Whitespace is trimmed from comma-separated values; empty values are ignored.
func InventoryFiltersForRequestHeaders(r *http.Request, builder *inventory.Builder) *inventory.Builder {
	if r.Header.Get(headers.MCPReadOnlyHeader) != "" {
		builder = builder.WithReadOnly(true)
	}

	if toolsetsStr := r.Header.Get(headers.MCPToolsetsHeader); toolsetsStr != "" {
		toolsets := parseCommaSeparatedHeader(toolsetsStr)
		builder = builder.WithToolsets(toolsets)
	}

	if toolsStr := r.Header.Get(headers.MCPToolsHeader); toolsStr != "" {
		tools := parseCommaSeparatedHeader(toolsStr)
		builder = builder.WithTools(github.CleanTools(tools))
	}

	return builder
}

// parseCommaSeparatedHeader splits a header value by comma, trims whitespace,
// and filters out empty values.
func parseCommaSeparatedHeader(value string) []string {
	if value == "" {
		return []string{}
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
