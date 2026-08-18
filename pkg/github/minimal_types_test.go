package github

import (
	"net/url"
	"testing"

	"github.com/google/go-github/v89/github"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bodyWithHiddenPayload embeds Unicode tag characters, which are invisible to a
// human reviewer but legible to a model.
const bodyWithHiddenPayload = "Looks good\U000E0001\U000E0049\U000E0067\U000E006E\U000E006F\U000E0072\U000E0065"

const bodyWithCode = "Compare with:\n```go\nif a<b { fmt.Println(\"x\") }\n```\nand <Foo/> in JSX."

func TestConvertToMinimalIssueCommentSanitizesBody(t *testing.T) {
	t.Run("strips hidden characters", func(t *testing.T) {
		m := convertToMinimalIssueComment(&github.IssueComment{
			ID:   github.Ptr(int64(1)),
			Body: github.Ptr(bodyWithHiddenPayload),
		})
		assert.Equal(t, "Looks good", m.Body)
	})

	t.Run("preserves code content", func(t *testing.T) {
		m := convertToMinimalIssueComment(&github.IssueComment{
			ID:   github.Ptr(int64(1)),
			Body: github.Ptr(bodyWithCode),
		})
		assert.Equal(t, bodyWithCode, m.Body)
	})
}

func TestConvertToMinimalPullRequestReviewSanitizesBody(t *testing.T) {
	t.Run("strips hidden characters", func(t *testing.T) {
		m := convertToMinimalPullRequestReview(&github.PullRequestReview{
			ID:   github.Ptr(int64(1)),
			Body: github.Ptr(bodyWithHiddenPayload),
		})
		assert.Equal(t, "Looks good", m.Body)
	})

	t.Run("preserves code content", func(t *testing.T) {
		m := convertToMinimalPullRequestReview(&github.PullRequestReview{
			ID:   github.Ptr(int64(1)),
			Body: github.Ptr(bodyWithCode),
		})
		assert.Equal(t, bodyWithCode, m.Body)
	})
}

func TestConvertToMinimalReviewCommentSanitizesBody(t *testing.T) {
	commentURL, err := url.Parse("https://github.com/owner/repo/pull/1#discussion_r1")
	require.NoError(t, err)

	t.Run("strips hidden characters", func(t *testing.T) {
		m := convertToMinimalReviewComment(reviewCommentNode{
			Body: githubv4.String(bodyWithHiddenPayload),
			Path: githubv4.String("main.go"),
			URL:  githubv4.URI{URL: commentURL},
		})
		assert.Equal(t, "Looks good", m.Body)
	})

	t.Run("preserves code content", func(t *testing.T) {
		m := convertToMinimalReviewComment(reviewCommentNode{
			Body: githubv4.String(bodyWithCode),
			Path: githubv4.String("main.go"),
			URL:  githubv4.URI{URL: commentURL},
		})
		assert.Equal(t, bodyWithCode, m.Body)
	})
}
