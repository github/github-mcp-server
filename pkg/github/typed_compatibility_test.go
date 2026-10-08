package github

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/githubv4mock"
	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypedCompatibilityCommitInputs(t *testing.T) {
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		for _, tool := range []inventory.ServerTool{GetCommit(translations.NullTranslationHelper), ListCommits(translations.NullTranslationHelper)} {
			for _, pagination := range []map[string]any{
				{"page": -1, "perPage": 101},
				{"page": "-1", "perPage": "-2"},
				{"page": 2, "perPage": 101},
				{"page": "2", "perPage": "-2"},
				{"fields": nil},
			} {
				t.Run(protocol+"/"+tool.Tool.Name+"/"+mustMarshalJSON(t, pagination), func(t *testing.T) {
					calls := 0
					handler := func(w http.ResponseWriter, r *http.Request) {
						calls++
						if pagination["page"] != nil {
							assert.Equal(t, fmt.Sprint(pagination["page"]), r.URL.Query().Get("page"))
							assert.Equal(t, fmt.Sprint(pagination["perPage"]), r.URL.Query().Get("per_page"))
						}
						var response any = &github.RepositoryCommit{SHA: new("abc")}
						if tool.Tool.Name == "list_commits" {
							response = []*github.RepositoryCommit{{SHA: new("abc")}}
						}
						mockResponse(t, http.StatusOK, response)(w, r)
					}
					deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
						GetReposCommitsByOwnerByRepoByRef: handler,
						GetReposCommitsByOwnerByRepo:      handler,
					}))}
					args := map[string]any{"owner": "owner", "repo": "repo", "sha": "abc"}
					maps.Copy(args, pagination)
					session := connectTypedSearchClient(t, tool, deps, protocol)
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool.Tool.Name, Arguments: args})
					require.NoError(t, err)
					require.False(t, result.IsError, mustMarshalJSON(t, result))
					assert.Equal(t, 1, calls)
					if protocol == "2025-11-25" {
						assert.Nil(t, result.StructuredContent)
					} else {
						assert.JSONEq(t, getTextResult(t, result).Text, mustMarshalJSON(t, result.StructuredContent))
					}
					request := createMCPRequest(args)
					direct, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
					require.NoError(t, err)
					require.False(t, direct.IsError, mustMarshalJSON(t, direct))
					assert.Equal(t, 2, calls)
				})
			}
		}
	}
}
func TestTypedCompatibilityGranularOptionalZero(t *testing.T) {
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		for _, tc := range []struct {
			tool inventory.ServerTool
			args map[string]any
			body string
		}{
			{GranularReprioritizeSubIssue(translations.NullTranslationHelper), map[string]any{"sub_issue_id": 7, "after_id": "0", "before_id": 8}, `{"sub_issue_id":7,"before_id":8}`},
			{GranularReprioritizeSubIssue(translations.NullTranslationHelper), map[string]any{"sub_issue_id": 7, "after_id": 8, "before_id": "0"}, `{"sub_issue_id":7,"after_id":8}`},
			{GranularUpdateIssueState(translations.NullTranslationHelper), map[string]any{"state": "open", "duplicate_of": "0"}, `{"state":"open"}`},
		} {
			t.Run(protocol+"/"+tc.tool.Tool.Name+"/"+tc.body, func(t *testing.T) {
				calls := 0
				client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					var body any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					assert.JSONEq(t, tc.body, mustMarshalJSON(t, body))
					_, _ = w.Write([]byte(`{"id":1,"number":1}`))
				})}}
				deps := BaseDeps{Client: mustNewGHClient(t, client), featureChecker: featureCheckerFor(inventory.FeatureFlag(FeatureFlagIssuesGranular))}
				args := map[string]any{"owner": "owner", "repo": "repo", "issue_number": 1}
				maps.Copy(args, tc.args)
				session := connectTypedSearchClient(t, tc.tool, deps, protocol)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool.Tool.Name, Arguments: args})
				require.NoError(t, err)
				require.False(t, result.IsError, mustMarshalJSON(t, result))
				assert.Equal(t, 1, calls)
			})
		}
	}
}

