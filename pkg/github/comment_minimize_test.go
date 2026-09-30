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

// minimizedReason is the lowercase, hyphenated form GitHub returns (e.g. "off-topic"), not the classifier enum.
func minimizeCommentMatcher(nodeID, classifier, minimizedReason string) githubv4mock.Matcher {
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
					"minimizedReason": minimizedReason,
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

func minimizeCommentErrorMatcher(nodeID, classifier string) githubv4mock.Matcher {
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
		githubv4mock.ErrorResponse("Resource not accessible by integration"),
	)
}

func unminimizeCommentErrorMatcher(nodeID string) githubv4mock.Matcher {
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
		githubv4mock.ErrorResponse("Resource not accessible by integration"),
	)
}

func Test_CommentVisibilityToolSchemas(t *testing.T) {
	tests := []struct {
		tool            inventory.ServerTool
		name            string
		toolset         inventory.ToolsetID
		featureFlag     string
		expectedRequire []string
	}{
		{
			tool:            GranularHideIssueComment(translations.NullTranslationHelper),
			name:            "hide_issue_comment",
			toolset:         ToolsetMetadataIssues.ID,
			featureFlag:     FeatureFlagIssuesGranular,
			expectedRequire: []string{"owner", "repo", "comment_id", "classifier"},
		},
		{
			tool:            GranularUnhideIssueComment(translations.NullTranslationHelper),
			name:            "unhide_issue_comment",
			toolset:         ToolsetMetadataIssues.ID,
			featureFlag:     FeatureFlagIssuesGranular,
			expectedRequire: []string{"owner", "repo", "comment_id"},
		},
		{
			tool:            GranularHidePullRequestReviewComment(translations.NullTranslationHelper),
			name:            "hide_pull_request_review_comment",
			toolset:         ToolsetMetadataPullRequests.ID,
			featureFlag:     FeatureFlagPullRequestsGranular,
			expectedRequire: []string{"owner", "repo", "comment_id", "classifier"},
		},
		{
			tool:            GranularUnhidePullRequestReviewComment(translations.NullTranslationHelper),
			name:            "unhide_pull_request_review_comment",
			toolset:         ToolsetMetadataPullRequests.ID,
			featureFlag:     FeatureFlagPullRequestsGranular,
			expectedRequire: []string{"owner", "repo", "comment_id"},
		},
		{
			tool:            GranularHidePullRequestReview(translations.NullTranslationHelper),
			name:            "hide_pull_request_review",
			toolset:         ToolsetMetadataPullRequests.ID,
			featureFlag:     FeatureFlagPullRequestsGranular,
			expectedRequire: []string{"owner", "repo", "pullNumber", "review_id", "classifier"},
		},
		{
			tool:            GranularUnhidePullRequestReview(translations.NullTranslationHelper),
			name:            "unhide_pull_request_review",
			toolset:         ToolsetMetadataPullRequests.ID,
			featureFlag:     FeatureFlagPullRequestsGranular,
			expectedRequire: []string{"owner", "repo", "pullNumber", "review_id"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tool := tc.tool.Tool
			require.NoError(t, toolsnaps.Test(tool.Name, tool))

			assert.Equal(t, tc.name, tool.Name)
			assert.NotEmpty(t, tool.Description)
			assert.False(t, tool.Annotations.ReadOnlyHint)
			assert.Equal(t, tc.toolset, tc.tool.Toolset.ID)
			assert.Equal(t, []inventory.FeatureFlag{inventory.FeatureFlag(tc.featureFlag)}, tc.tool.FeatureRule.Features())

			schema := tool.InputSchema.(*jsonschema.Schema)
			assert.ElementsMatch(t, tc.expectedRequire, schema.Required)
			assert.Len(t, schema.Properties, len(tc.expectedRequire), "every property should be required")
		})
	}
}

