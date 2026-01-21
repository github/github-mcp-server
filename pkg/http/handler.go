package http

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/github/github-mcp-server/pkg/http/middleware"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type InventoryFactoryFunc func(r *http.Request) *inventory.Inventory
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
		ctx := ghcontext.WithToolset(r.Context(), toolset)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *HTTPMcpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	inventory := h.inventoryFactoryFunc(r)

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
	return func(r *http.Request) *inventory.Inventory {
		b := github.NewInventory(t).WithDeprecatedAliases(github.DeprecatedToolAliases)

		// Feature checker composition
		headerFeatures := parseCommaSeparatedHeader(r.Header.Get(headers.MCPFeaturesHeader))
		if checker := ComposeFeatureChecker(headerFeatures, staticChecker); checker != nil {
			b = b.WithFeatureChecker(checker)
		}

		b = InventoryFiltersForRequest(r, b)
		return b.Build()
	}
}

// InventoryFiltersForRequest applies inventory filters from request context and headers
// Whitespace is trimmed from comma-separated values; empty values are ignored
// Route configuration (context) takes precedence over headers for toolsets
func InventoryFiltersForRequest(r *http.Request, builder *inventory.Builder) *inventory.Builder {
	ctx := r.Context()

	// Enable readonly mode if set in context or via header
	if ghcontext.IsReadonly(ctx) || relaxedParseBool(r.Header.Get(headers.MCPReadOnlyHeader)) {
		builder = builder.WithReadOnly(true)
	}

	// Parse request configuration
	contextToolset := ghcontext.GetToolset(ctx)
	headerToolsets := parseCommaSeparatedHeader(r.Header.Get(headers.MCPToolsetsHeader))
	tools := parseCommaSeparatedHeader(r.Header.Get(headers.MCPToolsHeader))

	// Apply toolset filtering (route wins, then header, then tools-only mode, else defaults)
	switch {
	case contextToolset != "":
		builder = builder.WithToolsets([]string{contextToolset})
	case len(headerToolsets) > 0:
		builder = builder.WithToolsets(headerToolsets)
	case len(tools) > 0:
		builder = builder.WithToolsets([]string{})
	}

	if len(tools) > 0 {
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

// relaxedParseBool parses a string into a boolean value, treating various
// common false values or empty strings as false, and everything else as true.
// It is case-insensitive and trims whitespace.
func relaxedParseBool(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	falseValues := []string{"", "false", "0", "no", "off", "n", "f"}
	return !slices.Contains(falseValues, s)
}
