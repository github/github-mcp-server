package http

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/github/github-mcp-server/pkg/http/middleware"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
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
		string(scopes.DeleteRepo),
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
	if disableFlag != "" {
		tool.FeatureFlagDisable = []string{disableFlag}
	}
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
			staticTools, staticResources, staticPrompts, err := buildStaticInventoryFromTools(tt.config, tools)
			require.NoError(t, err)
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

func TestDefaultInventoryFactoriesRejectInvalidEnabledTools(t *testing.T) {
	tests := []struct {
		name         string
		enabledTools []string
		unknownTools []string
	}{
		{
			name:         "mixed valid and invalid tools",
			enabledTools: []string{"get_file_contents", "nonexistent_tool"},
			unknownTools: []string{"nonexistent_tool"},
		},
		{
			name:         "all invalid tools",
			enabledTools: []string{"nonexistent_tool", "another_nonexistent_tool"},
			unknownTools: []string{"nonexistent_tool", "another_nonexistent_tool"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ServerConfig{Version: "test", EnabledTools: tt.enabledTools}
			factory, err := NewDefaultInventoryFactory(
				cfg,
				translations.NullTranslationHelper,
				nil,
				allScopesFetcher{},
			)
			require.ErrorIs(t, err, inventory.ErrUnknownTools)
			for _, unknownTool := range tt.unknownTools {
				assert.Contains(t, err.Error(), unknownTool)
			}
			assert.Nil(t, factory, "invalid static configuration must not produce a widened inventory factory")

			factory = DefaultInventoryFactory(
				cfg,
				translations.NullTranslationHelper,
				nil,
				allScopesFetcher{},
			)
			inv, err := factory(httptest.NewRequest(http.MethodPost, "/", nil))
			require.ErrorIs(t, err, inventory.ErrUnknownTools)
			assert.Nil(t, inv, "the compatibility factory must preserve the validation error")
		})
	}
}

func TestStaticInventoryInvalidEnabledToolsReturnsBadRequest(t *testing.T) {
	apiHost, err := utils.NewAPIHost("https://api.github.com")
	require.NoError(t, err)

	handler := NewHTTPMcpHandler(
		context.Background(),
		&ServerConfig{Version: "test", EnabledTools: []string{"nonexistent_tool"}},
		nil,
		translations.NullTranslationHelper,
		slog.Default(),
		apiHost,
	)

	r := chi.NewRouter()
	handler.RegisterMiddleware(r)
	handler.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(headers.AuthorizationHeader, "Bearer ghp_testtoken")

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "unknown tools specified")
}

func TestStaticInventoryPreservesPerRequestFeatureVariants(t *testing.T) {
	tools := []inventory.ServerTool{
		mockToolWithFeatureFlag("list_issues", "issues", true, "", github.FeatureFlagCSVOutput),
		mockToolWithFeatureFlag("list_issues", "issues", true, github.FeatureFlagCSVOutput, ""),
	}

	cfg := &ServerConfig{Version: "test", EnabledToolsets: []string{"issues"}}
	featureChecker := createHTTPFeatureChecker(nil, false)

	staticTools, _, _, err := buildStaticInventoryFromTools(cfg, tools)
	require.NoError(t, err)
	require.Len(t, staticTools, 2, "static upper bounds should preserve both feature variants")

	inv, err := inventory.NewBuilder().
		SetTools(staticTools).
		WithFeatureChecker(featureChecker).
		WithToolsets([]string{"all"}).
		Build()
	require.NoError(t, err)

	ctx := ghcontext.WithInsidersMode(context.Background(), true)
	available := inv.AvailableTools(ctx)
	require.Len(t, available, 1)
	assert.Equal(t, "list_issues", available[0].Tool.Name)
	assert.Equal(t, github.FeatureFlagCSVOutput, available[0].FeatureFlagEnable)
}

func TestStaticInventoryDisablesOnlyDeleteRepository(t *testing.T) {
	cfg := &ServerConfig{disableDeleteRepository: true}
	tools, _, _, err := buildStaticInventory(cfg, translations.NullTranslationHelper)
	require.NoError(t, err)

	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Tool.Name)
	}
	assert.NotContains(t, names, github.DeleteRepositoryToolName)
	assert.Contains(t, names, "actions_list", "non-default toolsets must remain available for per-request selection")
}

