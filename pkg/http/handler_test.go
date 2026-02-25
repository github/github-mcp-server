package http

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
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
	return inventory.ServerTool{
		Tool: mcp.Tool{
			Name:        name,
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly},
		},
		Toolset: inventory.ToolsetMetadata{
			ID:          inventory.ToolsetID(toolsetID),
			Description: "Test: " + toolsetID,
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
