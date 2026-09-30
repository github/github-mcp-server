package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	getIssueCommentRoute  = "GET /repos/{owner}/{repo}/issues/comments/{comment_id}"
	getReviewCommentRoute = "GET /repos/{owner}/{repo}/pulls/comments/{comment_id}"
	getReviewRoute        = "GET /repos/{owner}/{repo}/pulls/{pull_number}/reviews/{review_id}"
)

func minimizeCommentMatcher(nodeID, classifier string) githubv4mock.Matcher {
	return githubv4mock.NewMutationMatcher(
		struct {
			MinimizeComment struct {
				MinimizedComment struct {
					IsMinimized     githubv4.Boolean
					MinimizedReason githubv4.String
				}
			} `graphql:"minimizeComment(input: $input)"`
		}{},
		githubv4.MinimizeCommentInput{
			SubjectID:  githubv4.ID(nodeID),
			Classifier: githubv4.ReportedContentClassifiers(classifier),
		},
		nil,
		githubv4mock.DataResponse(map[string]any{
			"minimizeComment": map[string]any{
				"minimizedComment": map[string]any{
					"isMinimized":     true,
					"minimizedReason": classifier,
				},
			},
		}),
	)
}

func unminimizeCommentMatcher(nodeID string) githubv4mock.Matcher {
	return githubv4mock.NewMutationMatcher(
		struct {
			UnminimizeComment struct {
				UnminimizedComment struct {
					IsMinimized githubv4.Boolean
				}
			} `graphql:"unminimizeComment(input: $input)"`
		}{},
		githubv4.UnminimizeCommentInput{SubjectID: githubv4.ID(nodeID)},
		nil,
		githubv4mock.DataResponse(map[string]any{
			"unminimizeComment": map[string]any{
				"unminimizedComment": map[string]any{"isMinimized": false},
			},
		}),
	)
}

func Test_CommentVisibilityToolSchemas(t *testing.T) {
	hide := GranularHideComment(translations.NullTranslationHelper).Tool
	require.NoError(t, toolsnaps.Test(hide.Name, hide))
	assert.Equal(t, "hide_comment", hide.Name)
	assert.False(t, hide.Annotations.ReadOnlyHint)
	assert.ElementsMatch(t, hide.InputSchema.(*jsonschema.Schema).Required,
		[]string{"owner", "repo", "comment_type", "comment_id", "classifier"})

	unhide := GranularUnhideComment(translations.NullTranslationHelper).Tool
	require.NoError(t, toolsnaps.Test(unhide.Name, unhide))
	assert.Equal(t, "unhide_comment", unhide.Name)
	assert.False(t, unhide.Annotations.ReadOnlyHint)
	unhideSchema := unhide.InputSchema.(*jsonschema.Schema)
	assert.ElementsMatch(t, unhideSchema.Required, []string{"owner", "repo", "comment_type", "comment_id"})
	assert.NotContains(t, unhideSchema.Properties, "classifier")
}

