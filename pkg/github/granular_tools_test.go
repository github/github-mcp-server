package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/http/headers"
	transportpkg "github.com/github/github-mcp-server/pkg/http/transport"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v89/github"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func granularToolsForToolset(toolsetID inventory.ToolsetID, featureFlag string) []inventory.ServerTool {
	var result []inventory.ServerTool
	for _, tool := range AllTools(translations.NullTranslationHelper) {
		if tool.Toolset.ID == toolsetID && tool.FeatureFlagEnable == featureFlag {
			result = append(result, tool)
		}
	}
	return result
}

func TestGranularToolSnaps(t *testing.T) {
	// Test toolsnaps for all granular tools
	toolConstructors := []func(translations.TranslationHelperFunc) inventory.ServerTool{
		GranularCreateIssue,
		GranularUpdateIssueTitle,
		GranularUpdateIssueBody,
		GranularUpdateIssueAssignees,
		GranularUpdateIssueLabels,
		GranularUpdateIssueMilestone,
		GranularUpdateIssueType,
		GranularUpdateIssueState,
		GranularAddSubIssue,
		GranularRemoveSubIssue,
		GranularReprioritizeSubIssue,
		GranularSetIssueFields,
		GranularAddIssueReaction,
		GranularAddIssueCommentReaction,
		GranularUpdatePullRequestTitle,
		GranularUpdatePullRequestBody,
		GranularUpdatePullRequestState,
		GranularUpdatePullRequestDraftState,
		GranularRequestPullRequestReviewers,
		GranularCreatePullRequestReview,
		GranularSubmitPendingPullRequestReview,
		GranularDeletePendingPullRequestReview,
		GranularAddPullRequestReviewComment,
		GranularResolveReviewThread,
		GranularUnresolveReviewThread,
		GranularAddPullRequestReviewCommentReaction,
	}

	for _, constructor := range toolConstructors {
		serverTool := constructor(translations.NullTranslationHelper)
		t.Run(serverTool.Tool.Name, func(t *testing.T) {
			require.NoError(t, toolsnaps.Test(serverTool.Tool.Name, serverTool.Tool))
		})
	}
}

func TestIssuesGranularToolset(t *testing.T) {
	t.Run("toolset contains expected granular tools", func(t *testing.T) {
		tools := granularToolsForToolset(ToolsetMetadataIssues.ID, FeatureFlagIssuesGranular)

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
			"set_issue_fields",
			"add_issue_reaction",
			"add_issue_comment_reaction",
		}
		for _, name := range expected {
			assert.Contains(t, toolNames, name)
		}
		assert.Len(t, tools, len(expected))
	})

	t.Run("all granular tools have correct feature flag", func(t *testing.T) {
		for _, tool := range granularToolsForToolset(ToolsetMetadataIssues.ID, FeatureFlagIssuesGranular) {
			assert.Equal(t, FeatureFlagIssuesGranular, tool.FeatureFlagEnable, "tool %s", tool.Tool.Name)
		}
	})
}

func TestPullRequestsGranularToolset(t *testing.T) {
	t.Run("toolset contains expected granular tools", func(t *testing.T) {
		tools := granularToolsForToolset(ToolsetMetadataPullRequests.ID, FeatureFlagPullRequestsGranular)

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
			"resolve_review_thread",
			"unresolve_review_thread",
			"add_pull_request_review_comment_reaction",
		}
		for _, name := range expected {
			assert.Contains(t, toolNames, name)
		}
		assert.Len(t, tools, len(expected))
	})

	t.Run("all granular tools have correct feature flag", func(t *testing.T) {
		for _, tool := range granularToolsForToolset(ToolsetMetadataPullRequests.ID, FeatureFlagPullRequestsGranular) {
			assert.Equal(t, FeatureFlagPullRequestsGranular, tool.FeatureFlagEnable, "tool %s", tool.Tool.Name)
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
			client := mustNewGHClient(t, tc.mockedClient)
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
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
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
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
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
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
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

func TestGranularUpdateIssueAssigneesObjectForm(t *testing.T) {
	tests := []struct {
		name        string
		requestArgs map[string]any
		expectedReq map[string]any
	}{
		{
			name: "assignee objects without intent serialize as strings",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"assignees": []any{
					map[string]any{"login": "octocat"},
					"monalisa",
				},
			},
			expectedReq: map[string]any{
				"assignees": []any{"octocat", "monalisa"},
			},
		},
		{
			name: "assignee suggested without rationale",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"assignees": []any{
					map[string]any{"login": "octocat", "is_suggestion": true},
				},
			},
			expectedReq: map[string]any{
				"assignees": []any{
					map[string]any{"login": "octocat", "suggest": true},
				},
			},
		},
		{
			name: "suggested assignee with rationale and confidence",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"assignees": []any{
					map[string]any{"login": "octocat", "rationale": "  Authored the crashing file  ", "confidence": "high", "is_suggestion": true},
				},
			},
			expectedReq: map[string]any{
				"assignees": []any{
					map[string]any{"login": "octocat", "rationale": "Authored the crashing file", "confidence": "HIGH", "suggest": true},
				},
			},
		},
		{
			name: "mix of plain and suggested assignees",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"assignees": []any{
					"monalisa",
					map[string]any{"login": "octocat", "is_suggestion": true},
				},
			},
			expectedReq: map[string]any{
				"assignees": []any{
					"monalisa",
					map[string]any{"login": "octocat", "suggest": true},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, tc.expectedReq).
					andThen(mockResponse(t, http.StatusOK, &gogithub.Issue{Number: gogithub.Ptr(1)})),
			}))
			deps := BaseDeps{Client: client}
			serverTool := GranularUpdateIssueAssignees(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			assert.False(t, result.IsError)
		})
	}
}

func TestGranularUpdateIssueAssigneesInvalidInput(t *testing.T) {
	tests := []struct {
		name            string
		requestArgs     map[string]any
		expectedErrText string
	}{
		{
			name: "rationale too long",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"assignees": []any{
					map[string]any{"login": "octocat", "rationale": strings.Repeat("a", 281)},
				},
			},
			expectedErrText: "assignee rationale must be 280 characters or less",
		},
		{
			name: "assignee object missing login",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"assignees": []any{
					map[string]any{"rationale": "no login provided"},
				},
			},
			expectedErrText: "each assignee object must have a 'login' string",
		},
		{
			name: "invalid confidence value",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"assignees": []any{
					map[string]any{"login": "octocat", "confidence": "maybe"},
				},
			},
			expectedErrText: "confidence must be one of: LOW, MEDIUM, HIGH",
		},
		{
			name: "assignee entry is neither string nor object",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"assignees":    []any{float64(123)},
			},
			expectedErrText: "each assignee must be a string or an object",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(nil))}
			serverTool := GranularUpdateIssueAssignees(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			errorContent := getErrorResult(t, result)
			assert.Contains(t, errorContent.Text, tc.expectedErrText)
		})
	}
}

