package github

import (
	"time"

	"github.com/google/go-github/v89/github"

	"github.com/github/github-mcp-server/pkg/sanitize"
)

// ProjectedPullRequest is the projection-aware output item for list_pull_requests.
// Pointer-backed scalar fields distinguish an omitted projection from an explicit
// zero value in the source response.
type ProjectedPullRequest struct {
	Number             *int             `json:"number,omitempty"`
	Title              *string          `json:"title,omitempty"`
	Body               *string          `json:"body,omitempty"`
	State              *string          `json:"state,omitempty"`
	Draft              *bool            `json:"draft,omitempty"`
	Merged             *bool            `json:"merged,omitempty"`
	MergeableState     *string          `json:"mergeable_state,omitempty"`
	HTMLURL            *string          `json:"html_url,omitempty"`
	User               *ProjectedUser   `json:"user,omitempty"`
	Labels             []string         `json:"labels,omitempty"`
	Assignees          []string         `json:"assignees,omitempty"`
	RequestedReviewers []string         `json:"requested_reviewers,omitempty"`
	MergedBy           *string          `json:"merged_by,omitempty"`
	Head               *MinimalPRBranch `json:"head,omitempty"`
	Base               *MinimalPRBranch `json:"base,omitempty"`
	Additions          *int             `json:"additions,omitempty"`
	Deletions          *int             `json:"deletions,omitempty"`
	ChangedFiles       *int             `json:"changed_files,omitempty"`
	Commits            *int             `json:"commits,omitempty"`
	Comments           *int             `json:"comments,omitempty"`
	CreatedAt          *string          `json:"created_at,omitempty"`
	UpdatedAt          *string          `json:"updated_at,omitempty"`
	ClosedAt           *string          `json:"closed_at,omitempty"`
	MergedAt           *string          `json:"merged_at,omitempty"`
	Milestone          *string          `json:"milestone,omitempty"`
}

// ProjectedListIssue is the projection-aware output item for list_issues.
type ProjectedListIssue struct {
	Number      *int                `json:"number,omitempty"`
	Title       *string             `json:"title,omitempty"`
	Body        *string             `json:"body,omitempty"`
	State       *string             `json:"state,omitempty"`
	User        *ProjectedUser      `json:"user,omitempty"`
	Labels      []string            `json:"labels,omitempty"`
	Assignees   *[]string           `json:"assignees,omitempty"`
	Comments    *int                `json:"comments,omitempty"`
	CreatedAt   *string             `json:"created_at,omitempty"`
	UpdatedAt   *string             `json:"updated_at,omitempty"`
	FieldValues []MinimalFieldValue `json:"field_values,omitempty"`
}

// ProjectedIssuesResponse is the typed paginated output for list_issues.
type ProjectedIssuesResponse struct {
	Issues     []ProjectedListIssue `json:"issues"`
	TotalCount int                  `json:"totalCount"`
	PageInfo   MinimalPageInfo      `json:"pageInfo"`
}

// ProjectedIssueSearchItem contains the compact fields shared by issue and
// pull-request search results.
type ProjectedIssueSearchItem struct {
	Number            *int                       `json:"number,omitempty"`
	Title             *string                    `json:"title,omitempty"`
	Body              *string                    `json:"body,omitempty"`
	State             *string                    `json:"state,omitempty"`
	StateReason       *string                    `json:"state_reason,omitempty"`
	Draft             *bool                      `json:"draft,omitempty"`
	Locked            *bool                      `json:"locked,omitempty"`
	HTMLURL           *string                    `json:"html_url,omitempty"`
	User              *ProjectedUser             `json:"user,omitempty"`
	AuthorAssociation *string                    `json:"author_association,omitempty"`
	Labels            []string                   `json:"labels,omitempty"`
	Assignee          *string                    `json:"assignee,omitempty"`
	Assignees         []string                   `json:"assignees,omitempty"`
	Milestone         *string                    `json:"milestone,omitempty"`
	Comments          *int                       `json:"comments,omitempty"`
	Reactions         *MinimalReactions          `json:"reactions,omitempty"`
	CreatedAt         *string                    `json:"created_at,omitempty"`
	UpdatedAt         *string                    `json:"updated_at,omitempty"`
	ClosedAt          *string                    `json:"closed_at,omitempty"`
	ClosedBy          *string                    `json:"closed_by,omitempty"`
	PullRequest       *ProjectedPullRequestLinks `json:"pull_request,omitempty"`
	RepositoryURL     *string                    `json:"repository_url,omitempty"`
}

// ProjectedUser is the compact user reference used by projected result DTOs.
type ProjectedUser struct {
	Login      string `json:"login"`
	ID         int64  `json:"id,omitempty"`
	ProfileURL string `json:"profile_url,omitempty"`
	AvatarURL  string `json:"avatar_url,omitempty"`
}

