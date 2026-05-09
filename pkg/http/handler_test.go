package http

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strings"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockTool(name, toolsetID string, readOnly bool) inventory.ServerTool {
	return mockToolFull(name, toolsetID, readOnly, false)
}

func mockToolFull(name, toolsetID string, readOnly bool, isDefault bool) inventory.ServerTool {
	return inventory.ServerTool{
		Tool: mcp.Tool{
			Name:        name,
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly},
		},
		Toolset: inventory.ToolsetMetadata{
			ID:          inventory.ToolsetID(toolsetID),
			Description: "Test: " + toolsetID,
			Default:     isDefault,
		},
	}
}

type allScopesFetcher struct{}

func (f allScopesFetcher) FetchTokenScopes(_ context.Context, _ string) ([]string, error) {
	return []string{
		string(scopes.Repo),
		string(scopes.WriteOrg),
		string(scopes.User),
		string(scopes.Gist),
		string(scopes.Notifications),
	}, nil
}

var _ scopes.FetcherInterface = allScopesFetcher{}

func mockToolWithFeatureFlag(name, toolsetID string, readOnly bool, enableFlag, disableFlag string) inventory.ServerTool {
	tool := mockTool(name, toolsetID, readOnly)
	tool.FeatureFlagEnable = enableFlag
	tool.FeatureFlagDisable = disableFlag
	return tool
}

func TestInventoryFiltersForRequest(t *testing.T) {
	tools := []inventory.ServerTool{
		mockTool("get_file_contents", "repos", true),
		mockTool("create_repository", "repos", false),
		mockTool("list_issues", "issues", true),
		mockTool("issue_write", "issues", false),
	}

	tests := []struct {
		name          string
		contextSetup  func(context.Context) context.Context
		expectedTools []string
	}{
		{
			name:          "no filters applies defaults",
			contextSetup:  func(ctx context.Context) context.Context { return ctx },
			expectedTools: []string{"get_file_contents", "create_repository", "list_issues", "issue_write"},
		},
		{
			name: "readonly from context filters write tools",
			contextSetup: func(ctx context.Context) context.Context {
				return ghcontext.WithReadonly(ctx, true)
			},
			expectedTools: []string{"get_file_contents", "list_issues"},
		},
		{
			name: "toolset from context filters to toolset",
			contextSetup: func(ctx context.Context) context.Context {
				return ghcontext.WithToolsets(ctx, []string{"repos"})
			},
			expectedTools: []string{"get_file_contents", "create_repository"},
		},
		{
			name: "tools alone clears default toolsets",
			contextSetup: func(ctx context.Context) context.Context {
				return ghcontext.WithTools(ctx, []string{"list_issues"})
			},
			expectedTools: []string{"list_issues"},
		},
		{
			name: "tools are additive with toolsets",
			contextSetup: func(ctx context.Context) context.Context {
				ctx = ghcontext.WithToolsets(ctx, []string{"repos"})
				ctx = ghcontext.WithTools(ctx, []string{"list_issues"})
				return ctx
			},
			expectedTools: []string{"get_file_contents", "create_repository", "list_issues"},
		},
		{
			name: "excluded tools removes specific tools",
			contextSetup: func(ctx context.Context) context.Context {
				return ghcontext.WithExcludeTools(ctx, []string{"create_repository", "issue_write"})
			},
			expectedTools: []string{"get_file_contents", "list_issues"},
		},
		{
			name: "excluded tools overrides explicit tools",
			contextSetup: func(ctx context.Context) context.Context {
				ctx = ghcontext.WithTools(ctx, []string{"list_issues", "create_repository"})
				ctx = ghcontext.WithExcludeTools(ctx, []string{"create_repository"})
				return ctx
			},
			expectedTools: []string{"list_issues"},
		},
		{
			name: "excluded tools combines with readonly",
			contextSetup: func(ctx context.Context) context.Context {
				ctx = ghcontext.WithReadonly(ctx, true)
				ctx = ghcontext.WithExcludeTools(ctx, []string{"list_issues"})
				return ctx
			},
			expectedTools: []string{"get_file_contents"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = req.WithContext(tt.contextSetup(req.Context()))

			builder := inventory.NewBuilder().
				SetTools(tools).
				WithToolsets([]string{"all"})

			builder = InventoryFiltersForRequest(req, builder)
			inv, err := builder.Build()
			require.NoError(t, err)

			available := inv.AvailableTools(context.Background())
			toolNames := make([]string, len(available))
			for i, tool := range available {
				toolNames[i] = tool.Tool.Name
			}

			assert.ElementsMatch(t, tt.expectedTools, toolNames)
		})
	}
}

