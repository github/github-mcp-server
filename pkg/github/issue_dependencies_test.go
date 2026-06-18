package github

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_IssueDependencyRead(t *testing.T) {
	// Verify tool definition once (flag-gated variant snap)
	serverTool := IssueDependencyRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name+"_ff_"+FeatureFlagIssueDependencies, tool))
	require.Equal(t, FeatureFlagIssueDependencies, serverTool.FeatureFlagEnable)

	assert.Equal(t, "issue_dependency_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "method")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "issue_number")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo", "issue_number"})

	blockedByQuery := githubv4mock.NewQueryMatcher(
		struct {
			Repository struct {
				Issue struct {
					BlockedBy dependencyConnection `graphql:"blockedBy(first: $first, after: $after)"`
				} `graphql:"issue(number: $issueNumber)"`
			} `graphql:"repository(owner: $owner, name: $repo)"`
		}{},
		map[string]any{
			"owner":       githubv4.String("owner"),
			"repo":        githubv4.String("repo"),
			"issueNumber": githubv4.Int(123),
			"first":       githubv4.Int(30),
			"after":       (*githubv4.String)(nil),
		},
		githubv4mock.DataResponse(map[string]any{
			"repository": map[string]any{
				"issue": map[string]any{
					"blockedBy": map[string]any{
						"totalCount": 1,
						"pageInfo": map[string]any{
							"hasNextPage": false,
							"endCursor":   "",
						},
						"nodes": []map[string]any{
							{
								"number":     7,
								"title":      "Blocker",
								"state":      "OPEN",
								"url":        "https://github.com/owner/repo/issues/7",
								"repository": map[string]any{"nameWithOwner": "owner/repo"},
							},
						},
					},
				},
			},
		}),
	)

	blockingQuery := githubv4mock.NewQueryMatcher(
		struct {
			Repository struct {
				Issue struct {
					Blocking dependencyConnection `graphql:"blocking(first: $first, after: $after)"`
				} `graphql:"issue(number: $issueNumber)"`
			} `graphql:"repository(owner: $owner, name: $repo)"`
		}{},
		map[string]any{
			"owner":       githubv4.String("owner"),
			"repo":        githubv4.String("repo"),
			"issueNumber": githubv4.Int(123),
			"first":       githubv4.Int(30),
			"after":       (*githubv4.String)(nil),
		},
		githubv4mock.DataResponse(map[string]any{
			"repository": map[string]any{
				"issue": map[string]any{
					"blocking": map[string]any{
						"totalCount": 2,
						"pageInfo": map[string]any{
							"hasNextPage": true,
							"endCursor":   "Y3Vyc29y",
						},
						"nodes": []map[string]any{
							{
								"number":     8,
								"title":      "Blocked A",
								"state":      "OPEN",
								"url":        "https://github.com/owner/repo/issues/8",
								"repository": map[string]any{"nameWithOwner": "owner/repo"},
							},
							{
								"number":     9,
								"title":      "Blocked B",
								"state":      "CLOSED",
								"url":        "https://github.com/owner/repo/issues/9",
								"repository": map[string]any{"nameWithOwner": "owner/repo"},
							},
						},
					},
				},
			},
		}),
	)

	tests := []struct {
		name          string
		method        string
		matcher       githubv4mock.Matcher
		expectError   bool
		expectedCount int
		expectedFirst int
		expectedNext  bool
	}{
		{
			name:          "get_blocked_by returns blockers",
			method:        "get_blocked_by",
			matcher:       blockedByQuery,
			expectedCount: 1,
			expectedFirst: 7,
			expectedNext:  false,
		},
		{
			name:          "get_blocking returns blocked issues",
			method:        "get_blocking",
			matcher:       blockingQuery,
			expectedCount: 2,
			expectedFirst: 8,
			expectedNext:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(tc.matcher))
			deps := BaseDeps{GQLClient: gqlClient}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(map[string]any{
				"method":       tc.method,
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
			})
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError, "expected result to not be an error")

			text := getTextResult(t, result)
			var payload struct {
				Issues []minimalDependencyIssue `json:"issues"`
				Total  int                      `json:"totalCount"`
				Page   struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
			}
			require.NoError(t, json.Unmarshal([]byte(text.Text), &payload))
			require.Len(t, payload.Issues, tc.expectedCount)
			assert.Equal(t, tc.expectedFirst, payload.Issues[0].Number)
			assert.Equal(t, tc.expectedNext, payload.Page.HasNextPage)
		})
	}
}

