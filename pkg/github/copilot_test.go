package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-github/v82/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
)

func TestAssignCopilotToIssue(t *testing.T) {
	t.Parallel()

	// Verify tool definition
	serverTool := AssignCopilotToIssue(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "assign_copilot_to_issue", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "issue_number")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "base_ref")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "custom_instructions")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"owner", "repo", "issue_number"})

	// Helper function to create pointer to githubv4.String
	ptrGitHubv4String := func(s string) *githubv4.String {
		v := githubv4.String(s)
		return &v
	}

	pageOfFakeBots := func(n int) []struct{} {
		// We don't _really_ need real bots here, just objects that count as entries for the page
		bots := make([]struct{}, n)
		for i := range n {
			bots[i] = struct{}{}
		}
		return bots
	}

	tests := []struct {
		name               string
		requestArgs        map[string]any
		mockedClient       *http.Client
		expectToolError    bool
		expectedToolErrMsg string
	}{
		{
			name: "successful assignment when there are no existing assignees",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							SuggestedActors struct {
								Nodes []struct {
									Bot struct {
										ID       githubv4.ID
										Login    githubv4.String
										TypeName string `graphql:"__typename"`
									} `graphql:"... on Bot"`
								}
								PageInfo struct {
									HasNextPage bool
									EndCursor   string
								}
							} `graphql:"suggestedActors(first: 100, after: $endCursor, capabilities: CAN_BE_ASSIGNED)"`
						} `graphql:"repository(owner: $owner, name: $name)"`
					}{},
					map[string]any{
						"owner":     githubv4.String("owner"),
						"name":      githubv4.String("repo"),
						"endCursor": (*githubv4.String)(nil),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"suggestedActors": map[string]any{
								"nodes": []any{
									map[string]any{
										"id":         githubv4.ID("copilot-swe-agent-id"),
										"login":      githubv4.String("copilot-swe-agent"),
										"__typename": "Bot",
									},
								},
							},
						},
					}),
				),
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							ID    githubv4.ID
							Issue struct {
								ID        githubv4.ID
								Assignees struct {
									Nodes []struct {
										ID githubv4.ID
									}
								} `graphql:"assignees(first: 100)"`
							} `graphql:"issue(number: $number)"`
						} `graphql:"repository(owner: $owner, name: $name)"`
					}{},
					map[string]any{
						"owner":  githubv4.String("owner"),
						"name":   githubv4.String("repo"),
						"number": githubv4.Int(123),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"id": githubv4.ID("test-repo-id"),
							"issue": map[string]any{
								"id": githubv4.ID("test-issue-id"),
								"assignees": map[string]any{
									"nodes": []any{},
								},
							},
						},
					}),
				),
				githubv4mock.NewMutationMatcher(
					struct {
						UpdateIssue struct {
							Issue struct {
								ID     githubv4.ID
								Number githubv4.Int
								URL    githubv4.String
							}
						} `graphql:"updateIssue(input: $input)"`
					}{},
					UpdateIssueInput{
						ID:          githubv4.ID("test-issue-id"),
						AssigneeIDs: []githubv4.ID{githubv4.ID("copilot-swe-agent-id")},
						AgentAssignment: &AgentAssignmentInput{
							BaseRef:            nil,
							CustomAgent:        ptrGitHubv4String(""),
							CustomInstructions: ptrGitHubv4String(""),
							TargetRepositoryID: githubv4.ID("test-repo-id"),
						},
					},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"updateIssue": map[string]any{
							"issue": map[string]any{
								"id":     githubv4.ID("test-issue-id"),
								"number": githubv4.Int(123),
								"url":    githubv4.String("https://github.com/owner/repo/issues/123"),
							},
						},
					}),
				),
			),
		},
		{
			name: "successful assignment when there are existing assignees",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							SuggestedActors struct {
								Nodes []struct {
									Bot struct {
										ID       githubv4.ID
										Login    githubv4.String
										TypeName string `graphql:"__typename"`
									} `graphql:"... on Bot"`
								}
								PageInfo struct {
									HasNextPage bool
									EndCursor   string
								}
							} `graphql:"suggestedActors(first: 100, after: $endCursor, capabilities: CAN_BE_ASSIGNED)"`
						} `graphql:"repository(owner: $owner, name: $name)"`
					}{},
					map[string]any{
						"owner":     githubv4.String("owner"),
						"name":      githubv4.String("repo"),
						"endCursor": (*githubv4.String)(nil),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"suggestedActors": map[string]any{
								"nodes": []any{
									map[string]any{
										"id":         githubv4.ID("copilot-swe-agent-id"),
										"login":      githubv4.String("copilot-swe-agent"),
										"__typename": "Bot",
									},
								},
							},
						},
					}),
				),
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							ID    githubv4.ID
							Issue struct {
								ID        githubv4.ID
								Assignees struct {
									Nodes []struct {
										ID githubv4.ID
									}
								} `graphql:"assignees(first: 100)"`
							} `graphql:"issue(number: $number)"`
						} `graphql:"repository(owner: $owner, name: $name)"`
					}{},
					map[string]any{
						"owner":  githubv4.String("owner"),
						"name":   githubv4.String("repo"),
						"number": githubv4.Int(123),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"id": githubv4.ID("test-repo-id"),
							"issue": map[string]any{
								"id": githubv4.ID("test-issue-id"),
								"assignees": map[string]any{
									"nodes": []any{
										map[string]any{
											"id": githubv4.ID("existing-assignee-id"),
										},
										map[string]any{
											"id": githubv4.ID("existing-assignee-id-2"),
										},
									},
								},
							},
						},
					}),
				),
				githubv4mock.NewMutationMatcher(
					struct {
						UpdateIssue struct {
							Issue struct {
								ID     githubv4.ID
								Number githubv4.Int
								URL    githubv4.String
							}
						} `graphql:"updateIssue(input: $input)"`
					}{},
					UpdateIssueInput{
						ID: githubv4.ID("test-issue-id"),
						AssigneeIDs: []githubv4.ID{
							githubv4.ID("existing-assignee-id"),
							githubv4.ID("existing-assignee-id-2"),
							githubv4.ID("copilot-swe-agent-id"),
						},
						AgentAssignment: &AgentAssignmentInput{
							BaseRef:            nil,
							CustomAgent:        ptrGitHubv4String(""),
							CustomInstructions: ptrGitHubv4String(""),
							TargetRepositoryID: githubv4.ID("test-repo-id"),
						},
					},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"updateIssue": map[string]any{
							"issue": map[string]any{
								"id":     githubv4.ID("test-issue-id"),
								"number": githubv4.Int(123),
								"url":    githubv4.String("https://github.com/owner/repo/issues/123"),
							},
						},
					}),
				),
			),
		},
		{
			name: "copilot bot not on first page of suggested actors",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				// First page of suggested actors
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							SuggestedActors struct {
								Nodes []struct {
									Bot struct {
										ID       githubv4.ID
										Login    githubv4.String
										TypeName string `graphql:"__typename"`
									} `graphql:"... on Bot"`
								}
								PageInfo struct {
									HasNextPage bool
									EndCursor   string
								}
							} `graphql:"suggestedActors(first: 100, after: $endCursor, capabilities: CAN_BE_ASSIGNED)"`
						} `graphql:"repository(owner: $owner, name: $name)"`
					}{},
					map[string]any{
						"owner":     githubv4.String("owner"),
						"name":      githubv4.String("repo"),
						"endCursor": (*githubv4.String)(nil),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"suggestedActors": map[string]any{
								"nodes": pageOfFakeBots(100),
								"pageInfo": map[string]any{
									"hasNextPage": true,
									"endCursor":   githubv4.String("next-page-cursor"),
								},
							},
						},
					}),
				),
				// Second page of suggested actors
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							SuggestedActors struct {
								Nodes []struct {
									Bot struct {
										ID       githubv4.ID
										Login    githubv4.String
										TypeName string `graphql:"__typename"`
									} `graphql:"... on Bot"`
								}
								PageInfo struct {
									HasNextPage bool
									EndCursor   string
								}
							} `graphql:"suggestedActors(first: 100, after: $endCursor, capabilities: CAN_BE_ASSIGNED)"`
						} `graphql:"repository(owner: $owner, name: $name)"`
					}{},
					map[string]any{
						"owner":     githubv4.String("owner"),
						"name":      githubv4.String("repo"),
						"endCursor": githubv4.String("next-page-cursor"),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"suggestedActors": map[string]any{
								"nodes": []any{
									map[string]any{
										"id":         githubv4.ID("copilot-swe-agent-id"),
										"login":      githubv4.String("copilot-swe-agent"),
										"__typename": "Bot",
									},
								},
							},
						},
					}),
				),
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							ID    githubv4.ID
							Issue struct {
								ID        githubv4.ID
								Assignees struct {
									Nodes []struct {
										ID githubv4.ID
									}
								} `graphql:"assignees(first: 100)"`
							} `graphql:"issue(number: $number)"`
						} `graphql:"repository(owner: $owner, name: $name)"`
					}{},
					map[string]any{
						"owner":  githubv4.String("owner"),
						"name":   githubv4.String("repo"),
						"number": githubv4.Int(123),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"id": githubv4.ID("test-repo-id"),
							"issue": map[string]any{
								"id": githubv4.ID("test-issue-id"),
								"assignees": map[string]any{
									"nodes": []any{},
								},
							},
						},
					}),
				),
				githubv4mock.NewMutationMatcher(
					struct {
						UpdateIssue struct {
							Issue struct {
								ID     githubv4.ID
								Number githubv4.Int
								URL    githubv4.String
							}
						} `graphql:"updateIssue(input: $input)"`
					}{},
					UpdateIssueInput{
						ID:          githubv4.ID("test-issue-id"),
						AssigneeIDs: []githubv4.ID{githubv4.ID("copilot-swe-agent-id")},
						AgentAssignment: &AgentAssignmentInput{
							BaseRef:            nil,
							CustomAgent:        ptrGitHubv4String(""),
							CustomInstructions: ptrGitHubv4String(""),
							TargetRepositoryID: githubv4.ID("test-repo-id"),
						},
					},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"updateIssue": map[string]any{
							"issue": map[string]any{
								"id":     githubv4.ID("test-issue-id"),
								"number": githubv4.Int(123),
								"url":    githubv4.String("https://github.com/owner/repo/issues/123"),
							},
						},
					}),
				),
			),
		},
		{
			name: "copilot not a suggested actor",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							SuggestedActors struct {
								Nodes []struct {
									Bot struct {
										ID       githubv4.ID
										Login    githubv4.String
										TypeName string `graphql:"__typename"`
									} `graphql:"... on Bot"`
								}
								PageInfo struct {
									HasNextPage bool
									EndCursor   string
								}
							} `graphql:"suggestedActors(first: 100, after: $endCursor, capabilities: CAN_BE_ASSIGNED)"`
						} `graphql:"repository(owner: $owner, name: $name)"`
					}{},
					map[string]any{
						"owner":     githubv4.String("owner"),
						"name":      githubv4.String("repo"),
						"endCursor": (*githubv4.String)(nil),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"suggestedActors": map[string]any{
								"nodes": []any{},
							},
						},
					}),
				),
			),
			expectToolError:    true,
			expectedToolErrMsg: "copilot isn't available as an assignee for this issue. Please inform the user to visit https://docs.github.com/en/copilot/using-github-copilot/using-copilot-coding-agent-to-work-on-tasks/about-assigning-tasks-to-copilot for more information.",
		},
		{
			name: "successful assignment with base_ref specified",
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
				"base_ref":     "feature-branch",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							SuggestedActors struct {
								Nodes []struct {
									Bot struct {
										ID       githubv4.ID
										Login    githubv4.String
										TypeName string `graphql:"__typename"`
									} `graphql:"... on Bot"`
								}
								PageInfo struct {
									HasNextPage bool
									EndCursor   string
								}
							} `graphql:"suggestedActors(first: 100, after: $endCursor, capabilities: CAN_BE_ASSIGNED)"`
						} `graphql:"repository(owner: $owner, name: $name)"`
					}{},
					map[string]any{
						"owner":     githubv4.String("owner"),
						"name":      githubv4.String("repo"),
						"endCursor": (*githubv4.String)(nil),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"suggestedActors": map[string]any{
								"nodes": []any{
									map[string]any{
										"id":         githubv4.ID("copilot-swe-agent-id"),
										"login":      githubv4.String("copilot-swe-agent"),
										"__typename": "Bot",
									},
								},
							},
						},
					}),
				),
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							ID    githubv4.ID
							Issue struct {
								ID        githubv4.ID
								Assignees struct {
									Nodes []struct {
										ID githubv4.ID
									}
								} `graphql:"assignees(first: 100)"`
							} `graphql:"issue(number: $number)"`
						} `graphql:"repository(owner: $owner, name: $name)"`
					}{},
					map[string]any{
						"owner":  githubv4.String("owner"),
						"name":   githubv4.String("repo"),
						"number": githubv4.Int(123),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"id": githubv4.ID("test-repo-id"),
							"issue": map[string]any{
								"id": githubv4.ID("test-issue-id"),
								"assignees": map[string]any{
									"nodes": []any{},
								},
							},
						},
					}),
				),
				githubv4mock.NewMutationMatcher(
					struct {
						UpdateIssue struct {
							Issue struct {
								ID     githubv4.ID
								Number githubv4.Int
								URL    githubv4.String
							}
						} `graphql:"updateIssue(input: $input)"`
					}{},
					UpdateIssueInput{
						ID:          githubv4.ID("test-issue-id"),
						AssigneeIDs: []githubv4.ID{githubv4.ID("copilot-swe-agent-id")},
						AgentAssignment: &AgentAssignmentInput{
							BaseRef:            ptrGitHubv4String("feature-branch"),
							CustomAgent:        ptrGitHubv4String(""),
							CustomInstructions: ptrGitHubv4String(""),
							TargetRepositoryID: githubv4.ID("test-repo-id"),
						},
					},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"updateIssue": map[string]any{
							"issue": map[string]any{
								"id":     githubv4.ID("test-issue-id"),
								"number": githubv4.Int(123),
								"url":    githubv4.String("https://github.com/owner/repo/issues/123"),
							},
						},
					}),
				),
			),
		},
		{
			name: "successful assignment with custom_instructions specified",
			requestArgs: map[string]any{
				"owner":               "owner",
				"repo":                "repo",
				"issue_number":        float64(123),
				"custom_instructions": "Please ensure all code follows PEP 8 style guidelines and includes comprehensive docstrings",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							SuggestedActors struct {
								Nodes []struct {
									Bot struct {
										ID       githubv4.ID
										Login    githubv4.String
										TypeName string `graphql:"__typename"`
									} `graphql:"... on Bot"`
								}
								PageInfo struct {
									HasNextPage bool
									EndCursor   string
								}
							} `graphql:"suggestedActors(first: 100, after: $endCursor, capabilities: CAN_BE_ASSIGNED)"`
						} `graphql:"repository(owner: $owner, name: $name)"`
					}{},
					map[string]any{
						"owner":     githubv4.String("owner"),
						"name":      githubv4.String("repo"),
						"endCursor": (*githubv4.String)(nil),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"suggestedActors": map[string]any{
								"nodes": []any{
									map[string]any{
										"id":         githubv4.ID("copilot-swe-agent-id"),
										"login":      githubv4.String("copilot-swe-agent"),
										"__typename": "Bot",
									},
								},
							},
						},
					}),
				),
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							ID    githubv4.ID
							Issue struct {
								ID        githubv4.ID
								Assignees struct {
									Nodes []struct {
										ID githubv4.ID
									}
								} `graphql:"assignees(first: 100)"`
							} `graphql:"issue(number: $number)"`
						} `graphql:"repository(owner: $owner, name: $name)"`
					}{},
					map[string]any{
						"owner":  githubv4.String("owner"),
						"name":   githubv4.String("repo"),
						"number": githubv4.Int(123),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"id": githubv4.ID("test-repo-id"),
							"issue": map[string]any{
								"id": githubv4.ID("test-issue-id"),
								"assignees": map[string]any{
									"nodes": []any{},
								},
							},
						},
					}),
				),
				githubv4mock.NewMutationMatcher(
					struct {
						UpdateIssue struct {
							Issue struct {
								ID     githubv4.ID
								Number githubv4.Int
								URL    githubv4.String
							}
						} `graphql:"updateIssue(input: $input)"`
					}{},
					UpdateIssueInput{
						ID:          githubv4.ID("test-issue-id"),
						AssigneeIDs: []githubv4.ID{githubv4.ID("copilot-swe-agent-id")},
						AgentAssignment: &AgentAssignmentInput{
							BaseRef:            nil,
							CustomAgent:        ptrGitHubv4String(""),
							CustomInstructions: ptrGitHubv4String("Please ensure all code follows PEP 8 style guidelines and includes comprehensive docstrings"),
							TargetRepositoryID: githubv4.ID("test-repo-id"),
						},
					},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"updateIssue": map[string]any{
							"issue": map[string]any{
								"id":     githubv4.ID("test-issue-id"),
								"number": githubv4.Int(123),
								"url":    githubv4.String("https://github.com/owner/repo/issues/123"),
							},
						},
					}),
				),
			),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Setup client with mock
			client := githubv4.NewClient(tc.mockedClient)
			deps := BaseDeps{
				GQLClient: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Disable polling in tests to avoid timeouts
			ctx := ContextWithPollConfig(context.Background(), PollConfig{MaxAttempts: 0})
			ctx = ContextWithDeps(ctx, deps)

			// Call handler
			result, err := handler(ctx, &request)
			require.NoError(t, err)

			textContent := getTextResult(t, result)

			if tc.expectToolError {
				require.True(t, result.IsError)
				assert.Contains(t, textContent.Text, tc.expectedToolErrMsg)
				return
			}

			require.False(t, result.IsError, fmt.Sprintf("expected there to be no tool error, text was %s", textContent.Text))

			// Verify the JSON response contains expected fields
			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err, "response should be valid JSON")
			assert.Equal(t, float64(123), response["issue_number"])
			assert.Equal(t, "https://github.com/owner/repo/issues/123", response["issue_url"])
			assert.Equal(t, "owner", response["owner"])
			assert.Equal(t, "repo", response["repo"])
			assert.Contains(t, response["message"], "successfully assigned copilot to issue")
		})
	}
}