// ProjectedSearchIssue adds issue-only fields to the shared search item.
type ProjectedSearchIssue struct {
	ProjectedIssueSearchItem
	Type        *string              `json:"type,omitempty"`
	FieldValues *[]MinimalFieldValue `json:"field_values,omitempty"`
}

// ProjectedSearchIssuesResponse is the typed output for search_issues.
type ProjectedSearchIssuesResponse struct {
	TotalCount        *int                   `json:"total_count,omitempty"`
	IncompleteResults *bool                  `json:"incomplete_results,omitempty"`
	Items             []ProjectedSearchIssue `json:"items"`
}

// ProjectedSearchPullRequestsResponse is the typed output for search_pull_requests.
type ProjectedSearchPullRequestsResponse struct {
	TotalCount        *int                       `json:"total_count,omitempty"`
	IncompleteResults *bool                      `json:"incomplete_results,omitempty"`
	Items             []ProjectedIssueSearchItem `json:"items"`
}

// ProjectedPullRequestLinks is the compact pull_request marker attached to
// issue-search hits that represent pull requests.
type ProjectedPullRequestLinks struct {
	URL      string `json:"url,omitempty"`
	HTMLURL  string `json:"html_url,omitempty"`
	DiffURL  string `json:"diff_url,omitempty"`
	PatchURL string `json:"patch_url,omitempty"`
	MergedAt string `json:"merged_at,omitempty"`
}

func convertToProjectedPullRequest(pr MinimalPullRequest) ProjectedPullRequest {
	result := ProjectedPullRequest{
		Number:  &pr.Number,
		Title:   &pr.Title,
		State:   &pr.State,
		Draft:   &pr.Draft,
		Merged:  &pr.Merged,
		HTMLURL: &pr.HTMLURL,
		User:    convertMinimalUserToProjectedUser(pr.User),
		Head:    pr.Head,
		Base:    pr.Base,
	}
	if pr.Body != "" {
		result.Body = &pr.Body
	}
	if pr.MergeableState != "" {
		result.MergeableState = &pr.MergeableState
	}
	if len(pr.Labels) > 0 {
		result.Labels = pr.Labels
	}
	if len(pr.Assignees) > 0 {
		result.Assignees = pr.Assignees
	}
	if len(pr.RequestedReviewers) > 0 {
		result.RequestedReviewers = pr.RequestedReviewers
	}
	if pr.MergedBy != "" {
		result.MergedBy = &pr.MergedBy
	}
	if pr.Additions != 0 {
		result.Additions = &pr.Additions
	}
	if pr.Deletions != 0 {
		result.Deletions = &pr.Deletions
	}
	if pr.ChangedFiles != 0 {
		result.ChangedFiles = &pr.ChangedFiles
	}
	if pr.Commits != 0 {
		result.Commits = &pr.Commits
	}
	if pr.Comments != 0 {
		result.Comments = &pr.Comments
	}
	if pr.CreatedAt != "" {
		result.CreatedAt = &pr.CreatedAt
	}
	if pr.UpdatedAt != "" {
		result.UpdatedAt = &pr.UpdatedAt
	}
	if pr.ClosedAt != "" {
		result.ClosedAt = &pr.ClosedAt
	}
	if pr.MergedAt != "" {
		result.MergedAt = &pr.MergedAt
	}
	if pr.Milestone != "" {
		result.Milestone = &pr.Milestone
	}
	return result
}

func convertToProjectedListIssue(issue MinimalIssue) ProjectedListIssue {
	result := ProjectedListIssue{
		Number:    &issue.Number,
		Title:     &issue.Title,
		State:     &issue.State,
		User:      convertMinimalUserToProjectedUser(issue.User),
		Assignees: &issue.Assignees,
	}
	if issue.Body != "" {
		result.Body = &issue.Body
	}
	if len(issue.Labels) > 0 {
		result.Labels = issue.Labels
	}
	if issue.Comments != 0 {
		result.Comments = &issue.Comments
	}
	if issue.CreatedAt != "" {
		result.CreatedAt = &issue.CreatedAt
	}
	if issue.UpdatedAt != "" {
		result.UpdatedAt = &issue.UpdatedAt
	}
	if len(issue.FieldValues) > 0 {
		result.FieldValues = issue.FieldValues
	}
	return result
}

