package github

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// helpers

func makeComment(login, body string) MinimalIssueComment {
	return MinimalIssueComment{
		Body: body,
		User: &MinimalUser{Login: login},
	}
}

func makeReview(login, state string) MinimalPullRequestReview {
	return MinimalPullRequestReview{
		State: state,
		User:  &MinimalUser{Login: login},
	}
}

func makeThread(author, path, body string) MinimalReviewThread {
	return MinimalReviewThread{
		Comments: []MinimalReviewComment{
			{Author: author, Path: path, Body: body},
		},
	}
}

// --- applyCommentFilters ---

func TestApplyCommentFilters_NoFilters(t *testing.T) {
	t.Parallel()
	comments := []MinimalIssueComment{
		makeComment("alice", "hello"),
		makeComment("bob", "world"),
	}
	result := applyCommentFilters(comments, CommentFilters{})
	assert.Equal(t, comments, result)
}

func TestApplyCommentFilters_ByAuthor(t *testing.T) {
	t.Parallel()
	comments := []MinimalIssueComment{
		makeComment("alice", "hello"),
		makeComment("bob", "world"),
		makeComment("Alice", "case insensitive"), // same as alice, different case
	}
	result := applyCommentFilters(comments, CommentFilters{Author: "alice"})
	assert.Len(t, result, 2)
	assert.Equal(t, "alice", result[0].User.Login)
	assert.Equal(t, "Alice", result[1].User.Login)
}

func TestApplyCommentFilters_ByBodySubstring(t *testing.T) {
	t.Parallel()
	comments := []MinimalIssueComment{
		makeComment("alice", "fix the bug"),
		makeComment("bob", "this is a feature"),
		makeComment("carol", "BUG report"), // uppercase
	}
	result := applyCommentFilters(comments, CommentFilters{BodyContains: "bug"})
	assert.Len(t, result, 2)
}

func TestApplyCommentFilters_ByBodyRegex(t *testing.T) {
	t.Parallel()
	comments := []MinimalIssueComment{
		makeComment("alice", "fixes #123"),
		makeComment("bob", "closes #456"),
		makeComment("carol", "unrelated comment"),
	}
	result := applyCommentFilters(comments, CommentFilters{BodyContains: `(fixes|closes) #\d+`})
	assert.Len(t, result, 2)
}

func TestApplyCommentFilters_AuthorAndBody(t *testing.T) {
	t.Parallel()
	comments := []MinimalIssueComment{
		makeComment("alice", "fix the bug"),
		makeComment("alice", "add feature"),
		makeComment("bob", "fix the bug"),
	}
	result := applyCommentFilters(comments, CommentFilters{Author: "alice", BodyContains: "fix"})
	assert.Len(t, result, 1)
	assert.Equal(t, "alice", result[0].User.Login)
	assert.Contains(t, result[0].Body, "fix")
}

func TestApplyCommentFilters_NilUser(t *testing.T) {
	t.Parallel()
	comments := []MinimalIssueComment{
		{Body: "hello", User: nil},
	}
	// author filter with nil user — should not match
	result := applyCommentFilters(comments, CommentFilters{Author: "alice"})
	assert.Empty(t, result)
}

// --- applyReviewFilters ---

func TestApplyReviewFilters_NoFilters(t *testing.T) {
	t.Parallel()
	reviews := []MinimalPullRequestReview{
		makeReview("alice", "APPROVED"),
		makeReview("bob", "CHANGES_REQUESTED"),
	}
	result := applyReviewFilters(reviews, ReviewFilters{})
	assert.Equal(t, reviews, result)
}

func TestApplyReviewFilters_ByReviewer(t *testing.T) {
	t.Parallel()
	reviews := []MinimalPullRequestReview{
		makeReview("alice", "APPROVED"),
		makeReview("bob", "COMMENTED"),
		makeReview("Alice", "CHANGES_REQUESTED"),
	}
	result := applyReviewFilters(reviews, ReviewFilters{Reviewer: "alice"})
	assert.Len(t, result, 2)
}

func TestApplyReviewFilters_ByState(t *testing.T) {
	t.Parallel()
	reviews := []MinimalPullRequestReview{
		makeReview("alice", "APPROVED"),
		makeReview("bob", "CHANGES_REQUESTED"),
		makeReview("carol", "approved"), // lowercase
	}
	result := applyReviewFilters(reviews, ReviewFilters{State: "APPROVED"})
	assert.Len(t, result, 2)
}

func TestApplyReviewFilters_ReviewerAndState(t *testing.T) {
	t.Parallel()
	reviews := []MinimalPullRequestReview{
		makeReview("alice", "APPROVED"),
		makeReview("alice", "COMMENTED"),
		makeReview("bob", "APPROVED"),
	}
	result := applyReviewFilters(reviews, ReviewFilters{Reviewer: "alice", State: "APPROVED"})
	assert.Len(t, result, 1)
	assert.Equal(t, "alice", result[0].User.Login)
	assert.Equal(t, "APPROVED", result[0].State)
}

// --- applyReviewCommentFilters ---

