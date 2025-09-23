package github

import (
	"os"
	"strings"
	"testing"
)

func TestGenerateInstructions(t *testing.T) {
	tests := []struct {
		name             string
		enabledToolsets  []string
		expectedContains []string
		expectedEmpty    bool
	}{
		{
			name:            "empty toolsets",
			enabledToolsets: []string{},
			expectedContains: []string{
				"GitHub MCP Server provides GitHub API tools",
				"Use 'list_*' tools for broad, simple retrieval",
				"Use 'search_*' tools for targeted queries",
				"context windows",
			},
		},
		{
			name:            "only context toolset",
			enabledToolsets: []string{"context"},
			expectedContains: []string{
				"GitHub MCP Server provides GitHub API tools",
				"Always call 'get_me' first",
			},
		},
		{
			name:            "pull requests toolset",
			enabledToolsets: []string{"pull_requests"},
			expectedContains: []string{"## Pull Requests"},
		},
		{
			name:            "issues toolset",
			enabledToolsets: []string{"issues"},
			expectedContains: []string{"## Issues"},
		},
		{
			name:            "discussions toolset",
			enabledToolsets: []string{"discussions"},
			expectedContains: []string{"## Discussions"},
		},
		{
			name:            "multiple toolsets (context + pull_requests)",
			enabledToolsets: []string{"context", "pull_requests"},
			expectedContains: []string{
				"get_me",
				"## Pull Requests",
			},
		},
		{
			name:            "multiple toolsets (issues + pull_requests)",
			enabledToolsets: []string{"issues", "pull_requests"},
			expectedContains: []string{
				"## Issues",
				"## Pull Requests",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateInstructions(tt.enabledToolsets)

			if tt.expectedEmpty {
				if result != "" {
					t.Errorf("Expected empty instructions but got: %s", result)
				}
				return
			}

			for _, expectedContent := range tt.expectedContains {
				if !strings.Contains(result, expectedContent) {
					t.Errorf("Expected instructions to contain '%s', but got: %s", expectedContent, result)
				}
			}
		})
	}
}

func TestGenerateInstructionsWithDisableFlag(t *testing.T) {
	tests := []struct {
		name              string
		disableEnvValue   string
		enabledToolsets   []string
		expectedEmpty     bool
		expectedContains  []string
	}{
		{
			name:            "DISABLE_INSTRUCTIONS=true returns empty",
			disableEnvValue: "true",
			enabledToolsets: []string{"context", "issues", "pull_requests"},
			expectedEmpty:   true,
		},
		{
			name:            "DISABLE_INSTRUCTIONS=false returns normal instructions",
			disableEnvValue: "false",
			enabledToolsets: []string{"context"},
			expectedEmpty:   false,
			expectedContains: []string{
				"GitHub MCP Server provides GitHub API tools",
				"Always call 'get_me' first",
			},
		},
		{
			name:            "DISABLE_INSTRUCTIONS unset returns normal instructions",
			disableEnvValue: "",
			enabledToolsets: []string{"issues"},
			expectedEmpty:   false,
			expectedContains: []string{
				"GitHub MCP Server provides GitHub API tools",
				"search_issues",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env value
			originalValue := os.Getenv("DISABLE_INSTRUCTIONS")
			defer func() {
				if originalValue == "" {
					os.Unsetenv("DISABLE_INSTRUCTIONS")
				} else {
					os.Setenv("DISABLE_INSTRUCTIONS", originalValue)
				}
			}()

			// Set test env value
			if tt.disableEnvValue == "" {
				os.Unsetenv("DISABLE_INSTRUCTIONS")
			} else {
				os.Setenv("DISABLE_INSTRUCTIONS", tt.disableEnvValue)
			}

			result := GenerateInstructions(tt.enabledToolsets)

			if tt.expectedEmpty {
				if result != "" {
					t.Errorf("Expected empty instructions but got: %s", result)
				}
				return
			}

			for _, expectedContent := range tt.expectedContains {
				if !strings.Contains(result, expectedContent) {
					t.Errorf("Expected instructions to contain '%s', but got: %s", expectedContent, result)
				}
			}
		})
	}
}

func TestGetToolsetInstructions(t *testing.T) {
	tests := []struct {
		toolset  string
		expected string
	}{
		{
			toolset:  "pull_requests",
			expected: "create_pending_pull_request_review",
		},
		{
			toolset:  "issues",
			expected: "list_issue_types",
		},
		{
			toolset:  "notifications",
			expected: "participating",
		},
		{
			toolset:  "discussions",
			expected: "list_discussion_categories",
		},
		{
			toolset:  "nonexistent",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.toolset, func(t *testing.T) {
			result := getToolsetInstructions(tt.toolset)
			if tt.expected == "" {
				if result != "" {
					t.Errorf("Expected empty result for toolset '%s', but got: %s", tt.toolset, result)
				}
			} else {
				if !strings.Contains(result, tt.expected) {
					t.Errorf("Expected instructions for '%s' to contain '%s', but got: %s", tt.toolset, tt.expected, result)
				}
			}
		})
	}
}