func TestTypedCompatibilityGranularInputDiagnostics(t *testing.T) {
	for _, tool := range granularToolsForToolset(ToolsetMetadataIssues.ID, FeatureFlagIssuesGranular) {
		if tool.Tool.Name == "hide_issue_comment" || tool.Tool.Name == "unhide_issue_comment" {
			continue
		}
		_, err := tool.GetInputNormalizer()(json.RawMessage(`{}`))
		var inputError *inventory.ToolInputError
		require.ErrorAs(t, err, &inputError, tool.Tool.Name)
		assert.Equal(t, "missing required parameter: owner", inputError.Message, tool.Tool.Name)
	}
	tool := GranularUpdateIssueState(translations.NullTranslationHelper)
	deps := BaseDeps{featureChecker: featureCheckerFor(inventory.FeatureFlag(FeatureFlagIssuesGranular))}
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		session := connectTypedSearchClient(t, tool, deps, protocol)
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
			Name: tool.Tool.Name, Arguments: map[string]any{"owner": "owner", "repo": "repo", "issue_number": 1, "state": "open", "state_reason": "completed"},
		})
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Equal(t, "state_reason can only be used when state is 'closed'", getErrorResult(t, result).Text)
	}
}

func TestTypedCompatibilityPullRequestAfter(t *testing.T) {
	tool := PullRequestRead(translations.NullTranslationHelper)
	for _, method := range pullRequestArgumentSpecs["read"].methods {
		args := map[string]any{"method": method, "owner": "owner", "repo": "repo", "pullNumber": 1, "after": 42}
		normalized, err := tool.GetInputNormalizer()(json.RawMessage(mustMarshalJSON(t, args)))
		if method == "get_review_comments" {
			require.ErrorContains(t, err, "after")
		} else {
			require.NoError(t, err)
			var parsed map[string]any
			require.NoError(t, json.Unmarshal(normalized, &parsed))
			assert.NotContains(t, parsed, "after", method)
		}
	}
	deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposPullsFilesByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, []*github.CommitFile{}),
	}))}
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		session := connectTypedSearchClient(t, tool, deps, protocol)
		for _, method := range []string{"get_files", "get_review_comments"} {
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name: tool.Tool.Name, Arguments: map[string]any{"method": method, "owner": "owner", "repo": "repo", "pullNumber": 1, "after": 42},
			})
			require.NoError(t, err)
			if method == "get_review_comments" {
				assert.True(t, result.IsError)
				assert.Contains(t, getErrorResult(t, result).Text, "after")
			} else {
				require.False(t, result.IsError, mustMarshalJSON(t, result))
				assert.Equal(t, "[]", getTextResult(t, result).Text)
			}

		}
	}
}

func TestTypedCompatibilitySearchFieldValuePresence(t *testing.T) {
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		for _, tool := range []inventory.ServerTool{SearchIssues(translations.NullTranslationHelper), SearchPullRequests(translations.NullTranslationHelper)} {
			for _, available := range []bool{false, true} {
				if available && tool.Tool.Name == "search_pull_requests" {
					continue // Pull request search does not enrich custom issue fields.
				}
				t.Run(fmt.Sprintf("%s/%s/available=%t", protocol, tool.Tool.Name, available), func(t *testing.T) {
					issue := &github.Issue{Number: new(1), Title: new("Subject"), NodeID: new("I_1")}
					response := githubv4mock.ErrorResponse("Field 'issueFieldValues' doesn't exist on type 'Issue'")
					if available {
						response = githubv4mock.DataResponse(map[string]any{"nodes": []any{
							map[string]any{"id": "I_1", "issueFieldValues": map[string]any{"nodes": []any{}}},
						}})
					}
					matcher := githubv4mock.NewQueryMatcher(searchIssueFieldValuesQueryString, map[string]any{"ids": []any{"I_1"}}, response)
					deps := BaseDeps{
						Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
							GetSearchIssues: mockResponse(t, http.StatusOK, &github.IssuesSearchResult{Total: new(1), IncompleteResults: new(false), Issues: []*github.Issue{issue}}),
						})),
						GQLClient: githubv4.NewClient(githubv4mock.NewMockedHTTPClient(matcher)),
					}
					session := connectTypedSearchClient(t, tool, deps, protocol)
					for _, fields := range [][]string{nil, {"field_values"}, {"title"}} {
						if tool.Tool.Name == "search_pull_requests" && len(fields) > 0 && fields[0] == "field_values" {
							continue
						}
						args := map[string]any{"query": "repo:owner/repo"}
						if fields != nil {
							args["fields"] = fields
						}
						result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool.Tool.Name, Arguments: args})
						require.NoError(t, err)
						require.False(t, result.IsError, mustMarshalJSON(t, result))
						var output struct {
							Items []map[string]json.RawMessage `json:"items"`
						}
						require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &output))
						require.Len(t, output.Items, 1)
						if available && (fields == nil || fields[0] == "field_values") {
							assert.Equal(t, "[]", string(output.Items[0]["field_values"]))
						} else {
							assert.NotContains(t, output.Items[0], "field_values")
						}
						if protocol == inventory.ProtocolVersionMultiRoundTrip {
							assert.JSONEq(t, getTextResult(t, result).Text, mustMarshalJSON(t, result.StructuredContent))
						} else {
							assert.Nil(t, result.StructuredContent)
						}
					}
				})
			}
		}
	}
}