func TestStaticInventoryKeepsExplicitDeleteRepositoryDisabled(t *testing.T) {
	cfg := &ServerConfig{
		EnabledTools:            []string{github.DeleteRepositoryToolName},
		disableDeleteRepository: true,
	}
	tools, _, _, err := buildStaticInventory(cfg, translations.NullTranslationHelper)
	require.NoError(t, err)

	assert.Empty(t, tools, "an unavailable explicit allowlist must not widen to other tools")
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
func buildStaticInventoryFromTools(cfg *ServerConfig, tools []inventory.ServerTool) ([]inventory.ServerTool, []inventory.ServerResourceTemplate, []inventory.ServerPrompt, error) {
	if !hasStaticConfig(cfg) {
		return tools, nil, nil, nil
	}

	b := inventory.NewBuilder().
		SetTools(tools).
		WithReadOnly(cfg.ReadOnly).
		WithToolsets(github.ResolvedEnabledToolsets(cfg.EnabledToolsets, cfg.EnabledTools))

	if len(cfg.EnabledTools) > 0 {
		b = b.WithTools(github.CleanTools(cfg.EnabledTools))
	}

	if len(cfg.ExcludeTools) > 0 {
		b = b.WithExcludeTools(cfg.ExcludeTools)
	}

	inv, err := b.Build()
	if err != nil {
		return nil, nil, nil, err
	}

	ctx := context.Background()
	return inv.AvailableTools(ctx), inv.AvailableResourceTemplates(ctx), inv.AvailablePrompts(ctx), nil
}

// TestStaticInventoryAppliesHostCapabilities guards against HTTP deployments
// silently getting dotcom behaviour. ServerConfig.Host can point at GHES, where
// semantic issue search 403s, so the static inventory has to classify the host
// rather than fall through to the zero value.
func TestStaticInventoryAppliesHostCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		host            string
		wantDescription string
	}{
		{
			name:            "empty host defaults to dotcom",
			host:            "",
			wantDescription: "semantic",
		},
		{
			name:            "dotcom",
			host:            "https://github.com",
			wantDescription: "semantic",
		},
		{
			name:            "GHES falls back to lexical",
			host:            "https://ghes.example.com",
			wantDescription: "lexical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &ServerConfig{Version: "test", Host: tt.host}
			staticTools, _, _, _ := buildStaticInventory(cfg, translations.NullTranslationHelper)

			var found bool
			for _, st := range staticTools {
				if st.Tool.Name != "search_issues" {
					continue
				}
				found = true
				isSemantic := strings.Contains(st.Tool.Description, "semantic matching")
				if tt.wantDescription == "semantic" {
					assert.True(t, isSemantic, "expected semantic description, got: %s", st.Tool.Description)
				} else {
					assert.False(t, isSemantic, "expected lexical description, got: %s", st.Tool.Description)
				}
			}
			require.True(t, found, "search_issues should be in the static inventory")
		})
	}
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

