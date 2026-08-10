package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	gogithub "github.com/google/go-github/v89/github"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/pkg/translations"
)

// The end-to-end guarantee for pull_request_read, mirroring
// TestIssueReadOutputValidatesAgainstDeclaredSchema: what the real handler
// emits must validate against the schema the tool advertises. The
// structured-content mirror publishes the exact bytes of the single text block
// as structuredContent, so validating the text block here is equivalent to
// validating what a client receives.
//
// Every value of the tool's `method` enum is covered. The awkward cases are
// deliberate: list methods returning zero items (an empty array satisfies
// several array branches at once, which is why the union must be anyOf), and
// sparse objects with every optional field absent (which is what actually
// exercises each branch's required set rather than a convenient fixture).
//
// method=get_diff is the one exception, and it is asserted rather than skipped:
// it returns a raw unified diff, which is not JSON, so the mirror publishes no
// structuredContent and the schema declares no string branch. The subtest pins
// that invariant — if the diff ever became JSON, the tool would start emitting
// structuredContent that no branch describes.
func TestPullRequestReadOutputValidatesAgainstDeclaredSchema(t *testing.T) {
	t.Parallel()

	serverTool := PullRequestRead(translations.NullTranslationHelper)
	resolved := resolveToolSchema(t, pullRequestReadOutputSchema)

	// A populated PR. Head.SHA matters beyond method=get: get_status and
	// get_check_runs resolve the head SHA from this response first.
	mockPR := &gogithub.PullRequest{
		Number:  gogithub.Ptr(42),
		Title:   gogithub.Ptr("Test PR"),
		Body:    gogithub.Ptr("This is a test PR"),
		State:   gogithub.Ptr("open"),
		Draft:   gogithub.Ptr(false),
		Merged:  gogithub.Ptr(false),
		HTMLURL: gogithub.Ptr("https://github.com/owner/repo/pull/42"),
		User:    &gogithub.User{Login: gogithub.Ptr("octocat")},
		Head: &gogithub.PullRequestBranch{
			Ref:  gogithub.Ptr("feature-branch"),
			SHA:  gogithub.Ptr("abcd1234"),
			Repo: &gogithub.Repository{FullName: gogithub.Ptr("owner/repo")},
		},
		Base: &gogithub.PullRequestBranch{
			Ref: gogithub.Ptr("main"),
			SHA: gogithub.Ptr("ef567890"),
		},
		Labels:             []*gogithub.Label{{Name: "bug"}},
		Assignees:          []*gogithub.User{{Login: gogithub.Ptr("octocat")}},
		RequestedReviewers: []*gogithub.User{{Login: gogithub.Ptr("reviewer")}},
		Additions:          gogithub.Ptr(10),
		Deletions:          gogithub.Ptr(5),
		ChangedFiles:       gogithub.Ptr(2),
		Comments:           gogithub.Ptr(1),
		Commits:            gogithub.Ptr(3),
		CreatedAt:          &gogithub.Timestamp{Time: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)},
		UpdatedAt:          &gogithub.Timestamp{Time: time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)},
		Milestone:          &gogithub.Milestone{Title: gogithub.Ptr("v1")},
	}

	// The minimum a PR fetch can carry for the methods that need a head SHA.
	mockPRHeadOnly := &gogithub.PullRequest{
		Number: gogithub.Ptr(42),
		Head:   &gogithub.PullRequestBranch{SHA: gogithub.Ptr("abcd1234")},
	}

	stubbedDiff := `diff --git a/README.md b/README.md
index 5d6e7b2..8a4f5c3 100644
--- a/README.md
+++ b/README.md
@@ -1,4 +1,6 @@
 # Hello-World

+## New Section`

	reviewThreadsMatcher := func(nodes []map[string]any, pageInfo map[string]any, totalCount int) *http.Client {
		return githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				reviewThreadsQuery{},
				map[string]any{
					"owner":             githubv4.String("owner"),
					"repo":              githubv4.String("repo"),
					"prNum":             githubv4.Int(42),
					"first":             githubv4.Int(30),
					"commentsPerThread": githubv4.Int(100),
					"after":             (*githubv4.String)(nil),
				},
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"pullRequest": map[string]any{
							"reviewThreads": map[string]any{
								"nodes":      nodes,
								"pageInfo":   pageInfo,
								"totalCount": totalCount,
							},
						},
					},
				}),
			),
		)
	}

	tests := []struct {
		name          string
		method        string
		handlers      map[string]http.HandlerFunc
		gqlHTTPClient *http.Client
		// nonJSONText marks the one method whose result is deliberately not
		// JSON, so nothing is mirrored into structuredContent and the schema
		// declares no branch for it.
		nonJSONText bool
	}{
		{
			name:   "get",
			method: "get",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, mockPR),
			},
		},
		{
			// A fully sparse pull request: every optional field absent. Exercises
			// the branch's required set rather than a convenient fixture.
			name:   "get with all optionals absent",
			method: "get",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, &gogithub.PullRequest{}),
			},
		},
		{
			// Raw unified diff: not JSON, so no structuredContent is published
			// and the schema deliberately carries no string branch.
			name:   "get_diff",
			method: "get_diff",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, stubbedDiff),
			},
			nonJSONText: true,
		},
		{
			name:   "get_status",
			method: "get_status",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, mockPR),
				GetReposCommitsStatusByOwnerByRepoByRef: mockResponse(t, http.StatusOK, &gogithub.CombinedStatus{
					State:      gogithub.Ptr("success"),
					SHA:        gogithub.Ptr("abcd1234"),
					TotalCount: gogithub.Ptr(1),
					Statuses: []*gogithub.RepoStatus{{
						State:       gogithub.Ptr("success"),
						Context:     gogithub.Ptr("ci/build"),
						Description: gogithub.Ptr("Build succeeded"),
						TargetURL:   gogithub.Ptr("https://ci.example.com/1"),
					}},
				}),
			},
		},
		{
			// *github.CombinedStatus is all pointers+omitempty, so an all-nil
			// value really marshals to {}. The union has to keep that valid.
			name:   "get_status with an all-nil combined status",
			method: "get_status",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsByOwnerByRepoByPullNumber:  mockResponse(t, http.StatusOK, mockPRHeadOnly),
				GetReposCommitsStatusByOwnerByRepoByRef: mockResponse(t, http.StatusOK, &gogithub.CombinedStatus{}),
			},
		},
		{
			// Same story one level down: an all-nil *github.RepoStatus element
			// marshals to {} inside the statuses array.
			name:   "get_status with an all-nil status element",
			method: "get_status",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, mockPRHeadOnly),
				GetReposCommitsStatusByOwnerByRepoByRef: mockResponse(t, http.StatusOK, &gogithub.CombinedStatus{
					Statuses: []*gogithub.RepoStatus{{}},
				}),
			},
		},
		{
			name:   "get_files",
			method: "get_files",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsFilesByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, []*gogithub.CommitFile{{
					Filename:  gogithub.Ptr("file1.go"),
					Status:    gogithub.Ptr("modified"),
					Additions: gogithub.Ptr(10),
					Deletions: gogithub.Ptr(5),
					Changes:   gogithub.Ptr(15),
					Patch:     gogithub.Ptr("@@ -1,5 +1,10 @@"),
				}}),
			},
		},
		{
			// The ambiguous case that makes oneOf unusable: [] satisfies every
			// array branch at once.
			name:   "get_files with no files",
			method: "get_files",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsFilesByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, []*gogithub.CommitFile{}),
			},
		},
		{
			name:   "get_files with all optionals absent",
			method: "get_files",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsFilesByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, []*gogithub.CommitFile{{}}),
			},
		},
		{
			name:   "get_commits",
			method: "get_commits",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsCommitsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, []*gogithub.RepositoryCommit{{
					SHA:     gogithub.Ptr("abc123"),
					HTMLURL: gogithub.Ptr("https://github.com/owner/repo/commit/abc123"),
					Commit: &gogithub.Commit{
						Message: gogithub.Ptr("feat: add a thing"),
						Author: &gogithub.CommitAuthor{
							Name:  gogithub.Ptr("Test User"),
							Email: gogithub.Ptr("test@example.com"),
							Date:  &gogithub.Timestamp{Time: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)},
						},
					},
				}}),
			},
		},
		{
			name:   "get_commits with no commits",
			method: "get_commits",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsCommitsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, []*gogithub.RepositoryCommit{}),
			},
		},
		{
			name:   "get_commits with all optionals absent",
			method: "get_commits",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsCommitsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, []*gogithub.RepositoryCommit{{}}),
			},
		},
		{
			name:   "get_review_comments",
			method: "get_review_comments",
			gqlHTTPClient: reviewThreadsMatcher(
				[]map[string]any{{
					"id":          "RT_kwDOA0xdyM4AX1Yz",
					"isResolved":  false,
					"isOutdated":  false,
					"isCollapsed": false,
					"comments": map[string]any{
						"totalCount": 1,
						"nodes": []map[string]any{{
							"id":        "PRRC_kwDOA0xdyM4AX1Y0",
							"body":      "This looks good",
							"path":      "file1.go",
							"line":      5,
							"author":    map[string]any{"login": "reviewer1"},
							"createdAt": "2024-01-01T12:00:00Z",
							"updatedAt": "2024-01-01T12:00:00Z",
							"url":       "https://github.com/owner/repo/pull/42#discussion_r101",
						}},
					},
				}},
				map[string]any{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     "cursor1",
					"endCursor":       "cursor2",
				},
				1,
			),
		},
		{
			name:   "get_review_comments with no threads",
			method: "get_review_comments",
			gqlHTTPClient: reviewThreadsMatcher(
				[]map[string]any{},
				map[string]any{"hasNextPage": false, "hasPreviousPage": false},
				0,
			),
		},
		{
			// Sparse throughout: a thread with no comments, a comment with no
			// line/body/author, and page info with both cursors absent
			// (startCursor/endCursor are omitempty on MinimalPageInfo).
			name:   "get_review_comments with all optionals absent",
			method: "get_review_comments",
			gqlHTTPClient: reviewThreadsMatcher(
				[]map[string]any{
					{
						"id":       "RT_empty",
						"comments": map[string]any{"totalCount": 0, "nodes": []map[string]any{}},
					},
					{
						"id": "RT_sparse_comment",
						"comments": map[string]any{
							"totalCount": 1,
							"nodes": []map[string]any{{
								"path": "file1.go",
								"url":  "https://github.com/owner/repo/pull/42#discussion_r102",
							}},
						},
					},
				},
				map[string]any{"hasNextPage": false, "hasPreviousPage": false},
				2,
			),
		},
		{
			name:   "get_reviews",
			method: "get_reviews",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsReviewsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, []*gogithub.PullRequestReview{{
					ID:                gogithub.Ptr(int64(1)),
					State:             gogithub.Ptr("APPROVED"),
					Body:              gogithub.Ptr("LGTM"),
					HTMLURL:           gogithub.Ptr("https://github.com/owner/repo/pull/42#pullrequestreview-1"),
					User:              &gogithub.User{Login: gogithub.Ptr("reviewer")},
					CommitID:          gogithub.Ptr("abcd1234"),
					SubmittedAt:       &gogithub.Timestamp{Time: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)},
					AuthorAssociation: gogithub.Ptr("COLLABORATOR"),
				}}),
			},
		},
		{
			name:   "get_reviews with no reviews",
			method: "get_reviews",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsReviewsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, []*gogithub.PullRequestReview{}),
			},
		},
		{
			name:   "get_reviews with all optionals absent",
			method: "get_reviews",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsReviewsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, []*gogithub.PullRequestReview{{}}),
			},
		},
		{
			name:   "get_comments",
			method: "get_comments",
			handlers: map[string]http.HandlerFunc{
				GetReposIssuesCommentsByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, []*gogithub.IssueComment{{
					ID:                gogithub.Ptr(int64(1)),
					Body:              gogithub.Ptr("hello"),
					HTMLURL:           gogithub.Ptr("https://github.com/owner/repo/pull/42#issuecomment-1"),
					User:              &gogithub.User{Login: gogithub.Ptr("octocat")},
					AuthorAssociation: gogithub.Ptr("MEMBER"),
					Reactions:         &gogithub.Reactions{TotalCount: gogithub.Ptr(1), PlusOne: gogithub.Ptr(1)},
					CreatedAt:         &gogithub.Timestamp{Time: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)},
				}}),
			},
		},
		{
			name:   "get_comments with no comments",
			method: "get_comments",
			handlers: map[string]http.HandlerFunc{
				GetReposIssuesCommentsByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, []*gogithub.IssueComment{}),
			},
		},
		{
			name:   "get_comments with all optionals absent",
			method: "get_comments",
			handlers: map[string]http.HandlerFunc{
				GetReposIssuesCommentsByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, []*gogithub.IssueComment{{}}),
			},
		},
		{
			name:   "get_check_runs",
			method: "get_check_runs",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, mockPR),
				GetReposCommitsCheckRunsByOwnerByRepoByRef: mockResponse(t, http.StatusOK, &gogithub.ListCheckRunsResults{
					Total: gogithub.Ptr(1),
					CheckRuns: []*gogithub.CheckRun{{
						ID:          gogithub.Ptr(int64(1)),
						Name:        gogithub.Ptr("build"),
						Status:      gogithub.Ptr("completed"),
						Conclusion:  gogithub.Ptr("success"),
						HTMLURL:     gogithub.Ptr("https://github.com/owner/repo/runs/1"),
						DetailsURL:  gogithub.Ptr("https://ci.example.com/1"),
						StartedAt:   &gogithub.Timestamp{Time: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)},
						CompletedAt: &gogithub.Timestamp{Time: time.Date(2026, 4, 1, 12, 5, 0, 0, time.UTC)},
					}},
				}),
			},
		},
		{
			name:   "get_check_runs with no check runs",
			method: "get_check_runs",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, mockPRHeadOnly),
				GetReposCommitsCheckRunsByOwnerByRepoByRef: mockResponse(t, http.StatusOK, &gogithub.ListCheckRunsResults{
					Total:     gogithub.Ptr(0),
					CheckRuns: []*gogithub.CheckRun{},
				}),
			},
		},
		{
			name:   "get_check_runs with all optionals absent",
			method: "get_check_runs",
			handlers: map[string]http.HandlerFunc{
				GetReposPullsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, mockPRHeadOnly),
				GetReposCommitsCheckRunsByOwnerByRepoByRef: mockResponse(t, http.StatusOK, &gogithub.ListCheckRunsResults{
					CheckRuns: []*gogithub.CheckRun{{}},
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(tt.handlers))}
			if tt.gqlHTTPClient != nil {
				deps.GQLClient = githubv4.NewClient(tt.gqlHTTPClient)
			}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(map[string]any{
				"method":     tt.method,
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			})
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError, "handler should succeed")

			text := getTextResult(t, result)

			if tt.nonJSONText {
				require.False(t, json.Valid([]byte(text.Text)),
					"method=%s emits a raw body the mirror must not publish as structuredContent; "+
						"if it is now JSON, the schema needs a branch describing it", tt.method)
				return
			}

			var payload any
			require.NoError(t, json.Unmarshal([]byte(text.Text), &payload),
				"handler output must be JSON for the mirror to publish it as structuredContent")

			require.NoError(t, resolved.Validate(payload),
				"real handler output for method=%s must conform to the advertised outputSchema", tt.method)
		})
	}
}