func TestGranularUpdateIssueLabels(t *testing.T) {
	tests := []struct {
		name        string
		requestArgs map[string]any
		expectedReq map[string]any
	}{
		{
			name: "labels as plain strings",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"labels":       []any{"bug", "enhancement"},
			},
			expectedReq: map[string]any{
				"labels": []any{"bug", "enhancement"},
			},
		},
		{
			name: "label objects without rationale serialize as strings",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"labels": []any{
					map[string]any{"name": "bug"},
					"enhancement",
				},
			},
			expectedReq: map[string]any{
				"labels": []any{"bug", "enhancement"},
			},
		},
		{
			name: "mixed strings and label objects with rationale",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"labels": []any{
					"triage",
					map[string]any{"name": "bug", "rationale": "  Reports a crash when saving  "},
					map[string]any{"name": "frontend", "rationale": "Mentions the UI button"},
				},
			},
			expectedReq: map[string]any{
				"labels": []any{
					"triage",
					map[string]any{"name": "bug", "rationale": "Reports a crash when saving"},
					map[string]any{"name": "frontend", "rationale": "Mentions the UI button"},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, tc.expectedReq).
					andThen(mockResponse(t, http.StatusOK, &gogithub.Issue{Number: gogithub.Ptr(1)})),
			}))
			deps := BaseDeps{Client: client}
			serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			assert.False(t, result.IsError)
		})
	}
}

func TestGranularUpdateIssueLabelsSuggest(t *testing.T) {
	tests := []struct {
		name        string
		requestArgs map[string]any
		expectedReq map[string]any
	}{
		{
			name: "single label suggested without rationale",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"labels": []any{
					map[string]any{"name": "bug", "is_suggestion": true},
				},
			},
			expectedReq: map[string]any{
				"labels": []any{
					map[string]any{"name": "bug", "suggest": true},
				},
			},
		},
		{
			name: "suggested label with rationale",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"labels": []any{
					map[string]any{"name": "frontend", "rationale": "Mentions the UI button", "is_suggestion": true},
				},
			},
			expectedReq: map[string]any{
				"labels": []any{
					map[string]any{"name": "frontend", "rationale": "Mentions the UI button", "suggest": true},
				},
			},
		},
		{
			name: "mix of plain, applied-with-rationale, and suggested labels",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"labels": []any{
					"triage",
					map[string]any{"name": "bug", "rationale": "Reports a crash when saving"},
					map[string]any{"name": "needs-design", "is_suggestion": true},
				},
			},
			expectedReq: map[string]any{
				"labels": []any{
					"triage",
					map[string]any{"name": "bug", "rationale": "Reports a crash when saving"},
					map[string]any{"name": "needs-design", "suggest": true},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, tc.expectedReq).
					andThen(mockResponse(t, http.StatusOK, &gogithub.Issue{Number: gogithub.Ptr(1)})),
			}))
			deps := BaseDeps{Client: client}
			serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			assert.False(t, result.IsError)
		})
	}
}

func TestGranularUpdateIssueLabelsInvalidRationale(t *testing.T) {
	tests := []struct {
		name            string
		requestArgs     map[string]any
		expectedErrText string
	}{
		{
			name: "rationale too long",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"labels": []any{
					map[string]any{"name": "bug", "rationale": strings.Repeat("a", 281)},
				},
			},
			expectedErrText: "label rationale must be 280 characters or less",
		},
		{
			name: "label object missing name",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"labels": []any{
					map[string]any{"rationale": "no name provided"},
				},
			},
			expectedErrText: "each label object must have a 'name' string",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(nil))}
			serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			errorContent := getErrorResult(t, result)
			assert.Contains(t, errorContent.Text, tc.expectedErrText)
		})
	}
}

func TestGranularUpdateIssueLabelsConfidence(t *testing.T) {
	tests := []struct {
		name        string
		requestArgs map[string]any
		expectedReq map[string]any
	}{
		{
			name: "label with confidence triggers object form",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"labels": []any{
					map[string]any{"name": "bug", "confidence": "HIGH"},
				},
			},
			expectedReq: map[string]any{
				"labels": []any{
					map[string]any{"name": "bug", "confidence": "HIGH"},
				},
			},
		},
		{
			name: "label with confidence and rationale",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"labels": []any{
					map[string]any{"name": "bug", "rationale": "Reports a crash", "confidence": "MEDIUM"},
				},
			},
			expectedReq: map[string]any{
				"labels": []any{
					map[string]any{"name": "bug", "rationale": "Reports a crash", "confidence": "MEDIUM"},
				},
			},
		},
		{
			name: "label confidence is normalized",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"labels": []any{
					map[string]any{"name": "bug", "confidence": " high\t"},
				},
			},
			expectedReq: map[string]any{
				"labels": []any{
					map[string]any{"name": "bug", "confidence": "HIGH"},
				},
			},
		},
		{
			name: "invalid confidence value",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"labels": []any{
					map[string]any{"name": "bug", "confidence": "very_high"},
				},
			},
			expectedReq: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectedReq == nil {
				// Error case
				deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(nil))}
				serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
				handler := serverTool.Handler(deps)

				request := createMCPRequest(tc.requestArgs)
				result, err := handler(ContextWithDeps(context.Background(), deps), &request)
				require.NoError(t, err)

				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, "confidence must be one of: LOW, MEDIUM, HIGH")
				return
			}

			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, tc.expectedReq).
					andThen(mockResponse(t, http.StatusOK, &gogithub.Issue{Number: gogithub.Ptr(1)})),
			}))
			deps := BaseDeps{Client: client}
			serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			assert.False(t, result.IsError)
		})
	}
}

func TestGranularUpdateIssueMilestone(t *testing.T) {
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
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
	tests := []struct {
		name        string
		requestArgs map[string]any
		expectedReq map[string]any
	}{
		{
			name: "type only",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"issue_type":   "bug",
			},
			expectedReq: map[string]any{
				"type": "bug",
			},
		},
		{
			name: "type with rationale",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"issue_type":   "feature",
				"rationale":    "  This issue requests a new capability  ",
			},
			expectedReq: map[string]any{
				"type": map[string]any{
					"value":     "feature",
					"rationale": "This issue requests a new capability",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, tc.expectedReq).
					andThen(mockResponse(t, http.StatusOK, &gogithub.Issue{Number: gogithub.Ptr(1)})),
			}))
			deps := BaseDeps{Client: client}
			serverTool := GranularUpdateIssueType(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			assert.False(t, result.IsError)
		})
	}
}

