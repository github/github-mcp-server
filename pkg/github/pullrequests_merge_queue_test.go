package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/v2/internal/githubv4mock"
	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mergeQueueTestPullRequestID = "PR_kwDOA0xdyM50BPaO"

// pullRequestNodeIDMatcher matches the lookup resolvePullRequestNodeID performs.
func pullRequestNodeIDMatcher(pullNumber int32, response githubv4mock.GQLResponse) githubv4mock.Matcher {
	return githubv4mock.NewQueryMatcher(
		struct {
			Repository struct {
				PullRequest struct {
					ID githubv4.ID
				} `graphql:"pullRequest(number: $prNumber)"`
			} `graphql:"repository(owner: $owner, name: $repo)"`
		}{},
		map[string]any{
			"owner":    githubv4.String("owner"),
			"repo":     githubv4.String("repo"),
			"prNumber": githubv4.Int(pullNumber),
		},
		response,
	)
}

func pullRequestNodeIDResponse() githubv4mock.GQLResponse {
	return githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{"pullRequest": map[string]any{"id": mergeQueueTestPullRequestID}},
	})
}

func mergeQueueEntryResponse(position int, state string, jump bool) map[string]any {
	return map[string]any{
		"id":                   "MQE_kwDOA0xdyM4AAAAB",
		"position":             position,
		"state":                state,
		"enqueuedAt":           "2026-11-16T19:07:37Z",
		"estimatedTimeToMerge": 840,
		"jump":                 jump,
		"solo":                 false,
	}
}

func runMergeQueueTool(t *testing.T, tool func(translations.TranslationHelperFunc) inventory.ServerTool, gqlHTTPClient *http.Client, args map[string]any) (string, bool) {
	t.Helper()
	serverTool := tool(translations.NullTranslationHelper)
	deps := BaseDeps{
		Client:    mustNewGHClient(t, nil),
		GQLClient: githubv4.NewClient(gqlHTTPClient),
	}
	handler := serverTool.Handler(deps)
	request := createMCPRequest(args)
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	if result.IsError {
		return getErrorResult(t, result).Text, true
	}
	return getTextResult(t, result).Text, false
}

func Test_EnqueuePullRequest(t *testing.T) {
	serverTool := EnqueuePullRequest(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "enqueue_pull_request", tool.Name)
	assert.False(t, tool.Annotations.ReadOnlyHint)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "jump")
	assert.Contains(t, schema.Properties, "expectedHeadSha")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "pullNumber"})

	type enqueueMutation struct {
		EnqueuePullRequest struct {
			MergeQueueEntry *mergeQueueEntryNode
		} `graphql:"enqueuePullRequest(input: $input)"`
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectedErrMsg string
		expectedEntry  *MinimalMergeQueueEntry
	}{
		{
			name: "enqueues at the back of the queue",
			mockedClient: githubv4mock.NewMockedHTTPClient(
				pullRequestNodeIDMatcher(42, pullRequestNodeIDResponse()),
				githubv4mock.NewMutationMatcher(
					enqueueMutation{},
					githubv4.EnqueuePullRequestInput{PullRequestID: mergeQueueTestPullRequestID},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"enqueuePullRequest": map[string]any{"mergeQueueEntry": mergeQueueEntryResponse(3, "QUEUED", false)},
					}),
				),
			),
			requestArgs: map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(42)},
			expectedEntry: &MinimalMergeQueueEntry{
				ID: "MQE_kwDOA0xdyM4AAAAB", Position: 3, State: "QUEUED", EnqueuedAt: "2026-11-16T19:07:37Z",
				EstimatedTimeToMerge: new(840),
			},
		},
		{
			name: "jumps the queue only when the head still matches",
			mockedClient: githubv4mock.NewMockedHTTPClient(
				pullRequestNodeIDMatcher(42, pullRequestNodeIDResponse()),
				githubv4mock.NewMutationMatcher(
					enqueueMutation{},
					githubv4.EnqueuePullRequestInput{
						PullRequestID:   mergeQueueTestPullRequestID,
						Jump:            githubv4.NewBoolean(true),
						ExpectedHeadOid: githubv4.NewGitObjectID("abc123"),
					},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"enqueuePullRequest": map[string]any{"mergeQueueEntry": mergeQueueEntryResponse(1, "AWAITING_CHECKS", true)},
					}),
				),
			),
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo", "pullNumber": float64(42), "jump": true, "expectedHeadSha": "abc123",
			},
			expectedEntry: &MinimalMergeQueueEntry{
				ID: "MQE_kwDOA0xdyM4AAAAB", Position: 1, State: "AWAITING_CHECKS", EnqueuedAt: "2026-11-16T19:07:37Z",
				EstimatedTimeToMerge: new(840), Jump: true,
			},
		},
		{
			name: "jump false is not sent",
			mockedClient: githubv4mock.NewMockedHTTPClient(
				pullRequestNodeIDMatcher(42, pullRequestNodeIDResponse()),
				githubv4mock.NewMutationMatcher(
					enqueueMutation{},
					githubv4.EnqueuePullRequestInput{PullRequestID: mergeQueueTestPullRequestID},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"enqueuePullRequest": map[string]any{"mergeQueueEntry": mergeQueueEntryResponse(2, "QUEUED", false)},
					}),
				),
			),
			requestArgs: map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(42), "jump": false},
			expectedEntry: &MinimalMergeQueueEntry{
				ID: "MQE_kwDOA0xdyM4AAAAB", Position: 2, State: "QUEUED", EnqueuedAt: "2026-11-16T19:07:37Z",
				EstimatedTimeToMerge: new(840),
			},
		},
		{
			name: "base branch without a merge queue",
			mockedClient: githubv4mock.NewMockedHTTPClient(
				pullRequestNodeIDMatcher(42, pullRequestNodeIDResponse()),
				githubv4mock.NewMutationMatcher(
					enqueueMutation{},
					githubv4.EnqueuePullRequestInput{PullRequestID: mergeQueueTestPullRequestID},
					nil,
					githubv4mock.ErrorResponse("Merge queue is not enabled for the base branch of this pull request"),
				),
			),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(42)},
			expectedErrMsg: "failed to add pull request to the merge queue",
		},
		{
			name: "pull request not found",
			mockedClient: githubv4mock.NewMockedHTTPClient(
				pullRequestNodeIDMatcher(999, githubv4mock.ErrorResponse("Could not resolve to a PullRequest with the number of 999.")),
			),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(999)},
			expectedErrMsg: "failed to find pull request",
		},
		{
			name:           "missing pull request number",
			mockedClient:   githubv4mock.NewMockedHTTPClient(),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo"},
			expectedErrMsg: "missing required parameter: pullNumber",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text, isError := runMergeQueueTool(t, EnqueuePullRequest, tc.mockedClient, tc.requestArgs)
			if tc.expectedErrMsg != "" {
				require.True(t, isError, text)
				assert.Contains(t, text, tc.expectedErrMsg)
				return
			}
			require.False(t, isError, text)

			var entry MinimalMergeQueueEntry
			require.NoError(t, json.Unmarshal([]byte(text), &entry))
			assert.Equal(t, *tc.expectedEntry, entry)
		})
	}
}

