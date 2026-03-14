package github

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"
)

// CommentFilters holds optional client-side filters for issue and PR comments.
type CommentFilters struct {
	// Author filters to comments whose author login matches exactly (case-insensitive).
	// Empty means no filter.
	Author string

	// BodyContains filters to comments whose body contains this string or matches
	// this regex. Empty means no filter.
	BodyContains string

	// CreatedAfter filters to comments created after this time (exclusive).
	// Zero value means no filter.
	CreatedAfter time.Time

	// CreatedBefore filters to comments created before this time (exclusive).
	// Zero value means no filter.
	CreatedBefore time.Time
}

// ReviewCommentFilters holds optional client-side filters for PR inline review comments.
type ReviewCommentFilters struct {
	// Author filters to threads that contain at least one comment by this author login (case-insensitive).
	// Empty means no filter.
	Author string

	// FilePath is a glob pattern applied to the file path of each review thread.
	// Only threads whose file path matches are kept. Empty means no filter.
	// Example: "src/**/*.ts"
	FilePath string

	// BodyContains filters to threads that contain at least one comment whose body
	// contains this string or matches this regex. Empty means no filter.
	BodyContains string

	// CreatedAfter filters to threads that contain at least one comment created after this time (exclusive).
	// Zero value means no filter.
	CreatedAfter time.Time

	// CreatedBefore filters to threads that contain at least one comment created before this time (exclusive).
	// Zero value means no filter.
	CreatedBefore time.Time
}

// ReviewFilters holds optional client-side filters for PR reviews.
type ReviewFilters struct {
	// Reviewer filters to reviews submitted by this author login (case-insensitive).
	// Empty means no filter.
	Reviewer string

	// State filters to reviews in this state.
	// Valid values: APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED, PENDING.
	// Empty means no filter.
	State string

	// SubmittedAfter filters to reviews submitted after this time (exclusive).
	// Zero value means no filter.
	SubmittedAfter time.Time

	// SubmittedBefore filters to reviews submitted before this time (exclusive).
	// Zero value means no filter.
	SubmittedBefore time.Time
}

// optionalCommentFilters extracts CommentFilters from tool args.
func optionalCommentFilters(args map[string]any) (CommentFilters, error) {
	author, err := OptionalParam[string](args, "author")
	if err != nil {
		return CommentFilters{}, err
	}
	bodyContains, err := OptionalParam[string](args, "bodyContains")
	if err != nil {
		return CommentFilters{}, err
	}
	createdAfter, err := optionalTimeParam(args, "createdAfter")
	if err != nil {
		return CommentFilters{}, err
	}
	createdBefore, err := optionalTimeParam(args, "createdBefore")
	if err != nil {
		return CommentFilters{}, err
	}
	return CommentFilters{
		Author:        author,
		BodyContains:  bodyContains,
		CreatedAfter:  createdAfter,
		CreatedBefore: createdBefore,
	}, nil
}

// optionalReviewCommentFilters extracts ReviewCommentFilters from tool args.
func optionalReviewCommentFilters(args map[string]any) (ReviewCommentFilters, error) {
	author, err := OptionalParam[string](args, "author")
	if err != nil {
		return ReviewCommentFilters{}, err
	}
	filePath, err := OptionalParam[string](args, "filePath")
	if err != nil {
		return ReviewCommentFilters{}, err
	}
	bodyContains, err := OptionalParam[string](args, "bodyContains")
	if err != nil {
		return ReviewCommentFilters{}, err
	}
	createdAfter, err := optionalTimeParam(args, "createdAfter")
	if err != nil {
		return ReviewCommentFilters{}, err
	}
	createdBefore, err := optionalTimeParam(args, "createdBefore")
	if err != nil {
		return ReviewCommentFilters{}, err
	}
	return ReviewCommentFilters{
		Author:        author,
		FilePath:      filePath,
		BodyContains:  bodyContains,
		CreatedAfter:  createdAfter,
		CreatedBefore: createdBefore,
	}, nil
}

// optionalReviewFilters extracts ReviewFilters from tool args.
func optionalReviewFilters(args map[string]any) (ReviewFilters, error) {
	reviewer, err := OptionalParam[string](args, "reviewer")
	if err != nil {
		return ReviewFilters{}, err
	}
	state, err := OptionalParam[string](args, "state")
	if err != nil {
		return ReviewFilters{}, err
	}
	submittedAfter, err := optionalTimeParam(args, "submittedAfter")
	if err != nil {
		return ReviewFilters{}, err
	}
	submittedBefore, err := optionalTimeParam(args, "submittedBefore")
	if err != nil {
		return ReviewFilters{}, err
	}
	return ReviewFilters{
		Reviewer:        reviewer,
		State:           state,
		SubmittedAfter:  submittedAfter,
		SubmittedBefore: submittedBefore,
	}, nil
}

// optionalTimeParam reads an optional ISO 8601 datetime string from args and parses it.
// Returns zero time if the param is absent or empty.
func optionalTimeParam(args map[string]any, key string) (time.Time, error) {
	s, err := OptionalParam[string](args, key)
	if err != nil {
		return time.Time{}, err
	}
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid datetime for %q (expected RFC3339, e.g. 2006-01-02T15:04:05Z): %w", key, err)
	}
	return t, nil
}

