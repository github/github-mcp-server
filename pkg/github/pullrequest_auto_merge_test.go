package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/githubv4mock"
	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func enableAutoMergeMatcher(input githubv4.EnablePullRequestAutoMergeInput, mergeMethod string) githubv4mock.Matcher {
	return githubv4mock.NewMutationMatcher(
		struct {
			EnablePullRequestAutoMerge struct {
				PullRequest struct {
					AutoMergeRequest struct {
						MergeMethod githubv4.String
					}
				}
			} `graphql:"enablePullRequestAutoMerge(input: $input)"`
		}{},
		input,
		nil,
		githubv4mock.DataResponse(map[string]any{
			"enablePullRequestAutoMerge": map[string]any{
				"pullRequest": map[string]any{
					"autoMergeRequest": map[string]any{
						"mergeMethod": mergeMethod,
					},
				},
			},
		}),
	)
}

func disableAutoMergeMatcher(nodeID string) githubv4mock.Matcher {
	return githubv4mock.NewMutationMatcher(
		struct {
			DisablePullRequestAutoMerge struct {
				PullRequest struct {
					ID githubv4.ID
				}
			} `graphql:"disablePullRequestAutoMerge(input: $input)"`
		}{},
		githubv4.DisablePullRequestAutoMergeInput{
			PullRequestID: githubv4.ID(nodeID),
		},
		nil,
		githubv4mock.DataResponse(map[string]any{
			"disablePullRequestAutoMerge": map[string]any{
				"pullRequest": map[string]any{
					"id": nodeID,
				},
			},
		}),
	)
}

func autoMergeDeps(t *testing.T, prStatus int, matchers ...githubv4mock.Matcher) BaseDeps {
	t.Helper()
	var prHandler http.HandlerFunc
	if prStatus == http.StatusOK {
		prHandler = mockResponse(t, http.StatusOK, map[string]any{"node_id": "PR_42"})
	} else {
		prHandler = func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(prStatus)
			_, _ = w.Write([]byte(`{"message": "Not Found"}`))
		}
	}
	return BaseDeps{
		Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposPullsByOwnerByRepoByPullNumber: prHandler,
		})),
		GQLClient: githubv4.NewClient(githubv4mock.NewMockedHTTPClient(matchers...)),
	}
}

func Test_EnablePullRequestAutoMerge(t *testing.T) {
	serverTool := EnablePullRequestAutoMerge(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "enable_pull_request_auto_merge", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.Contains(t, schema.Properties, "merge_method")
	assert.Contains(t, schema.Properties, "commit_title")
	assert.Contains(t, schema.Properties, "commit_message")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "pullNumber"})

	squash := githubv4.PullRequestMergeMethodSquash
	headline := githubv4.String("Add the thing")
	body := githubv4.String("Extra detail")

	tests := []struct {
		name           string
		deps           BaseDeps
		requestArgs    map[string]any
		expectError    bool
		expectedResult *PullRequestAutoMergeResult
		expectedErrMsg string
	}{
		{
			name: "enable with method and commit text",
			deps: autoMergeDeps(t, http.StatusOK, enableAutoMergeMatcher(githubv4.EnablePullRequestAutoMergeInput{
				PullRequestID:  githubv4.ID("PR_42"),
				MergeMethod:    &squash,
				CommitHeadline: &headline,
				CommitBody:     &body,
			}, "SQUASH")),
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo", "pullNumber": float64(42),
				"merge_method": "squash", "commit_title": "Add the thing", "commit_message": "Extra detail",
			},
			expectedResult: &PullRequestAutoMergeResult{NodeID: "PR_42", AutoMergeEnabled: true, MergeMethod: "squash"},
		},
		{
			name: "enable with defaults only",
			deps: autoMergeDeps(t, http.StatusOK, enableAutoMergeMatcher(githubv4.EnablePullRequestAutoMergeInput{
				PullRequestID: githubv4.ID("PR_42"),
			}, "MERGE")),
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo", "pullNumber": float64(42),
			},
			expectedResult: &PullRequestAutoMergeResult{NodeID: "PR_42", AutoMergeEnabled: true, MergeMethod: "merge"},
		},
		{
			name: "pull request not found",
			deps: autoMergeDeps(t, http.StatusNotFound),
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo", "pullNumber": float64(42),
			},
			expectError:    true,
			expectedErrMsg: "failed to get pull request",
		},
		{
			name: "mutation failure surfaces",
			deps: autoMergeDeps(t, http.StatusOK, githubv4mock.NewMutationMatcher(
				struct {
					EnablePullRequestAutoMerge struct {
						PullRequest struct {
							AutoMergeRequest struct {
								MergeMethod githubv4.String
							}
						}
					} `graphql:"enablePullRequestAutoMerge(input: $input)"`
				}{},
				githubv4.EnablePullRequestAutoMergeInput{PullRequestID: githubv4.ID("PR_42")},
				nil,
				githubv4mock.ErrorResponse("auto-merge is not allowed for this repository"),
			)),
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo", "pullNumber": float64(42),
			},
			expectError:    true,
			expectedErrMsg: "failed to enable auto-merge for pull request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			serverTool := EnablePullRequestAutoMerge(translations.NullTranslationHelper)
			handler := serverTool.Handler(tc.deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), tc.deps), &request)
			require.NoError(t, err)
			if tc.expectError {
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}
			require.False(t, result.IsError)
			textContent := getTextResult(t, result)
			var returnedResult PullRequestAutoMergeResult
			require.NoError(t, json.Unmarshal([]byte(textContent.Text), &returnedResult))
			assert.Equal(t, *tc.expectedResult, returnedResult)
		})
	}
}

func Test_DisablePullRequestAutoMerge(t *testing.T) {
	serverTool := DisablePullRequestAutoMerge(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "disable_pull_request_auto_merge", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.NotContains(t, schema.Properties, "merge_method")
	assert.NotContains(t, schema.Properties, "commit_title")
	assert.NotContains(t, schema.Properties, "commit_message")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "pullNumber"})

	tests := []struct {
		name           string
		deps           BaseDeps
		requestArgs    map[string]any
		expectError    bool
		expectedResult *PullRequestAutoMergeResult
		expectedErrMsg string
	}{
		{
			name: "disable succeeds",
			deps: autoMergeDeps(t, http.StatusOK, disableAutoMergeMatcher("PR_42")),
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo", "pullNumber": float64(42),
			},
			expectedResult: &PullRequestAutoMergeResult{NodeID: "PR_42", AutoMergeEnabled: false},
		},
		{
			name: "pull request not found",
			deps: autoMergeDeps(t, http.StatusNotFound),
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo", "pullNumber": float64(42),
			},
			expectError:    true,
			expectedErrMsg: "failed to get pull request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			serverTool := DisablePullRequestAutoMerge(translations.NullTranslationHelper)
			handler := serverTool.Handler(tc.deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), tc.deps), &request)
			require.NoError(t, err)
			if tc.expectError {
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}
			require.False(t, result.IsError)
			textContent := getTextResult(t, result)
			var returnedResult PullRequestAutoMergeResult
			require.NoError(t, json.Unmarshal([]byte(textContent.Text), &returnedResult))
			assert.Equal(t, *tc.expectedResult, returnedResult)
		})
	}
}