// testTools returns a set of mock tools across different toolsets with mixed read-only/write capabilities
func testTools() []inventory.ServerTool {
	return []inventory.ServerTool{
		mockTool("get_file_contents", "repos", true),
		mockTool("create_repository", "repos", false),
		mockTool("list_issues", "issues", true),
		mockTool("create_issue", "issues", false),
		mockTool("list_pull_requests", "pull_requests", true),
		mockTool("create_pull_request", "pull_requests", false),
		// Feature-flagged tools for testing X-MCP-Features header
		mockToolWithFeatureFlag("needs_holdback", "repos", true, "mcp_holdback_consolidated_projects", ""),
		mockToolWithFeatureFlag("hidden_by_holdback", "repos", true, "", "mcp_holdback_consolidated_projects"),
	}
}

// extractToolNames extracts tool names from an inventory
func extractToolNames(ctx context.Context, inv *inventory.Inventory) []string {
	available := inv.AvailableTools(ctx)
	names := make([]string, len(available))
	for i, tool := range available {
		names[i] = tool.Tool.Name
	}
	sort.Strings(names)
	return names
}

func TestHTTPHandlerRoutes(t *testing.T) {
	tools := testTools()

	tests := []struct {
		name          string
		path          string
		headers       map[string]string
		expectedTools []string
	}{
		{
			name:          "root path returns all tools",
			path:          "/",
			expectedTools: []string{"get_file_contents", "create_repository", "list_issues", "create_issue", "list_pull_requests", "create_pull_request", "hidden_by_holdback"},
		},
		{
			name:          "readonly path filters write tools",
			path:          "/readonly",
			expectedTools: []string{"get_file_contents", "list_issues", "list_pull_requests", "hidden_by_holdback"},
		},
		{
			name:          "toolset path filters to toolset",
			path:          "/x/repos",
			expectedTools: []string{"get_file_contents", "create_repository", "hidden_by_holdback"},
		},
		{
			name:          "toolset path with issues",
			path:          "/x/issues",
			expectedTools: []string{"list_issues", "create_issue"},
		},
		{
			name:          "toolset readonly path filters to readonly tools in toolset",
			path:          "/x/repos/readonly",
			expectedTools: []string{"get_file_contents", "hidden_by_holdback"},
		},
		{
			name:          "toolset readonly path with issues",
			path:          "/x/issues/readonly",
			expectedTools: []string{"list_issues"},
		},
		{
			name: "X-MCP-Tools header filters to specific tools",
			path: "/",
			headers: map[string]string{
				headers.MCPToolsHeader: "list_issues",
			},
			expectedTools: []string{"list_issues"},
		},
		{
			name: "X-MCP-Tools header with multiple tools",
			path: "/",
			headers: map[string]string{
				headers.MCPToolsHeader: "list_issues,get_file_contents",
			},
			expectedTools: []string{"list_issues", "get_file_contents"},
		},
		{
			name: "X-MCP-Tools header does not expose extra tools",
			path: "/",
			headers: map[string]string{
				headers.MCPToolsHeader: "list_issues",
			},
			expectedTools: []string{"list_issues"},
		},
		{
			name: "X-MCP-Readonly header filters write tools",
			path: "/",
			headers: map[string]string{
				headers.MCPReadOnlyHeader: "true",
			},
			expectedTools: []string{"get_file_contents", "list_issues", "list_pull_requests", "hidden_by_holdback"},
		},
		{
			name: "X-MCP-Toolsets header filters to toolset",
			path: "/",
			headers: map[string]string{
				headers.MCPToolsetsHeader: "repos",
			},
			expectedTools: []string{"get_file_contents", "create_repository", "hidden_by_holdback"},
		},
		{
			name: "URL toolset takes precedence over header toolset",
			path: "/x/issues",
			headers: map[string]string{
				headers.MCPToolsetsHeader: "repos",
			},
			expectedTools: []string{"list_issues", "create_issue"},
		},
		{
			name: "URL readonly takes precedence over header",
			path: "/readonly",
			headers: map[string]string{
				headers.MCPReadOnlyHeader: "false",
			},
			expectedTools: []string{"get_file_contents", "list_issues", "list_pull_requests", "hidden_by_holdback"},
		},
		{
			name: "X-MCP-Features header enables flagged tool",
			path: "/",
			headers: map[string]string{
				headers.MCPFeaturesHeader: "mcp_holdback_consolidated_projects",
			},
			expectedTools: []string{"get_file_contents", "create_repository", "list_issues", "create_issue", "list_pull_requests", "create_pull_request", "needs_holdback"},
		},
		{
			name: "X-MCP-Features header with unknown flag is ignored",
			path: "/",
			headers: map[string]string{
				headers.MCPFeaturesHeader: "unknown_flag",
			},
			expectedTools: []string{"get_file_contents", "create_repository", "list_issues", "create_issue", "list_pull_requests", "create_pull_request", "hidden_by_holdback"},
		},
		{
			name: "X-MCP-Exclude-Tools header removes specific tools",
			path: "/",
			headers: map[string]string{
				headers.MCPExcludeToolsHeader: "create_issue,create_pull_request",
			},
			expectedTools: []string{"get_file_contents", "create_repository", "list_issues", "list_pull_requests", "hidden_by_holdback"},
		},
		{
			name: "X-MCP-Exclude-Tools with toolset header",
			path: "/",
			headers: map[string]string{
				headers.MCPToolsetsHeader:     "issues",
				headers.MCPExcludeToolsHeader: "create_issue",
			},
			expectedTools: []string{"list_issues"},
		},
		{
			name: "X-MCP-Exclude-Tools overrides X-MCP-Tools",
			path: "/",
			headers: map[string]string{
				headers.MCPToolsHeader:        "list_issues,create_issue",
				headers.MCPExcludeToolsHeader: "create_issue",
			},
			expectedTools: []string{"list_issues"},
		},
		{
			name: "X-MCP-Exclude-Tools with readonly path",
			path: "/readonly",
			headers: map[string]string{
				headers.MCPExcludeToolsHeader: "list_issues",
			},
			expectedTools: []string{"get_file_contents", "list_pull_requests", "hidden_by_holdback"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedInventory *inventory.Inventory
			var capturedCtx context.Context

			// Create feature checker that reads from context without whitelist validation
			// (the whitelist is tested separately; here we test the filtering logic)
			featureChecker := func(ctx context.Context, flag string) (bool, error) {
				return slices.Contains(ghcontext.GetHeaderFeatures(ctx), flag), nil
			}

			apiHost, err := utils.NewAPIHost("https://api.github.com")
			require.NoError(t, err)

			// Create inventory factory that captures the built inventory
			inventoryFactory := func(r *http.Request) (*inventory.Inventory, error) {
				capturedCtx = r.Context()
				builder := inventory.NewBuilder().
					SetTools(tools).
					WithToolsets([]string{"all"}).
					WithFeatureChecker(featureChecker)
				builder = InventoryFiltersForRequest(r, builder)
				inv, err := builder.Build()
				if err != nil {
					return nil, err
				}
				capturedInventory = inv
				return inv, nil
			}

			// Create mock MCP server factory that just returns a minimal server
			mcpServerFactory := func(_ *http.Request, _ github.ToolDependencies, _ *inventory.Inventory, _ *github.MCPServerConfig) (*mcp.Server, error) {
				return mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil), nil
			}

			allScopesFetcher := allScopesFetcher{}

			// Create handler with our factories
			handler := NewHTTPMcpHandler(
				context.Background(),
				&ServerConfig{Version: "test"},
				nil, // deps not needed for this test
				translations.NullTranslationHelper,
				slog.Default(),
				apiHost,
				WithInventoryFactory(inventoryFactory),
				WithGitHubMCPServerFactory(mcpServerFactory),
				WithScopeFetcher(allScopesFetcher),
			)

			// Create router and register routes
			r := chi.NewRouter()
			handler.RegisterMiddleware(r)
			handler.RegisterRoutes(r)

			// Create request
			req := httptest.NewRequest(http.MethodPost, tt.path, nil)

			// Ensure we're setting Authorization header for token context
			req.Header.Set(headers.AuthorizationHeader, "Bearer ghp_testtoken")

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			// Execute request
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			// Verify the inventory was captured and has the expected tools
			require.NotNil(t, capturedInventory, "inventory should have been created")

			toolNames := extractToolNames(capturedCtx, capturedInventory)
			expectedSorted := make([]string, len(tt.expectedTools))
			copy(expectedSorted, tt.expectedTools)
			sort.Strings(expectedSorted)

			assert.Equal(t, expectedSorted, toolNames, "tools should match expected")
		})
	}
}

