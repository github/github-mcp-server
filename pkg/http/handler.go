package http

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/http/middleware"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type InventoryFactoryFunc func(r *http.Request) *inventory.Inventory

type HttpMcpHandler struct {
	config               *HTTPServerConfig
	apiHosts             utils.ApiHost
	logger               *slog.Logger
	t                    translations.TranslationHelperFunc
	repoAccessOpts       []lockdown.RepoAccessOption
	inventoryFactoryFunc InventoryFactoryFunc
}

func NewHttpMcpHandler(cfg *HTTPServerConfig,
	t translations.TranslationHelperFunc,
	apiHosts *utils.ApiHost,
	repoAccessOptions []lockdown.RepoAccessOption,
	logger *slog.Logger,
	inventoryFactory InventoryFactoryFunc) *HttpMcpHandler {
	return &HttpMcpHandler{
		config:               cfg,
		apiHosts:             *apiHosts,
		logger:               logger,
		t:                    t,
		repoAccessOpts:       repoAccessOptions,
		inventoryFactoryFunc: inventoryFactory,
	}
}

func (s *HttpMcpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func DefaultInventoryFactory(cfg *HTTPServerConfig, t translations.TranslationHelperFunc) InventoryFactoryFunc {
	return func(r *http.Request) *inventory.Inventory {
		b := github.NewInventory(t).WithDeprecatedAliases(github.DeprecatedToolAliases)
		b = InventoryFiltersForRequestHeaders(r, b)
		return b.Build()
	}
}

func InventoryFiltersForRequestHeaders(r *http.Request, builder *inventory.Builder) *inventory.Builder {
	if r.Header.Get("X-MCP-Readonly") != "" {
		builder = builder.WithReadOnly(true)
	}

	if toolsetsStr := r.Header.Get("X-MCP-Toolsets"); toolsetsStr != "" {
		toolsets := strings.Split(toolsetsStr, ",")
		builder = builder.WithToolsets(toolsets)
	}

	if toolsStr := r.Header.Get("X-MCP-Tools"); toolsStr != "" {
		tools := strings.Split(toolsStr, ",")
		builder = builder.WithTools(github.CleanTools(tools))
	}

	return builder
}
