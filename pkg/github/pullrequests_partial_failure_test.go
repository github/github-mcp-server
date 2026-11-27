package github

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/githubv4mock"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reviewerRequestRejected(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusUnprocessableEntity)
	_, _ = w.Write([]byte(`{"message":"Review cannot be requested from pull request author."}`))
}

func Test_CreatePullRequest_ReviewerFailureNamesCreatedPullRequest(t *testing.T) {
	createdPR := &github.PullRequest{
		ID:      new(int64(4242)),
		Number:  new(42),
		HTMLURL: new("https://github.com/owner/repo/pull/42"),
	}
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PostReposPullsByOwnerByRepo:                               mockResponse(t, http.StatusCreated, createdPR),
		PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber: reviewerRequestRejected,
	}))
	deps := BaseDeps{Client: client}
	serverTool := CreatePullRequest(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":     "owner",
		"repo":      "repo",
		"title":     "Test PR",
		"head":      "feature-branch",
		"base":      "main",
		"reviewers": []any{"testuser"},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.True(t, result.IsError, "the reviewer request failed, so the call is still an error")

	text := getErrorResult(t, result).Text
	// The pull request exists now; a retry would only fail with "already exists", so the
	// error has to carry the number and URL.
	assert.Contains(t, text, "pull request #42 was created (https://github.com/owner/repo/pull/42), but failed to request reviewers")
	assert.Contains(t, text, "Review cannot be requested from pull request author.")
}

func Test_UpdatePullRequest_LaterStepFailureReportsEarlierChanges(t *testing.T) {
	updatedPR := &github.PullRequest{
		ID:      new(int64(4242)),
		Number:  new(42),
		Title:   new("Changed title"),
		HTMLURL: new("https://github.com/owner/repo/pull/42"),
	}

	draftQuery := func(isDraft bool) githubv4mock.Matcher {
		return githubv4mock.NewQueryMatcher(
			struct {
				Repository struct {
					PullRequest struct {
						ID      githubv4.ID
						IsDraft githubv4.Boolean
					} `graphql:"pullRequest(number: $prNum)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}{},
			map[string]any{
				"owner": githubv4.String("owner"),
				"repo":  githubv4.String("repo"),
				"prNum": githubv4.Int(42),
			},
			githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{
					"pullRequest": map[string]any{"id": "PR_42", "isDraft": isDraft},
				},
			}),
		)
	}
	convertToDraft := func(resp githubv4mock.GQLResponse) githubv4mock.Matcher {
		return githubv4mock.NewMutationMatcher(
			struct {
				ConvertPullRequestToDraft struct {
					PullRequest struct {
						ID      githubv4.ID
						IsDraft githubv4.Boolean
					}
				} `graphql:"convertPullRequestToDraft(input: $input)"`
			}{},
			githubv4.ConvertPullRequestToDraftInput{PullRequestID: "PR_42"},
			nil,
			resp,
		)
	}
	convertedToDraft := githubv4mock.DataResponse(map[string]any{
		"convertPullRequestToDraft": map[string]any{
			"pullRequest": map[string]any{"id": "PR_42", "isDraft": true},
		},
	})

	tests := []struct {
		name          string
		restHandlers  map[string]http.HandlerFunc
		gqlMatchers   []githubv4mock.Matcher
		args          map[string]any
		expectedErr   string
		notExpectText string
	}{
		{
			name: "reviewers rejected after the title was changed",
			restHandlers: map[string]http.HandlerFunc{
				PatchReposPullsByOwnerByRepoByPullNumber:                  mockResponse(t, http.StatusOK, updatedPR),
				PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber: reviewerRequestRejected,
			},
			args:        map[string]any{"title": "Changed title", "reviewers": []any{"testuser"}},
			expectedErr: "pull request #42 was updated, but failed to request reviewers",
		},
		{
			name: "reviewers rejected after the draft state was changed",
			restHandlers: map[string]http.HandlerFunc{
				PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber: reviewerRequestRejected,
			},
			gqlMatchers: []githubv4mock.Matcher{draftQuery(false), convertToDraft(convertedToDraft)},
			args:        map[string]any{"draft": true, "reviewers": []any{"testuser"}},
			expectedErr: "pull request #42 was updated, but failed to request reviewers",
		},
		{
			name: "draft conversion fails after the title was changed",
			restHandlers: map[string]http.HandlerFunc{
				PatchReposPullsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, updatedPR),
			},
			gqlMatchers: []githubv4mock.Matcher{draftQuery(false), convertToDraft(githubv4mock.ErrorResponse("conversion failed"))},
			args:        map[string]any{"title": "Changed title", "draft": true},
			expectedErr: "pull request #42 was updated, but failed to convert pull request to draft",
		},
		{
			name: "final read fails after the reviewers were requested",
			restHandlers: map[string]http.HandlerFunc{
				PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber: mockResponse(t, http.StatusCreated, updatedPR),
				GetReposPullsByOwnerByRepoByPullNumber: func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"message":"Server Error"}`))
				},
			},
			args:        map[string]any{"reviewers": []any{"testuser"}},
			expectedErr: "pull request #42 was updated, but failed to get pull request",
		},
		{
			name: "reviewers rejected when nothing else was requested",
			restHandlers: map[string]http.HandlerFunc{
				PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber: reviewerRequestRejected,
			},
			args:          map[string]any{"reviewers": []any{"testuser"}},
			expectedErr:   "failed to request reviewers",
			notExpectText: "was updated",
		},
		{
			name:          "draft conversion fails when nothing else was requested",
			restHandlers:  map[string]http.HandlerFunc{},
			gqlMatchers:   []githubv4mock.Matcher{draftQuery(false), convertToDraft(githubv4mock.ErrorResponse("conversion failed"))},
			args:          map[string]any{"draft": true},
			expectedErr:   "Failed to convert pull request to draft",
			notExpectText: "was updated",
		},
		{
			name: "title change rejected",
			restHandlers: map[string]http.HandlerFunc{
				PatchReposPullsByOwnerByRepoByPullNumber: func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusUnprocessableEntity)
					_, _ = w.Write([]byte(`{"message":"Validation Failed"}`))
				},
			},
			args:          map[string]any{"title": "Changed title", "reviewers": []any{"testuser"}},
			expectedErr:   "failed to update pull request",
			notExpectText: "was updated",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := BaseDeps{
				Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(tc.restHandlers)),
				GQLClient: githubv4.NewClient(githubv4mock.NewMockedHTTPClient(tc.gqlMatchers...)),
			}
			serverTool := UpdatePullRequest(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			args := map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(42)}
			maps.Copy(args, tc.args)
			request := createMCPRequest(args)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.True(t, result.IsError)

			text := getErrorResult(t, result).Text
			assert.Contains(t, text, tc.expectedErr)
			if tc.notExpectText != "" {
				assert.NotContains(t, text, tc.notExpectText)
			}
		})
	}
}

