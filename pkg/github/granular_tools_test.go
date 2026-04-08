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

		expected := []string{
			"create_issue",
			"update_issue_title",
			"update_issue_body",
			"update_issue_assignees",
			"update_issue_labels",
			"update_issue_milestone",
			"update_issue_type",
			"update_issue_state",
			"add_sub_issue",
			"remove_sub_issue",
			"reprioritize_sub_issue",
		}
		for _, name := range expected {
			assert.Contains(t, toolNames, name)
		}
		assert.Len(t, tools, len(expected))
	})

	t.Run("all tools belong to issues_granular toolset", func(t *testing.T) {
		for _, tool := range granularToolsForToolset(ToolsetMetadataIssuesGranular.ID) {
			assert.Equal(t, ToolsetMetadataIssuesGranular.ID, tool.Toolset.ID, "tool %s", tool.Tool.Name)
		}
	})

	t.Run("all tools are write tools", func(t *testing.T) {
		for _, tool := range granularToolsForToolset(ToolsetMetadataIssuesGranular.ID) {
			assert.False(t, tool.Tool.Annotations.ReadOnlyHint, "tool %s should have ReadOnlyHint=false", tool.Tool.Name)
		}
	})

	t.Run("toolset is non-default", func(t *testing.T) {
		assert.False(t, ToolsetMetadataIssuesGranular.Default)
	})

	t.Run("no duplicate names with issues toolset", func(t *testing.T) {
		issueTools := make(map[string]bool)
		for _, tool := range AllTools(translations.NullTranslationHelper) {
			if tool.Toolset.ID == ToolsetMetadataIssues.ID {
				issueTools[tool.Tool.Name] = true
			}
		}
		for _, tool := range granularToolsForToolset(ToolsetMetadataIssuesGranular.ID) {
			assert.False(t, issueTools[tool.Tool.Name], "tool %s duplicates a tool in the issues toolset", tool.Tool.Name)
		}
	})
}

func TestPullRequestsGranularToolset(t *testing.T) {
	t.Run("toolset contains expected tools", func(t *testing.T) {
		tools := granularToolsForToolset(ToolsetMetadataPullRequestsGranular.ID)

		toolNames := make([]string, 0, len(tools))
		for _, tool := range tools {
			toolNames = append(toolNames, tool.Tool.Name)
		}

		expected := []string{
			"update_pull_request_title",
			"update_pull_request_body",
			"update_pull_request_state",
			"update_pull_request_draft_state",
			"request_pull_request_reviewers",
			"create_pull_request_review",
			"submit_pending_pull_request_review",
			"delete_pending_pull_request_review",
			"add_pull_request_review_comment",
		}
		for _, name := range expected {
			assert.Contains(t, toolNames, name)
		}
		assert.Len(t, tools, len(expected))
	})

	t.Run("all tools belong to pull_requests_granular toolset", func(t *testing.T) {
		for _, tool := range granularToolsForToolset(ToolsetMetadataPullRequestsGranular.ID) {
			assert.Equal(t, ToolsetMetadataPullRequestsGranular.ID, tool.Toolset.ID, "tool %s", tool.Tool.Name)
		}
	})

	t.Run("toolset is non-default", func(t *testing.T) {
		assert.False(t, ToolsetMetadataPullRequestsGranular.Default)
	})

	t.Run("no duplicate names with pull_requests toolset", func(t *testing.T) {
		prTools := make(map[string]bool)
		for _, tool := range AllTools(translations.NullTranslationHelper) {
			if tool.Toolset.ID == ToolsetMetadataPullRequests.ID {
				prTools[tool.Tool.Name] = true
			}
		}
		for _, tool := range granularToolsForToolset(ToolsetMetadataPullRequestsGranular.ID) {
			assert.False(t, prTools[tool.Tool.Name], "tool %s duplicates a tool in the pull_requests toolset", tool.Tool.Name)
		}
	})
}

// --- Issue granular tool handler tests ---

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

