package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/http/middleware"
	"github.com/github/github-mcp-server/pkg/http/oauth"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type InventoryFactoryFunc func(r *http.Request) (*inventory.Inventory, error)

// GitHubMCPServerFactoryFunc is a function type for creating a new MCP Server instance.
// middleware are applied AFTER the default GitHub MCP Server middlewares (like error context injection)
type GitHubMCPServerFactoryFunc func(r *http.Request, deps github.ToolDependencies, inventory *inventory.Inventory, cfg *github.MCPServerConfig) (*mcp.Server, error)

type Handler struct {
	ctx                    context.Context
	config                 *ServerConfig
	deps                   github.ToolDependencies
	logger                 *slog.Logger
	apiHosts               utils.APIHostResolver
	t                      translations.TranslationHelperFunc
	githubMcpServerFactory GitHubMCPServerFactoryFunc
	inventoryFactoryFunc   InventoryFactoryFunc
	oauthCfg               *oauth.Config
	scopeFetcher           scopes.FetcherInterface
	schemaCache            *mcp.SchemaCache
}

type HandlerOptions struct {
	GitHubMcpServerFactory GitHubMCPServerFactoryFunc
	InventoryFactory       InventoryFactoryFunc
	OAuthConfig            *oauth.Config
	ScopeFetcher           scopes.FetcherInterface
	FeatureChecker         inventory.FeatureFlagChecker
}

type HandlerOption func(*HandlerOptions)

func WithScopeFetcher(f scopes.FetcherInterface) HandlerOption {
	return func(o *HandlerOptions) {
		o.ScopeFetcher = f
	}
}

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

func WithOAuthConfig(cfg *oauth.Config) HandlerOption {
	return func(o *HandlerOptions) {
		o.OAuthConfig = cfg
	}
}

func WithFeatureChecker(checker inventory.FeatureFlagChecker) HandlerOption {
	return func(o *HandlerOptions) {
		o.FeatureChecker = checker
	}
}

func NewHTTPMcpHandler(
	ctx context.Context,
	cfg *ServerConfig,
	deps github.ToolDependencies,
	t translations.TranslationHelperFunc,
	logger *slog.Logger,
	apiHost utils.APIHostResolver,
	options ...HandlerOption) *Handler {
	opts := &HandlerOptions{}
	for _, o := range options {
		o(opts)
	}

	githubMcpServerFactory := opts.GitHubMcpServerFactory
	if githubMcpServerFactory == nil {
		githubMcpServerFactory = DefaultGitHubMCPServerFactory
	}

	scopeFetcher := opts.ScopeFetcher
	if scopeFetcher == nil {
		scopeFetcher = scopes.NewFetcher(apiHost, scopes.FetcherOptions{})
	}

	inventoryFactory := opts.InventoryFactory
	if inventoryFactory == nil {
		inventoryFactory = DefaultInventoryFactory(cfg, t, opts.FeatureChecker, scopeFetcher)
	}

	// Create a shared schema cache to avoid repeated JSON schema reflection
	// when a new MCP Server is created per request in stateless mode.
	schemaCache := mcp.NewSchemaCache()

	return &Handler{
		ctx:                    ctx,
		config:                 cfg,
		deps:                   deps,
		logger:                 logger,
		apiHosts:               apiHost,
		t:                      t,
		githubMcpServerFactory: githubMcpServerFactory,
		inventoryFactoryFunc:   inventoryFactory,
		oauthCfg:               opts.OAuthConfig,
		scopeFetcher:           scopeFetcher,
		schemaCache:            schemaCache,
	}
}

func (h *Handler) RegisterMiddleware(r chi.Router) {
	r.Use(
		middleware.ExtractUserToken(h.oauthCfg),
		middleware.WithRequestConfig,
		middleware.WithMCPParse(),
		middleware.WithPATScopes(h.logger, h.scopeFetcher),
	)

	if h.config.ScopeChallenge {
		r.Use(middleware.WithScopeChallenge(h.oauthCfg, h.scopeFetcher))
	}
}

// RegisterRoutes registers the routes for the MCP server
// URL-based values take precedence over header-based values
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Base routes
	r.Mount("/", h)
	r.With(withReadonly).Mount("/readonly", h)
	r.With(withInsiders).Mount("/insiders", h)
	r.With(withReadonly, withInsiders).Mount("/readonly/insiders", h)

	// Toolset routes
	r.With(withToolset).Mount("/x/{toolset}", h)
	r.With(withToolset, withReadonly).Mount("/x/{toolset}/readonly", h)
	r.With(withToolset, withInsiders).Mount("/x/{toolset}/insiders", h)
	r.With(withToolset, withReadonly, withInsiders).Mount("/x/{toolset}/readonly/insiders", h)
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