func TestHTTPToolMinimumProtocolVersion(t *testing.T) {
	apiHost, err := utils.NewAPIHost("https://api.github.com")
	require.NoError(t, err)

	inventoryFactory := func(_ *http.Request) (*inventory.Inventory, error) {
		return inventory.NewBuilder().
			SetTools([]inventory.ServerTool{github.DeleteRepository(translations.NullTranslationHelper)}).
			WithToolsets([]string{"all"}).
			Build()
	}
	handler := NewHTTPMcpHandler(
		context.Background(),
		&ServerConfig{Version: "test"},
		github.BaseDeps{},
		translations.NullTranslationHelper,
		slog.Default(),
		apiHost,
		WithInventoryFactory(inventoryFactory),
		WithScopeFetcher(allScopesFetcher{}),
	)

	router := chi.NewRouter()
	handler.RegisterMiddleware(router)
	handler.RegisterRoutes(router)

	for _, tt := range []struct {
		name                    string
		protocolVersion         string
		elicitationCapabilities map[string]any
		wantDeleteRepoTool      bool
	}{
		{
			name:                    "current protocol with form elicitation includes delete repository",
			protocolVersion:         inventory.ProtocolVersionMultiRoundTrip,
			elicitationCapabilities: map[string]any{"form": map[string]any{}},
			wantDeleteRepoTool:      true,
		},
		{
			name:                    "current protocol with URL-only elicitation hides delete repository",
			protocolVersion:         inventory.ProtocolVersionMultiRoundTrip,
			elicitationCapabilities: map[string]any{"url": map[string]any{}},
		},
		{
			name:            "current protocol without elicitation hides delete repository",
			protocolVersion: inventory.ProtocolVersionMultiRoundTrip,
		},
		{
			name:                    "legacy protocol hides delete repository",
			protocolVersion:         "2025-11-25",
			elicitationCapabilities: map[string]any{"form": map[string]any{}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clientCapabilities := map[string]any{}
			if tt.elicitationCapabilities != nil {
				clientCapabilities["elicitation"] = tt.elicitationCapabilities
			}
			body, err := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "tools/list",
				"params": map[string]any{
					"_meta": map[string]any{
						mcp.MetaKeyProtocolVersion:    tt.protocolVersion,
						mcp.MetaKeyClientCapabilities: clientCapabilities,
						mcp.MetaKeyClientInfo:         map[string]any{"name": "test", "version": "v0.0.1"},
					},
				},
			})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(body)))
			req.Header.Set(headers.ContentTypeHeader, headers.ContentTypeJSON)
			req.Header.Set(headers.AcceptHeader, strings.Join([]string{headers.ContentTypeJSON, headers.ContentTypeEventStream}, ", "))
			req.Header.Set("Mcp-Protocol-Version", tt.protocolVersion)
			req.Header.Set("Mcp-Method", "tools/list")
			req.Header.Set(headers.AuthorizationHeader, strings.Join([]string{"ghs", "test-token"}, "_"))

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			require.Equal(t, http.StatusOK, recorder.Code, "response body: %s", recorder.Body.String())

			var response struct {
				Result struct {
					Tools []struct {
						Name string `json:"name"`
					} `json:"tools"`
				} `json:"result"`
			}
			responseBody := recorder.Body.String()
			for line := range strings.SplitSeq(responseBody, "\n") {
				if data, ok := strings.CutPrefix(line, "data: "); ok {
					responseBody = data
					break
				}
			}
			require.NoError(t, json.Unmarshal([]byte(responseBody), &response))

			toolNames := make([]string, 0, len(response.Result.Tools))
			for _, tool := range response.Result.Tools {
				toolNames = append(toolNames, tool.Name)
			}
			if tt.wantDeleteRepoTool {
				assert.Contains(t, toolNames, "delete_repository")
			} else {
				assert.NotContains(t, toolNames, "delete_repository")
			}
		})
	}
}

