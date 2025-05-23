package github

import (
	"github.com/github/github-mcp-server/pkg/filtering"
	"github.com/google/go-github/v69/github"
)

// ContentFilteringConfig holds configuration for content filtering
type ContentFilteringConfig struct {
	// DisableContentFiltering disables all content filtering when true
	DisableContentFiltering bool
}

// DefaultContentFilteringConfig returns the default content filtering configuration
func DefaultContentFilteringConfig() *ContentFilteringConfig {
	return &ContentFilteringConfig{
		DisableContentFiltering: false,
	}
}

// FilterIssue applies content filtering to issue bodies and titles
func FilterIssue(issue *github.Issue, cfg *ContentFilteringConfig) *github.Issue {
	if issue == nil {
		return nil
	}

	// Don't modify the original issue, create a copy
	filteredIssue := *issue

	// Filter the body if present
	if issue.Body != nil {
		filteredBody := filtering.FilterContent(*issue.Body, &filtering.Config{
			DisableContentFiltering: cfg.DisableContentFiltering,
		})
		filteredIssue.Body = github.Ptr(filteredBody)
	}

	// Filter the title if present
	if issue.Title != nil {
		filteredTitle := filtering.FilterContent(*issue.Title, &filtering.Config{
			DisableContentFiltering: cfg.DisableContentFiltering,
		})
		filteredIssue.Title = github.Ptr(filteredTitle)
	}

	return &filteredIssue
}

// FilterIssues applies content filtering to a list of issues
func FilterIssues(issues []*github.Issue, cfg *ContentFilteringConfig) []*github.Issue {
	if issues == nil {
		return nil
	}

	filteredIssues := make([]*github.Issue, len(issues))
	for i, issue := range issues {
		filteredIssues[i] = FilterIssue(issue, cfg)
	}

	return filteredIssues
}

// FilterPullRequest applies content filtering to pull request bodies and titles
func FilterPullRequest(pr *github.PullRequest, cfg *ContentFilteringConfig) *github.PullRequest {
	if pr == nil {
		return nil
	}

	// Don't modify the original PR, create a copy
	filteredPR := *pr

	// Filter the body if present
	if pr.Body != nil {
		filteredBody := filtering.FilterContent(*pr.Body, &filtering.Config{
			DisableContentFiltering: cfg.DisableContentFiltering,
		})
		filteredPR.Body = github.Ptr(filteredBody)
	}

	// Filter the title if present
	if pr.Title != nil {
		filteredTitle := filtering.FilterContent(*pr.Title, &filtering.Config{
			DisableContentFiltering: cfg.DisableContentFiltering,
		})
		filteredPR.Title = github.Ptr(filteredTitle)
	}

	return &filteredPR
}

// FilterPullRequests applies content filtering to a list of pull requests
func FilterPullRequests(prs []*github.PullRequest, cfg *ContentFilteringConfig) []*github.PullRequest {
	if prs == nil {
		return nil
	}

	filteredPRs := make([]*github.PullRequest, len(prs))
	for i, pr := range prs {
		filteredPRs[i] = FilterPullRequest(pr, cfg)
	}

	return filteredPRs
}

// FilterIssueComment applies content filtering to issue comment bodies
func FilterIssueComment(comment *github.IssueComment, cfg *ContentFilteringConfig) *github.IssueComment {
	if comment == nil {
		return nil
	}

	// Don't modify the original comment, create a copy
	filteredComment := *comment

	// Filter the body if present
	if comment.Body != nil {
		filteredBody := filtering.FilterContent(*comment.Body, &filtering.Config{
			DisableContentFiltering: cfg.DisableContentFiltering,
		})
		filteredComment.Body = github.Ptr(filteredBody)
	}

	return &filteredComment
}

// FilterIssueComments applies content filtering to a list of issue comments
func FilterIssueComments(comments []*github.IssueComment, cfg *ContentFilteringConfig) []*github.IssueComment {
	if comments == nil {
		return nil
	}

	filteredComments := make([]*github.IssueComment, len(comments))
	for i, comment := range comments {
		filteredComments[i] = FilterIssueComment(comment, cfg)
	}

	return filteredComments
}

// FilterPullRequestComment applies content filtering to pull request comment bodies
func FilterPullRequestComment(comment *github.PullRequestComment, cfg *ContentFilteringConfig) *github.PullRequestComment {
	if comment == nil {
		return nil
	}

	// Don't modify the original comment, create a copy
	filteredComment := *comment

	// Filter the body if present
	if comment.Body != nil {
		filteredBody := filtering.FilterContent(*comment.Body, &filtering.Config{
			DisableContentFiltering: cfg.DisableContentFiltering,
		})
		filteredComment.Body = github.Ptr(filteredBody)
	}

	return &filteredComment
}

// FilterPullRequestComments applies content filtering to a list of pull request comments
func FilterPullRequestComments(comments []*github.PullRequestComment, cfg *ContentFilteringConfig) []*github.PullRequestComment {
	if comments == nil {
		return nil
	}

	filteredComments := make([]*github.PullRequestComment, len(comments))
	for i, comment := range comments {
		filteredComments[i] = FilterPullRequestComment(comment, cfg)
	}

	return filteredComments
}

// FilterPullRequestReview applies content filtering to pull request review bodies
func FilterPullRequestReview(review *github.PullRequestReview, cfg *ContentFilteringConfig) *github.PullRequestReview {
	if review == nil {
		return nil
	}

	// Don't modify the original review, create a copy
	filteredReview := *review

	// Filter the body if present
	if review.Body != nil {
		filteredBody := filtering.FilterContent(*review.Body, &filtering.Config{
			DisableContentFiltering: cfg.DisableContentFiltering,
		})
		filteredReview.Body = github.Ptr(filteredBody)
	}

	return &filteredReview
}

// FilterPullRequestReviews applies content filtering to a list of pull request reviews
func FilterPullRequestReviews(reviews []*github.PullRequestReview, cfg *ContentFilteringConfig) []*github.PullRequestReview {
	if reviews == nil {
		return nil
	}

	filteredReviews := make([]*github.PullRequestReview, len(reviews))
	for i, review := range reviews {
		filteredReviews[i] = FilterPullRequestReview(review, cfg)
	}

	return filteredReviews
}