// clientFailingDeps fails GetClient from its failRESTFrom-th call on (1-based), and
// GetGQLClient on every call when failGQL is set. 0 disables the REST failure.
type clientFailingDeps struct {
	BaseDeps
	restCalls    *int
	failRESTFrom int
	failGQL      bool
}

func (d clientFailingDeps) GetClient(ctx context.Context) (*github.Client, error) {
	*d.restCalls++
	if d.failRESTFrom > 0 && *d.restCalls >= d.failRESTFrom {
		return nil, errors.New("client unavailable")
	}
	return d.BaseDeps.GetClient(ctx)
}

func (d clientFailingDeps) GetGQLClient(ctx context.Context) (*githubv4.Client, error) {
	if d.failGQL {
		return nil, errors.New("graphql client unavailable")
	}
	return d.BaseDeps.GetGQLClient(ctx)
}

func Test_UpdatePullRequest_ClientFailureAfterChangeReportsEarlierChanges(t *testing.T) {
	updatedPR := &github.PullRequest{
		ID:      new(int64(4242)),
		Number:  new(42),
		HTMLURL: new("https://github.com/owner/repo/pull/42"),
	}
	tests := []struct {
		name          string
		args          map[string]any
		failRESTFrom  int
		failGQL       bool
		expectedErr   string
		notExpectText string
	}{
		{
			name:        "GraphQL client fails after the title was changed",
			args:        map[string]any{"title": "Changed title", "draft": true},
			failGQL:     true,
			expectedErr: "pull request #42 was updated, but failed to get GitHub GraphQL client",
		},
		{
			name:         "client for the reviewer request fails after the title was changed",
			args:         map[string]any{"title": "Changed title", "reviewers": []any{"testuser"}},
			failRESTFrom: 2,
			expectedErr:  "pull request #42 was updated, but failed to get GitHub client",
		},
		{
			name:         "client for the final read fails after the reviewers were requested",
			args:         map[string]any{"reviewers": []any{"testuser"}},
			failRESTFrom: 2,
			expectedErr:  "pull request #42 was updated, but failed to get GitHub client",
		},
		{
			name:          "GraphQL client fails when nothing was changed",
			args:          map[string]any{"draft": true},
			failGQL:       true,
			expectedErr:   "failed to get GitHub GraphQL client",
			notExpectText: "was updated",
		},
		{
			name:          "client for the reviewer request fails when nothing was changed",
			args:          map[string]any{"reviewers": []any{"testuser"}},
			failRESTFrom:  1,
			expectedErr:   "failed to get GitHub client",
			notExpectText: "was updated",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			deps := clientFailingDeps{
				Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
					PatchReposPullsByOwnerByRepoByPullNumber:                  mockResponse(t, http.StatusOK, updatedPR),
					PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber: mockResponse(t, http.StatusCreated, updatedPR),
				})),
				GQLClient:    githubv4.NewClient(githubv4mock.NewMockedHTTPClient()),
				restCalls:    &calls,
				failRESTFrom: tc.failRESTFrom,
				failGQL:      tc.failGQL,
			}
			serverTool := UpdatePullRequest(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			args := map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(42)}
			maps.Copy(args, tc.args)
			request := createMCPRequest(args)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.True(t, result.IsError)

			text := getErrorResult(t, result).Text
			assert.Contains(t, text, tc.expectedErr)
			if tc.notExpectText != "" {
				assert.NotContains(t, text, tc.notExpectText)
			}
		})
	}
}