func Test_DequeuePullRequest(t *testing.T) {
	serverTool := DequeuePullRequest(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "dequeue_pull_request", tool.Name)
	assert.False(t, tool.Annotations.ReadOnlyHint)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "pullNumber"})
	assert.NotContains(t, schema.Properties, "jump")

	type dequeueMutation struct {
		DequeuePullRequest struct {
			MergeQueueEntry *mergeQueueEntryNode
		} `graphql:"dequeuePullRequest(input: $input)"`
	}

	t.Run("returns the removed entry", func(t *testing.T) {
		client := githubv4mock.NewMockedHTTPClient(
			pullRequestNodeIDMatcher(42, pullRequestNodeIDResponse()),
			githubv4mock.NewMutationMatcher(
				dequeueMutation{},
				githubv4.DequeuePullRequestInput{ID: mergeQueueTestPullRequestID},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"dequeuePullRequest": map[string]any{"mergeQueueEntry": mergeQueueEntryResponse(2, "QUEUED", false)},
				}),
			),
		)
		text, isError := runMergeQueueTool(t, DequeuePullRequest, client,
			map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(42)})
		require.False(t, isError, text)

		var entry MinimalMergeQueueEntry
		require.NoError(t, json.Unmarshal([]byte(text), &entry))
		assert.Equal(t, 2, entry.Position)
		assert.Equal(t, "QUEUED", entry.State)
	})

	t.Run("pull request not in the queue", func(t *testing.T) {
		client := githubv4mock.NewMockedHTTPClient(
			pullRequestNodeIDMatcher(42, pullRequestNodeIDResponse()),
			githubv4mock.NewMutationMatcher(
				dequeueMutation{},
				githubv4.DequeuePullRequestInput{ID: mergeQueueTestPullRequestID},
				nil,
				githubv4mock.ErrorResponse("Pull request is not in the merge queue"),
			),
		)
		text, isError := runMergeQueueTool(t, DequeuePullRequest, client,
			map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(42)})
		require.True(t, isError)
		assert.Contains(t, text, "failed to remove pull request from the merge queue")
	})
}

