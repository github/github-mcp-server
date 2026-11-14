package github

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/githubv4mock"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_MergePullRequest_AuthorEmail(t *testing.T) {
	tests := []struct {
		name          string
		protocol      string
		args          map[string]any
		wantInput     map[string]any
		queryError    string
		queryResult   map[string]any
		mutationError string
		mergeResult   map[string]any
		expectedError string
	}{
		{name: "default merge method"},
		{name: "legacy protocol text result", protocol: "2025-11-25"},
		{
			name:      "merge",
			args:      map[string]any{"merge_method": "merge"},
			wantInput: map[string]any{"mergeMethod": "MERGE"},
		},
		{
			name: "squash with commit metadata and head guard",
			args: map[string]any{
				"merge_method": "squash", "commit_title": "Land feature",
				"commit_message": "Details", "expectedHeadSha": "head-sha",
			},
			wantInput: map[string]any{
				"mergeMethod": "SQUASH", "commitHeadline": "Land feature",
				"commitBody": "Details", "expectedHeadOid": "head-sha",
			},
		},
		{
			name:      "rebase",
			args:      map[string]any{"merge_method": "rebase"},
			wantInput: map[string]any{"mergeMethod": "REBASE"},
		},
		{name: "lookup failure", queryError: "Could not resolve to a PullRequest"},
		{
			name:          "missing pull request ID",
			queryResult:   map[string]any{"repository": map[string]any{"pullRequest": nil}},
			expectedError: "failed to find pull request",
		},
		{name: "merge failure", mutationError: "Email is not verified"},
		{
			name:          "missing merge result",
			mergeResult:   map[string]any{"pullRequest": nil},
			expectedError: "pull request was not merged",
		},
		{
			name:          "pull request was not merged",
			mergeResult:   map[string]any{"pullRequest": map[string]any{"merged": false, "mergeCommit": nil}},
			expectedError: "pull request was not merged",
		},
		{
			name:          "head mismatch",
			args:          map[string]any{"expectedHeadSha": "stale-sha"},
			wantInput:     map[string]any{"expectedHeadOid": "stale-sha"},
			mutationError: "Head branch was modified",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]any{
				"owner": "owner", "repo": "repo", "pullNumber": 42,
				"author_email": "work@example.com",
			}
			maps.Copy(args, tc.args)
			wantInput := map[string]any{"pullRequestId": "PR_42", "authorEmail": "work@example.com"}
			maps.Copy(wantInput, tc.wantInput)
			queryResponse := githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{"pullRequest": map[string]any{"id": "PR_42"}},
			})
			if tc.queryError != "" {
				queryResponse = githubv4mock.ErrorResponse(tc.queryError)
			}
			if tc.queryResult != nil {
				queryResponse = githubv4mock.DataResponse(tc.queryResult)
			}
			mutationResponse := githubv4mock.DataResponse(map[string]any{
				"mergePullRequest": map[string]any{"pullRequest": map[string]any{
					"merged": true, "mergeCommit": map[string]any{"oid": "merge-sha"},
				}},
			})
			if tc.mutationError != "" {
				mutationResponse = githubv4mock.ErrorResponse(tc.mutationError)
			}
			if tc.mergeResult != nil {
				mutationResponse = githubv4mock.DataResponse(map[string]any{"mergePullRequest": tc.mergeResult})
			}
			gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					`query($owner:String!$prNum:Int!$repo:String!){repository(owner: $owner, name: $repo){pullRequest(number: $prNum){id}}}`,
					map[string]any{"owner": githubv4.String("owner"), "repo": githubv4.String("repo"), "prNum": githubv4.Int(42)},
					queryResponse,
				),
				githubv4mock.NewQueryMatcher(
					`mutation($input:MergePullRequestInput!){mergePullRequest(input: $input){pullRequest{merged,mergeCommit{oid}}}}`,
					map[string]any{"input": wantInput},
					mutationResponse,
				),
			))
			deps := BaseDeps{
				Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
					PutReposPullsMergeByOwnerByRepoByPullNumber: func(w http.ResponseWriter, _ *http.Request) {
						t.Error("author_email must not use the REST merge endpoint")
						w.WriteHeader(http.StatusInternalServerError)
					},
				})),
				GQLClient: gqlClient,
			}
			protocol := tc.protocol
			if protocol == "" {
				protocol = inventory.ProtocolVersionMultiRoundTrip
			}
			session, _ := consolidatedPullRequestSession(t, deps, protocol, false)
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "merge_pull_request", Arguments: args})
			require.NoError(t, err)
			if tc.queryError != "" || tc.mutationError != "" || tc.expectedError != "" {
				require.True(t, result.IsError)
				assert.Contains(t, getErrorResult(t, result).Text, tc.queryError+tc.mutationError+tc.expectedError)
				return
			}
			require.False(t, result.IsError, getTextResult(t, result).Text)
			const want = `{"sha":"merge-sha","merged":true,"message":"Pull Request successfully merged"}`
			assert.JSONEq(t, want, getTextResult(t, result).Text)
			if protocol != inventory.ProtocolVersionMultiRoundTrip {
				assert.Nil(t, result.StructuredContent)
				return
			}
			structured, err := json.Marshal(result.StructuredContent)
			require.NoError(t, err)
			assert.JSONEq(t, want, string(structured))
		})
	}
}