func TestStaticConfigEnforcement(t *testing.T) {
	// Use default toolsets to match real-world behavior where repos/issues/pull_requests are defaults
	tools := []inventory.ServerTool{
		mockToolFull("get_file_contents", "repos", true, true),
		mockToolFull("create_repository", "repos", false, true),
		mockToolFull("list_issues", "issues", true, true),
		mockToolFull("create_issue", "issues", false, true),
		mockToolFull("list_pull_requests", "pull_requests", true, true),
		mockToolFull("create_pull_request", "pull_requests", false, true),
		mockToolWithFeatureFlag("hidden_by_holdback", "repos", true, "", "mcp_holdback_consolidated_projects"),
	}

	tests := []struct {
		name          string
		config        *ServerConfig
		path          string
		headers       map[string]string
		expectedTools []string
	}{
		{
			name:          "no static config preserves existing behavior",
			config:        &ServerConfig{Version: "test"},
			path:          "/",
			expectedTools: []string{"get_file_contents", "create_repository", "list_issues", "create_issue", "list_pull_requests", "create_pull_request", "hidden_by_holdback"},
		},
		{
			name:          "static read-only filters write tools",
			config:        &ServerConfig{Version: "test", ReadOnly: true},
			path:          "/",
			expectedTools: []string{"get_file_contents", "list_issues", "list_pull_requests", "hidden_by_holdback"},
		},
		{
			name:   "static read-only cannot be overridden by header",
			config: &ServerConfig{Version: "test", ReadOnly: true},
			path:   "/",
			headers: map[string]string{
				headers.MCPReadOnlyHeader: "false",
			},
			expectedTools: []string{"get_file_contents", "list_issues", "list_pull_requests", "hidden_by_holdback"},
		},
		{
			name:          "static toolsets restricts available tools",
			config:        &ServerConfig{Version: "test", EnabledToolsets: []string{"repos"}},
			path:          "/",
			expectedTools: []string{"get_file_contents", "create_repository", "hidden_by_holdback"},
		},
		{
			name:   "static toolsets cannot be expanded by header",
			config: &ServerConfig{Version: "test", EnabledToolsets: []string{"repos"}},
			path:   "/",
			headers: map[string]string{
				headers.MCPToolsetsHeader: "issues",
			},
			// Header asks for "issues" but only "repos" tools exist in the static universe
			expectedTools: []string{},
		},
		{
			name:   "per-request header can narrow within static toolset bounds",
			config: &ServerConfig{Version: "test", EnabledToolsets: []string{"repos", "issues"}},
			path:   "/",
			headers: map[string]string{
				headers.MCPToolsetsHeader: "repos",
			},
			expectedTools: []string{"get_file_contents", "create_repository", "hidden_by_holdback"},
		},
		{
			name:          "static exclude-tools removes tools",
			config:        &ServerConfig{Version: "test", ExcludeTools: []string{"create_repository", "create_issue"}},
			path:          "/",
			expectedTools: []string{"get_file_contents", "list_issues", "list_pull_requests", "create_pull_request", "hidden_by_holdback"},
		},
		{
			name:   "static exclude-tools cannot be re-included by header",
			config: &ServerConfig{Version: "test", ExcludeTools: []string{"create_repository"}},
			path:   "/",
			headers: map[string]string{
				headers.MCPToolsHeader: "create_repository,list_issues",
			},
			// create_repository was excluded at static level, only list_issues available
			expectedTools: []string{"list_issues"},
		},
		{
			name:   "static read-only combined with per-request toolset",
			config: &ServerConfig{Version: "test", ReadOnly: true},
			path:   "/",
			headers: map[string]string{
				headers.MCPToolsetsHeader: "repos",
			},
			expectedTools: []string{"get_file_contents", "hidden_by_holdback"},
		},
		{
			name:          "static toolset with URL readonly",
			config:        &ServerConfig{Version: "test", EnabledToolsets: []string{"repos", "issues"}},
			path:          "/readonly",
			expectedTools: []string{"get_file_contents", "list_issues", "hidden_by_holdback"},
		},
		{
			name:          "static tools enables specific tools only",
			config:        &ServerConfig{Version: "test", EnabledTools: []string{"list_issues", "get_file_contents"}},
			path:          "/",
			expectedTools: []string{"list_issues", "get_file_contents"},
		},
		{
			name:   "static tools cannot be expanded by header",
			config: &ServerConfig{Version: "test", EnabledTools: []string{"list_issues"}},
			path:   "/",
			headers: map[string]string{
				headers.MCPToolsHeader: "create_repository",
			},
			// create_repository isn't in the static universe so it's silently dropped;
			// the empty filter shows all tools within static bounds
			expectedTools: []string{"list_issues"},
		},
		{
			name:   "static exclude-tools combined with per-request exclude",
			config: &ServerConfig{Version: "test", ExcludeTools: []string{"create_repository"}},
			path:   "/",
			headers: map[string]string{
				headers.MCPExcludeToolsHeader: "create_issue",
			},
			// Both static and per-request exclusions apply
			expectedTools: []string{"get_file_contents", "list_issues", "list_pull_requests", "create_pull_request", "hidden_by_holdback"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedInventory *inventory.Inventory
			var capturedCtx context.Context

			featureChecker := func(ctx context.Context, flag string) (bool, error) {
				return slices.Contains(ghcontext.GetHeaderFeatures(ctx), flag), nil
			}

			apiHost, err := utils.NewAPIHost("https://api.github.com")
			require.NoError(t, err)

			// Build static tools the same way the production code does
			staticTools, staticResources, staticPrompts := buildStaticInventoryFromTools(tt.config, tools, featureChecker)
			hasStatic := hasStaticConfig(tt.config)

			validToolNames := make(map[string]bool, len(staticTools))
			for _, tool := range staticTools {
				validToolNames[tool.Tool.Name] = true
			}

			inventoryFactory := func(r *http.Request) (*inventory.Inventory, error) {
				capturedCtx = r.Context()
				builder := inventory.NewBuilder().
					SetTools(staticTools).
					SetResources(staticResources).
					SetPrompts(staticPrompts).
					WithDeprecatedAliases(github.DeprecatedToolAliases).
					WithFeatureChecker(featureChecker)

				if hasStatic {
					builder = builder.WithToolsets([]string{"all"})
				}
				if tt.config.ReadOnly {
					builder = builder.WithReadOnly(true)
				}

				if hasStatic {
					r = filterRequestTools(r, validToolNames)
				}

				builder = InventoryFiltersForRequest(r, builder)
				inv, buildErr := builder.Build()
				if buildErr != nil {
					return nil, buildErr
				}
				capturedInventory = inv
				return inv, nil
			}

			mcpServerFactory := func(_ *http.Request, _ github.ToolDependencies, _ *inventory.Inventory, _ *github.MCPServerConfig) (*mcp.Server, error) {
				return mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil), nil
			}

			handler := NewHTTPMcpHandler(
				context.Background(),
				tt.config,
				nil,
				translations.NullTranslationHelper,
				slog.Default(),
				apiHost,
				WithInventoryFactory(inventoryFactory),
				WithGitHubMCPServerFactory(mcpServerFactory),
				WithScopeFetcher(allScopesFetcher{}),
			)

			r := chi.NewRouter()
			handler.RegisterMiddleware(r)
			handler.RegisterRoutes(r)

			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			req.Header.Set(headers.AuthorizationHeader, "Bearer ghp_testtoken")
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			require.NotNil(t, capturedInventory, "inventory should have been created")

			toolNames := extractToolNames(capturedCtx, capturedInventory)
			expectedSorted := make([]string, len(tt.expectedTools))
			copy(expectedSorted, tt.expectedTools)
			sort.Strings(expectedSorted)

			assert.Equal(t, expectedSorted, toolNames, "tools should match expected")
		})
	}
}

