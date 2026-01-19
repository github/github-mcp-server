package http

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/github/github-mcp-server/pkg/http/middleware"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type InventoryFactoryFunc func(r *http.Request) *inventory.Inventory

type HTTPMcpHandler struct {
	config               *HTTPServerConfig
	apiHosts             utils.APIHost
	logger               *slog.Logger
	t                    translations.TranslationHelperFunc
	repoAccessOpts       []lockdown.RepoAccessOption
	inventoryFactoryFunc InventoryFactoryFunc
}

func NewHTTPMcpHandler(cfg *HTTPServerConfig,
	t translations.TranslationHelperFunc,
	apiHosts *utils.APIHost,
	repoAccessOptions []lockdown.RepoAccessOption,
	logger *slog.Logger,
	inventoryFactory InventoryFactoryFunc) *HTTPMcpHandler {
	return &HTTPMcpHandler{
		config:               cfg,
		apiHosts:             *apiHosts,
		logger:               logger,
		t:                    t,
		repoAccessOpts:       repoAccessOptions,
		inventoryFactoryFunc: inventoryFactory,
	}
}

func (s *HTTPMcpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set up repo access cache for lockdown mode
	deps := github.NewRequestDeps(
		&s.apiHosts,
		s.config.Version,
		s.config.LockdownMode,
		s.repoAccessOpts,
		s.t,
		github.FeatureFlags{
			LockdownMode: s.config.LockdownMode,
		},
		s.config.ContentWindowSize,
	)

	inventory := s.inventoryFactoryFunc(r)

	ghServer, err := github.NewMCPServer(&github.MCPServerConfig{
		Version:           s.config.Version,
		Host:              s.config.Host,
		Translator:        s.t,
		ContentWindowSize: s.config.ContentWindowSize,
		Logger:            s.logger,
		RepoAccessTTL:     s.config.RepoAccessCacheTTL,
	}, deps, inventory)
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

func DefaultInventoryFactory(cfg *HTTPServerConfig, t translations.TranslationHelperFunc, staticChecker inventory.FeatureFlagChecker) InventoryFactoryFunc {
	return func(r *http.Request) *inventory.Inventory {
		b := github.NewInventory(t).WithDeprecatedAliases(github.DeprecatedToolAliases)

		// Feature checker composition
		headerFeatures := parseCommaSeparatedHeader(r.Header.Get(headers.MCPFeaturesHeader))
		if checker := ComposeFeatureChecker(headerFeatures, staticChecker); checker != nil {
			b = b.WithFeatureChecker(checker)
		}

		b = InventoryFiltersForRequestHeaders(r, b)
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