func Test_IssueDependencyRead_Errors(t *testing.T) {
	serverTool := IssueDependencyRead(translations.NullTranslationHelper)

	t.Run("rejects page-based pagination", func(t *testing.T) {
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient())
		deps := BaseDeps{GQLClient: gqlClient}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":       "get_blocked_by",
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(1),
			"page":         float64(2),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		errText := getErrorResult(t, result)
		assert.Contains(t, errText.Text, "cursor-based pagination")
	})

	t.Run("missing required param", func(t *testing.T) {
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient())
		deps := BaseDeps{GQLClient: gqlClient}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method": "get_blocked_by",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		getErrorResult(t, result)
	})
}

func Test_IssueDependencyWrite(t *testing.T) {
	// Verify tool definition once (flag-gated variant snap)
	serverTool := IssueDependencyWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name+"_ff_"+FeatureFlagIssueDependencies, tool))
	require.Equal(t, FeatureFlagIssueDependencies, serverTool.FeatureFlagEnable)

	assert.Equal(t, "issue_dependency_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "method")
	assert.Contains(t, schema.Properties, "type")
	assert.Contains(t, schema.Properties, "issue_number")
	assert.Contains(t, schema.Properties, "related_issue_number")
	assert.ElementsMatch(t, schema.Required, []string{"method", "type", "owner", "repo", "issue_number", "related_issue_number"})

	resolveMatcher := func(subjectID, relatedID string) githubv4mock.Matcher {
		return githubv4mock.NewQueryMatcher(
			struct {
				Subject struct {
					Issue struct {
						ID githubv4.ID
					} `graphql:"issue(number: $subjectNumber)"`
				} `graphql:"subject: repository(owner: $subjectOwner, name: $subjectRepo)"`
				Related struct {
					Issue struct {
						ID githubv4.ID
					} `graphql:"issue(number: $relatedNumber)"`
				} `graphql:"related: repository(owner: $relatedOwner, name: $relatedRepo)"`
			}{},
			map[string]any{
				"subjectOwner":  githubv4.String("owner"),
				"subjectRepo":   githubv4.String("repo"),
				"subjectNumber": githubv4.Int(1),
				"relatedOwner":  githubv4.String("owner"),
				"relatedRepo":   githubv4.String("repo"),
				"relatedNumber": githubv4.Int(2),
			},
			githubv4mock.DataResponse(map[string]any{
				"subject": map[string]any{"issue": map[string]any{"id": subjectID}},
				"related": map[string]any{"issue": map[string]any{"id": relatedID}},
			}),
		)
	}

	type mutationIssue struct {
		Number githubv4.Int
		URL    githubv4.String
	}

	addMutation := func(issueID, blockingID string) githubv4mock.Matcher {
		return githubv4mock.NewMutationMatcher(
			struct {
				AddBlockedBy struct {
					Issue         mutationIssue
					BlockingIssue mutationIssue
				} `graphql:"addBlockedBy(input: $input)"`
			}{},
			AddBlockedByInput{IssueID: githubv4.ID(issueID), BlockingIssueID: githubv4.ID(blockingID)},
			nil,
			githubv4mock.DataResponse(map[string]any{
				"addBlockedBy": map[string]any{
					"issue":         map[string]any{"number": 1, "url": "https://github.com/owner/repo/issues/1"},
					"blockingIssue": map[string]any{"number": 2, "url": "https://github.com/owner/repo/issues/2"},
				},
			}),
		)
	}

	removeMutation := func(issueID, blockingID string) githubv4mock.Matcher {
		return githubv4mock.NewMutationMatcher(
			struct {
				RemoveBlockedBy struct {
					Issue         mutationIssue
					BlockingIssue mutationIssue
				} `graphql:"removeBlockedBy(input: $input)"`
			}{},
			RemoveBlockedByInput{IssueID: githubv4.ID(issueID), BlockingIssueID: githubv4.ID(blockingID)},
			nil,
			githubv4mock.DataResponse(map[string]any{
				"removeBlockedBy": map[string]any{
					"issue":         map[string]any{"number": 1, "url": "https://github.com/owner/repo/issues/1"},
					"blockingIssue": map[string]any{"number": 2, "url": "https://github.com/owner/repo/issues/2"},
				},
			}),
		)
	}

	tests := []struct {
		name            string
		method          string
		relationship    string
		matchers        []githubv4mock.Matcher
		expectedMessage string
	}{
		{
			name:         "add blocked_by uses subject as blocked",
			method:       "add",
			relationship: "blocked_by",
			// subject(1) is blocked by related(2): issueId=subject, blockingIssueId=related
			matchers:        []githubv4mock.Matcher{resolveMatcher("I_subject", "I_related"), addMutation("I_subject", "I_related")},
			expectedMessage: "dependency added",
		},
		{
			name:         "add blocking swaps roles",
			method:       "add",
			relationship: "blocking",
			// subject(1) blocks related(2): issueId=related, blockingIssueId=subject
			matchers:        []githubv4mock.Matcher{resolveMatcher("I_subject", "I_related"), addMutation("I_related", "I_subject")},
			expectedMessage: "dependency added",
		},
		{
			name:            "remove blocked_by",
			method:          "remove",
			relationship:    "blocked_by",
			matchers:        []githubv4mock.Matcher{resolveMatcher("I_subject", "I_related"), removeMutation("I_subject", "I_related")},
			expectedMessage: "dependency removed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(tc.matchers...))
			deps := BaseDeps{GQLClient: gqlClient}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(map[string]any{
				"method":               tc.method,
				"type":                 tc.relationship,
				"owner":                "owner",
				"repo":                 "repo",
				"issue_number":         float64(1),
				"related_issue_number": float64(2),
			})
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError, "expected result to not be an error")

			text := getTextResult(t, result)
			var payload struct {
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal([]byte(text.Text), &payload))
			assert.Equal(t, tc.expectedMessage, payload.Message)
		})
	}

	t.Run("self dependency fails before any API call", func(t *testing.T) {
		// Register no matchers: the handler must return before resolving node IDs or mutating.
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient())
		deps := BaseDeps{GQLClient: gqlClient}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":               "add",
			"type":                 "blocked_by",
			"owner":                "owner",
			"repo":                 "repo",
			"issue_number":         float64(1),
			"related_issue_number": float64(1),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError, "expected result to be an error")

		text := getTextResult(t, result)
		assert.Contains(t, text.Text, "itself")
	})
}

func Test_IssueDependencyWrite_Validation(t *testing.T) {
	serverTool := IssueDependencyWrite(translations.NullTranslationHelper)

	cases := []struct {
		name string
		args map[string]any
	}{
		{
			name: "unknown type",
			args: map[string]any{
				"method":               "add",
				"type":                 "related_to",
				"owner":                "owner",
				"repo":                 "repo",
				"issue_number":         float64(1),
				"related_issue_number": float64(2),
			},
		},
		{
			name: "missing related_issue_number",
			args: map[string]any{
				"method":       "add",
				"type":         "blocked_by",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient())
			deps := BaseDeps{GQLClient: gqlClient}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.args)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			getErrorResult(t, result)
		})
	}
}