// TestContentTypeHandling verifies that the MCP StreamableHTTP handler
// accepts Content-Type values with additional parameters like charset=utf-8.
// This is a regression test for https://github.com/github/github-mcp-server/issues/2333
// where the Go SDK performs strict string matching against "application/json"
// and rejects requests with "application/json; charset=utf-8".
func TestContentTypeHandling(t *testing.T) {
	tests := []struct {
		name                   string
		contentType            string
		expectUnsupportedMedia bool
	}{
		{
			name:                   "exact application/json is accepted",
			contentType:            "application/json",
			expectUnsupportedMedia: false,
		},
		{
			name:                   "application/json with charset=utf-8 should be accepted",
			contentType:            "application/json; charset=utf-8",
			expectUnsupportedMedia: false,
		},
		{
			name:                   "application/json with charset=UTF-8 should be accepted",
			contentType:            "application/json; charset=UTF-8",
			expectUnsupportedMedia: false,
		},
		{
			name:                   "completely wrong content type is rejected",
			contentType:            "text/plain",
			expectUnsupportedMedia: true,
		},
		{
			name:                   "empty content type is rejected",
			contentType:            "",
			expectUnsupportedMedia: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal MCP server factory
			mcpServerFactory := func(_ *http.Request, _ github.ToolDependencies, _ *inventory.Inventory, _ *github.MCPServerConfig) (*mcp.Server, error) {
				return mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil), nil
			}

			// Create a simple inventory factory
			inventoryFactory := func(_ *http.Request) (*inventory.Inventory, error) {
				return inventory.NewBuilder().
					SetTools(testTools()).
					WithToolsets([]string{"all"}).
					Build()
			}

			apiHost, err := utils.NewAPIHost("https://api.github.com")
			require.NoError(t, err)

			handler := NewHTTPMcpHandler(
				context.Background(),
				&ServerConfig{Version: "test"},
				nil,
				translations.NullTranslationHelper,
				slog.Default(),
				apiHost,
				WithInventoryFactory(inventoryFactory),
				WithGitHubMCPServerFactory(mcpServerFactory),
				WithScopeFetcher(allScopesFetcher{}),
			)

			r := chi.NewRouter()
			handler.RegisterMiddleware(r)
			handler.RegisterRoutes(r)

			// Send an MCP initialize request as a POST with the given Content-Type
			body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			req.Header.Set(headers.AuthorizationHeader, "Bearer ghp_testtoken")
			req.Header.Set(headers.AcceptHeader, strings.Join([]string{headers.ContentTypeJSON, headers.ContentTypeEventStream}, ", "))
			if tt.contentType != "" {
				req.Header.Set(headers.ContentTypeHeader, tt.contentType)
			}

			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if tt.expectUnsupportedMedia {
				assert.Equal(t, http.StatusUnsupportedMediaType, rr.Code,
					"expected 415 Unsupported Media Type for Content-Type: %q", tt.contentType)
			} else {
				assert.NotEqual(t, http.StatusUnsupportedMediaType, rr.Code,
					"should not get 415 for Content-Type: %q, got status %d", tt.contentType, rr.Code)
			}
		})
	}
}