func convertToProjectedIssueSearchItem(issue *github.Issue) ProjectedIssueSearchItem {
	if issue == nil {
		return ProjectedIssueSearchItem{}
	}

	result := ProjectedIssueSearchItem{
		Number:        issue.Number,
		Title:         sanitizedStringPointer(issue.Title),
		Body:          sanitizedStringPointer(issue.Body),
		State:         issue.State,
		StateReason:   issue.StateReason,
		Draft:         issue.Draft,
		Locked:        issue.Locked,
		HTMLURL:       issue.HTMLURL,
		User:          convertToProjectedUser(issue.User),
		Comments:      issue.Comments,
		Reactions:     convertToMinimalReactions(issue.Reactions),
		CreatedAt:     timestampStringPointer(issue.CreatedAt),
		UpdatedAt:     timestampStringPointer(issue.UpdatedAt),
		ClosedAt:      timestampStringPointer(issue.ClosedAt),
		RepositoryURL: issue.RepositoryURL,
	}

	for _, label := range issue.Labels {
		if label != nil {
			result.Labels = append(result.Labels, label.GetName())
		}
	}
	if assignee := issue.Assignee; assignee != nil {
		result.Assignee = github.Ptr(assignee.GetLogin())
	}
	if authorAssociation := issue.GetAuthorAssociation(); authorAssociation != "" {
		result.AuthorAssociation = github.Ptr(authorAssociation)
	}
	for _, assignee := range issue.Assignees {
		if assignee != nil {
			result.Assignees = append(result.Assignees, assignee.GetLogin())
		}
	}
	if milestone := issue.Milestone; milestone != nil {
		result.Milestone = github.Ptr(milestone.GetTitle())
	}
	if closedBy := issue.ClosedBy; closedBy != nil {
		result.ClosedBy = github.Ptr(closedBy.GetLogin())
	}
	result.PullRequest = convertToProjectedPullRequestLinks(issue.PullRequestLinks)
	return result
}

func convertToProjectedUser(user *github.User) *ProjectedUser {
	if user == nil {
		return nil
	}
	return &ProjectedUser{
		Login:      user.GetLogin(),
		ID:         user.GetID(),
		ProfileURL: user.GetHTMLURL(),
		AvatarURL:  user.GetAvatarURL(),
	}
}

func convertMinimalUserToProjectedUser(user *MinimalUser) *ProjectedUser {
	if user == nil {
		return nil
	}
	return &ProjectedUser{
		Login:      user.Login,
		ID:         user.ID,
		ProfileURL: user.ProfileURL,
		AvatarURL:  user.AvatarURL,
	}
}

func convertToProjectedSearchIssue(issue *github.Issue, fieldValues []MinimalFieldValue) ProjectedSearchIssue {
	result := ProjectedSearchIssue{
		ProjectedIssueSearchItem: convertToProjectedIssueSearchItem(issue),
	}
	if issue != nil && issue.Type != nil {
		result.Type = github.Ptr(issue.Type.GetName())
	}
	if fieldValues != nil {
		result.FieldValues = &fieldValues
	}
	return result
}

func convertToProjectedPullRequestLinks(links *github.PullRequestLinks) *ProjectedPullRequestLinks {
	if links == nil {
		return nil
	}
	result := &ProjectedPullRequestLinks{
		URL:      links.GetURL(),
		HTMLURL:  links.GetHTMLURL(),
		DiffURL:  links.GetDiffURL(),
		PatchURL: links.GetPatchURL(),
	}
	if links.MergedAt != nil {
		result.MergedAt = links.MergedAt.Format(time.RFC3339)
	}
	return result
}

func convertToMinimalReactions(reactions *github.Reactions) *MinimalReactions {
	if reactions == nil {
		return nil
	}
	return &MinimalReactions{
		TotalCount: reactions.GetTotalCount(),
		PlusOne:    reactions.GetPlusOne(),
		MinusOne:   reactions.GetMinusOne(),
		Laugh:      reactions.GetLaugh(),
		Confused:   reactions.GetConfused(),
		Heart:      reactions.GetHeart(),
		Hooray:     reactions.GetHooray(),
		Rocket:     reactions.GetRocket(),
		Eyes:       reactions.GetEyes(),
	}
}

func sanitizedStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	return github.Ptr(sanitize.Sanitize(*value))
}

func timestampStringPointer(value *github.Timestamp) *string {
	if value == nil {
		return nil
	}
	return github.Ptr(value.Format(time.RFC3339))
}

func convertToMinimalTextMatches(textMatches []*github.TextMatch) []MinimalTextMatch {
	if len(textMatches) == 0 {
		return nil
	}

	result := make([]MinimalTextMatch, 0, len(textMatches))
	for _, textMatch := range textMatches {
		if textMatch == nil {
			continue
		}
		minimal := MinimalTextMatch{
			ObjectURL:  textMatch.GetObjectURL(),
			ObjectType: textMatch.GetObjectType(),
			Property:   textMatch.GetProperty(),
			Fragment:   textMatch.GetFragment(),
		}
		for _, match := range textMatch.Matches {
			if match == nil {
				continue
			}
			minimal.Matches = append(minimal.Matches, MinimalMatch{
				Text:    match.GetText(),
				Indices: append([]int(nil), match.Indices...),
			})
		}
		result = append(result, minimal)
	}
	return result
}