func Test_RequestCopilotReview(t *testing.T) {
	t.Parallel()

	serverTool := RequestCopilotReview(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "request_copilot_review", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "pullNumber"})

	// Setup mock PR for success case
	mockPR := &github.PullRequest{
		Number:  github.Ptr(42),
		Title:   github.Ptr("Test PR"),
		State:   github.Ptr("open"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/pull/42"),
		Head: &github.PullRequestBranch{
			SHA: github.Ptr("abcd1234"),
			Ref: github.Ptr("feature-branch"),
		},
		Base: &github.PullRequestBranch{
			Ref: github.Ptr("main"),
		},
		Body: github.Ptr("This is a test PR"),
		User: &github.User{
			Login: github.Ptr("testuser"),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful request",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber: expect(t, expectations{
					path: "/repos/owner/repo/pulls/1/requested_reviewers",
					requestBody: map[string]any{
						"reviewers": []any{"copilot-pull-request-reviewer[bot]"},
					},
				}).andThen(
					mockResponse(t, http.StatusCreated, mockPR),
				),
			}),
			requestArgs: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(1),
			},
			expectError: false,
		},
		{
			name: "request fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				}),
			}),
			requestArgs: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to request copilot review",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := github.NewClient(tc.mockedClient)
			serverTool := RequestCopilotReview(translations.NullTranslationHelper)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)
			assert.NotNil(t, result)
			assert.Len(t, result.Content, 1)

			textContent := getTextResult(t, result)
			require.Equal(t, "", textContent.Text)
		})
	}
}
