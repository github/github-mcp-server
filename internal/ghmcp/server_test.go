package ghmcp

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformSpecialToolsets(t *testing.T) {
	tests := []struct {
		name            string
		input           []string
		dynamicToolsets bool
		expected        []string
	}{
		{
			name:            "empty slice",
			input:           []string{},
			dynamicToolsets: false,
			expected:        []string{},
		},
		{
			name:            "nil input slice",
			input:           nil,
			dynamicToolsets: false,
			expected:        []string{},
		},
		// all test cases
		{
			name:            "all only",
			input:           []string{"all"},
			dynamicToolsets: false,
			expected:        []string{"all"},
		},
		{
			name:            "all appears multiple times",
			input:           []string{"all", "actions", "all"},
			dynamicToolsets: false,
			expected:        []string{"all"},
		},
		{
			name:            "all with other toolsets",
			input:           []string{"all", "actions", "gists"},
			dynamicToolsets: false,
			expected:        []string{"all"},
		},
		{
			name:            "all with default",
			input:           []string{"default", "all", "actions"},
			dynamicToolsets: false,
			expected:        []string{"all"},
		},
		// default test cases
		{
			name:            "default only",
			input:           []string{"default"},
			dynamicToolsets: false,
			expected: []string{
				"context",
				"repos",
				"issues",
				"pull_requests",
				"users",
			},
		},
		{
			name:            "default with additional toolsets",
			input:           []string{"default", "actions", "gists"},
			dynamicToolsets: false,
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
			name:            "no default present",
			input:           []string{"actions", "gists", "notifications"},
			dynamicToolsets: false,
			expected:        []string{"actions", "gists", "notifications"},
		},
		{
			name:            "duplicate toolsets without default",
			input:           []string{"actions", "gists", "actions"},
			dynamicToolsets: false,
			expected:        []string{"actions", "gists"},
		},
		{
			name:            "duplicate toolsets with default",
			input:           []string{"context", "repos", "issues", "pull_requests", "users", "default"},
			dynamicToolsets: false,
			expected: []string{
				"context",
				"repos",
				"issues",
				"pull_requests",
				"users",
			},
		},
		{
			name:            "default appears multiple times with different toolsets in between",
			input:           []string{"default", "actions", "default", "gists", "default"},
			dynamicToolsets: false,
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
		// Dynamic toolsets test cases
		{
			name:            "dynamic toolsets - all only should be filtered",
			input:           []string{"all"},
			dynamicToolsets: true,
			expected:        []string{},
		},
		{
			name:            "dynamic toolsets - all with other toolsets",
			input:           []string{"all", "actions", "gists"},
			dynamicToolsets: true,
			expected:        []string{"actions", "gists"},
		},
		{
			name:            "dynamic toolsets - all with default",
			input:           []string{"all", "default", "actions"},
			dynamicToolsets: true,
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
			name:            "dynamic toolsets - no all present",
			input:           []string{"actions", "gists"},
			dynamicToolsets: true,
			expected:        []string{"actions", "gists"},
		},
		{
			name:            "dynamic toolsets - default only",
			input:           []string{"default"},
			dynamicToolsets: true,
			expected: []string{
				"context",
				"repos",
				"issues",
				"pull_requests",
				"users",
			},
		},
		{
			name:            "only special keywords with dynamic mode",
			input:           []string{"all", "default"},
			dynamicToolsets: true,
			expected: []string{
				"context",
				"repos",
				"issues",
				"pull_requests",
				"users",
			},
		},
		{
			name:            "all with default and overlapping default toolsets in dynamic mode",
			input:           []string{"all", "default", "issues", "repos"},
			dynamicToolsets: true,
			expected: []string{
				"issues",
				"repos",
				"context",
				"pull_requests",
				"users",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanToolsets(tt.input, tt.dynamicToolsets)

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

func TestTransformSpecialToolsetsWithActualDefaults(t *testing.T) {
	// This test verifies that the function uses the actual default toolsets from GetDefaultToolsetIDs()
	input := []string{"default"}
	result := cleanToolsets(input, false)

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
