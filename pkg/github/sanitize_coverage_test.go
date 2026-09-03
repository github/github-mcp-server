package github

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-github/v89/github"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// maliciousText contains an HTML payload plus an invisible character.
const maliciousText = "<script>alert(1)</script>Hello\u200BWorld"

const sanitizedText = "HelloWorld"
const sanitizedContentText = "<script>alert(1)</script>HelloWorld"

// Test_MinimalConverters_SanitizeUserAuthoredText covers the converter fields that expose
// untrusted titles, bodies, comments, reviews, releases, and commit messages.
func Test_MinimalConverters_SanitizeUserAuthoredText(t *testing.T) {
	tests := []struct {
		name    string
		content bool
		got     func() string
	}{
		{
			name: "issue title (REST)",
			got: func() string {
				return convertToMinimalIssue(&github.Issue{
					Title: github.Ptr(maliciousText),
				}).Title
			},
		},
		{
			name:    "issue body (REST)",
			content: true,
			got: func() string {
				return convertToMinimalIssue(&github.Issue{
					Body: github.Ptr(maliciousText),
				}).Body
			},
		},
		{
			name:    "issue comment body",
			content: true,
			got: func() string {
				return convertToMinimalIssueComment(&github.IssueComment{
					Body: github.Ptr(maliciousText),
				}).Body
			},
		},
		{
			name: "pull request title",
			got: func() string {
				return convertToMinimalPullRequest(&github.PullRequest{
					Title: github.Ptr(maliciousText),
				}).Title
			},
		},
		{
			name:    "pull request body",
			content: true,
			got: func() string {
				return convertToMinimalPullRequest(&github.PullRequest{
					Body: github.Ptr(maliciousText),
				}).Body
			},
		},
		{
			name:    "pull request review body",
			content: true,
			got: func() string {
				return convertToMinimalPullRequestReview(&github.PullRequestReview{
					Body: github.Ptr(maliciousText),
				}).Body
			},
		},
		{
			name:    "pull request review comment body (GraphQL)",
			content: true,
			got: func() string {
				return convertToMinimalReviewComment(reviewCommentNode{
					Body: githubv4.String(maliciousText),
					URL:  githubv4.URI{URL: &url.URL{Scheme: "https", Host: "github.com"}},
				}).Body
			},
		},
		{
			name: "release name",
			got: func() string {
				return convertToMinimalRelease(&github.RepositoryRelease{
					Name: github.Ptr(maliciousText),
				}).Name
			},
		},
		{
			name:    "release body",
			content: true,
			got: func() string {
				return convertToMinimalRelease(&github.RepositoryRelease{
					Body: github.Ptr(maliciousText),
				}).Body
			},
		},
		{
			name:    "commit message (get_commit / list_commits)",
			content: true,
			got: func() string {
				commit := convertToMinimalCommit(&github.RepositoryCommit{
					Commit: &github.Commit{Message: github.Ptr(maliciousText)},
				}, commitDetailNone)
				require.NotNil(t, commit.Commit)
				return commit.Commit.Message
			},
		},
		{
			name:    "commit message (search_commits)",
			content: true,
			got: func() string {
				item := convertCommitResultToMinimalCommit(&github.CommitResult{
					Commit: &github.Commit{Message: github.Ptr(maliciousText)},
				})
				require.NotNil(t, item.Commit)
				return item.Commit.Message
			},
		},
		{
			name:    "pull request commit message (list_pull_request_commits)",
			content: true,
			got: func() string {
				commits := convertToMinimalPullRequestCommits([]*github.RepositoryCommit{
					{Commit: &github.Commit{Message: github.Ptr(maliciousText)}},
				})
				require.Len(t, commits, 1)
				return commits[0].Message
			},
		},
		{
			name:    "file commit message (create/update/delete file)",
			content: true,
			got: func() string {
				resp := convertToMinimalFileContentResponse(&github.RepositoryContentResponse{
					Commit: github.Commit{Message: github.Ptr(maliciousText)},
				})
				require.NotNil(t, resp.Commit)
				return resp.Commit.Message
			},
		},
		{
			name:    "workflow run head commit message",
			content: true,
			got: func() string {
				run := convertToMinimalWorkflowRun(&github.WorkflowRun{
					HeadCommit: &github.HeadCommit{Message: github.Ptr(maliciousText)},
				})
				require.NotNil(t, run.HeadCommit)
				return run.HeadCommit.Message
			},
		},
		{
			name: "project item content title (issue)",
			got: func() string {
				return convertIssueToMinimalProjectItemContent(&github.Issue{
					Title: github.Ptr(maliciousText),
				}).Title
			},
		},
		{
			name: "project item content title (pull request)",
			got: func() string {
				return convertPullRequestToMinimalProjectItemContent(&github.PullRequest{
					Title: github.Ptr(maliciousText),
				}).Title
			},
		},
		{
			name: "project item content title (draft issue)",
			got: func() string {
				return convertDraftIssueToMinimalProjectItemContent(&github.ProjectV2DraftIssue{
					Title: github.Ptr(maliciousText),
				}).Title
			},
		},
		{
			name: "project pull request ref title (from *github.PullRequest)",
			got: func() string {
				return minimalProjectPullRequestRefFromPullRequest(&github.PullRequest{
					Title: github.Ptr(maliciousText),
				}).Title
			},
		},
		{
			name: "project pull request ref title (from map)",
			got: func() string {
				return minimalProjectPullRequestRefFromMap(map[string]any{
					"title": maliciousText,
				}).Title
			},
		},
		{
			name:    "project status update body (projects_get / projects_list)",
			content: true,
			got: func() string {
				return convertToMinimalStatusUpdate(statusUpdateNode{
					Body:      githubv4.NewString(githubv4.String(maliciousText)),
					CreatedAt: githubv4.DateTime{Time: time.Unix(0, 0).UTC()},
				}).Body
			},
		},
		{
			name: "issue ref title (shared constructor)",
			got: func() string {
				return newMinimalIssueRef(1, maliciousText, "OPEN", "https://github.com/o/r/issues/1", "o/r").Title
			},
		},
		{
			name: "pull request ref title (shared constructor)",
			got: func() string {
				return newMinimalPullRequestRef(1, maliciousText, "OPEN", "https://github.com/o/r/pull/1", "o/r").Title
			},
		},
		{
			name: "issue dependency ref title (issue_dependency_read / issue_dependency_write)",
			got: func() string {
				return issueToDependencyRef(&github.Issue{
					Title: github.Ptr(maliciousText),
				}).Title
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := sanitizedText
			if tt.content {
				expected = sanitizedContentText
			}
			assert.Equal(t, expected, tt.got())
		})
	}
}