// matchesBody returns true if body contains the filter string as a case-insensitive
// substring or regex match. The filter is always applied case-insensitively.
// If the filter is not a valid regex, a literal case-insensitive substring match is used.
func matchesBody(body, filter string) bool {
	if filter == "" {
		return true
	}
	// Prepend (?i) to make the match case-insensitive regardless of what the caller provides.
	re, err := regexp.Compile("(?i)" + filter)
	if err != nil {
		// Invalid regex — fall back to case-insensitive substring match
		return strings.Contains(strings.ToLower(body), strings.ToLower(filter))
	}
	return re.MatchString(body)
}

// matchesAuthor returns true if login matches the filter (case-insensitive exact match).
func matchesAuthor(login, filter string) bool {
	if filter == "" {
		return true
	}
	return strings.EqualFold(login, filter)
}

// matchesCreatedAt returns true if t falls within the [after, before) window.
// Zero values for after/before mean no bound on that side.
func matchesCreatedAt(t, after, before time.Time) bool {
	if !after.IsZero() && !t.After(after) {
		return false
	}
	if !before.IsZero() && !t.Before(before) {
		return false
	}
	return true
}

// matchesFilePath returns true if filePath matches the glob pattern.
// An empty pattern always matches.
func matchesFilePath(filePath, pattern string) bool {
	if pattern == "" {
		return true
	}
	matched, err := path.Match(pattern, filePath)
	if err != nil {
		// Invalid glob — treat as no match to avoid silently returning wrong results
		return false
	}
	return matched
}

// applyCommentFilters filters a slice of MinimalIssueComment using the given filters.
// Returns the original slice if all filters are empty.
func applyCommentFilters(comments []MinimalIssueComment, f CommentFilters) []MinimalIssueComment {
	if f.Author == "" && f.BodyContains == "" && f.CreatedAfter.IsZero() && f.CreatedBefore.IsZero() {
		return comments
	}
	out := make([]MinimalIssueComment, 0, len(comments))
	for _, c := range comments {
		author := ""
		if c.User != nil {
			author = c.User.Login
		}
		if !matchesAuthor(author, f.Author) {
			continue
		}
		if !matchesBody(c.Body, f.BodyContains) {
			continue
		}
		if !f.CreatedAfter.IsZero() || !f.CreatedBefore.IsZero() {
			t, err := time.Parse(time.RFC3339, c.CreatedAt)
			if err != nil || !matchesCreatedAt(t, f.CreatedAfter, f.CreatedBefore) {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// applyReviewCommentFilters filters a MinimalReviewThreadsResponse in-place,
// keeping only threads that pass all active filters.
func applyReviewCommentFilters(resp *MinimalReviewThreadsResponse, f ReviewCommentFilters) {
	if f.Author == "" && f.FilePath == "" && f.BodyContains == "" && f.CreatedAfter.IsZero() && f.CreatedBefore.IsZero() {
		return
	}
	kept := resp.ReviewThreads[:0]
	for _, thread := range resp.ReviewThreads {
		if threadMatchesReviewCommentFilters(thread, f) {
			kept = append(kept, thread)
		}
	}
	resp.ReviewThreads = kept
	resp.TotalCount = len(kept)
}

// threadMatchesReviewCommentFilters returns true if the thread passes all filters.
// A thread passes if:
//   - Its file path matches the FilePath glob (checked on the first comment)
//   - At least one comment matches the Author and BodyContains filters
func threadMatchesReviewCommentFilters(thread MinimalReviewThread, f ReviewCommentFilters) bool {
	if len(thread.Comments) == 0 {
		return false
	}

	// File path filter — all comments in a thread share the same path; check the first.
	if !matchesFilePath(thread.Comments[0].Path, f.FilePath) {
		return false
	}

	// Author / body / date filter — thread passes if at least one comment matches all.
	for _, c := range thread.Comments {
		if !matchesAuthor(c.Author, f.Author) || !matchesBody(c.Body, f.BodyContains) {
			continue
		}
		if !f.CreatedAfter.IsZero() || !f.CreatedBefore.IsZero() {
			t, err := time.Parse(time.RFC3339, c.CreatedAt)
			if err != nil || !matchesCreatedAt(t, f.CreatedAfter, f.CreatedBefore) {
				continue
			}
		}
		return true
	}
	return false
}

// applyReviewFilters filters a slice of MinimalPullRequestReview using the given filters.
func applyReviewFilters(reviews []MinimalPullRequestReview, f ReviewFilters) []MinimalPullRequestReview {
	if f.Reviewer == "" && f.State == "" && f.SubmittedAfter.IsZero() && f.SubmittedBefore.IsZero() {
		return reviews
	}
	out := make([]MinimalPullRequestReview, 0, len(reviews))
	for _, r := range reviews {
		reviewer := ""
		if r.User != nil {
			reviewer = r.User.Login
		}
		if !matchesAuthor(reviewer, f.Reviewer) {
			continue
		}
		if f.State != "" && !strings.EqualFold(r.State, f.State) {
			continue
		}
		if !f.SubmittedAfter.IsZero() || !f.SubmittedBefore.IsZero() {
			t, err := time.Parse(time.RFC3339, r.SubmittedAt)
			if err != nil || !matchesCreatedAt(t, f.SubmittedAfter, f.SubmittedBefore) {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}