// withInsiders is middleware that sets insiders mode in the request context
func withInsiders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := ghcontext.WithInsidersMode(r.Context(), true)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	inv, err := h.inventoryFactoryFunc(r)
	if err != nil {
		if errors.Is(err, inventory.ErrUnknownTools) {
			w.WriteHeader(http.StatusBadRequest)
			if _, writeErr := w.Write([]byte(err.Error())); writeErr != nil {
				h.logger.Error("failed to write response", "error", writeErr)
			}
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	invToUse := inv
	if methodInfo, ok := ghcontext.MCPMethod(r.Context()); ok && methodInfo != nil {
		invToUse = inv.ForMCPRequest(methodInfo.Method, methodInfo.ItemName)
	}

	ghServer, err := h.githubMcpServerFactory(r, h.deps, invToUse, &github.MCPServerConfig{
		Version:           h.config.Version,
		Translator:        h.t,
		ContentWindowSize: h.config.ContentWindowSize,
		Logger:            h.logger,
		RepoAccessTTL:     h.config.RepoAccessCacheTTL,
		// Explicitly set empty capabilities. inv.ForMCPRequest currently returns nothing for Initialize.
		ServerOptions: []github.MCPServerOption{
			func(so *mcp.ServerOptions) {
				so.Capabilities = &mcp.ServerCapabilities{
					Tools:     &mcp.ToolCapabilities{},
					Resources: &mcp.ResourceCapabilities{},
					Prompts:   &mcp.PromptCapabilities{},
				}
				so.SchemaCache = h.schemaCache
			},
		},
	})

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	mcpHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return ghServer
	}, &mcp.StreamableHTTPOptions{
		Stateless: true,
	})

	mcpHandler.ServeHTTP(w, r)
}

func DefaultGitHubMCPServerFactory(r *http.Request, deps github.ToolDependencies, inventory *inventory.Inventory, cfg *github.MCPServerConfig) (*mcp.Server, error) {
	return github.NewMCPServer(r.Context(), cfg, deps, inventory)
}

// DefaultInventoryFactory creates the default inventory factory for HTTP mode
func DefaultInventoryFactory(_ *ServerConfig, t translations.TranslationHelperFunc, featureChecker inventory.FeatureFlagChecker, scopeFetcher scopes.FetcherInterface) InventoryFactoryFunc {
	return func(r *http.Request) (*inventory.Inventory, error) {
		b := github.NewInventory(t).
			WithDeprecatedAliases(github.DeprecatedToolAliases).
			WithFeatureChecker(featureChecker)

		b = InventoryFiltersForRequest(r, b)
		b = PATScopeFilter(b, r, scopeFetcher)

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

	toolsets := ghcontext.GetToolsets(ctx)
	tools := ghcontext.GetTools(ctx)

	if len(toolsets) > 0 {
		builder = builder.WithToolsets(github.ResolvedEnabledToolsets(false, toolsets, tools)) // No dynamic toolsets in HTTP mode
	}

	if len(tools) > 0 {
		if len(toolsets) == 0 {
			builder = builder.WithToolsets([]string{})
		}
		builder = builder.WithTools(github.CleanTools(tools))
	}

	return builder
}

func PATScopeFilter(b *inventory.Builder, r *http.Request, fetcher scopes.FetcherInterface) *inventory.Builder {
	ctx := r.Context()

	tokenInfo, ok := ghcontext.GetTokenInfo(ctx)
	if !ok || tokenInfo == nil {
		return b
	}

	// Scopes should have already been fetched by the WithPATScopes middleware.
	// Only classic PATs (ghp_ prefix) return OAuth scopes via X-OAuth-Scopes header.
	// Fine-grained PATs and other token types don't support this, so we skip filtering.
	if tokenInfo.TokenType == utils.TokenTypePersonalAccessToken {
		// Check if scopes are already in context (should be set by WithPATScopes). If not, fetch them.
		existingScopes, ok := ghcontext.GetTokenScopes(ctx)
		if ok {
			return b.WithFilter(github.CreateToolScopeFilter(existingScopes))
		}

		scopesList, err := fetcher.FetchTokenScopes(ctx, tokenInfo.Token)
		if err != nil {
			return b
		}

		return b.WithFilter(github.CreateToolScopeFilter(scopesList))
	}

	return b
}
