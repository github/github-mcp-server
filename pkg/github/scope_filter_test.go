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
	// Create test tools with various scope requirements.
	// RequiredScopes is the single source of truth for filtering.
	toolNoScopes := &inventory.ServerTool{
		Tool:           mcp.Tool{Name: "no_scopes_tool"},
		RequiredScopes: nil,
	}

	toolEmptyScopes := &inventory.ServerTool{
		Tool:           mcp.Tool{Name: "empty_scopes_tool"},
		RequiredScopes: []string{},
	}

	toolRepoScope := &inventory.ServerTool{
		Tool:           mcp.Tool{Name: "repo_tool"},
		RequiredScopes: []string{"repo"},
	}

	toolRepoScopeReadOnly := &inventory.ServerTool{
		Tool: mcp.Tool{
			Name:        "repo_tool_readonly",
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		},
		RequiredScopes: []string{"repo"},
	}

	toolPublicRepoScope := &inventory.ServerTool{
		Tool:           mcp.Tool{Name: "public_repo_tool"},
		RequiredScopes: []string{"public_repo"},
	}

	toolPublicRepoScopeReadOnly := &inventory.ServerTool{
		Tool: mcp.Tool{
			Name:        "public_repo_tool_readonly",
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		},
		RequiredScopes: []string{"public_repo"},
	}

	toolGistScope := &inventory.ServerTool{
		Tool:           mcp.Tool{Name: "gist_tool"},
		RequiredScopes: []string{"gist"},
	}

	// Models ui_get / list_issue_fields: read-only, but requires repo AND read:org.
	toolRepoAndReadOrg := &inventory.ServerTool{
		Tool: mcp.Tool{
			Name:        "ui_get",
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		},
		RequiredScopes: []string{"repo", "read:org"},
	}

	// Models security tools (code scanning etc.): read-only, single {security_events}.
	toolSecurityEvents := &inventory.ServerTool{
		Tool: mcp.Tool{
			Name:        "list_code_scanning_alerts",
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		},
		RequiredScopes: []string{"security_events"},
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
		{
			name:        "AND: repo-only classic PAT hides {repo, read:org} tool",
			tokenScopes: []string{"repo"},
			tool:        toolRepoAndReadOrg,
			expected:    false,
		},
		{
			name:        "AND: {repo, read:org} token shows {repo, read:org} tool",
			tokenScopes: []string{"repo", "read:org"},
			tool:        toolRepoAndReadOrg,
			expected:    true,
		},
		{
			name:        "AND: {repo, admin:org} token shows {repo, read:org} tool via hierarchy",
			tokenScopes: []string{"repo", "admin:org"},
			tool:        toolRepoAndReadOrg,
			expected:    true,
		},
		{
			name:        "security tool: repo token satisfies security_events (neutral)",
			tokenScopes: []string{"repo"},
			tool:        toolSecurityEvents,
			expected:    true,
		},
		{
			name:        "security tool: security_events token shows tool",
			tokenScopes: []string{"security_events"},
			tool:        toolSecurityEvents,
			expected:    true,
		},
		{
			name:        "security tool: public_repo (sibling) does not show tool",
			tokenScopes: []string{"public_repo"},
			tool:        toolSecurityEvents,
			expected:    false,
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
			RequiredScopes: nil, // No scopes required
		},
		{
			Tool:           mcp.Tool{Name: "repo_tool"},
			Toolset:        inventory.ToolsetMetadata{ID: "test"},
			RequiredScopes: []string{"repo"},
		},
		{
			Tool:           mcp.Tool{Name: "gist_tool"},
			Toolset:        inventory.ToolsetMetadata{ID: "test"},
			RequiredScopes: []string{"gist"},
		},
		{
			// Requires repo AND read:org; hidden for a {repo}-only token.
			Tool:           mcp.Tool{Name: "list_issue_fields"},
			Toolset:        inventory.ToolsetMetadata{ID: "test"},
			RequiredScopes: []string{"repo", "read:org"},
		},
	}

	// Create filter for token with only "repo" scope
	filter := CreateToolScopeFilter([]string{"repo"})

	// Build inventory with the filter
	inv, err := inventory.NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"test"}).
		WithFilter(filter).
		Build()
	require.NoError(t, err)

	// Get available tools
	availableTools := inv.AvailableTools(context.Background())

	// Should see public_tool and repo_tool, but not gist_tool or list_issue_fields
	assert.Len(t, availableTools, 2)

	toolNames := make([]string, len(availableTools))
	for i, tool := range availableTools {
		toolNames[i] = tool.Tool.Name
	}

	assert.Contains(t, toolNames, "public_tool")
	assert.Contains(t, toolNames, "repo_tool")
	assert.NotContains(t, toolNames, "gist_tool")
	assert.NotContains(t, toolNames, "list_issue_fields")
}