func TestTypedCompatibilityRepositorySearchContract(t *testing.T) {
	schema := SearchRepositories(translations.NullTranslationHelper).Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties["minimal_output"].Description, "modern clients receive additional curated repository details, not the complete GitHub API object")
	assert.Contains(t, schema.Properties["minimal_output"].Description, "Legacy clients retain full GitHub API repository objects")
}

func TestTypedCompatibilityGistRecoveryHandles(t *testing.T) {
	var gist github.Gist
	require.NoError(t, json.Unmarshal([]byte(gistBody), &gist))
	output := mustMarshalJSON(t, projectGist(&gist))
	assert.Contains(t, output, `"raw_url":"https://gist.githubusercontent.com/raw"`)
	assert.Contains(t, output, `"git_pull_url":"https://gist.github.com/abc.git"`)
}

func TestTypedCompatibilityReviewReactionParity(t *testing.T) {
	tool := GranularAddPullRequestReviewCommentReaction(translations.NullTranslationHelper)
	deps := BaseDeps{
		featureChecker: featureCheckerFor(inventory.FeatureFlag(FeatureFlagPullRequestsGranular)),
		Client: mustNewGHClient(t, &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":77,"content":"heart"}`))
		})}}),
	}
	expected := `{"id":"77","url":"https://api.github.com/repos/owner/repo/pulls/comments/42/reactions/77"}`
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		session := connectTypedSearchClient(t, tool, deps, protocol)
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
			Name: tool.Tool.Name, Arguments: map[string]any{"owner": "owner", "repo": "repo", "comment_id": 42, "content": "heart"},
		})
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.JSONEq(t, expected, getTextResult(t, result).Text)
		if protocol == inventory.ProtocolVersionMultiRoundTrip {
			assert.JSONEq(t, expected, mustMarshalJSON(t, result.StructuredContent))
		}
	}
}

func TestTypedCompatibilityDirectoryProjection(t *testing.T) {
	tool := GetFileContents(translations.NullTranslationHelper)
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		session := connectTypedSearchClient(t, tool, typedRepositoryDeps(t), protocol)
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
			Name: tool.Tool.Name, Arguments: map[string]any{"owner": "owner", "repo": "repo", "path": "directory", "fields": []string{"url", "git_url"}},
		})
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.JSONEq(t, `[{"url":"api-content-url","git_url":"api-git-url"}]`, getTextResult(t, result).Text)
	}
}

func TestTypedCompatibilityBlameIFC(t *testing.T) {
	for _, private := range []bool{false, true} {
		for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
			deps := typedRepositoryDeps(t)
			deps.featureChecker = featureCheckerFor(FeatureFlagIFCLabels)
			deps.Client = mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposByOwnerByRepo: mockResponse(t, http.StatusOK, &github.Repository{Private: &private}),
			}))
			session := connectTypedSearchClient(t, GetFileBlame(translations.NullTranslationHelper), deps, protocol)
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name: "get_file_blame", Arguments: map[string]any{"owner": "owner", "repo": "repo", "path": "file"},
			})
			require.NoError(t, err)
			require.False(t, result.IsError)
			assert.JSONEq(t, mustMarshalJSON(t, ifc.LabelCommitContents(private)), mustMarshalJSON(t, result.Meta["ifc"]))
		}
	}
}
