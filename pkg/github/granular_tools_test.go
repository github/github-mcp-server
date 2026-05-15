package github

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v82/github"
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
			client := gogithub.NewClient(MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
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
			deps := BaseDeps{Client: gogithub.NewClient(MockHTTPClientWithHandlers(nil))}
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
}