func Test_GetPullRequestMergeQueueStatus(t *testing.T) {
	type statusQuery struct {
		Repository struct {
			PullRequest struct {
				IsMergeQueueEnabled githubv4.Boolean
				IsInMergeQueue      githubv4.Boolean
				MergeQueueEntry     *mergeQueueEntryNode
			} `graphql:"pullRequest(number: $prNum)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}
	vars := map[string]any{
		"owner": githubv4.String("owner"),
		"repo":  githubv4.String("repo"),
		"prNum": githubv4.Int(42),
	}

	tests := []struct {
		name           string
		response       githubv4mock.GQLResponse
		expectedStatus *MinimalMergeQueueStatus
		expectedJSON   string
		expectedErrMsg string
	}{
		{
			name: "queued pull request",
			response: githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{"pullRequest": map[string]any{
					"isMergeQueueEnabled": true,
					"isInMergeQueue":      true,
					"mergeQueueEntry":     mergeQueueEntryResponse(1, "AWAITING_CHECKS", false),
				}},
			}),
			expectedJSON: `{"merge_queue_enabled":true,"in_merge_queue":true,"entry":{"id":"MQE_kwDOA0xdyM4AAAAB","position":1,"state":"AWAITING_CHECKS","enqueued_at":"2026-11-16T19:07:37Z","estimated_time_to_merge":840,"jump":false,"solo":false}}`,
		},
		{
			name: "merge queue required but pull request not queued",
			response: githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{"pullRequest": map[string]any{
					"isMergeQueueEnabled": true, "isInMergeQueue": false, "mergeQueueEntry": nil,
				}},
			}),
			expectedJSON: `{"merge_queue_enabled":true,"in_merge_queue":false}`,
		},
		{
			name: "base branch without a merge queue",
			response: githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{"pullRequest": map[string]any{
					"isMergeQueueEnabled": false, "isInMergeQueue": false, "mergeQueueEntry": nil,
				}},
			}),
			expectedJSON: `{"merge_queue_enabled":false,"in_merge_queue":false}`,
		},
		{
			name:           "query fails",
			response:       githubv4mock.ErrorResponse("Could not resolve to a PullRequest with the number of 42."),
			expectedErrMsg: "failed to get pull request merge queue status",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := githubv4mock.NewMockedHTTPClient(githubv4mock.NewQueryMatcher(statusQuery{}, vars, tc.response))
			text, isError := runMergeQueueTool(t, PullRequestRead, client, map[string]any{
				"method": "get_merge_queue_status", "owner": "owner", "repo": "repo", "pullNumber": float64(42),
			})
			if tc.expectedErrMsg != "" {
				require.True(t, isError, text)
				assert.Contains(t, text, tc.expectedErrMsg)
				return
			}
			require.False(t, isError, text)
			assert.JSONEq(t, tc.expectedJSON, text)
		})
	}
}

func TestMergeQueueOutputSchemas(t *testing.T) {
	entry := `{"id":"MQE_1","position":1,"state":"QUEUED","enqueued_at":"2026-11-16T19:07:37Z","jump":false,"solo":false}`
	for _, tc := range []struct {
		schema  *jsonschema.Schema
		valid   []string
		invalid []string
	}{
		{
			pullRequestMergeQueueEntryOutputSchema(),
			[]string{`null`, entry},
			[]string{`{}`, `[]`, `{"id":"MQE_1","position":1,"state":"WAITING","enqueued_at":"","jump":false,"solo":false}`},
		},
		{
			pullRequestReadOutputSchema(),
			[]string{
				`{"merge_queue_enabled":false,"in_merge_queue":false}`,
				`{"merge_queue_enabled":true,"in_merge_queue":true,"entry":` + entry + `}`,
			},
			[]string{`{"merge_queue_enabled":true}`, `{"merge_queue_enabled":true,"in_merge_queue":true,"entry":{}}`},
		},
	} {
		resolved, err := tc.schema.Resolve(nil)
		require.NoError(t, err)
		for _, raw := range tc.valid {
			var value any
			require.NoError(t, json.Unmarshal([]byte(raw), &value))
			require.NoError(t, resolved.Validate(value), raw)
		}
		for _, raw := range tc.invalid {
			var value any
			require.NoError(t, json.Unmarshal([]byte(raw), &value))
			require.Error(t, resolved.Validate(value), raw)
		}
	}

	// The SDK validates zero outputs even for error results.
	resolved, err := pullRequestMergeQueueEntryOutputSchema().Resolve(nil)
	require.NoError(t, err)
	var value any
	require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, PullRequestMergeQueueEntryOutput{})), &value))
	require.NoError(t, resolved.Validate(value))
}

func TestConvertToMinimalMergeQueueEntry(t *testing.T) {
	assert.Nil(t, convertToMinimalMergeQueueEntry(nil))

	enqueuedAt := time.Date(2026, 11, 16, 20, 7, 37, 0, time.FixedZone("CET", 3600))
	entry := convertToMinimalMergeQueueEntry(&mergeQueueEntryNode{
		ID:         "MQE_1",
		Position:   4,
		State:      githubv4.MergeQueueEntryStateLocked,
		EnqueuedAt: githubv4.DateTime{Time: enqueuedAt},
		Solo:       true,
	})
	assert.Equal(t, &MinimalMergeQueueEntry{
		ID: "MQE_1", Position: 4, State: "LOCKED", EnqueuedAt: "2026-11-16T19:07:37Z", Solo: true,
	}, entry)
}