func Test_HideAndUnhideComments(t *testing.T) {
	issueComment := mockResponse(t, http.StatusOK, &github.IssueComment{ID: github.Ptr(int64(1)), NodeID: github.Ptr("IC_1")})
	reviewComment := mockResponse(t, http.StatusOK, &github.PullRequestComment{ID: github.Ptr(int64(2)), NodeID: github.Ptr("PRRC_2")})
	review := mockResponse(t, http.StatusOK, &github.PullRequestReview{ID: github.Ptr(int64(3)), NodeID: github.Ptr("PRR_3")})
	notFound := mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`)

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
			name:           "hide issue comment",
			tool:           GranularHideIssueComment(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getIssueCommentRoute: issueComment},
			gqlMatchers:    []githubv4mock.Matcher{minimizeCommentMatcher("IC_1", "SPAM", "spam")},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(1), "classifier": "spam"},
			expectedResult: MinimizeCommentResult{NodeID: "IC_1", IsMinimized: true, MinimizedReason: "spam"},
		},
		{
			name:           "unhide issue comment",
			tool:           GranularUnhideIssueComment(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getIssueCommentRoute: issueComment},
			gqlMatchers:    []githubv4mock.Matcher{unminimizeCommentMatcher("IC_1")},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(1)},
			expectedResult: MinimizeCommentResult{NodeID: "IC_1", IsMinimized: false},
		},
		{
			name:           "hide pull request review comment",
			tool:           GranularHidePullRequestReviewComment(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getReviewCommentRoute: reviewComment},
			gqlMatchers:    []githubv4mock.Matcher{minimizeCommentMatcher("PRRC_2", "OUTDATED", "outdated")},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(2), "classifier": "OUTDATED"},
			expectedResult: MinimizeCommentResult{NodeID: "PRRC_2", IsMinimized: true, MinimizedReason: "outdated"},
		},
		{
			name:           "unhide pull request review comment",
			tool:           GranularUnhidePullRequestReviewComment(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getReviewCommentRoute: reviewComment},
			gqlMatchers:    []githubv4mock.Matcher{unminimizeCommentMatcher("PRRC_2")},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(2)},
			expectedResult: MinimizeCommentResult{NodeID: "PRRC_2", IsMinimized: false},
		},
		{
			name:           "hide pull request review",
			tool:           GranularHidePullRequestReview(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getReviewRoute: review},
			gqlMatchers:    []githubv4mock.Matcher{minimizeCommentMatcher("PRR_3", "RESOLVED", "resolved")},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(42), "review_id": float64(3), "classifier": "RESOLVED"},
			expectedResult: MinimizeCommentResult{NodeID: "PRR_3", IsMinimized: true, MinimizedReason: "resolved"},
		},
		{
			name:           "unhide pull request review",
			tool:           GranularUnhidePullRequestReview(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getReviewRoute: review},
			gqlMatchers:    []githubv4mock.Matcher{unminimizeCommentMatcher("PRR_3")},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(42), "review_id": float64(3)},
			expectedResult: MinimizeCommentResult{NodeID: "PRR_3", IsMinimized: false},
		},
		{
			name:           "issue comment not found",
			tool:           GranularUnhideIssueComment(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getIssueCommentRoute: notFound},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(1)},
			expectedErrMsg: "failed to get issue comment",
		},
		{
			name:           "pull request review comment not found",
			tool:           GranularUnhidePullRequestReviewComment(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getReviewCommentRoute: notFound},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(2)},
			expectedErrMsg: "failed to get pull request review comment",
		},
		{
			name:           "pull request review not found",
			tool:           GranularUnhidePullRequestReview(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getReviewRoute: notFound},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(42), "review_id": float64(3)},
			expectedErrMsg: "failed to get pull request review",
		},
		{
			name: "response without node ID",
			tool: GranularUnhideIssueComment(translations.NullTranslationHelper),
			restHandlers: map[string]http.HandlerFunc{
				getIssueCommentRoute: mockResponse(t, http.StatusOK, &github.IssueComment{ID: github.Ptr(int64(1))}),
			},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(1)},
			expectedErrMsg: "response has no node ID",
		},
		{
			name:           "hide mutation fails",
			tool:           GranularHideIssueComment(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getIssueCommentRoute: issueComment},
			gqlMatchers:    []githubv4mock.Matcher{minimizeCommentErrorMatcher("IC_1", "SPAM")},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(1), "classifier": "SPAM"},
			expectedErrMsg: "failed to minimize comment",
		},
		{
			name:           "unhide mutation fails",
			tool:           GranularUnhideIssueComment(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getIssueCommentRoute: issueComment},
			gqlMatchers:    []githubv4mock.Matcher{unminimizeCommentErrorMatcher("IC_1")},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(1)},
			expectedErrMsg: "failed to unminimize comment",
		},
		{
			name:           "invalid classifier is rejected before any API call",
			tool:           GranularHideIssueComment(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(1), "classifier": "BOGUS"},
			expectedErrMsg: `invalid classifier "BOGUS"`,
		},
		{
			name:           "negative issue comment_id",
			tool:           GranularHideIssueComment(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(-1), "classifier": "SPAM"},
			expectedErrMsg: "comment_id must be greater than 0",
		},
		{
			name:           "negative pull request review comment_id",
			tool:           GranularUnhidePullRequestReviewComment(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(-2)},
			expectedErrMsg: "comment_id must be greater than 0",
		},
		{
			name:           "negative review_id",
			tool:           GranularUnhidePullRequestReview(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(42), "review_id": float64(-3)},
			expectedErrMsg: "review_id must be greater than 0",
		},
		{
			name:           "negative pullNumber",
			tool:           GranularUnhidePullRequestReview(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(-42), "review_id": float64(3)},
			expectedErrMsg: "pullNumber must be greater than 0",
		},
		{
			name:           "missing owner",
			tool:           GranularUnhideIssueComment(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"repo": "repo", "comment_id": float64(1)},
			expectedErrMsg: "owner",
		},
		{
			name:           "missing repo",
			tool:           GranularUnhideIssueComment(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "comment_id": float64(1)},
			expectedErrMsg: "repo",
		},
		{
			name:           "missing classifier",
			tool:           GranularHideIssueComment(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(1)},
			expectedErrMsg: "classifier",
		},
		{
			name:           "missing issue comment_id",
			tool:           GranularUnhideIssueComment(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo"},
			expectedErrMsg: "comment_id",
		},
		{
			name:           "missing pull request review comment_id",
			tool:           GranularUnhidePullRequestReviewComment(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo"},
			expectedErrMsg: "comment_id",
		},
		{
			name:           "missing pullNumber",
			tool:           GranularUnhidePullRequestReview(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "review_id": float64(3)},
			expectedErrMsg: "pullNumber",
		},
		{
			name:           "missing review_id",
			tool:           GranularUnhidePullRequestReview(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(42)},
			expectedErrMsg: "review_id",
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
