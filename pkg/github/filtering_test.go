package github

import (
	"testing"

	"github.com/google/go-github/v69/github"
)

func TestFilterIssue(t *testing.T) {
	tests := []struct {
		name     string
		issue    *github.Issue
		filterOn bool
		expected *github.Issue
	}{
		{
			name:     "nil issue",
			issue:    nil,
			filterOn: true,
			expected: nil,
		},
		{
			name: "no invisible characters",
			issue: &github.Issue{
				Title: github.Ptr("Test Issue"),
				Body:  github.Ptr("This is a test issue"),
			},
			filterOn: true,
			expected: &github.Issue{
				Title: github.Ptr("Test Issue"),
				Body:  github.Ptr("This is a test issue"),
			},
		},
		{
			name: "with invisible characters",
			issue: &github.Issue{
				Title: github.Ptr("Test\u200BIssue"),
				Body:  github.Ptr("This\u200Bis a test issue"),
			},
			filterOn: true,
			expected: &github.Issue{
				Title: github.Ptr("TestIssue"),
				Body:  github.Ptr("Thisis a test issue"),
			},
		},
		{
			name: "with HTML comments",
			issue: &github.Issue{
				Title: github.Ptr("Test Issue"),
				Body:  github.Ptr("This is a <!-- hidden comment --> test issue"),
			},
			filterOn: true,
			expected: &github.Issue{
				Title: github.Ptr("Test Issue"),
				Body:  github.Ptr("This is a [HTML_COMMENT] test issue"),
			},
		},
		{
			name: "with filtering disabled",
			issue: &github.Issue{
				Title: github.Ptr("Test\u200BIssue"),
				Body:  github.Ptr("This\u200Bis a <!-- hidden comment --> test issue"),
			},
			filterOn: false,
			expected: &github.Issue{
				Title: github.Ptr("Test\u200BIssue"),
				Body:  github.Ptr("This\u200Bis a <!-- hidden comment --> test issue"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &ContentFilteringConfig{
				DisableContentFiltering: !tc.filterOn,
			}
			result := FilterIssue(tc.issue, cfg)

			// For nil input, we expect nil output
			if tc.issue == nil {
				if result != nil {
					t.Fatalf("FilterIssue() = %v, want %v", result, nil)
				}
				return
			}

			// Check title
			if *result.Title != *tc.expected.Title {
				t.Errorf("FilterIssue().Title = %q, want %q", *result.Title, *tc.expected.Title)
			}

			// Check body
			if *result.Body != *tc.expected.Body {
				t.Errorf("FilterIssue().Body = %q, want %q", *result.Body, *tc.expected.Body)
			}
		})
	}
}

func TestFilterPullRequest(t *testing.T) {
	tests := []struct {
		name     string
		pr       *github.PullRequest
		filterOn bool
		expected *github.PullRequest
	}{
		{
			name:     "nil pull request",
			pr:       nil,
			filterOn: true,
			expected: nil,
		},
		{
			name: "no invisible characters",
			pr: &github.PullRequest{
				Title: github.Ptr("Test PR"),
				Body:  github.Ptr("This is a test PR"),
			},
			filterOn: true,
			expected: &github.PullRequest{
				Title: github.Ptr("Test PR"),
				Body:  github.Ptr("This is a test PR"),
			},
		},
		{
			name: "with invisible characters",
			pr: &github.PullRequest{
				Title: github.Ptr("Test\u200BPR"),
				Body:  github.Ptr("This\u200Bis a test PR"),
			},
			filterOn: true,
			expected: &github.PullRequest{
				Title: github.Ptr("TestPR"),
				Body:  github.Ptr("Thisis a test PR"),
			},
		},
		{
			name: "with HTML comments",
			pr: &github.PullRequest{
				Title: github.Ptr("Test PR"),
				Body:  github.Ptr("This is a <!-- hidden comment --> test PR"),
			},
			filterOn: true,
			expected: &github.PullRequest{
				Title: github.Ptr("Test PR"),
				Body:  github.Ptr("This is a [HTML_COMMENT] test PR"),
			},
		},
		{
			name: "with filtering disabled",
			pr: &github.PullRequest{
				Title: github.Ptr("Test\u200BPR"),
				Body:  github.Ptr("This\u200Bis a <!-- hidden comment --> test PR"),
			},
			filterOn: false,
			expected: &github.PullRequest{
				Title: github.Ptr("Test\u200BPR"),
				Body:  github.Ptr("This\u200Bis a <!-- hidden comment --> test PR"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &ContentFilteringConfig{
				DisableContentFiltering: !tc.filterOn,
			}
			result := FilterPullRequest(tc.pr, cfg)

			// For nil input, we expect nil output
			if tc.pr == nil {
				if result != nil {
					t.Fatalf("FilterPullRequest() = %v, want %v", result, nil)
				}
				return
			}

			// Check title
			if *result.Title != *tc.expected.Title {
				t.Errorf("FilterPullRequest().Title = %q, want %q", *result.Title, *tc.expected.Title)
			}

			// Check body
			if *result.Body != *tc.expected.Body {
				t.Errorf("FilterPullRequest().Body = %q, want %q", *result.Body, *tc.expected.Body)
			}
		})
	}
}

func TestFilterIssueComment(t *testing.T) {
	tests := []struct {
		name     string
		comment  *github.IssueComment
		filterOn bool
		expected *github.IssueComment
	}{
		{
			name:     "nil comment",
			comment:  nil,
			filterOn: true,
			expected: nil,
		},
		{
			name: "no invisible characters",
			comment: &github.IssueComment{
				Body: github.Ptr("This is a test comment"),
			},
			filterOn: true,
			expected: &github.IssueComment{
				Body: github.Ptr("This is a test comment"),
			},
		},
		{
			name: "with invisible characters",
			comment: &github.IssueComment{
				Body: github.Ptr("This\u200Bis a test comment"),
			},
			filterOn: true,
			expected: &github.IssueComment{
				Body: github.Ptr("Thisis a test comment"),
			},
		},
		{
			name: "with HTML comments",
			comment: &github.IssueComment{
				Body: github.Ptr("This is a <!-- hidden comment --> test comment"),
			},
			filterOn: true,
			expected: &github.IssueComment{
				Body: github.Ptr("This is a [HTML_COMMENT] test comment"),
			},
		},
		{
			name: "with filtering disabled",
			comment: &github.IssueComment{
				Body: github.Ptr("This\u200Bis a <!-- hidden comment --> test comment"),
			},
			filterOn: false,
			expected: &github.IssueComment{
				Body: github.Ptr("This\u200Bis a <!-- hidden comment --> test comment"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &ContentFilteringConfig{
				DisableContentFiltering: !tc.filterOn,
			}
			result := FilterIssueComment(tc.comment, cfg)

			// For nil input, we expect nil output
			if tc.comment == nil {
				if result != nil {
					t.Fatalf("FilterIssueComment() = %v, want %v", result, nil)
				}
				return
			}

			// Check body
			if *result.Body != *tc.expected.Body {
				t.Errorf("FilterIssueComment().Body = %q, want %q", *result.Body, *tc.expected.Body)
			}
		})
	}
}

func TestFilterPullRequestComment(t *testing.T) {
	tests := []struct {
		name     string
		comment  *github.PullRequestComment
		filterOn bool
		expected *github.PullRequestComment
	}{
		{
			name:     "nil comment",
			comment:  nil,
			filterOn: true,
			expected: nil,
		},
		{
			name: "no invisible characters",
			comment: &github.PullRequestComment{
				Body: github.Ptr("This is a test comment"),
			},
			filterOn: true,
			expected: &github.PullRequestComment{
				Body: github.Ptr("This is a test comment"),
			},
		},
		{
			name: "with invisible characters",
			comment: &github.PullRequestComment{
				Body: github.Ptr("This\u200Bis a test comment"),
			},
			filterOn: true,
			expected: &github.PullRequestComment{
				Body: github.Ptr("Thisis a test comment"),
			},
		},
		{
			name: "with HTML comments",
			comment: &github.PullRequestComment{
				Body: github.Ptr("This is a <!-- hidden comment --> test comment"),
			},
			filterOn: true,
			expected: &github.PullRequestComment{
				Body: github.Ptr("This is a [HTML_COMMENT] test comment"),
			},
		},
		{
			name: "with filtering disabled",
			comment: &github.PullRequestComment{
				Body: github.Ptr("This\u200Bis a <!-- hidden comment --> test comment"),
			},
			filterOn: false,
			expected: &github.PullRequestComment{
				Body: github.Ptr("This\u200Bis a <!-- hidden comment --> test comment"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &ContentFilteringConfig{
				DisableContentFiltering: !tc.filterOn,
			}
			result := FilterPullRequestComment(tc.comment, cfg)

			// For nil input, we expect nil output
			if tc.comment == nil {
				if result != nil {
					t.Fatalf("FilterPullRequestComment() = %v, want %v", result, nil)
				}
				return
			}

			// Check body
			if *result.Body != *tc.expected.Body {
				t.Errorf("FilterPullRequestComment().Body = %q, want %q", *result.Body, *tc.expected.Body)
			}
		})
	}
}