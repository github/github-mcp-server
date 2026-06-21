package github

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/http/headers"
	transportpkg "github.com/github/github-mcp-server/pkg/http/transport"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v87/github"
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
		{
			name: "label names are trimmed for the replace operation",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"labels": []any{
					"  bug  ",
					map[string]any{"name": "  enhancement  "},
				},
			},
			expectedReq: map[string]any{
				"labels": []any{"bug", "enhancement"},
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
		{
			name: "whitespace-only string label rejected",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"labels":       []any{"   "},
			},
			expectedErrText: "label name cannot be empty",
		},
		{
			name: "whitespace-only object name rejected",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"labels": []any{
					map[string]any{"name": "   "},
				},
			},
			expectedErrText: "label name cannot be empty",
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

func TestGranularUpdateIssueLabelsAddOperation(t *testing.T) {
	mockLabels := []*gogithub.Label{
		{Name: gogithub.Ptr("bug")},
		{Name: gogithub.Ptr("enhancement")},
	}

	t.Run("adds labels without replacing existing ones", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			PostReposIssuesLabelsByOwnerByRepoByIssueNumber: expectRequestBody(t, []any{"enhancement"}).
				andThen(mockResponse(t, http.StatusOK, mockLabels)),
		}))
		deps := BaseDeps{Client: client}
		serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(1),
			"method":       "add",
			"labels":       []any{"enhancement"},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "bug")
		assert.Contains(t, textContent.Text, "enhancement")
	})

	t.Run("accepts label objects and uses their names", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			PostReposIssuesLabelsByOwnerByRepoByIssueNumber: expectRequestBody(t, []any{"enhancement"}).
				andThen(mockResponse(t, http.StatusOK, mockLabels)),
		}))
		deps := BaseDeps{Client: client}
		serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(1),
			"method":       "add",
			"labels":       []any{map[string]any{"name": "enhancement", "rationale": "ignored for add"}},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
	})

	t.Run("de-duplicates labels before sending the request", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			PostReposIssuesLabelsByOwnerByRepoByIssueNumber: expectRequestBody(t, []any{"enhancement"}).
				andThen(mockResponse(t, http.StatusOK, mockLabels)),
		}))
		deps := BaseDeps{Client: client}
		serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(1),
			"method":       "add",
			"labels":       []any{"enhancement", "enhancement", " enhancement "},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
	})

	t.Run("requires at least one label", func(t *testing.T) {
		deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(nil))}
		serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(1),
			"method":       "add",
			"labels":       []any{},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)

		errorContent := getErrorResult(t, result)
		assert.Contains(t, errorContent.Text, "at least one label is required")
	})
}

func TestGranularUpdateIssueLabelsRemoveOperation(t *testing.T) {
	t.Run("removes a single label and returns remaining labels", func(t *testing.T) {
		remaining := []*gogithub.Label{{Name: gogithub.Ptr("enhancement")}}
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			DeleteReposIssuesLabelsByOwnerByRepoByIssueNumberByName: mockResponse(t, http.StatusOK, remaining),
		}))
		deps := BaseDeps{Client: client}
		serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(1),
			"method":       "remove",
			"labels":       []any{"bug"},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "enhancement")
		assert.NotContains(t, textContent.Text, "bug")
	})

	t.Run("requires at least one label", func(t *testing.T) {
		deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(nil))}
		serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(1),
			"method":       "remove",
			"labels":       []any{},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)

		errorContent := getErrorResult(t, result)
		assert.Contains(t, errorContent.Text, "at least one label is required")
	})

	t.Run("surfaces API error when label is not present", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			DeleteReposIssuesLabelsByOwnerByRepoByIssueNumberByName: mockResponse(t, http.StatusNotFound, map[string]any{
				"message": "Label does not exist",
			}),
		}))
		deps := BaseDeps{Client: client}
		serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(1),
			"method":       "remove",
			"labels":       []any{"nonexistent"},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)

		errorContent := getErrorResult(t, result)
		assert.Contains(t, errorContent.Text, "failed to remove label from issue")
	})

	t.Run("rejects whitespace-only label names", func(t *testing.T) {
		deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(nil))}
		serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(1),
			"method":       "remove",
			"labels":       []any{"   "},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)

		errorContent := getErrorResult(t, result)
		assert.Contains(t, errorContent.Text, "label name cannot be empty")
	})

	t.Run("de-duplicates labels before issuing deletes", func(t *testing.T) {
		var deleteCount int
		remaining := []*gogithub.Label{{Name: gogithub.Ptr("enhancement")}}
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			DeleteReposIssuesLabelsByOwnerByRepoByIssueNumberByName: func(w http.ResponseWriter, r *http.Request) {
				deleteCount++
				mockResponse(t, http.StatusOK, remaining)(w, r)
			},
		}))
		deps := BaseDeps{Client: client}
		serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(1),
			"method":       "remove",
			"labels":       []any{"bug", "bug", " bug "},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		assert.Equal(t, 1, deleteCount, "duplicate labels should collapse to a single DELETE")
	})

	t.Run("escapes multi-word label names in the request path", func(t *testing.T) {
		var capturedPath string
		remaining := []*gogithub.Label{{Name: gogithub.Ptr("enhancement")}}
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			DeleteReposIssuesLabelsByOwnerByRepoByIssueNumberByName: func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.EscapedPath()
				mockResponse(t, http.StatusOK, remaining)(w, r)
			},
		}))
		deps := BaseDeps{Client: client}
		serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(1),
			"method":       "remove",
			"labels":       []any{"good first issue"},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		assert.Contains(t, capturedPath, "labels/good%20first%20issue")
	})

	t.Run("escapes slashes in label names in the request path", func(t *testing.T) {
		// A label like "status/blocked" must stay a single, escaped path
		// segment so the slash is not interpreted as a path separator. A
		// catch-all handler is used here (rather than the keyed router used
		// above) because the mock routes on the decoded path, where %2F would
		// look like an extra segment.
		var capturedPath string
		remaining := []*gogithub.Label{{Name: gogithub.Ptr("enhancement")}}
		client := mustNewGHClient(t, MockHTTPClientWithHandler(func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.EscapedPath()
			mockResponse(t, http.StatusOK, remaining)(w, r)
		}))
		deps := BaseDeps{Client: client}
		serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(1),
			"method":       "remove",
			"labels":       []any{"status/blocked"},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		assert.Contains(t, capturedPath, "labels/status%2Fblocked")
	})
}

