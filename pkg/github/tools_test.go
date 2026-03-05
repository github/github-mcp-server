package github

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddDefaultToolset(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no default keyword - return unchanged",
			input:    []string{"actions", "gists"},
			expected: []string{"actions", "gists"},
		},
		{
			name:  "default keyword present - expand and remove default",
			input: []string{"default"},
			expected: []string{
				"context",
				"copilot",
				"repos",
				"issues",
				"pull_requests",
				"users",
			},
		},
		{
			name:  "default with additional toolsets",
			input: []string{"default", "actions", "gists"},
			expected: []string{
				"actions",
				"gists",
				"context",
				"copilot",
				"repos",
				"issues",
				"pull_requests",
				"users",
			},
		},
		{
			name:  "default with overlapping toolsets - should not duplicate",
			input: []string{"default", "context", "repos"},
			expected: []string{
				"context",
				"copilot",
				"repos",
				"issues",
				"pull_requests",
				"users",
			},
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AddDefaultToolset(tt.input)

			require.Len(t, result, len(tt.expected), "result length should match expected length")

			resultMap := make(map[string]bool)
			for _, toolset := range result {
				resultMap[toolset] = true
			}

			expectedMap := make(map[string]bool)
			for _, toolset := range tt.expected {
				expectedMap[toolset] = true
			}

			assert.Equal(t, expectedMap, resultMap, "result should contain all expected toolsets")
			assert.False(t, resultMap["default"], "result should not contain 'default' keyword")
		})
	}
}