func TestGranularUpdateIssueBody(t *testing.T) {
	client := gogithub.NewClient(MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, map[string]any{
			"body": "Updated body",
		}).andThen(mockResponse(t, http.StatusOK, &gogithub.Issue{
			Number: gogithub.Ptr(1),
			Body:   gogithub.Ptr("Updated body"),
		})),
	}))
	deps := BaseDeps{Client: client}
	serverTool := GranularUpdateIssueBody(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(1),
		"body":         "Updated body",
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGranularUpdateIssueAssignees(t *testing.T) {
	client := gogithub.NewClient(MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, map[string]any{
			"assignees": []any{"user1", "user2"},
		}).andThen(mockResponse(t, http.StatusOK, &gogithub.Issue{Number: gogithub.Ptr(1)})),
	}))
	deps := BaseDeps{Client: client}
	serverTool := GranularUpdateIssueAssignees(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(1),
		"assignees":    []string{"user1", "user2"},
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

func TestGranularUpdateIssueMilestone(t *testing.T) {
	client := gogithub.NewClient(MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, map[string]any{
			"milestone": float64(5),
		}).andThen(mockResponse(t, http.StatusOK, &gogithub.Issue{Number: gogithub.Ptr(1)})),
	}))
	deps := BaseDeps{Client: client}
	serverTool := GranularUpdateIssueMilestone(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(1),
		"milestone":    float64(5),
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGranularUpdateIssueType(t *testing.T) {
	client := gogithub.NewClient(MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, map[string]any{
			"type": "bug",
		}).andThen(mockResponse(t, http.StatusOK, &gogithub.Issue{Number: gogithub.Ptr(1)})),
	}))
	deps := BaseDeps{Client: client}
	serverTool := GranularUpdateIssueType(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(1),
		"issue_type":   "bug",
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGranularUpdateIssueState(t *testing.T) {
	tests := []struct {
		name        string
		requestArgs map[string]any
		expectedReq map[string]any
	}{
		{
			name: "close with reason",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"state":        "closed",
				"state_reason": "completed",
			},
			expectedReq: map[string]any{
				"state":        "closed",
				"state_reason": "completed",
			},
		},
		{
			name: "reopen without reason",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"state":        "open",
			},
			expectedReq: map[string]any{
				"state": "open",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gogithub.NewClient(MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, tc.expectedReq).
					andThen(mockResponse(t, http.StatusOK, &gogithub.Issue{
						Number: gogithub.Ptr(1),
						State:  gogithub.Ptr(tc.requestArgs["state"].(string)),
					})),
			}))
			deps := BaseDeps{Client: client}
			serverTool := GranularUpdateIssueState(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			assert.False(t, result.IsError)
		})
	}
}

// --- Pull request granular tool handler tests ---

func TestGranularUpdatePullRequestTitle(t *testing.T) {
	client := gogithub.NewClient(MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposPullsByOwnerByRepoByPullNumber: expectRequestBody(t, map[string]any{
			"title": "New PR Title",
		}).andThen(mockResponse(t, http.StatusOK, &gogithub.PullRequest{
			Number: gogithub.Ptr(1),
			Title:  gogithub.Ptr("New PR Title"),
		})),
	}))
	deps := BaseDeps{Client: client}
	serverTool := GranularUpdatePullRequestTitle(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":      "owner",
		"repo":       "repo",
		"pullNumber": float64(1),
		"title":      "New PR Title",
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGranularUpdatePullRequestBody(t *testing.T) {
	client := gogithub.NewClient(MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposPullsByOwnerByRepoByPullNumber: expectRequestBody(t, map[string]any{
			"body": "Updated description",
		}).andThen(mockResponse(t, http.StatusOK, &gogithub.PullRequest{
			Number: gogithub.Ptr(1),
			Body:   gogithub.Ptr("Updated description"),
		})),
	}))
	deps := BaseDeps{Client: client}
	serverTool := GranularUpdatePullRequestBody(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":      "owner",
		"repo":       "repo",
		"pullNumber": float64(1),
		"body":       "Updated description",
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGranularUpdatePullRequestState(t *testing.T) {
	client := gogithub.NewClient(MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposPullsByOwnerByRepoByPullNumber: expectRequestBody(t, map[string]any{
			"state": "closed",
		}).andThen(mockResponse(t, http.StatusOK, &gogithub.PullRequest{
			Number: gogithub.Ptr(1),
			State:  gogithub.Ptr("closed"),
		})),
	}))
	deps := BaseDeps{Client: client}
	serverTool := GranularUpdatePullRequestState(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":      "owner",
		"repo":       "repo",
		"pullNumber": float64(1),
		"state":      "closed",
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

func TestGranularCreatePullRequestReview(t *testing.T) {
	client := gogithub.NewClient(MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		"POST /repos/{owner}/{repo}/pulls/{pull_number}/reviews": mockResponse(t, http.StatusOK, &gogithub.PullRequestReview{
			ID:    gogithub.Ptr(int64(1)),
			State: gogithub.Ptr("APPROVED"),
		}),
	}))
	deps := BaseDeps{Client: client}
	serverTool := GranularCreatePullRequestReview(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":      "owner",
		"repo":       "repo",
		"pullNumber": float64(1),
		"body":       "LGTM",
		"event":      "APPROVE",
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}