// Test_SearchIssueResult_SanitizesTitleAndBody covers search_issues, which marshals the raw
// *github.Issue REST search hit directly (via SearchIssueResult.MarshalJSON) instead of routing
// through a convertToMinimal* helper. Sanitization must happen at serialization time here.
func Test_SearchIssueResult_SanitizesTitleAndBody(t *testing.T) {
	result := SearchIssueResult{
		Issue: &github.Issue{
			Title: github.Ptr(maliciousText),
			Body:  github.Ptr(maliciousText),
		},
	}

	out, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	require.NoError(t, json.Unmarshal(out, &decoded))

	assert.Equal(t, sanitizedText, decoded.Title)
	assert.Equal(t, sanitizedContentText, decoded.Body)
}

// Test_SanitizeIssueTitleAndBody exercises the shared helper directly, including its nil-safety,
// since it backs both search_issues and search_pull_requests.
func Test_SanitizeIssueTitleAndBody(t *testing.T) {
	t.Run("nil issue is a no-op", func(t *testing.T) {
		assert.NotPanics(t, func() { sanitizeIssueTitleAndBody(nil) })
	})

	t.Run("nil title/body fields are left nil", func(t *testing.T) {
		issue := &github.Issue{}
		sanitizeIssueTitleAndBody(issue)
		assert.Nil(t, issue.Title)
		assert.Nil(t, issue.Body)
	})

	t.Run("sanitizes in place", func(t *testing.T) {
		issue := &github.Issue{
			Title: github.Ptr(maliciousText),
			Body:  github.Ptr(maliciousText),
		}
		sanitizeIssueTitleAndBody(issue)
		require.NotNil(t, issue.Title)
		require.NotNil(t, issue.Body)
		assert.Equal(t, sanitizedText, *issue.Title)
		assert.Equal(t, sanitizedContentText, *issue.Body)
	})
}

// Test_MinimalConverters_PreserveCodeFidelity ensures the sanitization work does not spill over
// into fidelity-sensitive fields (diffs/patches and raw file content), which must survive byte
// for byte so patches can still be applied and code isn't corrupted.
func Test_MinimalConverters_PreserveCodeFidelity(t *testing.T) {
	patch := "@@ -1,3 +1,3 @@\n-<script>old</script>\n+<script>new</script>\u200B\n"

	t.Run("commit file patch (get_commit)", func(t *testing.T) {
		commit := convertToMinimalCommit(&github.RepositoryCommit{
			Files: []*github.CommitFile{{Filename: github.Ptr("a.go"), Patch: github.Ptr(patch)}},
		}, commitDetailFullPatch)
		require.Len(t, commit.Files, 1)
		assert.Equal(t, patch, commit.Files[0].Patch)
	})

	t.Run("pull request file patch (get_pull_request_files)", func(t *testing.T) {
		files := convertToMinimalPRFiles([]*github.CommitFile{
			{Filename: github.Ptr("a.go"), Patch: github.Ptr(patch)},
		})
		require.Len(t, files, 1)
		assert.Equal(t, patch, files[0].Patch)
	})
}

// Test_Discussion_SanitizesUserAuthoredText covers the discussion helpers, which previously
// applied no sanitization at all to titles, bodies, or comments despite being user-authored,
// untrusted content equivalent to issue/PR text.
func Test_Discussion_SanitizesUserAuthoredText(t *testing.T) {
	t.Run("discussion title (fragmentToDiscussion, used by list_discussions)", func(t *testing.T) {
		discussion := fragmentToDiscussion(NodeFragment{Title: githubv4.String(maliciousText)})
		require.NotNil(t, discussion.Title)
		assert.Equal(t, sanitizedText, *discussion.Title)
	})

	t.Run("discussion comment body (newMinimalDiscussionComment, used by get_discussion_comments)", func(t *testing.T) {
		comment := newMinimalDiscussionComment("id", maliciousText, false)
		assert.Equal(t, sanitizedContentText, comment.Body)
	})
}