// buildStaticInventoryFromTools is a test helper that mirrors buildStaticInventory
// but uses the provided mock tools instead of calling github.AllTools.
func buildStaticInventoryFromTools(cfg *ServerConfig, tools []inventory.ServerTool, featureChecker inventory.FeatureFlagChecker) ([]inventory.ServerTool, []inventory.ServerResourceTemplate, []inventory.ServerPrompt) {
	if !hasStaticConfig(cfg) {
		return tools, nil, nil
	}

	b := inventory.NewBuilder().
		SetTools(tools).
		WithFeatureChecker(featureChecker).
		WithReadOnly(cfg.ReadOnly).
		WithToolsets(github.ResolvedEnabledToolsets(cfg.DynamicToolsets, cfg.EnabledToolsets, cfg.EnabledTools))

	if len(cfg.EnabledTools) > 0 {
		b = b.WithTools(github.CleanTools(cfg.EnabledTools))
	}

	if len(cfg.ExcludeTools) > 0 {
		b = b.WithExcludeTools(cfg.ExcludeTools)
	}

	inv, err := b.Build()
	if err != nil {
		return tools, nil, nil
	}

	ctx := context.Background()
	return inv.AvailableTools(ctx), inv.AvailableResourceTemplates(ctx), inv.AvailablePrompts(ctx)
}

