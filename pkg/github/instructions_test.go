package github

import (
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
			expectedContains: []string{
				"create_pending_pull_request_review",
				"add_comment_to_pending_review",
				"submit_pending_pull_request_review",
			},
		},
		{
			name:            "issues toolset",
			enabledToolsets: []string{"issues"},
			expectedContains: []string{
				"search_issues",
				"list_issue_types",
				"state_reason",
			},
		},
		{
			name:            "notifications toolset",
			enabledToolsets: []string{"notifications"},
			expectedContains: []string{
				"participating",
				"mark_all_notifications_read",
				"repository filters",
			},
		},
		{
			name:            "discussions toolset",
			enabledToolsets: []string{"discussions"},
			expectedContains: []string{
				"list_discussion_categories",
				"Filter by category",
			},
		},
		{
			name:            "multiple toolsets (context + pull_requests)",
			enabledToolsets: []string{"context", "pull_requests"},
			expectedContains: []string{
				"get_me",
				"create_pending_pull_request_review",
			},
		},
		{
			name:            "multiple toolsets (issues + pull_requests)",
			enabledToolsets: []string{"issues", "pull_requests"},
			expectedContains: []string{
				"search_issues",
				"list_issue_types",
				"create_pending_pull_request_review",
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
