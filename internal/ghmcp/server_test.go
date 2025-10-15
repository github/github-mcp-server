package ghmcp

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:  "default only",
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
			name:  "default with overlapping toolsets",
			input: []string{"default", "issues", "actions"},
			expected: []string{
				"issues",
				"actions",
				"context",
				"repos",
				"pull_requests",
				"users",
			},
		},
		{
			name:     "no default present",
			input:    []string{"actions", "gists", "notifications"},
			expected: []string{"actions", "gists", "notifications"},
		},
		{
			name:     "duplicate toolsets without default",
			input:    []string{"actions", "gists", "actions"},
			expected: []string{"actions", "gists"},
		},
		{
			name:  "duplicate toolsets with default",
			input: []string{"actions", "default", "actions", "issues"},
			expected: []string{
				"actions",
				"issues",
				"context",
				"repos",
				"pull_requests",
				"users",
			},
		},
		{
			name:  "multiple defaults (edge case)",
			input: []string{"default", "actions", "default"},
			expected: []string{
				"actions",
				"context",
				"repos",
				"issues",
				"pull_requests",
				"users",
			},
		},
		{
			name:  "all default toolsets already present with default",
			input: []string{"context", "repos", "issues", "pull_requests", "users", "default"},
			expected: []string{
				"context",
				"repos",
				"issues",
				"pull_requests",
				"users",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformDefault(tt.input)

			// Check that the result has the correct length
			require.Len(t, result, len(tt.expected), "result length should match expected length")

			// Create a map for easier comparison since order might vary
			resultMap := make(map[string]bool)
			for _, toolset := range result {
				resultMap[toolset] = true
			}

			expectedMap := make(map[string]bool)
			for _, toolset := range tt.expected {
				expectedMap[toolset] = true
			}

			// Check that both maps contain the same toolsets
			assert.Equal(t, expectedMap, resultMap, "result should contain all expected toolsets without duplicates")

			// Verify no duplicates in result
			assert.Len(t, resultMap, len(result), "result should not contain duplicates")

			// Verify "default" is not in the result
			assert.False(t, resultMap["default"], "result should not contain 'default'")
		})
	}
}

func TestTransformDefaultWithActualDefaults(t *testing.T) {
	// This test verifies that the function uses the actual default toolsets from GetDefaultToolsetIDs()
	input := []string{"default"}
	result := transformDefault(input)

	defaultToolsets := github.GetDefaultToolsetIDs()

	// Check that result contains all default toolsets
	require.Len(t, result, len(defaultToolsets), "result should contain all default toolsets")

	resultMap := make(map[string]bool)
	for _, toolset := range result {
		resultMap[toolset] = true
	}

	for _, defaultToolset := range defaultToolsets {
		assert.True(t, resultMap[defaultToolset], "result should contain default toolset: %s", defaultToolset)
	}

	// Verify "default" is not in the result
	assert.False(t, resultMap["default"], "result should not contain 'default'")
}