func TestCrossOriginProtection(t *testing.T) {
	jsonRPCBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0.1"}}}`

	apiHost, err := utils.NewAPIHost("https://api.githubcopilot.com")
	require.NoError(t, err)

	handler := NewHTTPMcpHandler(
		context.Background(),
		&ServerConfig{
			Version: "test",
		},
		nil,
		translations.NullTranslationHelper,
		slog.Default(),
		apiHost,
		WithInventoryFactory(func(_ *http.Request) (*inventory.Inventory, error) {
			return inventory.NewBuilder().Build()
		}),
		WithGitHubMCPServerFactory(func(_ *http.Request, _ github.ToolDependencies, _ *inventory.Inventory, _ *github.MCPServerConfig) (*mcp.Server, error) {
			return mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil), nil
		}),
		WithScopeFetcher(allScopesFetcher{}),
	)

	r := chi.NewRouter()
	handler.RegisterMiddleware(r)
	handler.RegisterRoutes(r)

	tests := []struct {
		name         string
		secFetchSite string
		origin       string
	}{
		{
			name:         "cross-site request with bearer token succeeds",
			secFetchSite: "cross-site",
			origin:       "https://example.com",
		},
		{
			name:         "same-origin request succeeds",
			secFetchSite: "same-origin",
		},
		{
			name:         "native client without Sec-Fetch-Site succeeds",
			secFetchSite: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(jsonRPCBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			req.Header.Set(headers.AuthorizationHeader, "Bearer github_pat_xyz")
			if tt.secFetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tt.secFetchSite)
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code, "unexpected status code; body: %s", rr.Body.String())
		})
	}
}

