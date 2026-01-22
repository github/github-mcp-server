package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
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
		headers       map[string]string
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
			name: "context toolset takes precedence over header",
			contextSetup: func(ctx context.Context) context.Context {
				return ghcontext.WithToolsets(ctx, []string{"repos"})
			},
			headers: map[string]string{
				headers.MCPToolsetsHeader: "issues",
			},
			expectedTools: []string{"get_file_contents", "create_repository"},
		},
		{
			name: "tools are additive with toolsets",
			contextSetup: func(ctx context.Context) context.Context {
				return ghcontext.WithToolsets(ctx, []string{"repos"})
			},
			headers: map[string]string{
				headers.MCPToolsHeader: "list_issues",
			},
			expectedTools: []string{"get_file_contents", "create_repository", "list_issues"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			req = req.WithContext(tt.contextSetup(req.Context()))

			builder := inventory.NewBuilder().
				SetTools(tools).
				WithToolsets([]string{"all"})

			builder = InventoryFiltersForRequest(req, builder)
			inv := builder.Build()

			available := inv.AvailableTools(context.Background())
			toolNames := make([]string, len(available))
			for i, tool := range available {
				toolNames[i] = tool.Tool.Name
			}

			assert.ElementsMatch(t, tt.expectedTools, toolNames)
		})
	}
}
