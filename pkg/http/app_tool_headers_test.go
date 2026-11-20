package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/github"
	"github.com/github/github-mcp-server/v2/pkg/http/headers"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAppOnlyToolsDoNotRequireProjectedHeaders pins the Streamable HTTP header
// contract for app-only tools. MCP App views reach the server through the
// host's tools/call proxy, and the MCP Apps bridge (App.callServerTool) gives a
// view no way to set HTTP request headers, so requiring Mcp-Param-owner on
// ui_get makes every view lookup fail with -32020 in hosts that do not project
// headers for proxied calls (issue #3834). Model-visible tools keep the
// projection requirement.
func TestAppOnlyToolsDoNotRequireProjectedHeaders(t *testing.T) {
	router := newAppToolHeaderRouter(t)

	uiGetArgs := map[string]any{"method": "labels", "owner": "octo-org", "repo": "hello-world"}

	t.Run("app-only tool without projected headers reaches the handler", func(t *testing.T) {
		rec := postToolCall(t, router, "ui_get", uiGetArgs, nil)
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		assert.NotContains(t, rec.Body.String(), "header mismatch")
	})

	t.Run("app-only tool with projected headers still works", func(t *testing.T) {
		rec := postToolCall(t, router, "ui_get", uiGetArgs, map[string]string{
			headers.MCPParamOwnerHeader: "octo-org",
			headers.MCPParamRepoHeader:  "hello-world",
		})
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		assert.NotContains(t, rec.Body.String(), "header mismatch")
	})

	t.Run("model-visible tool still requires projected headers", func(t *testing.T) {
		rec := postToolCall(t, router, "issue_write", map[string]any{
			"method": "create", "owner": "octo-org", "repo": "hello-world", "title": "x",
		}, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), "-32020")
		// Annotation iteration order is not fixed, so assert on the shared
		// prefix instead of naming one projected parameter.
		assert.Contains(t, rec.Body.String(), "header mismatch: missing Mcp-Param-")
	})

	t.Run("model-visible tool with projected headers reaches the handler", func(t *testing.T) {
		rec := postToolCall(t, router, "issue_write", map[string]any{
			"method": "create", "owner": "octo-org", "repo": "hello-world", "title": "x",
		}, map[string]string{
			headers.MCPParamOwnerHeader: "octo-org",
			headers.MCPParamRepoHeader:  "hello-world",
		})
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	})
}

// newAppToolHeaderRouter builds the production HTTP handler around the real
// GitHub inventory, with tools/call results stubbed so the boundary behaviour
// can be asserted without credentials.
func newAppToolHeaderRouter(t *testing.T) http.Handler {
	t.Helper()

	apiHost, err := utils.NewAPIHost("https://api.github.com")
	require.NoError(t, err)

	handler := NewHTTPMcpHandler(
		context.Background(),
		&ServerConfig{Version: "test"},
		github.BaseDeps{},
		translations.NullTranslationHelper,
		slog.Default(),
		apiHost,
		WithInventoryFactory(func(*http.Request) (*inventory.Inventory, error) {
			return github.NewInventory(translations.NullTranslationHelper).
				WithToolsets([]string{"all"}).
				Build()
		}),
		WithGitHubMCPServerFactory(func(r *http.Request, deps github.ToolDependencies, inv *inventory.Inventory, _ *github.MCPServerConfig) (*mcp.Server, error) {
			server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
			server.AddReceivingMiddleware(stubToolsCall)
			inv.RegisterTools(r.Context(), server, deps)
			return server, nil
		}),
		WithScopeFetcher(allScopesFetcher{}),
	)

	router := chi.NewRouter()
	handler.RegisterMiddleware(router)
	handler.RegisterRoutes(router)
	return router
}

// stubToolsCall answers tools/call without running a tool handler, so these
// tests exercise only the Streamable HTTP boundary.
func stubToolsCall(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if method == "tools/call" {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "stubbed"}}}, nil
		}
		return next(ctx, method, req)
	}
}

func postToolCall(t *testing.T, router http.Handler, name string, args map[string]any, paramHeaders map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	params, err := json.Marshal(map[string]any{
		"name":      name,
		"arguments": args,
		"_meta": map[string]any{
			mcp.MetaKeyProtocolVersion:    inventory.ProtocolVersionMultiRoundTrip,
			mcp.MetaKeyClientInfo:         map[string]any{"name": "test", "version": "1.0.0"},
			mcp.MetaKeyClientCapabilities: map[string]any{},
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":`+string(params)+`}`))
	req.Header.Set(headers.ContentTypeHeader, headers.ContentTypeJSON)
	req.Header.Set(headers.AcceptHeader, strings.Join([]string{headers.ContentTypeJSON, headers.ContentTypeEventStream}, ", "))
	req.Header.Set(headers.MCPProtocolVersionHeader, inventory.ProtocolVersionMultiRoundTrip)
	req.Header.Set(headers.MCPMethodHeader, "tools/call")
	req.Header.Set(headers.MCPNameHeader, name)
	req.Header.Set(headers.AuthorizationHeader, "ghs_test-token")
	for key, value := range paramHeaders {
		req.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}