func TestRemoveToolset(t *testing.T) {
	tests := []struct {
		name     string
		tools    []string
		toRemove string
		expected []string
	}{
		{
			name:     "remove existing toolset",
			tools:    []string{"actions", "gists", "notifications"},
			toRemove: "gists",
			expected: []string{"actions", "notifications"},
		},
		{
			name:     "remove from empty slice",
			tools:    []string{},
			toRemove: "actions",
			expected: []string{},
		},
		{
			name:     "remove duplicate entries",
			tools:    []string{"actions", "gists", "actions", "notifications"},
			toRemove: "actions",
			expected: []string{"gists", "notifications"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveToolset(tt.tools, tt.toRemove)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsToolset(t *testing.T) {
	tests := []struct {
		name     string
		tools    []string
		toCheck  string
		expected bool
	}{
		{
			name:     "toolset exists",
			tools:    []string{"actions", "gists", "notifications"},
			toCheck:  "gists",
			expected: true,
		},
		{
			name:     "toolset does not exist",
			tools:    []string{"actions", "gists", "notifications"},
			toCheck:  "repos",
			expected: false,
		},
		{
			name:     "empty slice",
			tools:    []string{},
			toCheck:  "actions",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsToolset(tt.tools, tt.toCheck)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateToolsetsHelp(t *testing.T) {
	// Generate the help text
	helpText := GenerateToolsetsHelp()

	// Verify help text is not empty
	require.NotEmpty(t, helpText)

	// Verify it contains expected sections
	assert.Contains(t, helpText, "Comma-separated list of tool groups to enable")
	assert.Contains(t, helpText, "Available:")
	assert.Contains(t, helpText, "Special toolset keywords:")
	assert.Contains(t, helpText, "all: Enables all available toolsets")
	assert.Contains(t, helpText, "default: Enables the default toolset configuration")
	assert.Contains(t, helpText, "Examples:")
	assert.Contains(t, helpText, "--toolsets=actions,gists,notifications")
	assert.Contains(t, helpText, "--toolsets=default,actions,gists")
	assert.Contains(t, helpText, "--toolsets=all")

	// Verify it contains some expected default toolsets
	assert.Contains(t, helpText, "context")
	assert.Contains(t, helpText, "repos")
	assert.Contains(t, helpText, "issues")
	assert.Contains(t, helpText, "pull_requests")
	assert.Contains(t, helpText, "users")

	// Verify it contains some expected available toolsets
	assert.Contains(t, helpText, "actions")
	assert.Contains(t, helpText, "gists")
	assert.Contains(t, helpText, "notifications")
}

// --- ParseToolsetModes tests ---

func TestParseToolsetModes_MixedModes(t *testing.T) {
	names, readOnly := ParseToolsetModes([]string{"repos:rw", "issues:ro", "users"}, nil)

	require.Equal(t, []string{"repos", "issues", "users"}, names)
	require.False(t, readOnly["repos"], "repos should be rw")
	require.True(t, readOnly["issues"], "issues should be ro")
	require.False(t, readOnly["users"], "users (no suffix) should be rw")
}

func TestParseToolsetModes_AllRo(t *testing.T) {
	knownToolsets := []inventory.ToolsetID{"repos", "issues", "users"}
	names, readOnly := ParseToolsetModes([]string{"all:ro"}, knownToolsets)

	require.Equal(t, []string{"all"}, names, "all:ro should produce 'all' as the name")
	require.True(t, readOnly["repos"], "repos should be read-only after all:ro expansion")
	require.True(t, readOnly["issues"], "issues should be read-only after all:ro expansion")
	require.True(t, readOnly["users"], "users should be read-only after all:ro expansion")
}

func TestParseToolsetModes_NoModes(t *testing.T) {
	names, readOnly := ParseToolsetModes([]string{"repos", "issues", "users"}, nil)

	require.Equal(t, []string{"repos", "issues", "users"}, names)
	require.Empty(t, readOnly, "no toolsets should be read-only when no modes specified")
}

func TestParseToolsetModes_CaseInsensitive(t *testing.T) {
	names, readOnly := ParseToolsetModes([]string{"repos:RO", "issues:RW", "users:ReadOnly"}, nil)

	require.Equal(t, []string{"repos", "issues", "users"}, names)
	require.True(t, readOnly["repos"], "repos:RO should be read-only (case-insensitive)")
	require.False(t, readOnly["issues"], "issues:RW should be rw")
	require.True(t, readOnly["users"], "users:ReadOnly should be read-only (case-insensitive)")
}

func TestParseToolsetModes_UnknownSuffix_TreatedAsName(t *testing.T) {
	// Unknown suffix → treat entire string as name (backwards compat)
	names, readOnly := ParseToolsetModes([]string{"repos:unknown"}, nil)

	require.Equal(t, []string{"repos:unknown"}, names, "unknown suffix should be kept as part of name")
	require.Empty(t, readOnly)
}

func TestParseToolsetModes_EmptyInput(t *testing.T) {
	names, readOnly := ParseToolsetModes([]string{}, nil)

	require.Empty(t, names)
	require.Empty(t, readOnly)
}

func TestParseToolsetModes_NilInput(t *testing.T) {
	names, modes := ParseToolsetModes(nil, nil)
	if names != nil {
		t.Errorf("expected nil names for nil input, got %v", names)
	}
	if modes != nil {
		t.Errorf("expected nil modes for nil input, got %v", modes)
	}
}

func TestParseToolsetModes_AllRoWithRwException(t *testing.T) {
	allToolsets := []inventory.ToolsetID{"repos", "issues", "pull_requests", "users"}
	names, readOnly := ParseToolsetModes([]string{"all:ro", "repos:rw"}, allToolsets)

	require.Equal(t, []string{"all", "repos"}, names)
	// repos should NOT be in readOnly because :rw overrides the prior all:ro
	require.False(t, readOnly[inventory.ToolsetID("repos")],
		"repos:rw should override all:ro for repos")
	// Other toolsets should still be read-only
	require.True(t, readOnly[inventory.ToolsetID("issues")])
	require.True(t, readOnly[inventory.ToolsetID("pull_requests")])
	require.True(t, readOnly[inventory.ToolsetID("users")])
}

func TestParseToolsetModes_AllRoWithRwException_ReversedOrder(t *testing.T) {
	// Order matters: "repos:rw,all:ro" → repos ends up read-only because all:ro
	// runs after the rw delete. This is the documented behavior.
	allToolsets := []inventory.ToolsetID{"repos", "issues", "pull_requests"}
	names, readOnly := ParseToolsetModes([]string{"repos:rw", "all:ro"}, allToolsets)

	require.Equal(t, []string{"repos", "all"}, names)
	// repos IS read-only because all:ro processed last
	require.True(t, readOnly[inventory.ToolsetID("repos")],
		"reversed order: all:ro after repos:rw should make repos read-only")
	require.True(t, readOnly[inventory.ToolsetID("issues")])
	require.True(t, readOnly[inventory.ToolsetID("pull_requests")])
}

func TestParseToolsetModes_RwWithoutPriorRo(t *testing.T) {
	// :rw on a toolset that was never marked :ro should be a no-op (no crash)
	names, readOnly := ParseToolsetModes([]string{"repos:rw", "issues:ro"}, nil)

	require.Equal(t, []string{"repos", "issues"}, names)
	require.False(t, readOnly[inventory.ToolsetID("repos")])
	require.True(t, readOnly[inventory.ToolsetID("issues")])
}