func TestGranularUpdateIssueTypeSuggest(t *testing.T) {
	tests := []struct {
		name        string
		requestArgs map[string]any
		expectedReq map[string]any
	}{
		{
			name: "suggest without rationale",
			requestArgs: map[string]any{
				"owner":         "owner",
				"repo":          "repo",
				"issue_number":  float64(1),
				"issue_type":    "bug",
				"is_suggestion": true,
			},
			expectedReq: map[string]any{
				"type": map[string]any{
					"value":   "bug",
					"suggest": true,
				},
			},
		},
		{
			name: "suggest with rationale",
			requestArgs: map[string]any{
				"owner":         "owner",
				"repo":          "repo",
				"issue_number":  float64(1),
				"issue_type":    "feature",
				"rationale":     "  Asks for dark mode support  ",
				"is_suggestion": true,
			},
			expectedReq: map[string]any{
				"type": map[string]any{
					"value":     "feature",
					"rationale": "Asks for dark mode support",
					"suggest":   true,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, tc.expectedReq).
					andThen(mockResponse(t, http.StatusOK, &gogithub.Issue{Number: gogithub.Ptr(1)})),
			}))
			deps := BaseDeps{Client: client}
			serverTool := GranularUpdateIssueType(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			assert.False(t, result.IsError)
		})
	}
}

func TestGranularUpdateIssueTypeInvalidRationale(t *testing.T) {
	tests := []struct {
		name            string
		requestArgs     map[string]any
		expectedErrText string
	}{
		{
			name: "rationale wrong type",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"issue_type":   "feature",
				"rationale":    float64(123),
			},
			expectedErrText: "parameter rationale is not of type string, is float64",
		},
		{
			name: "rationale too long",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"issue_type":   "feature",
				"rationale":    strings.Repeat("a", 281),
			},
			expectedErrText: "parameter rationale must be 280 characters or less",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(nil))}
			serverTool := GranularUpdateIssueType(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			errorContent := getErrorResult(t, result)
			assert.Contains(t, errorContent.Text, tc.expectedErrText)
		})
	}
}

func TestGranularUpdateIssueTypeConfidence(t *testing.T) {
	tests := []struct {
		name        string
		requestArgs map[string]any
		expectedReq map[string]any
	}{
		{
			name: "type with confidence only",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"issue_type":   "bug",
				"confidence":   "HIGH",
			},
			expectedReq: map[string]any{
				"type": map[string]any{
					"value":      "bug",
					"confidence": "HIGH",
				},
			},
		},
		{
			name: "type with confidence and rationale",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"issue_type":   "feature",
				"rationale":    "Asks for dark mode support",
				"confidence":   "MEDIUM",
			},
			expectedReq: map[string]any{
				"type": map[string]any{
					"value":      "feature",
					"rationale":  "Asks for dark mode support",
					"confidence": "MEDIUM",
				},
			},
		},
		{
			name: "type with low confidence triggers object form",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"issue_type":   "bug",
				"confidence":   "LOW",
			},
			expectedReq: map[string]any{
				"type": map[string]any{
					"value":      "bug",
					"confidence": "LOW",
				},
			},
		},
		{
			name: "type confidence is normalized",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"issue_type":   "bug",
				"confidence":   " medium ",
			},
			expectedReq: map[string]any{
				"type": map[string]any{
					"value":      "bug",
					"confidence": "MEDIUM",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, tc.expectedReq).
					andThen(mockResponse(t, http.StatusOK, &gogithub.Issue{Number: gogithub.Ptr(1)})),
			}))
			deps := BaseDeps{Client: client}
			serverTool := GranularUpdateIssueType(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			assert.False(t, result.IsError)
		})
	}
}

func TestGranularUpdateIssueTypeInvalidConfidence(t *testing.T) {
	tests := []struct {
		name            string
		requestArgs     map[string]any
		expectedErrText string
	}{
		{
			name: "invalid confidence value",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"issue_type":   "bug",
				"confidence":   "very_high",
			},
			expectedErrText: "confidence must be one of: LOW, MEDIUM, HIGH",
		},
		{
			name: "confidence wrong type",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"issue_type":   "bug",
				"confidence":   float64(85),
			},
			expectedErrText: "parameter confidence is not of type string",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(nil))}
			serverTool := GranularUpdateIssueType(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			errorContent := getErrorResult(t, result)
			assert.Contains(t, errorContent.Text, tc.expectedErrText)
		})
	}
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
			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
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

