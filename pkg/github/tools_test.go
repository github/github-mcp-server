package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanToolsets(t *testing.T) {
	tests := []struct {
		name            string
		input           []string
		expected        []string
		expectedInvalid []string
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "nil input slice",
			input:    nil,
			expected: []string{},
		},
		// CleanToolsets only cleans - it does NOT filter out special keywords
		{
			name:     "default keyword preserved",
			input:    []string{"default"},
			expected: []string{"default"},
		},
		{
			name:     "default with additional toolsets",
			input:    []string{"default", "actions", "gists"},
			expected: []string{"default", "actions", "gists"},
		},
		{
			name:     "all keyword preserved",
			input:    []string{"all", "actions"},
			expected: []string{"all", "actions"},
		},
		{
			name:     "no special keywords",
			input:    []string{"actions", "gists", "notifications"},
			expected: []string{"actions", "gists", "notifications"},
		},
		{
			name:     "duplicate toolsets without special keywords",
			input:    []string{"actions", "gists", "actions"},
			expected: []string{"actions", "gists"},
		},
		{
			name:     "duplicate toolsets with default",
			input:    []string{"context", "repos", "issues", "pull_requests", "users", "default"},
			expected: []string{"context", "repos", "issues", "pull_requests", "users", "default"},
		},
		{
			name:     "default appears multiple times - duplicates removed",
			input:    []string{"default", "actions", "default", "gists", "default"},
			expected: []string{"default", "actions", "gists"},
		},
		// Whitespace test cases
		{
			name:     "whitespace check - leading and trailing whitespace on regular toolsets",
			input:    []string{" actions ", "  gists  ", "notifications"},
			expected: []string{"actions", "gists", "notifications"},
		},
		{
			name:     "whitespace check - default toolset with whitespace",
			input:    []string{" actions ", "  default  ", "notifications"},
			expected: []string{"actions", "default", "notifications"},
		},
		{
			name:     "whitespace check - all toolset with whitespace",
			input:    []string{" all ", "  actions  "},
			expected: []string{"all", "actions"},
		},
		// Invalid toolset test cases
		{
			name:            "mix of valid and invalid toolsets",
			input:           []string{"actions", "invalid_toolset", "gists", "typo_repo"},
			expected:        []string{"actions", "invalid_toolset", "gists", "typo_repo"},
			expectedInvalid: []string{"invalid_toolset", "typo_repo"},
		},
		{
			name:            "invalid with whitespace",
			input:           []string{" invalid_tool ", "  actions  ", " typo_gist "},
			expected:        []string{"invalid_tool", "actions", "typo_gist"},
			expectedInvalid: []string{"invalid_tool", "typo_gist"},
		},
		{
			name:            "empty string in toolsets",
			input:           []string{"", "actions", "  ", "gists"},
			expected:        []string{"actions", "gists"},
			expectedInvalid: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, invalid := CleanToolsets(tt.input)

			require.Len(t, result, len(tt.expected), "result length should match expected length")

			if tt.expectedInvalid == nil {
				tt.expectedInvalid = []string{}
			}
			require.Len(t, invalid, len(tt.expectedInvalid), "invalid length should match expected invalid length")

			resultMap := make(map[string]bool)
			for _, toolset := range result {
				resultMap[toolset] = true
			}

			expectedMap := make(map[string]bool)
			for _, toolset := range tt.expected {
				expectedMap[toolset] = true
			}

			invalidMap := make(map[string]bool)
			for _, toolset := range invalid {
				invalidMap[toolset] = true
			}

			expectedInvalidMap := make(map[string]bool)
			for _, toolset := range tt.expectedInvalid {
				expectedInvalidMap[toolset] = true
			}

			assert.Equal(t, expectedMap, resultMap, "result should contain all expected toolsets without duplicates")
			assert.Equal(t, expectedInvalidMap, invalidMap, "invalid should contain all expected invalid toolsets")

			assert.Len(t, resultMap, len(result), "result should not contain duplicates")
		})
	}
}

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