func TestApplyReviewCommentFilters_NoFilters(t *testing.T) {
	t.Parallel()
	resp := &MinimalReviewThreadsResponse{
		ReviewThreads: []MinimalReviewThread{
			makeThread("alice", "src/foo.ts", "looks good"),
			makeThread("bob", "src/bar.go", "needs work"),
		},
		TotalCount: 2,
	}
	applyReviewCommentFilters(resp, ReviewCommentFilters{})
	assert.Len(t, resp.ReviewThreads, 2)
}

func TestApplyReviewCommentFilters_ByAuthor(t *testing.T) {
	t.Parallel()
	resp := &MinimalReviewThreadsResponse{
		ReviewThreads: []MinimalReviewThread{
			makeThread("alice", "src/foo.ts", "looks good"),
			makeThread("bob", "src/bar.go", "needs work"),
		},
		TotalCount: 2,
	}
	applyReviewCommentFilters(resp, ReviewCommentFilters{Author: "alice"})
	assert.Len(t, resp.ReviewThreads, 1)
	assert.Equal(t, 1, resp.TotalCount)
	assert.Equal(t, "alice", resp.ReviewThreads[0].Comments[0].Author)
}

func TestApplyReviewCommentFilters_ByFilePath(t *testing.T) {
	t.Parallel()
	resp := &MinimalReviewThreadsResponse{
		ReviewThreads: []MinimalReviewThread{
			makeThread("alice", "src/foo.ts", "comment"),
			makeThread("bob", "src/bar.go", "comment"),
			makeThread("carol", "src/deep/baz.ts", "comment"),
		},
		TotalCount: 3,
	}
	applyReviewCommentFilters(resp, ReviewCommentFilters{FilePath: "src/*.ts"})
	assert.Len(t, resp.ReviewThreads, 1)
	assert.Equal(t, "src/foo.ts", resp.ReviewThreads[0].Comments[0].Path)
}

func TestApplyReviewCommentFilters_ByBodyContains(t *testing.T) {
	t.Parallel()
	resp := &MinimalReviewThreadsResponse{
		ReviewThreads: []MinimalReviewThread{
			makeThread("alice", "src/foo.ts", "LGTM"),
			makeThread("bob", "src/bar.go", "needs work"),
			makeThread("carol", "src/baz.go", "lgtm please merge"),
		},
		TotalCount: 3,
	}
	applyReviewCommentFilters(resp, ReviewCommentFilters{BodyContains: "lgtm"})
	assert.Len(t, resp.ReviewThreads, 2)
	assert.Equal(t, 2, resp.TotalCount)
}

func TestApplyReviewCommentFilters_AllThree(t *testing.T) {
	t.Parallel()
	resp := &MinimalReviewThreadsResponse{
		ReviewThreads: []MinimalReviewThread{
			makeThread("alice", "src/foo.ts", "LGTM"),
			makeThread("alice", "src/bar.go", "LGTM"),
			makeThread("bob", "src/foo.ts", "LGTM"),
		},
		TotalCount: 3,
	}
	applyReviewCommentFilters(resp, ReviewCommentFilters{
		Author:       "alice",
		FilePath:     "src/*.ts",
		BodyContains: "lgtm",
	})
	assert.Len(t, resp.ReviewThreads, 1)
	assert.Equal(t, "alice", resp.ReviewThreads[0].Comments[0].Author)
	assert.Equal(t, "src/foo.ts", resp.ReviewThreads[0].Comments[0].Path)
}

func TestApplyReviewCommentFilters_EmptyThread(t *testing.T) {
	t.Parallel()
	resp := &MinimalReviewThreadsResponse{
		ReviewThreads: []MinimalReviewThread{
			{Comments: []MinimalReviewComment{}},
		},
		TotalCount: 1,
	}
	applyReviewCommentFilters(resp, ReviewCommentFilters{Author: "alice"})
	assert.Empty(t, resp.ReviewThreads)
	assert.Equal(t, 0, resp.TotalCount)
}

// --- date filter helpers ---

var (
	t1 = time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	t2 = time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC)
	t3 = time.Date(2024, 1, 30, 0, 0, 0, 0, time.UTC)
)

func makeCommentWithDate(login, body, createdAt string) MinimalIssueComment {
	return MinimalIssueComment{
		Body:      body,
		User:      &MinimalUser{Login: login},
		CreatedAt: createdAt,
	}
}

func makeReviewWithDate(login, state, submittedAt string) MinimalPullRequestReview {
	return MinimalPullRequestReview{
		State:       state,
		User:        &MinimalUser{Login: login},
		SubmittedAt: submittedAt,
	}
}

func makeThreadWithDate(author, path, body, createdAt string) MinimalReviewThread {
	return MinimalReviewThread{
		Comments: []MinimalReviewComment{
			{Author: author, Path: path, Body: body, CreatedAt: createdAt},
		},
	}
}

// --- applyCommentFilters date tests ---

