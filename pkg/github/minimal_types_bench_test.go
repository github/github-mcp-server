package github

import (
	"strings"
	"testing"

	"github.com/google/go-github/v89/github"
)

// Benchmarks for the minimal converters that sanitize user-authored prose. These
// model the response shapes called out in
// https://github.com/github/github-mcp-server/issues/3117: a 30-issue listing
// page and a 100-comment page.

func benchProse(n int) string {
	const para = "The converter allocates a new slice for every field it touches, which shows up " +
		"as GC pressure once the response contains a few hundred comments. Rework the hot path so " +
		"clean text is returned as-is. See the linked issue for measurements and the plan.\n\n" +
		"- item one\n- item two\n- item three\n\n"

	var b strings.Builder
	b.Grow(n + len(para))
	for b.Len() < n {
		b.WriteString(para)
	}
	return b.String()[:n]
}

func benchIssuePage(count, bodySize int) []*github.Issue {
	page := make([]*github.Issue, count)
	for i := range page {
		page[i] = &github.Issue{
			Number: github.Ptr(i + 1),
			Title:  github.Ptr("Converter allocates on every sanitized field for large listing responses"),
			Body:   github.Ptr(benchProse(bodySize + i)),
			State:  github.Ptr("open"),
			User:   &github.User{Login: github.Ptr("octocat")},
		}
	}
	return page
}

func benchCommentPage(count, bodySize int) []*github.IssueComment {
	page := make([]*github.IssueComment, count)
	for i := range page {
		page[i] = &github.IssueComment{
			ID:   github.Ptr(int64(i + 1)),
			Body: github.Ptr(benchProse(bodySize + i)),
			User: &github.User{Login: github.Ptr("octocat")},
		}
	}
	return page
}

// BenchmarkConvertToMinimalIssuePage measures a 30-issue page with 2 KiB bodies.
func BenchmarkConvertToMinimalIssuePage(b *testing.B) {
	page := benchIssuePage(30, 2048)
	b.ReportAllocs()
	for b.Loop() {
		for _, issue := range page {
			sinkIssue = convertToMinimalIssue(issue)
		}
	}
}

// BenchmarkConvertToMinimalCommentPage measures a 100-comment page with 1 KiB bodies.
func BenchmarkConvertToMinimalCommentPage(b *testing.B) {
	page := benchCommentPage(100, 1024)
	b.ReportAllocs()
	for b.Loop() {
		for _, comment := range page {
			sinkComment = convertToMinimalIssueComment(comment)
		}
	}
}

var (
	sinkIssue   MinimalIssue
	sinkComment MinimalIssueComment
)
