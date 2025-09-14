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
			expectedEmpty:   true,
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
			name:            "actions toolset",
			enabledToolsets: []string{"actions"},
			expectedContains: []string{
				"get_job_logs",
				"failed_only=true",
				"rerun_failed_jobs",
			},
		},
		{
			name:            "security toolsets",
			enabledToolsets: []string{"code_security", "secret_protection", "dependabot"},
			expectedContains: []string{
				"Security alert priority",
				"secret_scanning",
				"dependabot",
				"code_scanning",
			},
		},
		{
			name:            "cross-toolset instructions",
			enabledToolsets: []string{"context", "pull_requests"},
			expectedContains: []string{
				"get_me",
				"get_teams",
				"get_team_members",
			},
		},
		{
			name:            "issues and pull_requests combination",
			enabledToolsets: []string{"issues", "pull_requests"},
			expectedContains: []string{
				"closes #123",
				"fixes #123",
				"search_issues",
				"list_issue_types",
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
			toolset:  "actions",
			expected: "get_job_logs",
		},
		{
			toolset:  "issues",
			expected: "list_issue_types",
		},
		{
			toolset:  "repos",
			expected: "get_file_contents",
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

func TestHasAnySecurityToolset(t *testing.T) {
	tests := []struct {
		name     string
		toolsets []string
		expected bool
	}{
		{
			name:     "no security toolsets",
			toolsets: []string{"repos", "issues"},
			expected: false,
		},
		{
			name:     "has code_security",
			toolsets: []string{"repos", "code_security"},
			expected: true,
		},
		{
			name:     "has secret_protection",
			toolsets: []string{"secret_protection", "issues"},
			expected: true,
		},
		{
			name:     "has dependabot",
			toolsets: []string{"dependabot"},
			expected: true,
		},
		{
			name:     "has all security toolsets",
			toolsets: []string{"code_security", "secret_protection", "dependabot"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAnySecurityToolset(tt.toolsets)
			if result != tt.expected {
				t.Errorf("Expected %v for toolsets %v, but got %v", tt.expected, tt.toolsets, result)
			}
		})
	}
}