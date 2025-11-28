package github

import (
	"context"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetIssueFragment_AllQueryTypes(t *testing.T) {
	tests := []struct {
		name     string
		query    any
		expected string
	}{
		{
			name:     "ListIssuesQueryWithSince",
			query:    &ListIssuesQueryWithSince{},
			expected: "issues fragment",
		},
		{
			name:     "ListIssuesQueryTypeWithLabelsWithSince",
			query:    &ListIssuesQueryTypeWithLabelsWithSince{},
			expected: "issues fragment",
		},
		{
			name:     "ListIssuesQuery",
			query:    &ListIssuesQuery{},
			expected: "issues fragment",
		},
		{
			name:     "ListIssuesQueryTypeWithLabels",
			query:    &ListIssuesQueryTypeWithLabels{},
			expected: "issues fragment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fragment IssueQueryFragment
			switch q := tt.query.(type) {
			case *ListIssuesQueryWithSince:
				fragment = q.GetIssueFragment()
			case *ListIssuesQueryTypeWithLabelsWithSince:
				fragment = q.GetIssueFragment()
			case *ListIssuesQuery:
				fragment = q.GetIssueFragment()
			case *ListIssuesQueryTypeWithLabels:
				fragment = q.GetIssueFragment()
			}
			assert.NotNil(t, fragment)
		})
	}
}

func Test_GetIssueQueryType(t *testing.T) {
	tests := []struct {
		name      string
		hasLabels bool
		hasSince  bool
		wantType  string
	}{
		{
			name:      "both labels and since",
			hasLabels: true,
			hasSince:  true,
			wantType:  "*github.ListIssuesQueryTypeWithLabelsWithSince",
		},
		{
			name:      "labels only",
			hasLabels: true,
			hasSince:  false,
			wantType:  "*github.ListIssuesQueryTypeWithLabels",
		},
		{
			name:      "since only",
			hasLabels: false,
			hasSince:  true,
			wantType:  "*github.ListIssuesQueryWithSince",
		},
		{
			name:      "neither labels nor since",
			hasLabels: false,
			hasSince:  false,
			wantType:  "*github.ListIssuesQuery",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getIssueQueryType(tt.hasLabels, tt.hasSince)
			assert.NotNil(t, result)
			// Verify the correct type is returned by checking type assertion
			switch tt.wantType {
			case "*github.ListIssuesQueryTypeWithLabelsWithSince":
				_, ok := result.(*ListIssuesQueryTypeWithLabelsWithSince)
				assert.True(t, ok, "Expected ListIssuesQueryTypeWithLabelsWithSince")
			case "*github.ListIssuesQueryTypeWithLabels":
				_, ok := result.(*ListIssuesQueryTypeWithLabels)
				assert.True(t, ok, "Expected ListIssuesQueryTypeWithLabels")
			case "*github.ListIssuesQueryWithSince":
				_, ok := result.(*ListIssuesQueryWithSince)
				assert.True(t, ok, "Expected ListIssuesQueryWithSince")
			case "*github.ListIssuesQuery":
				_, ok := result.(*ListIssuesQuery)
				assert.True(t, ok, "Expected ListIssuesQuery")
			}
		})
	}
}

func Test_GetCloseStateReason(t *testing.T) {
	tests := []struct {
		name           string
		stateReason    string
		expectedResult IssueClosedStateReason
	}{
		{
			name:           "completed state reason",
			stateReason:    "completed",
			expectedResult: IssueClosedStateReasonCompleted,
		},
		{
			name:           "not_planned state reason",
			stateReason:    "not_planned",
			expectedResult: IssueClosedStateReasonNotPlanned,
		},
		{
			name:           "duplicate state reason",
			stateReason:    "duplicate",
			expectedResult: IssueClosedStateReasonDuplicate,
		},
		{
			name:           "empty state reason defaults to completed",
			stateReason:    "",
			expectedResult: IssueClosedStateReasonCompleted,
		},
		{
			name:           "unknown state reason defaults to completed",
			stateReason:    "unknown_reason",
			expectedResult: IssueClosedStateReasonCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCloseStateReason(tt.stateReason)
			assert.Equal(t, tt.expectedResult, result)
			assert.NotEmpty(t, result)
		})
	}
}

func Test_AssignCodingAgentPrompt(t *testing.T) {
	prompt, handler := AssignCodingAgentPrompt(translations.NullTranslationHelper)

	// Verify prompt definition
	assert.Equal(t, "AssignCodingAgent", prompt.Name)
	assert.NotEmpty(t, prompt.Description)

	// Check that "repo" argument exists
	foundRepoArg := false
	for _, arg := range prompt.Arguments {
		if arg.Name == "repo" {
			foundRepoArg = true
			assert.True(t, arg.Required)
			break
		}
	}
	assert.True(t, foundRepoArg, "Should have 'repo' argument")

	// Test handler with valid repo
	tests := []struct {
		name        string
		repo        string
		expectError bool
	}{
		{
			name:        "valid repo format",
			repo:        "owner/repo",
			expectError: false,
		},
		{
			name:        "simple repo name",
			repo:        "test-repo",
			expectError: false,
		},
		{
			name:        "complex repo name",
			repo:        "github/github-mcp-server",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := mcp.GetPromptRequest{
				Params: mcp.GetPromptParams{
					Arguments: map[string]string{
						"repo": tt.repo,
					},
				},
			}

			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotEmpty(t, result.Messages)

				// Verify the messages contain the repo reference
				foundRepoReference := false
				for _, msg := range result.Messages {
					if textContent, ok := msg.Content.(mcp.TextContent); ok {
						if strings.Contains(textContent.Text, tt.repo) {
							foundRepoReference = true
							break
						}
					}
				}
				assert.True(t, foundRepoReference, "Messages should reference the repository")
			}
		})
	}
}

func Test_ListAvailableToolsets(t *testing.T) {
	// Create a toolset group
	tsg := toolsets.NewToolsetGroup(false)

	// Add some toolsets
	issuesToolset := toolsets.NewToolset("issues", "GitHub Issues toolset")
	tsg.AddToolset(issuesToolset)

	prsToolset := toolsets.NewToolset("pullrequests", "GitHub Pull Requests toolset")
	tsg.AddToolset(prsToolset)

	tool, handler := ListAvailableToolsets(tsg, translations.NullTranslationHelper)

	assert.Equal(t, "list_available_toolsets", tool.Name)
	assert.NotEmpty(t, tool.Description)

	// Test the handler
	request := createMCPRequest(map[string]any{})
	result, err := handler(context.Background(), request)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent := getTextResult(t, result)
	assert.NotEmpty(t, textContent.Text)

	// The result should contain toolset information
	assert.Contains(t, textContent.Text, "toolset")
}
