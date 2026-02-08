package github

import (
	"testing"

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
