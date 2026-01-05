package github

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateToolScopeFilter(t *testing.T) {
	// Create test tools with various scope requirements
	toolNoScopes := &inventory.ServerTool{
		Tool:           mcp.Tool{Name: "no_scopes_tool"},
		AcceptedScopes: nil,
	}

	toolEmptyScopes := &inventory.ServerTool{
		Tool:           mcp.Tool{Name: "empty_scopes_tool"},
		AcceptedScopes: []string{},
	}

	toolRepoScope := &inventory.ServerTool{
		Tool:           mcp.Tool{Name: "repo_tool"},
		AcceptedScopes: []string{"repo"},
	}

	toolRepoScopeReadOnly := &inventory.ServerTool{
		Tool: mcp.Tool{
			Name:        "repo_tool_readonly",
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		},
		AcceptedScopes: []string{"repo"},
	}

	toolPublicRepoScope := &inventory.ServerTool{
		Tool:           mcp.Tool{Name: "public_repo_tool"},
		AcceptedScopes: []string{"public_repo", "repo"}, // repo is parent, also accepted
	}

	toolPublicRepoScopeReadOnly := &inventory.ServerTool{
		Tool: mcp.Tool{
			Name:        "public_repo_tool_readonly",
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		},
		AcceptedScopes: []string{"public_repo", "repo"},
	}

	toolGistScope := &inventory.ServerTool{
		Tool:           mcp.Tool{Name: "gist_tool"},
		AcceptedScopes: []string{"gist"},
	}

	toolMultiScope := &inventory.ServerTool{
		Tool:           mcp.Tool{Name: "multi_scope_tool"},
		AcceptedScopes: []string{"repo", "admin:org"},
	}

	tests := []struct {
		name        string
		tokenScopes []string
		tool        *inventory.ServerTool
		expected    bool
	}{
		{
			name:        "tool with no scopes is always visible",
			tokenScopes: []string{},
			tool:        toolNoScopes,
			expected:    true,
		},
		{
			name:        "tool with empty scopes is always visible",
			tokenScopes: []string{"repo"},
			tool:        toolEmptyScopes,
			expected:    true,
		},
		{
			name:        "token with exact scope can see tool",
			tokenScopes: []string{"repo"},
			tool:        toolRepoScope,
			expected:    true,
		},
		{
			name:        "token with parent scope can see child-scoped tool",
			tokenScopes: []string{"repo"},
			tool:        toolPublicRepoScope,
			expected:    true,
		},
		{
			name:        "token missing required scope cannot see tool",
			tokenScopes: []string{"gist"},
			tool:        toolRepoScope,
			expected:    false,
		},
		{
			name:        "token with unrelated scope cannot see tool",
			tokenScopes: []string{"repo"},
			tool:        toolGistScope,
			expected:    false,
		},
		{
			name:        "token with one of multiple accepted scopes can see tool",
			tokenScopes: []string{"admin:org"},
			tool:        toolMultiScope,
			expected:    true,
		},
		{
			name:        "empty token scopes cannot see scoped tools",
			tokenScopes: []string{},
			tool:        toolRepoScope,
			expected:    false,
		},
		{
			name:        "empty token scopes CAN see read-only repo tools (public repos)",
			tokenScopes: []string{},
			tool:        toolRepoScopeReadOnly,
			expected:    true,
		},
		{
			name:        "empty token scopes CAN see read-only public_repo tools",
			tokenScopes: []string{},
			tool:        toolPublicRepoScopeReadOnly,
			expected:    true,
		},
		{
			name:        "token with multiple scopes where one matches",
			tokenScopes: []string{"gist", "repo"},
			tool:        toolPublicRepoScope,
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := CreateToolScopeFilter(tt.tokenScopes)
			result, err := filter(context.Background(), tt.tool)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result, "filter result should match expected")
		})
	}
}

func TestCreateToolScopeFilter_Integration(t *testing.T) {
	// Test integration with inventory builder
	tools := []inventory.ServerTool{
		{
			Tool:           mcp.Tool{Name: "public_tool"},
			Toolset:        inventory.ToolsetMetadata{ID: "test"},
			AcceptedScopes: nil, // No scopes required
		},
		{
			Tool:           mcp.Tool{Name: "repo_tool"},
			Toolset:        inventory.ToolsetMetadata{ID: "test"},
			AcceptedScopes: []string{"repo"},
		},
		{
			Tool:           mcp.Tool{Name: "gist_tool"},
			Toolset:        inventory.ToolsetMetadata{ID: "test"},
			AcceptedScopes: []string{"gist"},
		},
	}

	// Create filter for token with only "repo" scope
	filter := CreateToolScopeFilter([]string{"repo"})

	// Build inventory with the filter
	inv := inventory.NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"test"}).
		WithFilter(filter).
		Build()

	// Get available tools
	availableTools := inv.AvailableTools(context.Background())

	// Should see public_tool and repo_tool, but not gist_tool
	assert.Len(t, availableTools, 2)

	toolNames := make([]string, len(availableTools))
	for i, tool := range availableTools {
		toolNames[i] = tool.Tool.Name
	}

	assert.Contains(t, toolNames, "public_tool")
	assert.Contains(t, toolNames, "repo_tool")
	assert.NotContains(t, toolNames, "gist_tool")
}