func TestApplyCommentFilters_CreatedAfter(t *testing.T) {
	t.Parallel()
	comments := []MinimalIssueComment{
		makeCommentWithDate("alice", "old", t1.Format(time.RFC3339)),
		makeCommentWithDate("alice", "mid", t2.Format(time.RFC3339)),
		makeCommentWithDate("alice", "new", t3.Format(time.RFC3339)),
	}
	result := applyCommentFilters(comments, CommentFilters{CreatedAfter: t1})
	assert.Len(t, result, 2)
	assert.Equal(t, "mid", result[0].Body)
	assert.Equal(t, "new", result[1].Body)
}

func TestApplyCommentFilters_CreatedBefore(t *testing.T) {
	t.Parallel()
	comments := []MinimalIssueComment{
		makeCommentWithDate("alice", "old", t1.Format(time.RFC3339)),
		makeCommentWithDate("alice", "mid", t2.Format(time.RFC3339)),
		makeCommentWithDate("alice", "new", t3.Format(time.RFC3339)),
	}
	result := applyCommentFilters(comments, CommentFilters{CreatedBefore: t3})
	assert.Len(t, result, 2)
	assert.Equal(t, "old", result[0].Body)
	assert.Equal(t, "mid", result[1].Body)
}

func TestApplyCommentFilters_CreatedAfterAndBefore(t *testing.T) {
	t.Parallel()
	comments := []MinimalIssueComment{
		makeCommentWithDate("alice", "old", t1.Format(time.RFC3339)),
		makeCommentWithDate("alice", "mid", t2.Format(time.RFC3339)),
		makeCommentWithDate("alice", "new", t3.Format(time.RFC3339)),
	}
	result := applyCommentFilters(comments, CommentFilters{CreatedAfter: t1, CreatedBefore: t3})
	assert.Len(t, result, 1)
	assert.Equal(t, "mid", result[0].Body)
}

func TestApplyCommentFilters_InvalidDateSkipped(t *testing.T) {
	t.Parallel()
	comments := []MinimalIssueComment{
		makeCommentWithDate("alice", "bad date", "not-a-date"),
		makeCommentWithDate("alice", "good", t2.Format(time.RFC3339)),
	}
	result := applyCommentFilters(comments, CommentFilters{CreatedAfter: t1})
	assert.Len(t, result, 1)
	assert.Equal(t, "good", result[0].Body)
}

// --- applyReviewFilters date tests ---

func TestApplyReviewFilters_SubmittedAfter(t *testing.T) {
	t.Parallel()
	reviews := []MinimalPullRequestReview{
		makeReviewWithDate("alice", "APPROVED", t1.Format(time.RFC3339)),
		makeReviewWithDate("alice", "COMMENTED", t2.Format(time.RFC3339)),
		makeReviewWithDate("alice", "CHANGES_REQUESTED", t3.Format(time.RFC3339)),
	}
	result := applyReviewFilters(reviews, ReviewFilters{SubmittedAfter: t2})
	assert.Len(t, result, 1)
	assert.Equal(t, "CHANGES_REQUESTED", result[0].State)
}

func TestApplyReviewFilters_SubmittedBefore(t *testing.T) {
	t.Parallel()
	reviews := []MinimalPullRequestReview{
		makeReviewWithDate("alice", "APPROVED", t1.Format(time.RFC3339)),
		makeReviewWithDate("alice", "COMMENTED", t2.Format(time.RFC3339)),
		makeReviewWithDate("alice", "CHANGES_REQUESTED", t3.Format(time.RFC3339)),
	}
	result := applyReviewFilters(reviews, ReviewFilters{SubmittedBefore: t2})
	assert.Len(t, result, 1)
	assert.Equal(t, "APPROVED", result[0].State)
}

// --- applyReviewCommentFilters date tests ---

func TestApplyReviewCommentFilters_CreatedAfter(t *testing.T) {
	t.Parallel()
	resp := &MinimalReviewThreadsResponse{
		ReviewThreads: []MinimalReviewThread{
			makeThreadWithDate("alice", "src/a.go", "old", t1.Format(time.RFC3339)),
			makeThreadWithDate("alice", "src/b.go", "mid", t2.Format(time.RFC3339)),
			makeThreadWithDate("alice", "src/c.go", "new", t3.Format(time.RFC3339)),
		},
		TotalCount: 3,
	}
	applyReviewCommentFilters(resp, ReviewCommentFilters{CreatedAfter: t1})
	assert.Len(t, resp.ReviewThreads, 2)
	assert.Equal(t, 2, resp.TotalCount)
}

func TestApplyReviewCommentFilters_CreatedBefore(t *testing.T) {
	t.Parallel()
	resp := &MinimalReviewThreadsResponse{
		ReviewThreads: []MinimalReviewThread{
			makeThreadWithDate("alice", "src/a.go", "old", t1.Format(time.RFC3339)),
			makeThreadWithDate("alice", "src/b.go", "mid", t2.Format(time.RFC3339)),
			makeThreadWithDate("alice", "src/c.go", "new", t3.Format(time.RFC3339)),
		},
		TotalCount: 3,
	}
	applyReviewCommentFilters(resp, ReviewCommentFilters{CreatedBefore: t3})
	assert.Len(t, resp.ReviewThreads, 2)
	assert.Equal(t, 2, resp.TotalCount)
}
