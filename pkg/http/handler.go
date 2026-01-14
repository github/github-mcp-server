package http

import (
	"log/slog"
	"net/http"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/http/middleware"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type HttpMcpHandler struct {
	config         *HTTPServerConfig
	apiHosts       utils.ApiHost
	logger         *slog.Logger
	t              translations.TranslationHelperFunc
	repoAccessOpts []lockdown.RepoAccessOption
}

func NewHttpMcpHandler(cfg *HTTPServerConfig,
	t translations.TranslationHelperFunc,
	apiHosts *utils.ApiHost,
	repoAccessOptions []lockdown.RepoAccessOption,
	logger *slog.Logger) *HttpMcpHandler {
	return &HttpMcpHandler{
		config:         cfg,
		apiHosts:       *apiHosts,
		logger:         logger,
		t:              t,
		repoAccessOpts: repoAccessOptions,
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

	ghServer, err := github.NewMcpServer(&github.MCPServerConfig{
		Version:           s.config.Version,
		Host:              s.config.Host,
		EnabledToolsets:   s.config.EnabledToolsets,
		EnabledTools:      s.config.EnabledTools,
		EnabledFeatures:   s.config.EnabledFeatures,
		DynamicToolsets:   s.config.DynamicToolsets,
		ReadOnly:          s.config.ReadOnly,
		Translator:        s.t,
		ContentWindowSize: s.config.ContentWindowSize,
		Logger:            s.logger,
		RepoAccessTTL:     s.config.RepoAccessCacheTTL,
	}, deps)
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
