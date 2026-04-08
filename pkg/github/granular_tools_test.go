package github

import (
	"context"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func granularToolsForToolset(id inventory.ToolsetID) []inventory.ServerTool {
	var result []inventory.ServerTool
	for _, tool := range AllTools(translations.NullTranslationHelper) {
		if tool.Toolset.ID == id {
			result = append(result, tool)
		}
	}
	return result
}

func TestIssuesGranularToolset(t *testing.T) {
	t.Run("toolset contains expected tools", func(t *testing.T) {
		tools := granularToolsForToolset(ToolsetMetadataIssuesGranular.ID)

		toolNames := make([]string, 0, len(tools))
		for _, tool := range tools {
			toolNames = append(toolNames, tool.Tool.Name)
		}

		assert.Contains(t, toolNames, "create_issue")
		assert.Contains(t, toolNames, "update_issue_title")
		assert.Contains(t, toolNames, "update_issue_body")
		assert.Contains(t, toolNames, "update_issue_assignees")
		assert.Contains(t, toolNames, "update_issue_labels")
		assert.Contains(t, toolNames, "update_issue_milestone")
		assert.Contains(t, toolNames, "update_issue_type")
		assert.Contains(t, toolNames, "update_issue_state")
		assert.Contains(t, toolNames, "add_sub_issue")
		assert.Contains(t, toolNames, "remove_sub_issue")
		assert.Contains(t, toolNames, "reprioritize_sub_issue")
	})

	t.Run("all tools belong to issues_granular toolset", func(t *testing.T) {
		tools := granularToolsForToolset(ToolsetMetadataIssuesGranular.ID)

		for _, tool := range tools {
			assert.Equal(t, ToolsetMetadataIssuesGranular.ID, tool.Toolset.ID, "tool %s should belong to issues_granular toolset", tool.Tool.Name)
		}
	})

	t.Run("all tools have ReadOnlyHint false", func(t *testing.T) {
		tools := granularToolsForToolset(ToolsetMetadataIssuesGranular.ID)

		for _, tool := range tools {
			assert.False(t, tool.Tool.Annotations.ReadOnlyHint, "tool %s should have ReadOnlyHint=false", tool.Tool.Name)
		}
	})

	t.Run("toolset is non-default", func(t *testing.T) {
		assert.False(t, ToolsetMetadataIssuesGranular.Default, "issues_granular toolset should not be default")
	})
}

func TestPullRequestsGranularToolset(t *testing.T) {
	t.Run("toolset contains expected tools", func(t *testing.T) {
		tools := granularToolsForToolset(ToolsetMetadataPullRequestsGranular.ID)

		toolNames := make([]string, 0, len(tools))
		for _, tool := range tools {
			toolNames = append(toolNames, tool.Tool.Name)
		}

		assert.Contains(t, toolNames, "update_pull_request_title")
		assert.Contains(t, toolNames, "update_pull_request_body")
		assert.Contains(t, toolNames, "update_pull_request_state")
		assert.Contains(t, toolNames, "update_pull_request_draft_state")
		assert.Contains(t, toolNames, "request_pull_request_reviewers")
		assert.Contains(t, toolNames, "create_pull_request_review")
		assert.Contains(t, toolNames, "submit_pending_pull_request_review")
		assert.Contains(t, toolNames, "delete_pending_pull_request_review")
		assert.Contains(t, toolNames, "add_pull_request_review_comment")
	})

	t.Run("all tools belong to pull_requests_granular toolset", func(t *testing.T) {
		tools := granularToolsForToolset(ToolsetMetadataPullRequestsGranular.ID)

		for _, tool := range tools {
			assert.Equal(t, ToolsetMetadataPullRequestsGranular.ID, tool.Toolset.ID, "tool %s should belong to pull_requests_granular toolset", tool.Tool.Name)
		}
	})

	t.Run("toolset is non-default", func(t *testing.T) {
		assert.False(t, ToolsetMetadataPullRequestsGranular.Default, "pull_requests_granular toolset should not be default")
	})
}

func TestGranularCreateIssue(t *testing.T) {
	mockIssue := &gogithub.Issue{
		Number: gogithub.Ptr(1),
		Title:  gogithub.Ptr("Test Issue"),
		Body:   gogithub.Ptr("Test body"),
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectedErrMsg string
	}{
		{
			name: "successful creation",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesByOwnerByRepo: expectRequestBody(t, map[string]any{
					"title": "Test Issue",
					"body":  "Test body",
				}).andThen(mockResponse(t, http.StatusCreated, mockIssue)),
			}),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"title": "Test Issue",
				"body":  "Test body",
			},
		},
		{
			name:         "missing required parameter",
			mockedClient: MockHTTPClientWithHandlers(nil),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectedErrMsg: "missing required parameter: title",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gogithub.NewClient(tc.mockedClient)
			deps := BaseDeps{Client: client}
			serverTool := GranularCreateIssue(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			if tc.expectedErrMsg != "" {
				textContent := getTextResult(t, result)
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}
			assert.False(t, result.IsError)
		})
	}
}

func TestGranularUpdateIssueTitle(t *testing.T) {
	client := gogithub.NewClient(MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, &gogithub.Issue{
			Number: gogithub.Ptr(42),
			Title:  gogithub.Ptr("New Title"),
		}),
	}))
	deps := BaseDeps{Client: client}
	serverTool := GranularUpdateIssueTitle(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(42),
		"title":        "New Title",
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGranularUpdateIssueLabels(t *testing.T) {
	client := gogithub.NewClient(MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, map[string]any{
			"labels": []any{"bug", "enhancement"},
		}).andThen(mockResponse(t, http.StatusOK, &gogithub.Issue{Number: gogithub.Ptr(1)})),
	}))
	deps := BaseDeps{Client: client}
	serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(1),
		"labels":       []string{"bug", "enhancement"},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGranularUpdateIssueState(t *testing.T) {
	client := gogithub.NewClient(MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, map[string]any{
			"state":        "closed",
			"state_reason": "completed",
		}).andThen(mockResponse(t, http.StatusOK, &gogithub.Issue{
			Number: gogithub.Ptr(1),
			State:  gogithub.Ptr("closed"),
		})),
	}))
	deps := BaseDeps{Client: client}
	serverTool := GranularUpdateIssueState(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(1),
		"state":        "closed",
		"state_reason": "completed",
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGranularRequestPullRequestReviewers(t *testing.T) {
	client := gogithub.NewClient(MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, &gogithub.PullRequest{Number: gogithub.Ptr(1)}),
	}))
	deps := BaseDeps{Client: client}
	serverTool := GranularRequestPullRequestReviewers(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":      "owner",
		"repo":       "repo",
		"pullNumber": float64(1),
		"reviewers":  []string{"user1", "user2"},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}
