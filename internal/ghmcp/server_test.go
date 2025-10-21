package ghmcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanToolsets(t *testing.T) {
	tests := []struct {
		name            string
		input           []string
		dynamicToolsets bool
		expected        []string
		expectedInvalid []string
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
		// Whitespace test cases
		{
			name:            "whitespace check - leading and trailing whitespace on regular toolsets",
			input:           []string{" actions ", "  gists  ", "notifications"},
			dynamicToolsets: false,
			expected:        []string{"actions", "gists", "notifications"},
		},
		{
			name:            "whitespace check - default toolset",
			input:           []string{" actions ", "  default  ", "notifications"},
			dynamicToolsets: false,
			expected: []string{
				"actions",
				"notifications",
				"context",
				"repos",
				"issues",
				"pull_requests",
				"users",
			},
		},
		{
			name:            "whitespace check - all toolset",
			input:           []string{" actions ", "  gists  ", "notifications", "  all   "},
			dynamicToolsets: false,
			expected:        []string{"all"},
		},
		// Invalid toolset test cases
		{
			name:            "mix of valid and invalid toolsets",
			input:           []string{"actions", "invalid_toolset", "gists", "typo_repo"},
			dynamicToolsets: false,
			expected:        []string{"actions", "gists"},
			expectedInvalid: []string{"invalid_toolset", "typo_repo"},
		},
		{
			name:            "invalid with whitespace",
			input:           []string{" invalid_tool ", "  actions  ", " typo_gist "},
			dynamicToolsets: false,
			expected:        []string{"actions"},
			expectedInvalid: []string{"invalid_tool", "typo_gist"},
		},
		{
			name:            "empty string in toolsets",
			input:           []string{"", "actions", "  ", "gists"},
			dynamicToolsets: false,
			expected:        []string{"actions", "gists"},
			expectedInvalid: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, invalid := cleanToolsets(tt.input, tt.dynamicToolsets)

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

			assert.False(t, resultMap["default"], "result should not contain 'default'")
		})
	}
}
