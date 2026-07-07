package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_hasFilter(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		filterType string
		expected   bool
	}{
		{
			name:       "query has is:issue filter",
			query:      "is:issue bug report",
			filterType: "is",
			expected:   true,
		},
		{
			name:       "query has repo: filter",
			query:      "repo:github/github-mcp-server critical bug",
			filterType: "repo",
			expected:   true,
		},
		{
			name:       "query has multiple is: filters",
			query:      "is:issue is:open bug",
			filterType: "is",
			expected:   true,
		},
		{
			name:       "query has filter at the beginning",
			query:      "is:issue some text",
			filterType: "is",
			expected:   true,
		},
		{
			name:       "query has filter in the middle",
			query:      "some text is:issue more text",
			filterType: "is",
			expected:   true,
		},
		{
			name:       "query has filter at the end",
			query:      "some text is:issue",
			filterType: "is",
			expected:   true,
		},
		{
			name:       "query does not have the filter",
			query:      "bug report critical",
			filterType: "is",
			expected:   false,
		},
		{
			name:       "query has similar text but not the filter",
			query:      "this issue is important",
			filterType: "is",
			expected:   false,
		},
		{
			name:       "empty query",
			query:      "",
			filterType: "is",
			expected:   false,
		},
		{
			name:       "query has label: filter but looking for is:",
			query:      "label:bug critical",
			filterType: "is",
			expected:   false,
		},
		{
			name:       "query has author: filter",
			query:      "author:octocat bug",
			filterType: "author",
			expected:   true,
		},
		{
			name:       "query with complex OR expression",
			query:      "repo:github/github-mcp-server is:issue (label:critical OR label:urgent)",
			filterType: "is",
			expected:   true,
		},
		{
			name:       "query with complex OR expression checking repo",
			query:      "repo:github/github-mcp-server is:issue (label:critical OR label:urgent)",
			filterType: "repo",
			expected:   true,
		},
		{
			name:       "filter in parentheses at start",
			query:      "(label:bug OR owner:bob) is:issue",
			filterType: "label",
			expected:   true,
		},
		{
			name:       "filter after opening parenthesis",
			query:      "is:issue (label:critical OR repo:test/test)",
			filterType: "label",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasFilter(tt.query, tt.filterType)
			assert.Equal(t, tt.expected, result, "hasFilter(%q, %q) = %v, expected %v", tt.query, tt.filterType, result, tt.expected)
		})
	}
}

func Test_hasRepoFilter(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{
			name:     "query with repo: filter at beginning",
			query:    "repo:github/github-mcp-server is:issue",
			expected: true,
		},
		{
			name:     "query with repo: filter in middle",
			query:    "is:issue repo:octocat/Hello-World bug",
			expected: true,
		},
		{
			name:     "query with repo: filter at end",
			query:    "is:issue critical repo:owner/repo-name",
			expected: true,
		},
		{
			name:     "query with complex repo name",
			query:    "repo:microsoft/vscode-extension-samples bug",
			expected: true,
		},
		{
			name:     "query without repo: filter",
			query:    "is:issue bug critical",
			expected: false,
		},
		{
			name:     "query with malformed repo: filter (no slash)",
			query:    "repo:github bug",
			expected: true, // hasRepoFilter only checks for repo: prefix, not format
		},
		{
			name:     "empty query",
			query:    "",
			expected: false,
		},
		{
			name:     "query with multiple repo: filters",
			query:    "repo:github/first repo:octocat/second",
			expected: true,
		},
		{
			name:     "query with repo: in text but not as filter",
			query:    "this repo: is important",
			expected: false,
		},
		{
			name:     "query with complex OR expression",
			query:    "repo:github/github-mcp-server is:issue (label:critical OR label:urgent)",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasRepoFilter(tt.query)
			assert.Equal(t, tt.expected, result, "hasRepoFilter(%q) = %v, expected %v", tt.query, result, tt.expected)
		})
	}
}