func Test_HideAndUnhideComment(t *testing.T) {
	tests := []struct {
		name           string
		tool           inventory.ServerTool
		restHandlers   map[string]http.HandlerFunc
		gqlMatchers    []githubv4mock.Matcher
		requestArgs    map[string]any
		expectedResult MinimizeCommentResult
		expectedErrMsg string
	}{
		{
			name: "hide issue comment",
			tool: GranularHideComment(translations.NullTranslationHelper),
			restHandlers: map[string]http.HandlerFunc{
				getIssueCommentRoute: mockResponse(t, http.StatusOK, &github.IssueComment{ID: github.Ptr(int64(1)), NodeID: github.Ptr("IC_1")}),
			},
			gqlMatchers: []githubv4mock.Matcher{minimizeCommentMatcher("IC_1", "SPAM")},
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo",
				"comment_type": "issue_comment", "comment_id": float64(1), "classifier": "spam",
			},
			expectedResult: MinimizeCommentResult{NodeID: "IC_1", IsMinimized: true, MinimizedReason: "SPAM"},
		},
		{
			name: "hide review comment",
			tool: GranularHideComment(translations.NullTranslationHelper),
			restHandlers: map[string]http.HandlerFunc{
				getReviewCommentRoute: mockResponse(t, http.StatusOK, &github.PullRequestComment{ID: github.Ptr(int64(2)), NodeID: github.Ptr("PRRC_2")}),
			},
			gqlMatchers: []githubv4mock.Matcher{minimizeCommentMatcher("PRRC_2", "OUTDATED")},
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo",
				"comment_type": "pull_request_review_comment", "comment_id": float64(2), "classifier": "OUTDATED",
			},
			expectedResult: MinimizeCommentResult{NodeID: "PRRC_2", IsMinimized: true, MinimizedReason: "OUTDATED"},
		},
		{
			name: "unhide review",
			tool: GranularUnhideComment(translations.NullTranslationHelper),
			restHandlers: map[string]http.HandlerFunc{
				getReviewRoute: mockResponse(t, http.StatusOK, &github.PullRequestReview{ID: github.Ptr(int64(3)), NodeID: github.Ptr("PRR_3")}),
			},
			gqlMatchers: []githubv4mock.Matcher{unminimizeCommentMatcher("PRR_3")},
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo",
				"comment_type": "pull_request_review", "comment_id": float64(3), "pull_number": float64(42),
			},
			expectedResult: MinimizeCommentResult{NodeID: "PRR_3", IsMinimized: false},
		},
		{
			name: "hide review",
			tool: GranularHideComment(translations.NullTranslationHelper),
			restHandlers: map[string]http.HandlerFunc{
				getReviewRoute: mockResponse(t, http.StatusOK, &github.PullRequestReview{ID: github.Ptr(int64(3)), NodeID: github.Ptr("PRR_3")}),
			},
			gqlMatchers: []githubv4mock.Matcher{minimizeCommentMatcher("PRR_3", "RESOLVED")},
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo",
				"comment_type": "pull_request_review", "comment_id": float64(3), "pull_number": float64(42), "classifier": "RESOLVED",
			},
			expectedResult: MinimizeCommentResult{NodeID: "PRR_3", IsMinimized: true, MinimizedReason: "RESOLVED"},
		},
		{
			name: "unhide review comment",
			tool: GranularUnhideComment(translations.NullTranslationHelper),
			restHandlers: map[string]http.HandlerFunc{
				getReviewCommentRoute: mockResponse(t, http.StatusOK, &github.PullRequestComment{ID: github.Ptr(int64(2)), NodeID: github.Ptr("PRRC_2")}),
			},
			gqlMatchers: []githubv4mock.Matcher{unminimizeCommentMatcher("PRRC_2")},
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo",
				"comment_type": "pull_request_review_comment", "comment_id": float64(2),
			},
			expectedResult: MinimizeCommentResult{NodeID: "PRRC_2", IsMinimized: false},
		},
		{
			name: "hide mutation fails",
			tool: GranularHideComment(translations.NullTranslationHelper),
			restHandlers: map[string]http.HandlerFunc{
				getIssueCommentRoute: mockResponse(t, http.StatusOK, &github.IssueComment{ID: github.Ptr(int64(1)), NodeID: github.Ptr("IC_1")}),
			},
			gqlMatchers: []githubv4mock.Matcher{
				githubv4mock.NewMutationMatcher(
					struct {
						MinimizeComment struct {
							MinimizedComment struct {
								IsMinimized     githubv4.Boolean
								MinimizedReason githubv4.String
							}
						} `graphql:"minimizeComment(input: $input)"`
					}{},
					githubv4.MinimizeCommentInput{SubjectID: githubv4.ID("IC_1"), Classifier: githubv4.ReportedContentClassifiersSpam},
					nil,
					githubv4mock.ErrorResponse("Resource not accessible by integration"),
				),
			},
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo",
				"comment_type": "issue_comment", "comment_id": float64(1), "classifier": "SPAM",
			},
			expectedErrMsg: "failed to minimize comment",
		},
		{
			name: "unhide mutation fails",
			tool: GranularUnhideComment(translations.NullTranslationHelper),
			restHandlers: map[string]http.HandlerFunc{
				getIssueCommentRoute: mockResponse(t, http.StatusOK, &github.IssueComment{ID: github.Ptr(int64(1)), NodeID: github.Ptr("IC_1")}),
			},
			gqlMatchers: []githubv4mock.Matcher{
				githubv4mock.NewMutationMatcher(
					struct {
						UnminimizeComment struct {
							UnminimizedComment struct {
								IsMinimized githubv4.Boolean
							}
						} `graphql:"unminimizeComment(input: $input)"`
					}{},
					githubv4.UnminimizeCommentInput{SubjectID: githubv4.ID("IC_1")},
					nil,
					githubv4mock.ErrorResponse("Resource not accessible by integration"),
				),
			},
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo",
				"comment_type": "issue_comment", "comment_id": float64(1),
			},
			expectedErrMsg: "failed to unminimize comment",
		},
		{
			name: "comment without node ID",
			tool: GranularUnhideComment(translations.NullTranslationHelper),
			restHandlers: map[string]http.HandlerFunc{
				getIssueCommentRoute: mockResponse(t, http.StatusOK, &github.IssueComment{ID: github.Ptr(int64(1))}),
			},
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo",
				"comment_type": "issue_comment", "comment_id": float64(1),
			},
			expectedErrMsg: "comment has no node ID",
		},
		{
			name: "missing owner",
			tool: GranularUnhideComment(translations.NullTranslationHelper),
			requestArgs: map[string]any{
				"repo": "repo", "comment_type": "issue_comment", "comment_id": float64(1),
			},
			expectedErrMsg: "owner",
		},
		{
			name: "missing repo",
			tool: GranularUnhideComment(translations.NullTranslationHelper),
			requestArgs: map[string]any{
				"owner": "owner", "comment_type": "issue_comment", "comment_id": float64(1),
			},
			expectedErrMsg: "repo",
		},
		{
			name: "missing comment_type",
			tool: GranularUnhideComment(translations.NullTranslationHelper),
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo", "comment_id": float64(1),
			},
			expectedErrMsg: "comment_type",
		},
		{
			name: "missing comment_id",
			tool: GranularHideComment(translations.NullTranslationHelper),
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo", "comment_type": "issue_comment", "classifier": "SPAM",
			},
			expectedErrMsg: "comment_id",
		},
		{
			name: "invalid pull_number type",
			tool: GranularUnhideComment(translations.NullTranslationHelper),
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo", "comment_type": "pull_request_review",
				"comment_id": float64(3), "pull_number": "forty-two",
			},
			expectedErrMsg: "pull_number",
		},
		{
			name: "hide without classifier",
			tool: GranularHideComment(translations.NullTranslationHelper),
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo",
				"comment_type": "issue_comment", "comment_id": float64(1),
			},
			expectedErrMsg: "classifier",
		},
		{
			name: "review without pull_number",
			tool: GranularUnhideComment(translations.NullTranslationHelper),
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo",
				"comment_type": "pull_request_review", "comment_id": float64(3),
			},
			expectedErrMsg: "pull_number is required",
		},
		{
			name: "unknown comment type",
			tool: GranularUnhideComment(translations.NullTranslationHelper),
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo",
				"comment_type": "commit_comment", "comment_id": float64(1),
			},
			expectedErrMsg: "unknown comment_type",
		},
		{
			name: "comment not found",
			tool: GranularUnhideComment(translations.NullTranslationHelper),
			restHandlers: map[string]http.HandlerFunc{
				getIssueCommentRoute: mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
			},
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo",
				"comment_type": "issue_comment", "comment_id": float64(1),
			},
			expectedErrMsg: "failed to get comment",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := BaseDeps{
				Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(tc.restHandlers)),
				GQLClient: githubv4.NewClient(githubv4mock.NewMockedHTTPClient(tc.gqlMatchers...)),
			}
			handler := tc.tool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			if tc.expectedErrMsg != "" {
				require.True(t, result.IsError)
				assert.Contains(t, getErrorResult(t, result).Text, tc.expectedErrMsg)
				return
			}

			require.False(t, result.IsError, getTextResult(t, result).Text)
			var got MinimizeCommentResult
			require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &got))
			assert.Equal(t, tc.expectedResult, got)
		})
	}
}