func TestSubscriptionsListenIsRejected(t *testing.T) {
	apiHost, err := utils.NewAPIHost("https://api.githubcopilot.com")
	require.NoError(t, err)

	handler := NewHTTPMcpHandler(
		context.Background(),
		&ServerConfig{Version: "test"},
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
	)

	body := `{"jsonrpc":"2.0","id":1,"method":"subscriptions/listen","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"test","version":"1.0.0"},"io.modelcontextprotocol/clientCapabilities":{}},"notifications":{"toolsListChanged":true}}}`
	tests := []struct {
		name             string
		methodHeader     string
		expectedStatus   int
		expectedJSONCode int
	}{
		{
			name:             "matching method header",
			methodHeader:     subscriptionsListenMethod,
			expectedStatus:   http.StatusNotFound,
			expectedJSONCode: jsonrpc.CodeMethodNotFound,
		},
		{
			name:             "missing method header",
			expectedStatus:   http.StatusBadRequest,
			expectedJSONCode: mcp.CodeHeaderMismatch,
		},
		{
			name:             "mismatched method header",
			methodHeader:     "tools/list",
			expectedStatus:   http.StatusBadRequest,
			expectedJSONCode: mcp.CodeHeaderMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			req.Header.Set(headers.ContentTypeHeader, headers.ContentTypeJSON)
			req.Header.Set(headers.AcceptHeader, strings.Join([]string{headers.ContentTypeJSON, headers.ContentTypeEventStream}, ", "))
			req.Header.Set("MCP-Protocol-Version", "2026-07-28")
			if tt.methodHeader != "" {
				req.Header.Set(headers.MCPMethodHeader, tt.methodHeader)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			assert.Equal(t, headers.ContentTypeJSON, rr.Header().Get(headers.ContentTypeHeader))

			var response struct {
				ID    int `json:"id"`
				Error struct {
					Code int `json:"code"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
			assert.Equal(t, 1, response.ID)
			assert.Equal(t, tt.expectedJSONCode, response.Error.Code)
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

	checker := createHTTPFeatureChecker(nil, false)
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

// TestUIMetaStrippedWhenClientLacksCapability verifies that even on the
// /insiders path (where the feature flag is on), UI metadata is stripped from
// tools/list responses when the client did NOT advertise the
// io.modelcontextprotocol/ui extension capability. Per the 2026-01-26 MCP
// Apps spec, servers SHOULD check client capabilities before exposing
// UI-enabled tools.
func TestUIMetaStrippedWhenClientLacksCapability(t *testing.T) {
	const uiURI = "ui://test/widget"
	uiTool := mockTool("with_ui", "repos", true)
	uiTool.Tool.Meta = mcp.Meta{"ui": map[string]any{"resourceUri": uiURI}}

	checker := createHTTPFeatureChecker(nil, false)
	build := func() *inventory.Inventory {
		inv, err := inventory.NewBuilder().
			SetTools([]inventory.ServerTool{uiTool}).
			WithFeatureChecker(checker).
			WithToolsets([]string{"all"}).
			Build()
		require.NoError(t, err)
		return inv
	}

	insidersCtx := ghcontext.WithInsidersMode(context.Background(), true)
	withoutUICap := ghcontext.WithUISupport(insidersCtx, false)
	withUICap := ghcontext.WithUISupport(insidersCtx, true)

	stripped := build().ToolsForRegistration(withoutUICap)
	require.Len(t, stripped, 1)
	require.Nil(t, stripped[0].Tool.Meta["ui"], "_meta.ui should be stripped when client lacks UI capability")

	preserved := build().ToolsForRegistration(withUICap)
	require.Len(t, preserved, 1)
	require.NotNil(t, preserved[0].Tool.Meta["ui"], "_meta.ui should be preserved when client advertises UI capability")
	require.Equal(t, uiURI, preserved[0].Tool.Meta["ui"].(map[string]any)["resourceUri"])

	// Unknown capability falls through to the FF gate (insiders ctx → kept).
	unknown := build().ToolsForRegistration(insidersCtx)
	require.Len(t, unknown, 1)
	require.NotNil(t, unknown[0].Tool.Meta["ui"], "_meta.ui should be preserved when capability is unknown and FF is on")
}

// TestMaxRequestBodyBytes checks the effective limit and, critically, that it
// sits above the MCP SDK default — which is why it must also be passed to
// StreamableHTTPOptions rather than left to the SDK.
func TestMaxRequestBodyBytes(t *testing.T) {
	t.Run("default leaves headroom above the MCP SDK limit", func(t *testing.T) {
		h := &Handler{config: &ServerConfig{}}

		assert.Equal(t, int64(5<<20), h.maxRequestBodyBytes())
		assert.Greater(t, h.maxRequestBodyBytes(), int64(mcp.DefaultMaxRequestBodyBytes),
			"the default intentionally exceeds the SDK limit, so the SDK must be told about it")
	})

	t.Run("configured value overrides the default", func(t *testing.T) {
		h := &Handler{config: &ServerConfig{MaxRequestBodyBytes: 1234}}
		assert.Equal(t, int64(1234), h.maxRequestBodyBytes())
	})
}

// TestMaxRequestBodySizeEnforcement exercises both layers the limit is applied
// at: the early middleware, and the MCP SDK handler the request is delegated to.
func TestMaxRequestBodySizeEnforcement(t *testing.T) {
	const limit = 256

	apiHost, err := utils.NewAPIHost("https://api.github.com")
	require.NoError(t, err)

	buildBody := func(size int) string {
		payload := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"pad":"PADDING"}}`
		if len(payload) >= size {
			return payload
		}
		pad := strings.Repeat("x", size-len(payload))
		return strings.Replace(payload, "PADDING", "PADDING"+pad, 1)
	}

	newHandler := func(t *testing.T, maxBytes int64, mcpServerFactoryCalled *bool) *Handler {
		t.Helper()
		return NewHTTPMcpHandler(
			context.Background(),
			&ServerConfig{Version: "test", MaxRequestBodyBytes: maxBytes},
			nil,
			translations.NullTranslationHelper,
			slog.Default(),
			apiHost,
			WithInventoryFactory(func(_ *http.Request) (*inventory.Inventory, error) {
				return inventory.NewBuilder().Build()
			}),
			WithGitHubMCPServerFactory(func(_ *http.Request, _ github.ToolDependencies, _ *inventory.Inventory, _ *github.MCPServerConfig) (*mcp.Server, error) {
				if mcpServerFactoryCalled != nil {
					*mcpServerFactoryCalled = true
				}
				return mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil), nil
			}),
			WithScopeFetcher(allScopesFetcher{}),
		)
	}

	newRouter := func(h *Handler) http.Handler {
		r := chi.NewRouter()
		h.RegisterMiddleware(r)
		h.RegisterRoutes(r)
		return r
	}

	newRequest := func(body string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set(headers.ContentTypeHeader, headers.ContentTypeJSON)
		req.Header.Set(headers.AcceptHeader, strings.Join([]string{headers.ContentTypeJSON, headers.ContentTypeEventStream}, ", "))
		req.Header.Set(headers.AuthorizationHeader, strings.Join([]string{"ghs", "test-token"}, "_"))
		return req
	}

	t.Run("middleware rejects an oversized request before the MCP server is built", func(t *testing.T) {
		var mcpServerFactoryCalled bool
		r := newRouter(newHandler(t, limit, &mcpServerFactoryCalled))

		body := buildBody(limit + 1)
		require.Greater(t, len(body), limit)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, newRequest(body))

		assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
		assert.Contains(t, rr.Body.String(), "request body too large")
		assert.False(t, mcpServerFactoryCalled, "the MCP server should never be constructed for an oversized request")
	})

	t.Run("request at the configured limit succeeds", func(t *testing.T) {
		var mcpServerFactoryCalled bool
		r := newRouter(newHandler(t, limit, &mcpServerFactoryCalled))

		body := buildBody(limit)
		require.Len(t, body, limit)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, newRequest(body))

		assert.Equal(t, http.StatusOK, rr.Code, "response body: %s", rr.Body.String())
		assert.True(t, mcpServerFactoryCalled, "the MCP server should be constructed for an allowed request")
	})

	t.Run("SDK handler enforces the configured limit when the middleware is bypassed", func(t *testing.T) {
		h := newHandler(t, limit, nil)

		body := buildBody(limit + 1)
		require.Greater(t, len(body), limit)

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, newRequest(body))

		assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
		assert.Contains(t, rr.Body.String(), fmt.Sprintf("request body exceeds %d bytes", limit),
			"the SDK should report the configured limit, not its own default")
	})

	// The default headroom only exists if it reaches the SDK as well; leaving
	// the SDK on its own default would silently reject this request.
	t.Run("unconfigured handler accepts a request above the MCP SDK limit", func(t *testing.T) {
		var mcpServerFactoryCalled bool
		r := newRouter(newHandler(t, 0, &mcpServerFactoryCalled))

		body := buildBody(mcp.DefaultMaxRequestBodyBytes + 1024)
		require.Greater(t, int64(len(body)), int64(mcp.DefaultMaxRequestBodyBytes))
		require.Less(t, int64(len(body)), middleware.DefaultMaxRequestBodyBytes)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, newRequest(body))

		assert.Equal(t, http.StatusOK, rr.Code, "response body: %s", rr.Body.String())
		assert.True(t, mcpServerFactoryCalled, "the MCP server should be constructed for an allowed request")
	})
}