func Test_hasSpecificFilter(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		filterType  string
		filterValue string
		expected    bool
	}{
		{
			name:        "query has exact is:issue filter",
			query:       "is:issue bug report",
			filterType:  "is",
			filterValue: "issue",
			expected:    true,
		},
		{
			name:        "query has is:open but looking for is:issue",
			query:       "is:open bug report",
			filterType:  "is",
			filterValue: "issue",
			expected:    false,
		},
		{
			name:        "query has both is:issue and is:open, looking for is:issue",
			query:       "is:issue is:open bug",
			filterType:  "is",
			filterValue: "issue",
			expected:    true,
		},
		{
			name:        "query has both is:issue and is:open, looking for is:open",
			query:       "is:issue is:open bug",
			filterType:  "is",
			filterValue: "open",
			expected:    true,
		},
		{
			name:        "query has is:issue at the beginning",
			query:       "is:issue some text",
			filterType:  "is",
			filterValue: "issue",
			expected:    true,
		},
		{
			name:        "query has is:issue in the middle",
			query:       "some text is:issue more text",
			filterType:  "is",
			filterValue: "issue",
			expected:    true,
		},
		{
			name:        "query has is:issue at the end",
			query:       "some text is:issue",
			filterType:  "is",
			filterValue: "issue",
			expected:    true,
		},
		{
			name:        "query does not have is:issue",
			query:       "bug report critical",
			filterType:  "is",
			filterValue: "issue",
			expected:    false,
		},
		{
			name:        "query has similar text but not the exact filter",
			query:       "this issue is important",
			filterType:  "is",
			filterValue: "issue",
			expected:    false,
		},
		{
			name:        "empty query",
			query:       "",
			filterType:  "is",
			filterValue: "issue",
			expected:    false,
		},
		{
			name:        "partial match should not count",
			query:       "is:issues bug", // "issues" vs "issue"
			filterType:  "is",
			filterValue: "issue",
			expected:    false,
		},
		{
			name:        "complex query with parentheses",
			query:       "repo:github/github-mcp-server is:issue (label:critical OR label:urgent)",
			filterType:  "is",
			filterValue: "issue",
			expected:    true,
		},
		{
			name:        "filter:value in parentheses at start",
			query:       "(is:issue OR is:pr) label:bug",
			filterType:  "is",
			filterValue: "issue",
			expected:    true,
		},
		{
			name:        "filter:value after opening parenthesis",
			query:       "repo:test/repo (is:issue AND label:bug)",
			filterType:  "is",
			filterValue: "issue",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasSpecificFilter(tt.query, tt.filterType, tt.filterValue)
			assert.Equal(t, tt.expected, result, "hasSpecificFilter(%q, %q, %q) = %v, expected %v", tt.query, tt.filterType, tt.filterValue, result, tt.expected)
		})
	}
}

func Test_hasTypeFilter(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{
			name:     "query with type:user filter at beginning",
			query:    "type:user location:seattle",
			expected: true,
		},
		{
			name:     "query with type:org filter in middle",
			query:    "location:california type:org followers:>100",
			expected: true,
		},
		{
			name:     "query with type:user filter at end",
			query:    "location:seattle followers:>50 type:user",
			expected: true,
		},
		{
			name:     "query without type: filter",
			query:    "location:seattle followers:>50",
			expected: false,
		},
		{
			name:     "empty query",
			query:    "",
			expected: false,
		},
		{
			name:     "query with type: in text but not as filter",
			query:    "this type: is important",
			expected: false,
		},
		{
			name:     "query with multiple type: filters",
			query:    "type:user type:org",
			expected: true,
		},
		{
			name:     "complex query with OR expression",
			query:    "type:user (location:seattle OR location:california)",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasTypeFilter(tt.query)
			assert.Equal(t, tt.expected, result, "hasTypeFilter(%q) = %v, expected %v", tt.query, result, tt.expected)
		})
	}
}
