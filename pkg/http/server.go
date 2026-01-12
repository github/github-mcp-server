//go:build ignore

package http

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type HTTPHandlerConfig struct {
	Version string
}

type MCPHTTPHandler struct {
	config             HTTPHandlerConfig
	serverDependencies serverDependencies
}

// serverDependencies holds the dependencies required by the MCP server.
type serverDependencies struct {
	getClient    github.GetClientFn
	getGQClient  github.GetGQLClientFn
	getRawClient raw.GetRawClientFn
	t            translations.TranslationHelperFunc
}

func NewMCPHTTPHandler(ctx context.Context, router chi.Router) (*MCPHTTPHandler, error) {
	return &MCPHTTPHandler{}, nil
}

type reqOptions struct {
	readonly     bool
	toolsetRoute bool
}

func (h *MCPHTTPHandler) RegisterRoutes(ctx context.Context, r chi.Router) error {
	spanNameFormatter := otelhttp.WithSpanNameFormatter(
		func(operation string, r *http.Request) string {
			return r.Method + " " + routePattern(r)
		},
	)

	mount := func(pattern string, opts reqOptions) {
		r.Mount(pattern, otelhttp.NewHandler(h.perRequestHTTPHandler(opts), pattern, spanNameFormatter))
	}

	// Always mount the root route as before
	mount("/", reqOptions{})

	// Use chi params for toolset and readonly
	mount("/x/{toolset}", reqOptions{toolsetRoute: true})
	mount("/x/{toolset}/readonly", reqOptions{toolsetRoute: true, readonly: true})
	mount("/readonly", reqOptions{readonly: true})

	return nil
}

func (h *MCPHTTPHandler) perRequestHTTPHandler(opts reqOptions) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// Initialize GitHub errors context early in the request lifecycle
		ctx := r.Context()
		ctx = ghErrors.ContextWithGitHubErrors(ctx)
		r = r.WithContext(ctx)

		// Set the toolset and readonly params in the context
		if opts.readonly {
			r = setReadOnly(r)
		}
		if opts.toolsetRoute {
			r = setToolset(r)
		}

		//nolint:contextcheck // ctx returned from newMCPServerForRequest contains injected deps via ContextWithDeps
		ctx, svr, _, err := m.newMCPServerForRequest(r)
		if err != nil {
			message := fmt.Sprintf("failed to create MCP server, %s", err.Error())
			status := http.StatusInternalServerError
			http.Error(w, message, status)
			return
		}

		// Then pass to the main handler logic
		h.serveHTTP(w, r)
	})
}

func (h *MCPHTTPHandler) newMCPServerForRequest(r *http.Request) (context.Context, *mcp.Server, *inventory.Inventory, error) {
	ctx := r.Context()

	getClient := h.serverDependencies.getClient
	getGQClient := h.serverDependencies.getGQClient
	getRawClient := h.serverDependencies.getRawClient
	t := h.serverDependencies.t

	deps := github.NewBaseDeps(
		clients.rest,
		clients.gql,
		clients.raw,
		clients.repoAccess,
		cfg.Translator,
		github.FeatureFlags{LockdownMode: cfg.LockdownMode},
		cfg.ContentWindowSize,
	)

	// Build the registry with all configuration applied
	builder := inventory.NewBuilder().
		SetTools(github.AllTools(t)).
		SetResources(github.AllResources(t)).
		SetPrompts(github.AllPrompts(t))

	inv := builder.Build()

	enabledToolsetIDs := inv.EnabledToolsetIDs()
	enabledToolsets := make([]string, len(enabledToolsetIDs))
	for i, id := range enabledToolsetIDs {
		enabledToolsets[i] = string(id)
	}

	svrOpts := mcp.ServerOptions{}

	svr := github.NewServer(h.config.Version, &svrOpts)

	return ctx, svr, inv, nil
}

// routePattern returns the route pattern for the given HTTP request.
// It retrieves the route pattern from the chi RouteContext, falling back to the request's URL path
// if the route context or pattern is unavailable. If the pattern is "/*", it is normalized to "/".
// Additionally, it replaces curly braces in the pattern with a colon prefix to ensure compatibility
// with Datadog, which does not accept curly braces in route patterns.
func routePattern(r *http.Request) string {
	rc := chi.RouteContext(r.Context())
	if rc == nil || rc.RoutePattern() == "" {
		return r.URL.Path
	}

	routePattern := rc.RoutePattern()

	// Normalise /* to / as even without an explicit catch all route,
	// mounting at / can result in /* as the route pattern for unmatched paths.
	if routePattern == "/*" {
		routePattern = "/"
	}

	// Replace { and } with : because Datadog doesn't like them.
	// Before: /x/{toolset}
	// After: /x/:toolset
	routePattern = strings.ReplaceAll(routePattern, "{", ":")
	routePattern = strings.ReplaceAll(routePattern, "}", "")

	return routePattern
}