func TestGranularUpdateIssueStateSuggest(t *testing.T) {
	tests := []struct {
		name        string
		requestArgs map[string]any
		expectedReq map[string]any
	}{
		{
			name: "suggest without rationale",
			requestArgs: map[string]any{
				"owner":         "owner",
				"repo":          "repo",
				"issue_number":  float64(1),
				"state":         "closed",
				"is_suggestion": true,
			},
			expectedReq: map[string]any{
				"state": map[string]any{
					"value":   "closed",
					"suggest": true,
				},
			},
		},
		{
			name: "suggest with rationale and state_reason",
			requestArgs: map[string]any{
				"owner":         "owner",
				"repo":          "repo",
				"issue_number":  float64(1),
				"state":         "closed",
				"state_reason":  "not_planned",
				"rationale":     "  No activity in 6 months  ",
				"is_suggestion": true,
			},
			expectedReq: map[string]any{
				"state": map[string]any{
					"value":     "closed",
					"rationale": "No activity in 6 months",
					"suggest":   true,
				},
				"state_reason": "not_planned",
			},
		},
		{
			name: "rationale applied directly (no suggestion)",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"state":        "closed",
				"rationale":    "The reported crash is fixed in v2.1",
				"confidence":   "HIGH",
			},
			expectedReq: map[string]any{
				"state": map[string]any{
					"value":      "closed",
					"rationale":  "The reported crash is fixed in v2.1",
					"confidence": "HIGH",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, tc.expectedReq).
					andThen(mockResponse(t, http.StatusOK, &gogithub.Issue{Number: gogithub.Ptr(1)})),
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

func TestGranularUpdateIssueStateDuplicate(t *testing.T) {
	const duplicateIssueID = int64(99999)

	tests := []struct {
		name        string
		requestArgs map[string]any
		expectedReq map[string]any
	}{
		{
			name: "suggestion duplicate close",
			requestArgs: map[string]any{
				"owner":         "owner",
				"repo":          "repo",
				"issue_number":  float64(1),
				"state":         "closed",
				"state_reason":  "duplicate",
				"is_suggestion": true,
				"duplicate_of":  float64(42),
			},
			expectedReq: map[string]any{
				"state": map[string]any{
					"value":   "closed",
					"suggest": true,
				},
				"state_reason":       "duplicate",
				"duplicate_issue_id": float64(duplicateIssueID),
			},
		},
		{
			name: "direct duplicate close",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"state":        "closed",
				"state_reason": "duplicate",
				"duplicate_of": float64(42),
			},
			expectedReq: map[string]any{
				"state":              map[string]any{"value": "closed"},
				"state_reason":       "duplicate",
				"duplicate_issue_id": float64(duplicateIssueID),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, &gogithub.Issue{
					ID:     gogithub.Ptr(duplicateIssueID),
					Number: gogithub.Ptr(42),
				}),
				PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, tc.expectedReq).
					andThen(mockResponse(t, http.StatusOK, &gogithub.Issue{Number: gogithub.Ptr(1)})),
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

func TestGranularUpdateIssueStateInvalidRationale(t *testing.T) {
	tests := []struct {
		name            string
		requestArgs     map[string]any
		expectedErrText string
	}{
		{
			name: "rationale too long",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"state":        "closed",
				"rationale":    strings.Repeat("a", 281),
			},
			expectedErrText: "parameter rationale must be 280 characters or less",
		},
		{
			name: "duplicate_of without state_reason duplicate",
			requestArgs: map[string]any{
				"owner":         "owner",
				"repo":          "repo",
				"issue_number":  float64(1),
				"state":         "closed",
				"state_reason":  "not_planned",
				"is_suggestion": true,
				"duplicate_of":  float64(42),
			},
			expectedErrText: "duplicate_of can only be used when state_reason is 'duplicate'",
		},
		{
			name: "suggestion duplicate without duplicate_of",
			requestArgs: map[string]any{
				"owner":         "owner",
				"repo":          "repo",
				"issue_number":  float64(1),
				"state":         "closed",
				"state_reason":  "duplicate",
				"is_suggestion": true,
			},
			expectedErrText: "duplicate_of is required when suggesting a close as duplicate",
		},
		{
			name: "state_reason with open state",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"state":        "open",
				"state_reason": "completed",
			},
			expectedErrText: "state_reason can only be used when state is 'closed'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(nil))}
			serverTool := GranularUpdateIssueState(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			errorContent := getErrorResult(t, result)
			assert.Contains(t, errorContent.Text, tc.expectedErrText)
		})
	}
}

func TestGranularUpdateIssueStateInvalidConfidence(t *testing.T) {
	deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(nil))}
	serverTool := GranularUpdateIssueState(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(1),
		"state":        "closed",
		"confidence":   "VERY_HIGH",
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)

	errorContent := getErrorResult(t, result)
	assert.Contains(t, errorContent.Text, "confidence must be one of: LOW, MEDIUM, HIGH")
}

// --- Pull request granular tool handler tests ---