func TestGranularUpdateIssueLabelsInvalidOperation(t *testing.T) {
	deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(nil))}
	serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(1),
		"method":       "toggle",
		"labels":       []any{"bug"},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)

	errorContent := getErrorResult(t, result)
	assert.Contains(t, errorContent.Text, "method must be one of: replace, add, remove")
}

func TestGranularUpdateIssueLabelsReplaceReturnsLabels(t *testing.T) {
	issue := &gogithub.Issue{
		Number: gogithub.Ptr(1),
		Labels: []*gogithub.Label{
			{Name: gogithub.Ptr("bug")},
			{Name: gogithub.Ptr("enhancement")},
		},
	}
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, issue),
	}))
	deps := BaseDeps{Client: client}
	serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(1),
		"method":       "replace",
		"labels":       []any{"bug", "enhancement"},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	// All methods return the resulting label set for a consistent response shape.
	textContent := getTextResult(t, result)
	assert.Contains(t, textContent.Text, "bug")
	assert.Contains(t, textContent.Text, "enhancement")
}

func TestGranularUpdateIssueLabelsOperationNormalization(t *testing.T) {
	t.Run("operation is trimmed and case-folded to add", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			PostReposIssuesLabelsByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, []*gogithub.Label{
				{Name: gogithub.Ptr("bug")},
			}),
		}))
		deps := BaseDeps{Client: client}
		serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(1),
			"method":       "  ADD  ",
			"labels":       []any{"bug"},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
	})

	t.Run("omitted operation defaults to replace", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			PatchReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, &gogithub.Issue{Number: gogithub.Ptr(1)}),
		}))
		deps := BaseDeps{Client: client}
		serverTool := GranularUpdateIssueLabels(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(1),
			"labels":       []any{"bug"},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
	})
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
		name  string
		draft bool
	}{
		{name: "convert to draft", draft: true},
		{name: "mark ready for review", draft: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var matchers []githubv4mock.Matcher

			matchers = append(matchers, githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						PullRequest struct {
							ID githubv4.ID
						} `graphql:"pullRequest(number: $number)"`
					} `graphql:"repository(owner: $owner, name: $name)"`
				}{},
				map[string]any{
					"owner":  githubv4.String("owner"),
					"name":   githubv4.String("repo"),
					"number": githubv4.Int(1),
				},
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"pullRequest": map[string]any{"id": "PR_123"},
					},
				}),
			))

			if tc.draft {
				matchers = append(matchers, githubv4mock.NewMutationMatcher(
					struct {
						ConvertPullRequestToDraft struct {
							PullRequest struct {
								ID      githubv4.ID
								IsDraft githubv4.Boolean
							}
						} `graphql:"convertPullRequestToDraft(input: $input)"`
					}{},
					githubv4.ConvertPullRequestToDraftInput{PullRequestID: githubv4.ID("PR_123")},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"convertPullRequestToDraft": map[string]any{
							"pullRequest": map[string]any{"id": "PR_123", "isDraft": true},
						},
					}),
				))
			} else {
				matchers = append(matchers, githubv4mock.NewMutationMatcher(
					struct {
						MarkPullRequestReadyForReview struct {
							PullRequest struct {
								ID      githubv4.ID
								IsDraft githubv4.Boolean
							}
						} `graphql:"markPullRequestReadyForReview(input: $input)"`
					}{},
					githubv4.MarkPullRequestReadyForReviewInput{PullRequestID: githubv4.ID("PR_123")},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"markPullRequestReadyForReview": map[string]any{
							"pullRequest": map[string]any{"id": "PR_123", "isDraft": false},
						},
					}),
				))
			}

			gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(matchers...))
			deps := BaseDeps{GQLClient: gqlClient}
			serverTool := GranularUpdatePullRequestDraftState(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			request := createMCPRequest(map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(1),
				"draft":      tc.draft,
			})
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			assert.False(t, result.IsError)
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