// TestInsidersRoutePreservesUIMeta is a regression test for the bug where
// _meta.ui was stripped from tools/list responses on the HTTP /insiders route.
//
// Before the fix:
//   - buildStaticInventory called Build() on a builder configured with the
//     HTTP feature checker (which reads insiders mode from the request ctx).
//   - Build() invoked checkFeatureFlag(context.Background()) — bg ctx has no
//     insiders mode, so the FF reported MCP Apps off, and stripMCPAppsMetadata
//     ran eagerly against the static tool slice at server startup.
//   - Per-request inventory factories then served pre-stripped tools regardless
//     of whether the request actually came in via /insiders.
//
// After the fix:
//   - Build() no longer touches MCP Apps metadata.
//   - RegisterTools applies the strip per-request, using the request context
//     where the HTTP feature checker correctly observes insiders mode.
func TestInsidersRoutePreservesUIMeta(t *testing.T) {
	const uiURI = "ui://test/widget"
	uiTool := mockTool("with_ui", "repos", true)
	uiTool.Tool.Meta = mcp.Meta{"ui": map[string]any{"resourceUri": uiURI}}

	checker := createHTTPFeatureChecker()
	build := func() *inventory.Inventory {
		inv, err := inventory.NewBuilder().
			SetTools([]inventory.ServerTool{uiTool}).
			WithFeatureChecker(checker).
			WithToolsets([]string{"all"}).
			Build()
		require.NoError(t, err)
		return inv
	}

	// Simulate a /insiders request: ctx has insiders mode set.
	insidersCtx := ghcontext.WithInsidersMode(context.Background(), true)

	// AvailableTools no longer strips _meta.ui (post-fix), regardless of ctx.
	// The strip lives in RegisterTools, gated on the per-request FF check.
	insidersTools := build().AvailableTools(insidersCtx)
	plainTools := build().AvailableTools(context.Background())

	// On the /insiders path, the FF check returns true → no strip → _meta preserved.
	enabled, _ := checker(insidersCtx, "remote_mcp_ui_apps")
	require.True(t, enabled, "FF should be on for /insiders ctx")
	require.Len(t, insidersTools, 1)
	require.NotNil(t, insidersTools[0].Tool.Meta, "_meta should be present on /insiders")
	require.Equal(t, uiURI, insidersTools[0].Tool.Meta["ui"].(map[string]any)["resourceUri"])

	// On the non-insiders path, RegisterTools strips _meta.ui.
	plainEnabled, _ := checker(context.Background(), "remote_mcp_ui_apps")
	require.False(t, plainEnabled, "FF should be off for non-insiders ctx")
	require.Len(t, plainTools, 1)
}