func TestGranularUpdatePullRequestTitle(t *testing.T) {
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
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
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
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
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
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
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber: expectRequestBody(t, map[string]any{
			"reviewers":      []any{"user1"},
			"team_reviewers": []any{"team1"},
		}).andThen(mockResponse(t, http.StatusOK, &gogithub.PullRequest{Number: gogithub.Ptr(1)})),
	}))
	deps := BaseDeps{Client: client}
	serverTool := GranularRequestPullRequestReviewers(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":      "owner",
		"repo":       "repo",
		"pullNumber": float64(1),
		"reviewers":  []string{"user1", "owner/team1"},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGranularCreatePullRequestReview(t *testing.T) {
	mockedClient := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			struct {
				Repository struct {
					PullRequest struct {
						ID githubv4.ID
					} `graphql:"pullRequest(number: $prNum)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}{},
			map[string]any{
				"owner": githubv4.String("owner"),
				"repo":  githubv4.String("repo"),
				"prNum": githubv4.Int(1),
			},
			githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{
					"pullRequest": map[string]any{
						"id": "PR_123",
					},
				},
			}),
		),
		githubv4mock.NewMutationMatcher(
			struct {
				AddPullRequestReview struct {
					PullRequestReview struct {
						ID githubv4.ID
					}
				} `graphql:"addPullRequestReview(input: $input)"`
			}{},
			githubv4.AddPullRequestReviewInput{
				PullRequestID: githubv4.ID("PR_123"),
				Body:          githubv4.NewString("LGTM"),
				Event:         githubv4mock.Ptr(githubv4.PullRequestReviewEventApprove),
			},
			nil,
			githubv4mock.DataResponse(map[string]any{}),
		),
	)
	gqlClient := githubv4.NewClient(mockedClient)
	deps := BaseDeps{GQLClient: gqlClient}
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

func TestGranularUpdatePullRequestDraftState(t *testing.T) {
	tests := []struct {
		name                string
		initialDraftState   bool
		requestedDraftState bool
		expectedActionCalls int
		getStatusCode       int
		actionStatusCode    int
		expectError         bool
		expectedErrContains string
	}{
		{name: "convert to draft", initialDraftState: false, requestedDraftState: true, expectedActionCalls: 1, getStatusCode: http.StatusOK, actionStatusCode: http.StatusOK},
		{name: "mark ready for review", initialDraftState: true, requestedDraftState: false, expectedActionCalls: 1, getStatusCode: http.StatusOK, actionStatusCode: http.StatusOK},
		{name: "no-op when already requested state", initialDraftState: true, requestedDraftState: true, expectedActionCalls: 0, getStatusCode: http.StatusOK, actionStatusCode: http.StatusOK},
		{name: "returns error when action endpoint is forbidden", initialDraftState: false, requestedDraftState: true, expectedActionCalls: 1, getStatusCode: http.StatusOK, actionStatusCode: http.StatusForbidden, expectError: true, expectedErrContains: "failed to convert pull request to draft"},
		{name: "returns error when pull request lookup is not found", initialDraftState: false, requestedDraftState: true, expectedActionCalls: 0, getStatusCode: http.StatusNotFound, actionStatusCode: http.StatusOK, expectError: true, expectedErrContains: "failed to get pull request"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actionCalls := 0
			convertToDraftCalls := 0
			readyForReviewCalls := 0
			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposPullsByOwnerByRepoByPullNumber: func(w http.ResponseWriter, _ *http.Request) {
					if tc.getStatusCode != http.StatusOK {
						writeJSONResponse(t, w, tc.getStatusCode, map[string]any{"message": "not found"})
						return
					}
					writeJSONResponse(t, w, http.StatusOK, &gogithub.PullRequest{
						Number: gogithub.Ptr(1),
						Draft:  gogithub.Ptr(tc.initialDraftState),
					})
				},
				PostReposPullsByOwnerByRepoByPullNumberConvertToDraft: func(w http.ResponseWriter, _ *http.Request) {
					actionCalls++
					convertToDraftCalls++
					writeJSONResponse(t, w, tc.actionStatusCode, map[string]any{"message": "forbidden"})
				},
				PostReposPullsByOwnerByRepoByPullNumberReadyForReview: func(w http.ResponseWriter, _ *http.Request) {
					actionCalls++
					readyForReviewCalls++
					writeJSONResponse(t, w, tc.actionStatusCode, map[string]any{"message": "forbidden"})
				},
			}))

			deps := BaseDeps{Client: client}
			serverTool := GranularUpdatePullRequestDraftState(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			request := createMCPRequest(map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(1),
				"draft":      tc.requestedDraftState,
			})
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			if tc.expectError {
				assert.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrContains)
				assert.Equal(t, tc.expectedActionCalls, actionCalls)
				if tc.expectedActionCalls == 1 {
					if tc.requestedDraftState {
						assert.Equal(t, 1, convertToDraftCalls)
						assert.Equal(t, 0, readyForReviewCalls)
					} else {
						assert.Equal(t, 0, convertToDraftCalls)
						assert.Equal(t, 1, readyForReviewCalls)
					}
				} else {
					assert.Equal(t, 0, convertToDraftCalls)
					assert.Equal(t, 0, readyForReviewCalls)
				}
				return
			}
			assert.False(t, result.IsError)
			assert.Equal(t, tc.expectedActionCalls, actionCalls)
			if tc.expectedActionCalls == 1 {
				if tc.requestedDraftState {
					assert.Equal(t, 1, convertToDraftCalls)
					assert.Equal(t, 0, readyForReviewCalls)
				} else {
					assert.Equal(t, 0, convertToDraftCalls)
					assert.Equal(t, 1, readyForReviewCalls)
				}
			} else {
				assert.Equal(t, 0, convertToDraftCalls)
				assert.Equal(t, 0, readyForReviewCalls)
			}
		})
	}
}

func TestGranularAddPullRequestReviewComment(t *testing.T) {
	mockedClient := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			struct {
				Viewer struct {
					Login githubv4.String
				}
			}{},
			nil,
			githubv4mock.DataResponse(map[string]any{
				"viewer": map[string]any{"login": "testuser"},
			}),
		),
		githubv4mock.NewQueryMatcher(
			struct {
				Repository struct {
					PullRequest struct {
						Reviews struct {
							Nodes []struct {
								ID    githubv4.ID
								State githubv4.PullRequestReviewState
								URL   githubv4.URI
							}
						} `graphql:"reviews(first: 1, author: $author)"`
					} `graphql:"pullRequest(number: $prNum)"`
				} `graphql:"repository(owner: $owner, name: $name)"`
			}{},
			map[string]any{
				"author": githubv4.String("testuser"),
				"owner":  githubv4.String("owner"),
				"name":   githubv4.String("repo"),
				"prNum":  githubv4.Int(1),
			},
			githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{
					"pullRequest": map[string]any{
						"reviews": map[string]any{
							"nodes": []map[string]any{
								{"id": "PRR_123", "state": "PENDING", "url": "https://github.com/owner/repo/pull/1#pullrequestreview-123"},
							},
						},
					},
				},
			}),
		),
		githubv4mock.NewMutationMatcher(
			struct {
				AddPullRequestReviewThread struct {
					Thread struct {
						ID githubv4.ID
					}
				} `graphql:"addPullRequestReviewThread(input: $input)"`
			}{},
			githubv4.AddPullRequestReviewThreadInput{
				Path:                githubv4.String("src/main.go"),
				Body:                githubv4.String("This needs a fix"),
				SubjectType:         githubv4mock.Ptr(githubv4.PullRequestReviewThreadSubjectTypeLine),
				Line:                githubv4mock.Ptr(githubv4.Int(42)),
				Side:                githubv4mock.Ptr(githubv4.DiffSideRight),
				PullRequestReviewID: githubv4mock.Ptr(githubv4.ID("PRR_123")),
			},
			nil,
			githubv4mock.DataResponse(map[string]any{
				"addPullRequestReviewThread": map[string]any{
					"thread": map[string]any{"id": "PRRT_456"},
				},
			}),
		),
	)
	gqlClient := githubv4.NewClient(mockedClient)
	deps := BaseDeps{GQLClient: gqlClient}
	serverTool := GranularAddPullRequestReviewComment(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":       "owner",
		"repo":        "repo",
		"pullNumber":  float64(1),
		"path":        "src/main.go",
		"body":        "This needs a fix",
		"subjectType": "LINE",
		"line":        float64(42),
		"side":        "RIGHT",
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGranularResolveReviewThread(t *testing.T) {
	mockedClient := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewMutationMatcher(
			struct {
				ResolveReviewThread struct {
					Thread struct {
						ID         githubv4.ID
						IsResolved githubv4.Boolean
					}
				} `graphql:"resolveReviewThread(input: $input)"`
			}{},
			githubv4.ResolveReviewThreadInput{
				ThreadID: githubv4.ID("PRRT_123"),
			},
			nil,
			githubv4mock.DataResponse(map[string]any{
				"resolveReviewThread": map[string]any{
					"thread": map[string]any{"id": "PRRT_123", "isResolved": true},
				},
			}),
		),
	)
	gqlClient := githubv4.NewClient(mockedClient)
	deps := BaseDeps{GQLClient: gqlClient}
	serverTool := GranularResolveReviewThread(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"threadID": "PRRT_123",
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGranularUnresolveReviewThread(t *testing.T) {
	mockedClient := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewMutationMatcher(
			struct {
				UnresolveReviewThread struct {
					Thread struct {
						ID         githubv4.ID
						IsResolved githubv4.Boolean
					}
				} `graphql:"unresolveReviewThread(input: $input)"`
			}{},
			githubv4.UnresolveReviewThreadInput{
				ThreadID: githubv4.ID("PRRT_123"),
			},
			nil,
			githubv4mock.DataResponse(map[string]any{
				"unresolveReviewThread": map[string]any{
					"thread": map[string]any{"id": "PRRT_123", "isResolved": false},
				},
			}),
		),
	)
	gqlClient := githubv4.NewClient(mockedClient)
	deps := BaseDeps{GQLClient: gqlClient}
	serverTool := GranularUnresolveReviewThread(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"threadID": "PRRT_123",
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGranularSetIssueFields(t *testing.T) {
	t.Run("successful set with text value", func(t *testing.T) {
		matchers := []githubv4mock.Matcher{
			// Mock the issue ID query
			githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						Issue struct {
							ID githubv4.ID
						} `graphql:"issue(number: $issueNumber)"`
					} `graphql:"repository(owner: $owner, name: $repo)"`
				}{},
				map[string]any{
					"owner":       githubv4.String("owner"),
					"repo":        githubv4.String("repo"),
					"issueNumber": githubv4.Int(5),
				},
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"issue": map[string]any{"id": "ISSUE_123"},
					},
				}),
			),
			// Mock the setIssueFieldValue mutation
			githubv4mock.NewMutationMatcher(
				struct {
					SetIssueFieldValue struct {
						Issue struct {
							ID     githubv4.ID
							Number githubv4.Int
							URL    githubv4.String
						}
						IssueFieldValues []struct {
							TextValue struct {
								Value string
							} `graphql:"... on IssueFieldTextValue"`
							SingleSelectValue struct {
								Name string
							} `graphql:"... on IssueFieldSingleSelectValue"`
							DateValue struct {
								Value string
							} `graphql:"... on IssueFieldDateValue"`
							NumberValue struct {
								Value float64
							} `graphql:"... on IssueFieldNumberValue"`
						}
					} `graphql:"setIssueFieldValue(input: $input)"`
				}{},
				SetIssueFieldValueInput{
					IssueID: githubv4.ID("ISSUE_123"),
					IssueFields: []IssueFieldCreateOrUpdateInput{
						{
							FieldID:   githubv4.ID("FIELD_1"),
							TextValue: githubv4.NewString(githubv4.String("hello")),
						},
					},
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"setIssueFieldValue": map[string]any{
						"issue": map[string]any{
							"id":     "ISSUE_123",
							"number": 5,
							"url":    "https://github.com/owner/repo/issues/5",
						},
					},
				}),
			),
		}

		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(matchers...))
		deps := BaseDeps{GQLClient: gqlClient}
		serverTool := GranularSetIssueFields(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(5),
			"fields": []any{
				map[string]any{"field_id": "FIELD_1", "text_value": "hello"},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})

	t.Run("missing required parameter fields", func(t *testing.T) {
		deps := BaseDeps{}
		serverTool := GranularSetIssueFields(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(5),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "missing required parameter: fields")
	})

	t.Run("empty fields array", func(t *testing.T) {
		deps := BaseDeps{}
		serverTool := GranularSetIssueFields(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(5),
			"fields":       []any{},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "fields array must not be empty")
	})

	t.Run("field missing value", func(t *testing.T) {
		deps := BaseDeps{}
		serverTool := GranularSetIssueFields(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(5),
			"fields": []any{
				map[string]any{"field_id": "FIELD_1"},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "each field must have a value")
	})

	t.Run("multiple value keys returns error", func(t *testing.T) {
		deps := BaseDeps{}
		serverTool := GranularSetIssueFields(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(5),
			"fields": []any{
				map[string]any{"field_id": "FIELD_1", "text_value": "hello", "number_value": float64(42)},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "each field must have exactly one value")
	})

	t.Run("value key with delete returns error", func(t *testing.T) {
		deps := BaseDeps{}
		serverTool := GranularSetIssueFields(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(5),
			"fields": []any{
				map[string]any{"field_id": "FIELD_1", "text_value": "hello", "delete": true},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "each field must have exactly one value")
	})

	t.Run("successful set with text value and rationale", func(t *testing.T) {
		matchers := []githubv4mock.Matcher{
			githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						Issue struct {
							ID githubv4.ID
						} `graphql:"issue(number: $issueNumber)"`
					} `graphql:"repository(owner: $owner, name: $repo)"`
				}{},
				map[string]any{
					"owner":       githubv4.String("owner"),
					"repo":        githubv4.String("repo"),
					"issueNumber": githubv4.Int(5),
				},
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"issue": map[string]any{"id": "ISSUE_123"},
					},
				}),
			),
			githubv4mock.NewMutationMatcher(
				struct {
					SetIssueFieldValue struct {
						Issue struct {
							ID     githubv4.ID
							Number githubv4.Int
							URL    githubv4.String
						}
						IssueFieldValues []struct {
							TextValue struct {
								Value string
							} `graphql:"... on IssueFieldTextValue"`
							SingleSelectValue struct {
								Name string
							} `graphql:"... on IssueFieldSingleSelectValue"`
							DateValue struct {
								Value string
							} `graphql:"... on IssueFieldDateValue"`
							NumberValue struct {
								Value float64
							} `graphql:"... on IssueFieldNumberValue"`
						}
					} `graphql:"setIssueFieldValue(input: $input)"`
				}{},
				SetIssueFieldValueInput{
					IssueID: githubv4.ID("ISSUE_123"),
					IssueFields: []IssueFieldCreateOrUpdateInput{
						{
							FieldID:   githubv4.ID("FIELD_1"),
							TextValue: githubv4.NewString(githubv4.String("hello")),
							Rationale: githubv4.NewString(githubv4.String("Reflects the reported severity")),
						},
					},
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"setIssueFieldValue": map[string]any{
						"issue": map[string]any{
							"id":     "ISSUE_123",
							"number": 5,
							"url":    "https://github.com/owner/repo/issues/5",
						},
					},
				}),
			),
		}

		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(matchers...))
		deps := BaseDeps{GQLClient: gqlClient}
		serverTool := GranularSetIssueFields(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(5),
			"fields": []any{
				map[string]any{
					"field_id":   "FIELD_1",
					"text_value": "hello",
					"rationale":  "  Reflects the reported severity  ",
				},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})

	t.Run("rationale too long returns error", func(t *testing.T) {
		deps := BaseDeps{}
		serverTool := GranularSetIssueFields(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(5),
			"fields": []any{
				map[string]any{
					"field_id":   "FIELD_1",
					"text_value": "hello",
					"rationale":  strings.Repeat("a", 281),
				},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "field rationale must be 280 characters or less")
	})

	t.Run("successful set with confidence", func(t *testing.T) {
		confidence := "HIGH"
		matchers := []githubv4mock.Matcher{
			githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						Issue struct {
							ID githubv4.ID
						} `graphql:"issue(number: $issueNumber)"`
					} `graphql:"repository(owner: $owner, name: $repo)"`
				}{},
				map[string]any{
					"owner":       githubv4.String("owner"),
					"repo":        githubv4.String("repo"),
					"issueNumber": githubv4.Int(5),
				},
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"issue": map[string]any{"id": "ISSUE_123"},
					},
				}),
			),
			githubv4mock.NewMutationMatcher(
				struct {
					SetIssueFieldValue struct {
						Issue struct {
							ID     githubv4.ID
							Number githubv4.Int
							URL    githubv4.String
						}
						IssueFieldValues []struct {
							TextValue struct {
								Value string
							} `graphql:"... on IssueFieldTextValue"`
							SingleSelectValue struct {
								Name string
							} `graphql:"... on IssueFieldSingleSelectValue"`
							DateValue struct {
								Value string
							} `graphql:"... on IssueFieldDateValue"`
							NumberValue struct {
								Value float64
							} `graphql:"... on IssueFieldNumberValue"`
						}
					} `graphql:"setIssueFieldValue(input: $input)"`
				}{},
				SetIssueFieldValueInput{
					IssueID: githubv4.ID("ISSUE_123"),
					IssueFields: []IssueFieldCreateOrUpdateInput{
						{
							FieldID:    githubv4.ID("FIELD_1"),
							TextValue:  githubv4.NewString(githubv4.String("hello")),
							Confidence: &confidence,
						},
					},
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"setIssueFieldValue": map[string]any{
						"issue": map[string]any{
							"id":     "ISSUE_123",
							"number": 5,
							"url":    "https://github.com/owner/repo/issues/5",
						},
					},
				}),
			),
		}

		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(matchers...))
		deps := BaseDeps{GQLClient: gqlClient}
		serverTool := GranularSetIssueFields(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(5),
			"fields": []any{
				map[string]any{
					"field_id":   "FIELD_1",
					"text_value": "hello",
					"confidence": " high ",
				},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})

	t.Run("invalid confidence value returns error", func(t *testing.T) {
		deps := BaseDeps{}
		serverTool := GranularSetIssueFields(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(5),
			"fields": []any{
				map[string]any{
					"field_id":   "FIELD_1",
					"text_value": "hello",
					"confidence": "very_high",
				},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "confidence must be one of: LOW, MEDIUM, HIGH")
	})

	t.Run("confidence is sent when supplied", func(t *testing.T) {
		confidence := "HIGH"
		matchers := []githubv4mock.Matcher{
			githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						Issue struct {
							ID githubv4.ID
						} `graphql:"issue(number: $issueNumber)"`
					} `graphql:"repository(owner: $owner, name: $repo)"`
				}{},
				map[string]any{
					"owner":       githubv4.String("owner"),
					"repo":        githubv4.String("repo"),
					"issueNumber": githubv4.Int(5),
				},
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"issue": map[string]any{"id": "ISSUE_123"},
					},
				}),
			),
			githubv4mock.NewMutationMatcher(
				struct {
					SetIssueFieldValue struct {
						Issue struct {
							ID     githubv4.ID
							Number githubv4.Int
							URL    githubv4.String
						}
						IssueFieldValues []struct {
							TextValue struct {
								Value string
							} `graphql:"... on IssueFieldTextValue"`
							SingleSelectValue struct {
								Name string
							} `graphql:"... on IssueFieldSingleSelectValue"`
							DateValue struct {
								Value string
							} `graphql:"... on IssueFieldDateValue"`
							NumberValue struct {
								Value float64
							} `graphql:"... on IssueFieldNumberValue"`
						}
					} `graphql:"setIssueFieldValue(input: $input)"`
				}{},
				SetIssueFieldValueInput{
					IssueID: githubv4.ID("ISSUE_123"),
					IssueFields: []IssueFieldCreateOrUpdateInput{
						{
							FieldID:    githubv4.ID("FIELD_1"),
							TextValue:  githubv4.NewString(githubv4.String("hello")),
							Confidence: &confidence,
						},
					},
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"setIssueFieldValue": map[string]any{
						"issue": map[string]any{
							"id":     "ISSUE_123",
							"number": 5,
							"url":    "https://github.com/owner/repo/issues/5",
						},
					},
				}),
			),
		}

		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(matchers...))
		deps := BaseDeps{GQLClient: gqlClient}
		serverTool := GranularSetIssueFields(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(5),
			"fields": []any{
				map[string]any{
					"field_id":   "FIELD_1",
					"text_value": "hello",
					"confidence": "HIGH",
				},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.False(t, result.IsError, getTextResult(t, result).Text)
	})

	t.Run("successful set with suggest flag", func(t *testing.T) {
		suggestTrue := githubv4.Boolean(true)
		matchers := []githubv4mock.Matcher{
			githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						Issue struct {
							ID githubv4.ID
						} `graphql:"issue(number: $issueNumber)"`
					} `graphql:"repository(owner: $owner, name: $repo)"`
				}{},
				map[string]any{
					"owner":       githubv4.String("owner"),
					"repo":        githubv4.String("repo"),
					"issueNumber": githubv4.Int(5),
				},
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"issue": map[string]any{"id": "ISSUE_123"},
					},
				}),
			),
			githubv4mock.NewMutationMatcher(
				struct {
					SetIssueFieldValue struct {
						Issue struct {
							ID     githubv4.ID
							Number githubv4.Int
							URL    githubv4.String
						}
						IssueFieldValues []struct {
							TextValue struct {
								Value string
							} `graphql:"... on IssueFieldTextValue"`
							SingleSelectValue struct {
								Name string
							} `graphql:"... on IssueFieldSingleSelectValue"`
							DateValue struct {
								Value string
							} `graphql:"... on IssueFieldDateValue"`
							NumberValue struct {
								Value float64
							} `graphql:"... on IssueFieldNumberValue"`
						}
					} `graphql:"setIssueFieldValue(input: $input)"`
				}{},
				SetIssueFieldValueInput{
					IssueID: githubv4.ID("ISSUE_123"),
					IssueFields: []IssueFieldCreateOrUpdateInput{
						{
							FieldID:   githubv4.ID("FIELD_1"),
							TextValue: githubv4.NewString(githubv4.String("hello")),
							Rationale: githubv4.NewString(githubv4.String("Reflects the reported severity")),
							Suggest:   &suggestTrue,
						},
					},
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"setIssueFieldValue": map[string]any{
						"issue": map[string]any{
							"id":     "ISSUE_123",
							"number": 5,
							"url":    "https://github.com/owner/repo/issues/5",
						},
					},
				}),
			),
		}

		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(matchers...))
		deps := BaseDeps{GQLClient: gqlClient}
		serverTool := GranularSetIssueFields(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(5),
			"fields": []any{
				map[string]any{
					"field_id":      "FIELD_1",
					"text_value":    "hello",
					"rationale":     "Reflects the reported severity",
					"is_suggestion": true,
				},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})

	t.Run("sends GraphQL-Features: update_issue_suggestions header on mutation", func(t *testing.T) {
		matchers := []githubv4mock.Matcher{
			githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						Issue struct {
							ID githubv4.ID
						} `graphql:"issue(number: $issueNumber)"`
					} `graphql:"repository(owner: $owner, name: $repo)"`
				}{},
				map[string]any{
					"owner":       githubv4.String("owner"),
					"repo":        githubv4.String("repo"),
					"issueNumber": githubv4.Int(5),
				},
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"issue": map[string]any{"id": "ISSUE_123"},
					},
				}),
			),
			githubv4mock.NewMutationMatcher(
				struct {
					SetIssueFieldValue struct {
						Issue struct {
							ID     githubv4.ID
							Number githubv4.Int
							URL    githubv4.String
						}
						IssueFieldValues []struct {
							TextValue struct {
								Value string
							} `graphql:"... on IssueFieldTextValue"`
							SingleSelectValue struct {
								Name string
							} `graphql:"... on IssueFieldSingleSelectValue"`
							DateValue struct {
								Value string
							} `graphql:"... on IssueFieldDateValue"`
							NumberValue struct {
								Value float64
							} `graphql:"... on IssueFieldNumberValue"`
						}
					} `graphql:"setIssueFieldValue(input: $input)"`
				}{},
				SetIssueFieldValueInput{
					IssueID: githubv4.ID("ISSUE_123"),
					IssueFields: []IssueFieldCreateOrUpdateInput{
						{
							FieldID:   githubv4.ID("FIELD_1"),
							TextValue: githubv4.NewString(githubv4.String("hello")),
						},
					},
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"setIssueFieldValue": map[string]any{
						"issue": map[string]any{
							"id":     "ISSUE_123",
							"number": 5,
							"url":    "https://github.com/owner/repo/issues/5",
						},
					},
				}),
			),
		}

		// Build a transport chain matching production: GraphQLFeaturesTransport
		// wraps a header-capturing spy, which forwards to the mock's RoundTripper.
		// This verifies the mutation request sets the update_issue_suggestions
		// feature flag so the rationale/suggest input fields are accepted.
		mockClient := githubv4mock.NewMockedHTTPClient(matchers...)
		spy := &headerCaptureTransport{inner: mockClient.Transport}
		httpClient := &http.Client{
			Transport: &transportpkg.GraphQLFeaturesTransport{Transport: spy},
		}
		gqlClient := githubv4.NewClient(httpClient)
		deps := BaseDeps{GQLClient: gqlClient}
		serverTool := GranularSetIssueFields(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(5),
			"fields": []any{
				map[string]any{"field_id": "FIELD_1", "text_value": "hello"},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, getTextResult(t, result).Text)
		// The last request captured is the mutation; the preceding issue ID
		// query does not require the feature flag.
		assert.Equal(t, "update_issue_suggestions", spy.captured.Get(headers.GraphQLFeaturesHeader))
	})
}

// --- Reaction granular tool handler tests ---

func TestGranularAddIssueReaction(t *testing.T) {
	mockReaction := &gogithub.Reaction{
		ID:      gogithub.Ptr(int64(12345)),
		Content: gogithub.Ptr("+1"),
	}

	tests := []struct {
		name         string
		mockedClient *http.Client
		args         map[string]any
		expectErr    bool
	}{
		{
			name: "add reaction to issue successfully",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesReactionsByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusCreated, mockReaction),
			}),
			args: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"content":      "+1",
			},
			expectErr: false,
		},
		{
			name:         "missing owner returns error",
			mockedClient: MockHTTPClientWithHandlers(nil),
			args: map[string]any{
				"repo":         "repo",
				"issue_number": float64(42),
				"content":      "+1",
			},
			expectErr: true,
		},
		{
			name:         "missing content returns error",
			mockedClient: MockHTTPClientWithHandlers(nil),
			args: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, tc.mockedClient)
			deps := BaseDeps{Client: client}
			serverTool := GranularAddIssueReaction(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.args)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			if tc.expectErr {
				assert.True(t, result.IsError)
			} else {
				assert.False(t, result.IsError)
				textContent := getTextResult(t, result)
				var response MinimalResponse
				require.NoError(t, json.Unmarshal([]byte(textContent.Text), &response))
				assert.Equal(t, "12345", response.ID)
				assert.Equal(t, "https://api.github.com/repos/owner/repo/issues/42/reactions/12345", response.URL)
			}
		})
	}
}

func TestGranularAddIssueCommentReaction(t *testing.T) {
	mockReaction := &gogithub.Reaction{
		ID:      gogithub.Ptr(int64(67890)),
		Content: gogithub.Ptr("heart"),
	}

	tests := []struct {
		name         string
		mockedClient *http.Client
		args         map[string]any
		expectErr    bool
	}{
		{
			name: "add reaction to issue comment successfully",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesCommentsReactionsByOwnerByRepoByCommentID: mockResponse(t, http.StatusCreated, mockReaction),
			}),
			args: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"comment_id": float64(999),
				"content":    "heart",
			},
			expectErr: false,
		},
		{
			name:         "missing comment_id returns error",
			mockedClient: MockHTTPClientWithHandlers(nil),
			args: map[string]any{
				"owner":   "owner",
				"repo":    "repo",
				"content": "heart",
			},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, tc.mockedClient)
			deps := BaseDeps{Client: client}
			serverTool := GranularAddIssueCommentReaction(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.args)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			if tc.expectErr {
				assert.True(t, result.IsError)
			} else {
				assert.False(t, result.IsError)
				textContent := getTextResult(t, result)
				var response MinimalResponse
				require.NoError(t, json.Unmarshal([]byte(textContent.Text), &response))
				assert.Equal(t, "67890", response.ID)
				assert.Equal(t, "https://api.github.com/repos/owner/repo/issues/comments/999/reactions/67890", response.URL)
			}
		})
	}
}

func TestGranularAddPullRequestReviewCommentReaction(t *testing.T) {
	mockReaction := &gogithub.Reaction{
		ID:      gogithub.Ptr(int64(54321)),
		Content: gogithub.Ptr("rocket"),
	}

	tests := []struct {
		name         string
		mockedClient *http.Client
		args         map[string]any
		expectErr    bool
	}{
		{
			name: "add reaction to PR review comment successfully",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposPullsCommentsReactionsByOwnerByRepoByCommentID: mockResponse(t, http.StatusCreated, mockReaction),
			}),
			args: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"comment_id": float64(888),
				"content":    "rocket",
			},
			expectErr: false,
		},
		{
			name:         "missing repo returns error",
			mockedClient: MockHTTPClientWithHandlers(nil),
			args: map[string]any{
				"owner":      "owner",
				"comment_id": float64(888),
				"content":    "rocket",
			},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, tc.mockedClient)
			deps := BaseDeps{Client: client}
			serverTool := GranularAddPullRequestReviewCommentReaction(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.args)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			if tc.expectErr {
				assert.True(t, result.IsError)
			} else {
				assert.False(t, result.IsError)
				textContent := getTextResult(t, result)
				var response MinimalResponse
				require.NoError(t, json.Unmarshal([]byte(textContent.Text), &response))
				assert.Equal(t, "54321", response.ID)
				assert.Equal(t, "https://api.github.com/repos/owner/repo/pulls/comments/888/reactions/54321", response.URL)
			}
		})
	}
}
