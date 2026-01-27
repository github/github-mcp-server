package http

import (
	"context"
	"log/slog"
	"net/http"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/github/github-mcp-server/pkg/http/middleware"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type InventoryFactoryFunc func(r *http.Request) (*inventory.Inventory, error)
type GitHubMCPServerFactoryFunc func(r *http.Request, deps github.ToolDependencies, inventory *inventory.Inventory, cfg *github.MCPServerConfig) (*mcp.Server, error)

type Handler struct {
	ctx                    context.Context
	config                 *ServerConfig
	deps                   github.ToolDependencies
	logger                 *slog.Logger
	t                      translations.TranslationHelperFunc
	githubMcpServerFactory GitHubMCPServerFactoryFunc
	inventoryFactoryFunc   InventoryFactoryFunc
}

type HandlerOptions struct {
	GitHubMcpServerFactory GitHubMCPServerFactoryFunc
	InventoryFactory       InventoryFactoryFunc
}

type HandlerOption func(*HandlerOptions)

func WithGitHubMCPServerFactory(f GitHubMCPServerFactoryFunc) HandlerOption {
	return func(o *HandlerOptions) {
		o.GitHubMcpServerFactory = f
	}
}

func WithInventoryFactory(f InventoryFactoryFunc) HandlerOption {
	return func(o *HandlerOptions) {
		o.InventoryFactory = f
	}
}

func NewHTTPMcpHandler(
	ctx context.Context,
	cfg *ServerConfig,
	deps github.ToolDependencies,
	t translations.TranslationHelperFunc,
	logger *slog.Logger,
	options ...HandlerOption) *Handler {
	opts := &HandlerOptions{}
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

	return &Handler{
		ctx:                    ctx,
		config:                 cfg,
		deps:                   deps,
		logger:                 logger,
		t:                      t,
		githubMcpServerFactory: githubMcpServerFactory,
		inventoryFactoryFunc:   inventoryFactory,
	}
}

// RegisterRoutes registers the routes for the MCP server
// URL-based values take precedence over header-based values
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Use(middleware.WithRequestConfig)

	r.Mount("/", h)
	// Mount readonly and toolset routes
	r.With(withToolset).Mount("/x/{toolset}", h)
	r.With(withReadonly, withToolset).Mount("/x/{toolset}/readonly", h)
	r.With(withReadonly).Mount("/readonly", h)
}

// withReadonly is middleware that sets readonly mode in the request context
func withReadonly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := ghcontext.WithReadonly(r.Context(), true)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// withToolset is middleware that extracts the toolset from the URL and sets it in the request context
func withToolset(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		toolset := chi.URLParam(r, "toolset")
		ctx := ghcontext.WithToolsets(r.Context(), []string{toolset})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	inventory, err := h.inventoryFactoryFunc(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ghServer, err := h.githubMcpServerFactory(r, h.deps, inventory, &github.MCPServerConfig{
		Version:           h.config.Version,
		Translator:        h.t,
		ContentWindowSize: h.config.ContentWindowSize,
		Logger:            h.logger,
		RepoAccessTTL:     h.config.RepoAccessCacheTTL,
	})

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}

	mcpHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return ghServer
	}, &mcp.StreamableHTTPOptions{
		Stateless: true,
	})

	middleware.ExtractUserToken()(mcpHandler).ServeHTTP(w, r)
}

func DefaultGitHubMCPServerFactory(r *http.Request, deps github.ToolDependencies, inventory *inventory.Inventory, cfg *github.MCPServerConfig) (*mcp.Server, error) {
	return github.NewMCPServer(r.Context(), cfg, deps, inventory)
}

func DefaultInventoryFactory(_ *ServerConfig, t translations.TranslationHelperFunc, staticChecker inventory.FeatureFlagChecker) InventoryFactoryFunc {
	return func(r *http.Request) (*inventory.Inventory, error) {
		b := github.NewInventory(t).WithDeprecatedAliases(github.DeprecatedToolAliases)

		// Feature checker composition
		headerFeatures := headers.ParseCommaSeparated(r.Header.Get(headers.MCPFeaturesHeader))
		if checker := ComposeFeatureChecker(headerFeatures, staticChecker); checker != nil {
			b = b.WithFeatureChecker(checker)
		}

		b = InventoryFiltersForRequest(r, b)
		b.WithServerInstructions()

		return b.Build()
	}
}

// InventoryFiltersForRequest applies filters to the inventory builder
// based on the request context and headers
func InventoryFiltersForRequest(r *http.Request, builder *inventory.Builder) *inventory.Builder {
	ctx := r.Context()

	if ghcontext.IsReadonly(ctx) {
		builder = builder.WithReadOnly(true)
	}

	if toolsets := ghcontext.GetToolsets(ctx); len(toolsets) > 0 {
		builder = builder.WithToolsets(toolsets)
	}

	if tools := ghcontext.GetTools(ctx); len(tools) > 0 {
		if len(ghcontext.GetToolsets(ctx)) == 0 {
			builder = builder.WithToolsets([]string{})
		}
		builder = builder.WithTools(github.CleanTools(tools))
	}

	return builder
}
