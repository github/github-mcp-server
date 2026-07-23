package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"

	"github.com/github/github-mcp-server/pkg/sanitize"
)

// codeSearchItemFieldEnum lists the selectable fields for search_code result
// items, matching the JSON field names of MinimalCodeResult. The repository and
// text_matches fields are the heaviest, so omitting them is the main lever for
// shrinking large result sets.
var codeSearchItemFieldEnum = []any{"name", "path", "sha", "repository", "text_matches"}

// fileContentFieldEnum lists the selectable fields for get_file_contents
// directory listings, matching the JSON field names of
// github.RepositoryContent that appear for directory entries. Only applied when
// the requested path is a directory; ignored for single files.
var fileContentFieldEnum = []any{"type", "name", "path", "size", "sha", "url", "git_url", "html_url", "download_url"}

// listIssuesItemFieldEnum lists the selectable fields for list_issues result
// items, matching the JSON field names MinimalIssue actually populates via the
// list_issues GraphQL fragment (fragmentToMinimalIssue). Fields that only the
// REST conversion sets (for example html_url, reactions, issue_field_values) are
// never emitted here and are intentionally omitted. The body and field_values
// fields are the heaviest, so omitting them is the main lever for shrinking large
// result sets.
var listIssuesItemFieldEnum = []any{
	"number", "title", "body", "state", "user", "labels",
	"comments", "created_at", "updated_at", "field_values",
}

// listPullRequestsItemFieldEnum lists the selectable fields for
// list_pull_requests result items, matching the JSON field names of
// MinimalPullRequest. The body field is the heaviest, so omitting it is the main
// lever for shrinking large result sets.
var listPullRequestsItemFieldEnum = []any{
	"number", "title", "body", "state", "draft", "merged", "mergeable_state",
	"html_url", "user", "labels", "assignees", "requested_reviewers", "merged_by",
	"head", "base", "additions", "deletions", "changed_files", "commits",
	"comments", "created_at", "updated_at", "closed_at", "merged_at", "milestone",
}

// listCommitsItemFieldEnum lists the selectable fields for list_commits result
// items, matching the JSON field names MinimalCommit populates for list_commits.
// list_commits requests commits without per-file detail (commitDetailNone), so
// the stats and files fields are never emitted and are intentionally omitted
// here. The commit field (message plus author/committer metadata) is the
// heaviest, so omitting it is the main lever for shrinking large result sets.
var listCommitsItemFieldEnum = []any{
	"sha", "html_url", "commit", "author", "committer",
}

// listReleasesItemFieldEnum lists the selectable fields for list_releases result
// items, matching the JSON field names of MinimalRelease. The body field is the
// heaviest, so omitting it is the main lever for shrinking large result sets.
var listReleasesItemFieldEnum = []any{
	"id", "tag_name", "name", "body", "html_url", "published_at",
	"prerelease", "draft", "author",
}

// searchIssuesItemFieldEnum lists the selectable fields for search_issues result
// items. Items are full github.Issue objects enriched with normalized
// field_values, so this is a curated subset of the most useful JSON field names.
// The body, reactions, and labels fields are the heaviest, so omitting them is
// the main lever for shrinking large result sets.
var searchIssuesItemFieldEnum = []any{
	"number", "title", "body", "state", "state_reason", "draft", "locked",
	"html_url", "user", "author_association", "labels", "assignee", "assignees",
	"milestone", "comments", "reactions", "created_at", "updated_at", "closed_at",
	"closed_by", "type", "repository_url", "pull_request", "field_values",
}

// searchPullRequestsItemFieldEnum lists the selectable fields for
// search_pull_requests result items. Issue search returns pull requests as
// github.Issue objects, so this is a curated subset of those JSON field names.
// The body, reactions, and labels fields are the heaviest, so omitting them is
// the main lever for shrinking large result sets.
var searchPullRequestsItemFieldEnum = []any{
	"number", "title", "body", "state", "state_reason", "draft", "locked",
	"html_url", "user", "author_association", "labels", "assignee", "assignees",
	"milestone", "comments", "reactions", "created_at", "updated_at", "closed_at",
	"closed_by", "pull_request", "repository_url",
}

// filterFields marshals v to a JSON object and returns a map containing only the
// requested fields. Fields that are unknown or absent from the JSON (for example
// empty values dropped via omitempty) are skipped.
func filterFields(v any, fields []string) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber() // preserve integer precision for fields such as IDs
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		return nil, err
	}

	picked := make(map[string]any, len(fields))
	for _, field := range fields {
		if value, ok := object[field]; ok {
			picked[field] = value
		}
	}
	return picked, nil
}

// filterEachField applies filterFields to every item, returning a slice in which
// each element contains only the requested fields.
func filterEachField[T any](items []T, fields []string) ([]map[string]any, error) {
	filtered := make([]map[string]any, 0, len(items))
	for _, item := range items {
		picked, err := filterFields(item, fields)
		if err != nil {
			return nil, err
		}
		filtered = append(filtered, picked)
	}
	return filtered, nil
}

// fieldsSchemaProperty builds the optional `fields` array parameter shared by
// every fields-enabled tool: an array of strings constrained to the given enum
// of selectable field names, with a per-tool description.
func fieldsSchemaProperty(description string, enum []any) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "array",
		Description: description,
		Items: &jsonschema.Schema{
			Type: "string",
			Enum: enum,
		},
	}
}

// MinimalUser is the output type for user and organization search results.
type MinimalUser struct {
	Login      string       `json:"login"`
	ID         int64        `json:"id,omitempty"`
	ProfileURL string       `json:"profile_url,omitempty"`
	AvatarURL  string       `json:"avatar_url,omitempty"`
	Details    *UserDetails `json:"details,omitempty"` // Optional field for additional user details
}

// MinimalSearchUsersResult is the trimmed output type for user search results.
type MinimalSearchUsersResult struct {
	TotalCount        int           `json:"total_count"`
	IncompleteResults bool          `json:"incomplete_results"`
	Items             []MinimalUser `json:"items"`
}

// MinimalRepository is the trimmed output type for repository objects to reduce verbosity.
type MinimalRepository struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	FullName      string   `json:"full_name"`
	Description   string   `json:"description,omitempty"`
	HTMLURL       string   `json:"html_url"`
	Language      string   `json:"language,omitempty"`
	Stars         int      `json:"stargazers_count"`
	Forks         int      `json:"forks_count"`
	OpenIssues    int      `json:"open_issues_count"`
	UpdatedAt     string   `json:"updated_at,omitempty"`
	CreatedAt     string   `json:"created_at,omitempty"`
	Topics        []string `json:"topics,omitempty"`
	Private       bool     `json:"private"`
	Fork          bool     `json:"fork"`
	Archived      bool     `json:"archived"`
	DefaultBranch string   `json:"default_branch,omitempty"`
}

// MinimalSearchRepositoriesResult is the trimmed output type for repository search results.
type MinimalSearchRepositoriesResult struct {
	TotalCount        int                 `json:"total_count"`
	IncompleteResults bool                `json:"incomplete_results"`
	Items             []MinimalRepository `json:"items"`
}

// MinimalDiscussionComment is the trimmed output type for discussion comment objects.
type MinimalDiscussionComment struct {
	ID              string                     `json:"id"`
	Body            string                     `json:"body"`
	IsAnswer        bool                       `json:"isAnswer,omitempty"`
	Replies         []MinimalDiscussionComment `json:"replies,omitempty"`
	ReplyTotalCount int                        `json:"replyTotalCount,omitempty"`
}

// MinimalCodeSearchResult is the trimmed output type for code search results.
type MinimalCodeSearchResult struct {
	TotalCount        int                 `json:"total_count"`
	IncompleteResults bool                `json:"incomplete_results"`
	Items             []MinimalCodeResult `json:"items"`
}

// MinimalCodeResult is the trimmed output type for a single code search hit.
type MinimalCodeResult struct {
	Name        string              `json:"name"`
	Path        string              `json:"path"`
	SHA         string              `json:"sha"`
	Repository  string              `json:"repository"`
	TextMatches []*github.TextMatch `json:"text_matches,omitempty"`
}

// MinimalCommitAuthor represents commit author information.
type MinimalCommitAuthor struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Date  string `json:"date,omitempty"`
}

// MinimalCommitInfo represents core commit information.
type MinimalCommitInfo struct {
	Message   string               `json:"message"`
	Author    *MinimalCommitAuthor `json:"author,omitempty"`
	Committer *MinimalCommitAuthor `json:"committer,omitempty"`
}

// MinimalCommitStats represents commit statistics.
type MinimalCommitStats struct {
	Additions int `json:"additions,omitempty"`
	Deletions int `json:"deletions,omitempty"`
	Total     int `json:"total,omitempty"`
}

// MinimalCommitFile represents a file changed in a commit.
type MinimalCommitFile struct {
	Filename  string `json:"filename"`
	Status    string `json:"status,omitempty"`
	Additions int    `json:"additions,omitempty"`
	Deletions int    `json:"deletions,omitempty"`
	Changes   int    `json:"changes,omitempty"`
	Patch     string `json:"patch,omitempty"`
}

// MinimalPRFile represents a file changed in a pull request.
// Compared to MinimalCommitFile, it includes the patch diff and previous filename for renames.
type MinimalPRFile struct {
	Filename         string `json:"filename"`
	Status           string `json:"status,omitempty"`
	Additions        int    `json:"additions,omitempty"`
	Deletions        int    `json:"deletions,omitempty"`
	Changes          int    `json:"changes,omitempty"`
	Patch            string `json:"patch,omitempty"`
	PreviousFilename string `json:"previous_filename,omitempty"`
}

// MinimalPullRequestCommit is the trimmed output type for commits listed on a pull request.
type MinimalPullRequestCommit struct {
	SHA     string               `json:"sha"`
	HTMLURL string               `json:"html_url,omitempty"`
	Message string               `json:"message,omitempty"`
	Author  *MinimalCommitAuthor `json:"author,omitempty"`
}

// MinimalCommit is the trimmed output type for commit objects.
type MinimalCommit struct {
	SHA       string              `json:"sha"`
	HTMLURL   string              `json:"html_url"`
	Commit    *MinimalCommitInfo  `json:"commit,omitempty"`
	Author    *MinimalUser        `json:"author,omitempty"`
	Committer *MinimalUser        `json:"committer,omitempty"`
	Stats     *MinimalCommitStats `json:"stats,omitempty"`
	Files     []MinimalCommitFile `json:"files,omitempty"`
}

// MinimalRepoRef is a lightweight reference to a repository, used when a
// result needs to identify which repository it belongs to (for example, in
// cross-repo commit search results).
type MinimalRepoRef struct {
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url,omitempty"`
	Private  bool   `json:"private,omitempty"`
}

// MinimalCommitSearchItem extends MinimalCommit with the containing
// repository, since commit search spans repositories and callers need to
// know which repo each result came from.
type MinimalCommitSearchItem struct {
	MinimalCommit
	Repository *MinimalRepoRef `json:"repository,omitempty"`
}

// MinimalRelease is the trimmed output type for release objects.
type MinimalRelease struct {
	ID          int64        `json:"id"`
	TagName     string       `json:"tag_name"`
	Name        string       `json:"name,omitempty"`
	Body        string       `json:"body,omitempty"`
	HTMLURL     string       `json:"html_url"`
	PublishedAt string       `json:"published_at,omitempty"`
	Prerelease  bool         `json:"prerelease"`
	Draft       bool         `json:"draft"`
	Author      *MinimalUser `json:"author,omitempty"`
}

// MinimalBranch is the trimmed output type for branch objects.
type MinimalBranch struct {
	Name      string `json:"name"`
	SHA       string `json:"sha"`
	Protected bool   `json:"protected"`
}

// MinimalTag is the trimmed output type for tag objects.
type MinimalTag struct {
	Name string `json:"name"`
	SHA  string `json:"sha"`
}

// MinimalResponse represents a minimal response for all CRUD operations.
// Success is implicit in the HTTP response status, and all other information
// can be derived from the URL or fetched separately if needed.
type MinimalResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// MinimalCollaborator is the trimmed output type for repository collaborators.
type MinimalCollaborator struct {
	Login    string `json:"login"`
	ID       int64  `json:"id"`
	RoleName string `json:"role_name"`
}

type MinimalProject struct {
	ID               *int64            `json:"id,omitempty"`
	NodeID           *string           `json:"node_id,omitempty"`
	Owner            *MinimalUser      `json:"owner,omitempty"`
	Creator          *MinimalUser      `json:"creator,omitempty"`
	Title            *string           `json:"title,omitempty"`
	Description      *string           `json:"description,omitempty"`
	Public           *bool             `json:"public,omitempty"`
	ClosedAt         *github.Timestamp `json:"closed_at,omitempty"`
	CreatedAt        *github.Timestamp `json:"created_at,omitempty"`
	UpdatedAt        *github.Timestamp `json:"updated_at,omitempty"`
	DeletedAt        *github.Timestamp `json:"deleted_at,omitempty"`
	Number           *int              `json:"number,omitempty"`
	ShortDescription *string           `json:"short_description,omitempty"`
	DeletedBy        *MinimalUser      `json:"deleted_by,omitempty"`
	OwnerType        string            `json:"owner_type,omitempty"`
}

type MinimalProjectItem struct {
	ID          int64                          `json:"id"`
	NodeID      string                         `json:"node_id,omitempty"`
	ContentType string                         `json:"content_type,omitempty"`
	Content     *MinimalProjectItemContent     `json:"content,omitempty"`
	Fields      []MinimalProjectItemFieldValue `json:"fields,omitempty"`
	ArchivedAt  string                         `json:"archived_at,omitempty"`
	CreatedAt   string                         `json:"created_at,omitempty"`
	UpdatedAt   string                         `json:"updated_at,omitempty"`
	Creator     string                         `json:"creator,omitempty"`
}

type MinimalProjectItemContent struct {
	ID          int64    `json:"id,omitempty"`
	NodeID      string   `json:"node_id,omitempty"`
	Number      int      `json:"number,omitempty"`
	Title       string   `json:"title,omitempty"`
	State       string   `json:"state,omitempty"`
	StateReason string   `json:"state_reason,omitempty"`
	HTMLURL     string   `json:"html_url,omitempty"`
	Repository  string   `json:"repository,omitempty"`
	Author      string   `json:"author,omitempty"`
	Assignees   []string `json:"assignees,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Milestone   string   `json:"milestone,omitempty"`
	Comments    int      `json:"comments,omitempty"`
	Draft       bool     `json:"draft,omitempty"`
	Merged      bool     `json:"merged,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty"`
	UpdatedAt   string   `json:"updated_at,omitempty"`
	ClosedAt    string   `json:"closed_at,omitempty"`
	MergedAt    string   `json:"merged_at,omitempty"`
}

type MinimalProjectItemFieldValue struct {
	ID       int64  `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	DataType string `json:"data_type,omitempty"`
	Value    any    `json:"value,omitempty"`
}

type minimalProjectOptionValue struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Color string `json:"color,omitempty"`
}

type minimalProjectIterationValue struct {
	ID        string `json:"id,omitempty"`
	Title     string `json:"title,omitempty"`
	StartDate string `json:"start_date,omitempty"`
	Duration  int    `json:"duration,omitempty"`
}

type minimalProjectPullRequestRef struct {
	Number     int    `json:"number,omitempty"`
	Title      string `json:"title,omitempty"`
	State      string `json:"state,omitempty"`
	HTMLURL    string `json:"html_url,omitempty"`
	Repository string `json:"repository,omitempty"`
}

// MinimalReactions is the trimmed output type for reaction summaries, dropping the API URL.
type MinimalReactions struct {
	TotalCount int `json:"total_count"`
	PlusOne    int `json:"+1"`
	MinusOne   int `json:"-1"`
	Laugh      int `json:"laugh"`
	Confused   int `json:"confused"`
	Heart      int `json:"heart"`
	Hooray     int `json:"hooray"`
	Rocket     int `json:"rocket"`
	Eyes       int `json:"eyes"`
}

// MinimalIssueFieldValueSingleSelectOption is the trimmed output type for a single-select option of an issue field value.
type MinimalIssueFieldValueSingleSelectOption struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// MinimalIssueFieldValue is the trimmed output type for a custom field value attached to an issue,
// populated from REST API responses (e.g. get_issue). For GraphQL-sourced field values see MinimalFieldValue.
type MinimalIssueFieldValue struct {
	IssueFieldID       int64                                     `json:"issue_field_id,omitempty"`
	NodeID             string                                    `json:"node_id,omitempty"`
	DataType           string                                    `json:"data_type,omitempty"`
	Value              any                                       `json:"value,omitempty"`
	SingleSelectOption *MinimalIssueFieldValueSingleSelectOption `json:"single_select_option,omitempty"`
}

// MinimalFieldValue is the trimmed output type for a custom field value resolved via GraphQL
// (e.g. list_issues, search_issues). Single-value variants populate Value; Values is reserved for multi-select.
type MinimalFieldValue struct {
	Field  string   `json:"field"`
	Value  string   `json:"value,omitempty"`
	Values []string `json:"values,omitempty"`
}

// MinimalIssue is the trimmed output type for issue objects to reduce verbosity.
type MinimalIssue struct {
	Number            int                      `json:"number"`
	Title             string                   `json:"title"`
	Body              string                   `json:"body,omitempty"`
	State             string                   `json:"state"`
	StateReason       string                   `json:"state_reason,omitempty"`
	Draft             bool                     `json:"draft,omitempty"`
	Locked            bool                     `json:"locked,omitempty"`
	HTMLURL           string                   `json:"html_url,omitempty"`
	User              *MinimalUser             `json:"user,omitempty"`
	AuthorAssociation string                   `json:"author_association,omitempty"`
	Labels            []string                 `json:"labels,omitempty"`
	Assignees         []string                 `json:"assignees,omitempty"`
	Milestone         string                   `json:"milestone,omitempty"`
	Comments          int                      `json:"comments,omitempty"`
	Reactions         *MinimalReactions        `json:"reactions,omitempty"`
	CreatedAt         string                   `json:"created_at,omitempty"`
	UpdatedAt         string                   `json:"updated_at,omitempty"`
	ClosedAt          string                   `json:"closed_at,omitempty"`
	ClosedBy          string                   `json:"closed_by,omitempty"`
	IssueType         string                   `json:"issue_type,omitempty"`
	IssueFieldValues  []MinimalIssueFieldValue `json:"issue_field_values,omitempty"`
	FieldValues       []MinimalFieldValue      `json:"field_values,omitempty"`

	// Hierarchy relationship signals. HasParent and HasChildren are populated when
	// hierarchy enrichment succeeds; SubIssuesSummary is populated when children exist,
	// and Parent when a parent exists and may be surfaced (under lockdown an unverified
	// parent reference is omitted while HasParent stays true).
	HasParent        *bool                    `json:"has_parent,omitempty"`
	HasChildren      *bool                    `json:"has_children,omitempty"`
	Parent           *MinimalIssueRef         `json:"parent,omitempty"`
	SubIssuesSummary *MinimalSubIssuesSummary `json:"sub_issues_summary,omitempty"`
}

// MinimalIssueRef is a compact reference to a related issue (e.g. a parent issue).
// Its keys mirror the get_parent (GetIssueParent) response shape.
type MinimalIssueRef struct {
	Number     int    `json:"number"`
	Title      string `json:"title"`
	State      string `json:"state"`
	URL        string `json:"url"`
	Repository string `json:"repository,omitempty"`
}

// MinimalSubIssuesSummary holds the native GraphQL subIssuesSummary counts for an issue.
type MinimalSubIssuesSummary struct {
	Total            int `json:"total"`
	Completed        int `json:"completed"`
	PercentCompleted int `json:"percent_completed"`
}

// MinimalIssuesResponse is the trimmed output for a paginated list of issues.
type MinimalIssuesResponse struct {
	Issues     []MinimalIssue  `json:"issues"`
	TotalCount int             `json:"totalCount"`
	PageInfo   MinimalPageInfo `json:"pageInfo"`
}

// MinimalIssueComment is the trimmed output type for issue comment objects to reduce verbosity.
type MinimalIssueComment struct {
	ID                int64             `json:"id"`
	Body              string            `json:"body,omitempty"`
	HTMLURL           string            `json:"html_url"`
	User              *MinimalUser      `json:"user,omitempty"`
	AuthorAssociation string            `json:"author_association,omitempty"`
	Reactions         *MinimalReactions `json:"reactions,omitempty"`
	CreatedAt         string            `json:"created_at,omitempty"`
	UpdatedAt         string            `json:"updated_at,omitempty"`
}

// MinimalSearchCommitsResult is the trimmed output type for commit search results.
type MinimalSearchCommitsResult struct {
	TotalCount        int                       `json:"total_count"`
	IncompleteResults bool                      `json:"incomplete_results"`
	Items             []MinimalCommitSearchItem `json:"items"`
}

// MinimalFileContentResponse is the trimmed output type for create/update/delete file responses.
type MinimalFileContentResponse struct {
	Content *MinimalFileContent `json:"content,omitempty"`
	Commit  *MinimalFileCommit  `json:"commit,omitempty"`
}

// MinimalFileContent is the trimmed content portion of a file operation response.
type MinimalFileContent struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	SHA     string `json:"sha"`
	Size    int    `json:"size,omitempty"`
	HTMLURL string `json:"html_url"`
}

// MinimalFileCommit is the trimmed commit portion of a file operation response.
type MinimalFileCommit struct {
	SHA     string               `json:"sha"`
	Message string               `json:"message,omitempty"`
	HTMLURL string               `json:"html_url,omitempty"`
	Author  *MinimalCommitAuthor `json:"author,omitempty"`
}

// MinimalPullRequest is the trimmed output type for pull request objects to reduce verbosity.
type MinimalPullRequest struct {
	Number             int              `json:"number"`
	Title              string           `json:"title"`
	Body               string           `json:"body,omitempty"`
	State              string           `json:"state"`
	Draft              bool             `json:"draft"`
	Merged             bool             `json:"merged"`
	MergeableState     string           `json:"mergeable_state,omitempty"`
	HTMLURL            string           `json:"html_url"`
	User               *MinimalUser     `json:"user,omitempty"`
	Labels             []string         `json:"labels,omitempty"`
	Assignees          []string         `json:"assignees,omitempty"`
	RequestedReviewers []string         `json:"requested_reviewers,omitempty"`
	MergedBy           string           `json:"merged_by,omitempty"`
	Head               *MinimalPRBranch `json:"head,omitempty"`
	Base               *MinimalPRBranch `json:"base,omitempty"`
	Additions          int              `json:"additions,omitempty"`
	Deletions          int              `json:"deletions,omitempty"`
	ChangedFiles       int              `json:"changed_files,omitempty"`
	Commits            int              `json:"commits,omitempty"`
	Comments           int              `json:"comments,omitempty"`
	CreatedAt          string           `json:"created_at,omitempty"`
	UpdatedAt          string           `json:"updated_at,omitempty"`
	ClosedAt           string           `json:"closed_at,omitempty"`
	MergedAt           string           `json:"merged_at,omitempty"`
	Milestone          string           `json:"milestone,omitempty"`
}

// MinimalPRBranch is the trimmed output type for pull request branch references.
type MinimalPRBranch struct {
	Ref  string               `json:"ref"`
	SHA  string               `json:"sha"`
	Repo *MinimalPRBranchRepo `json:"repo,omitempty"`
}

// MinimalPRBranchRepo is the trimmed repo info nested inside a PR branch.
type MinimalPRBranchRepo struct {
	FullName    string `json:"full_name"`
	Description string `json:"description,omitempty"`
}

type MinimalProjectStatusUpdate struct {
	ID         string       `json:"id"`
	Body       string       `json:"body,omitempty"`
	Status     string       `json:"status,omitempty"`
	CreatedAt  string       `json:"created_at,omitempty"`
	StartDate  string       `json:"start_date,omitempty"`
	TargetDate string       `json:"target_date,omitempty"`
	Creator    *MinimalUser `json:"creator,omitempty"`
}

// MinimalPullRequestReview is the trimmed output type for pull request review objects to reduce verbosity.
type MinimalPullRequestReview struct {
	ID                int64        `json:"id"`
	State             string       `json:"state"`
	Body              string       `json:"body,omitempty"`
	HTMLURL           string       `json:"html_url"`
	User              *MinimalUser `json:"user,omitempty"`
	CommitID          string       `json:"commit_id,omitempty"`
	SubmittedAt       string       `json:"submitted_at,omitempty"`
	AuthorAssociation string       `json:"author_association,omitempty"`
}

// Helper functions

func convertToMinimalPullRequestReview(review *github.PullRequestReview) MinimalPullRequestReview {
	m := MinimalPullRequestReview{
		ID:                review.GetID(),
		State:             review.GetState(),
		Body:              review.GetBody(),
		HTMLURL:           review.GetHTMLURL(),
		User:              convertToMinimalUser(review.GetUser()),
		CommitID:          review.GetCommitID(),
		AuthorAssociation: review.GetAuthorAssociation(),
	}

	if review.SubmittedAt != nil {
		m.SubmittedAt = review.SubmittedAt.Format(time.RFC3339)
	}

	return m
}

func convertToMinimalIssue(issue *github.Issue) MinimalIssue {
	m := MinimalIssue{
		Number:            issue.GetNumber(),
		Title:             issue.GetTitle(),
		Body:              issue.GetBody(),
		State:             issue.GetState(),
		StateReason:       issue.GetStateReason(),
		Draft:             issue.GetDraft(),
		Locked:            issue.GetLocked(),
		HTMLURL:           issue.GetHTMLURL(),
		User:              convertToMinimalUser(issue.GetUser()),
		AuthorAssociation: issue.GetAuthorAssociation(),
		Comments:          issue.GetComments(),
	}

	if issue.CreatedAt != nil {
		m.CreatedAt = issue.CreatedAt.Format(time.RFC3339)
	}
	if issue.UpdatedAt != nil {
		m.UpdatedAt = issue.UpdatedAt.Format(time.RFC3339)
	}
	if issue.ClosedAt != nil {
		m.ClosedAt = issue.ClosedAt.Format(time.RFC3339)
	}

	for _, label := range issue.Labels {
		if label != nil {
			m.Labels = append(m.Labels, label.GetName())
		}
	}

	for _, assignee := range issue.Assignees {
		if assignee != nil {
			m.Assignees = append(m.Assignees, assignee.GetLogin())
		}
	}

	if closedBy := issue.GetClosedBy(); closedBy != nil {
		m.ClosedBy = closedBy.GetLogin()
	}

	if milestone := issue.GetMilestone(); milestone != nil {
		m.Milestone = milestone.GetTitle()
	}

	if issueType := issue.GetType(); issueType != nil {
		m.IssueType = issueType.GetName()
	}

	for _, fv := range issue.IssueFieldValues {
		if fv == nil {
			continue
		}
		mfv := MinimalIssueFieldValue{
			IssueFieldID: fv.IssueFieldID,
			NodeID:       fv.NodeID,
			DataType:     fv.DataType,
			Value:        fv.Value,
		}
		if opt := fv.SingleSelectOption; opt != nil {
			mfv.SingleSelectOption = &MinimalIssueFieldValueSingleSelectOption{
				ID:    opt.ID,
				Name:  opt.Name,
				Color: opt.Color,
			}
		}
		m.IssueFieldValues = append(m.IssueFieldValues, mfv)
	}

	if r := issue.Reactions; r != nil {
		m.Reactions = &MinimalReactions{
			TotalCount: r.GetTotalCount(),
			PlusOne:    r.GetPlusOne(),
			MinusOne:   r.GetMinusOne(),
			Laugh:      r.GetLaugh(),
			Confused:   r.GetConfused(),
			Heart:      r.GetHeart(),
			Hooray:     r.GetHooray(),
			Rocket:     r.GetRocket(),
			Eyes:       r.GetEyes(),
		}
	}

	return m
}

func fragmentToMinimalIssue(fragment IssueFragment) MinimalIssue {
	m := MinimalIssue{
		Number:    int(fragment.Number),
		Title:     sanitize.Sanitize(string(fragment.Title)),
		Body:      sanitize.Sanitize(string(fragment.Body)),
		State:     string(fragment.State),
		Comments:  int(fragment.Comments.TotalCount),
		CreatedAt: fragment.CreatedAt.Format(time.RFC3339),
		UpdatedAt: fragment.UpdatedAt.Format(time.RFC3339),
		User: &MinimalUser{
			Login: string(fragment.Author.Login),
		},
	}

	for _, label := range fragment.Labels.Nodes {
		m.Labels = append(m.Labels, string(label.Name))
	}

	for _, fv := range fragment.IssueFieldValues.Nodes {
		if mfv, ok := fragmentToMinimalFieldValue(fv); ok {
			m.FieldValues = append(m.FieldValues, mfv)
		}
	}

	return m
}

// fragmentToMinimalFieldValue flattens the union value fragment into a single
// {field, value} pair. Returns ok=false if the typename is unrecognised.
func fragmentToMinimalFieldValue(fv IssueFieldValueFragment) (MinimalFieldValue, bool) {
	switch fv.TypeName {
	case "IssueFieldDateValue":
		return MinimalFieldValue{
			Field: fv.DateValue.Field.Name(),
			Value: string(fv.DateValue.Value),
		}, true
	case "IssueFieldNumberValue":
		return MinimalFieldValue{
			Field: fv.NumberValue.Field.Name(),
			Value: strconv.FormatFloat(float64(fv.NumberValue.Value), 'f', -1, 64),
		}, true
	case "IssueFieldSingleSelectValue":
		return MinimalFieldValue{
			Field: fv.SingleSelectValue.Field.Name(),
			Value: string(fv.SingleSelectValue.Value),
		}, true
	case "IssueFieldTextValue":
		return MinimalFieldValue{
			Field: fv.TextValue.Field.Name(),
			Value: string(fv.TextValue.Value),
		}, true
	}
	return MinimalFieldValue{}, false
}

func convertToMinimalIssuesResponse(fragment IssueQueryFragment) MinimalIssuesResponse {
	minimalIssues := make([]MinimalIssue, 0, len(fragment.Nodes))
	for _, issue := range fragment.Nodes {
		minimalIssues = append(minimalIssues, fragmentToMinimalIssue(issue))
	}

	return MinimalIssuesResponse{
		Issues:     minimalIssues,
		TotalCount: fragment.TotalCount,
		PageInfo: MinimalPageInfo{
			HasNextPage:     bool(fragment.PageInfo.HasNextPage),
			HasPreviousPage: bool(fragment.PageInfo.HasPreviousPage),
			StartCursor:     string(fragment.PageInfo.StartCursor),
			EndCursor:       string(fragment.PageInfo.EndCursor),
		},
	}
}

func convertToMinimalIssueComment(comment *github.IssueComment) MinimalIssueComment {
	m := MinimalIssueComment{
		ID:                comment.GetID(),
		Body:              comment.GetBody(),
		HTMLURL:           comment.GetHTMLURL(),
		User:              convertToMinimalUser(comment.GetUser()),
		AuthorAssociation: comment.GetAuthorAssociation(),
	}

	if comment.CreatedAt != nil {
		m.CreatedAt = comment.CreatedAt.Format(time.RFC3339)
	}
	if comment.UpdatedAt != nil {
		m.UpdatedAt = comment.UpdatedAt.Format(time.RFC3339)
	}

	if r := comment.Reactions; r != nil {
		m.Reactions = &MinimalReactions{
			TotalCount: r.GetTotalCount(),
			PlusOne:    r.GetPlusOne(),
			MinusOne:   r.GetMinusOne(),
			Laugh:      r.GetLaugh(),
			Confused:   r.GetConfused(),
			Heart:      r.GetHeart(),
			Hooray:     r.GetHooray(),
			Rocket:     r.GetRocket(),
			Eyes:       r.GetEyes(),
		}
	}

	return m
}

func convertToMinimalFileContentResponse(resp *github.RepositoryContentResponse) MinimalFileContentResponse {
	m := MinimalFileContentResponse{}

	if resp == nil {
		return m
	}

	if c := resp.Content; c != nil {
		m.Content = &MinimalFileContent{
			Name:    c.GetName(),
			Path:    c.GetPath(),
			SHA:     c.GetSHA(),
			Size:    c.GetSize(),
			HTMLURL: c.GetHTMLURL(),
		}
	}

	m.Commit = &MinimalFileCommit{
		SHA:     resp.Commit.GetSHA(),
		Message: resp.Commit.GetMessage(),
		HTMLURL: resp.Commit.GetHTMLURL(),
	}

	if author := resp.Commit.Author; author != nil {
		m.Commit.Author = &MinimalCommitAuthor{
			Name:  author.GetName(),
			Email: author.GetEmail(),
		}
		if author.Date != nil {
			m.Commit.Author.Date = author.Date.Format(time.RFC3339)
		}
	}

	return m
}

func convertToMinimalPullRequest(pr *github.PullRequest) MinimalPullRequest {
	m := MinimalPullRequest{
		Number:         pr.GetNumber(),
		Title:          pr.GetTitle(),
		Body:           pr.GetBody(),
		State:          pr.GetState(),
		Draft:          pr.GetDraft(),
		Merged:         pr.GetMerged(),
		MergeableState: pr.GetMergeableState(),
		HTMLURL:        pr.GetHTMLURL(),
		User:           convertToMinimalUser(pr.GetUser()),
		Additions:      pr.GetAdditions(),
		Deletions:      pr.GetDeletions(),
		ChangedFiles:   pr.GetChangedFiles(),
		Commits:        pr.GetCommits(),
		Comments:       pr.GetComments(),
	}

	if pr.CreatedAt != nil {
		m.CreatedAt = pr.CreatedAt.Format(time.RFC3339)
	}
	if pr.UpdatedAt != nil {
		m.UpdatedAt = pr.UpdatedAt.Format(time.RFC3339)
	}
	if pr.ClosedAt != nil {
		m.ClosedAt = pr.ClosedAt.Format(time.RFC3339)
	}
	if pr.MergedAt != nil {
		m.MergedAt = pr.MergedAt.Format(time.RFC3339)
	}

	for _, label := range pr.Labels {
		if label != nil {
			m.Labels = append(m.Labels, label.GetName())
		}
	}

	for _, assignee := range pr.Assignees {
		if assignee != nil {
			m.Assignees = append(m.Assignees, assignee.GetLogin())
		}
	}

	for _, reviewer := range pr.RequestedReviewers {
		if reviewer != nil {
			m.RequestedReviewers = append(m.RequestedReviewers, reviewer.GetLogin())
		}
	}

	if mergedBy := pr.GetMergedBy(); mergedBy != nil {
		m.MergedBy = mergedBy.GetLogin()
	}

	if head := pr.Head; head != nil {
		m.Head = convertToMinimalPRBranch(head)
	}

	if base := pr.Base; base != nil {
		m.Base = convertToMinimalPRBranch(base)
	}

	if milestone := pr.GetMilestone(); milestone != nil {
		m.Milestone = milestone.GetTitle()
	}

	return m
}

func convertToMinimalPRBranch(branch *github.PullRequestBranch) *MinimalPRBranch {
	if branch == nil {
		return nil
	}

	b := &MinimalPRBranch{
		Ref: branch.GetRef(),
		SHA: branch.GetSHA(),
	}

	if repo := branch.GetRepo(); repo != nil {
		b.Repo = &MinimalPRBranchRepo{
			FullName:    repo.GetFullName(),
			Description: repo.GetDescription(),
		}
	}

	return b
}

func convertToMinimalProject(fullProject *github.ProjectV2) *MinimalProject {
	if fullProject == nil {
		return nil
	}

	return &MinimalProject{
		ID:               github.Ptr(fullProject.GetID()),
		NodeID:           github.Ptr(fullProject.GetNodeID()),
		Owner:            convertToMinimalUser(fullProject.GetOwner()),
		Creator:          convertToMinimalUser(fullProject.GetCreator()),
		Title:            github.Ptr(fullProject.GetTitle()),
		Description:      github.Ptr(fullProject.GetDescription()),
		Public:           github.Ptr(fullProject.GetPublic()),
		ClosedAt:         github.Ptr(fullProject.GetClosedAt()),
		CreatedAt:        github.Ptr(fullProject.GetCreatedAt()),
		UpdatedAt:        github.Ptr(fullProject.GetUpdatedAt()),
		DeletedAt:        github.Ptr(fullProject.GetDeletedAt()),
		Number:           github.Ptr(fullProject.GetNumber()),
		ShortDescription: github.Ptr(fullProject.GetShortDescription()),
		DeletedBy:        convertToMinimalUser(fullProject.GetDeletedBy()),
	}
}

func convertToMinimalProjectItem(item *github.ProjectV2Item) MinimalProjectItem {
	if item == nil {
		return MinimalProjectItem{}
	}

	contentType := ""
	if item.ContentType != nil {
		contentType = string(*item.ContentType)
	}

	creator := ""
	if item.Creator != nil {
		creator = item.Creator.GetLogin()
	}

	return MinimalProjectItem{
		ID:          item.GetID(),
		NodeID:      item.GetNodeID(),
		ContentType: contentType,
		Content:     convertToMinimalProjectItemContent(item.GetContent()),
		Fields:      convertToMinimalProjectItemFields(item.GetFields()),
		ArchivedAt:  formatProjectTimestamp(item.ArchivedAt),
		CreatedAt:   formatProjectTimestamp(item.CreatedAt),
		UpdatedAt:   formatProjectTimestamp(item.UpdatedAt),
		Creator:     creator,
	}
}

func convertToMinimalProjectItemContent(content *github.ProjectV2ItemContent) *MinimalProjectItemContent {
	if content == nil {
		return nil
	}

	if issue := content.GetIssue(); issue != nil {
		return convertIssueToMinimalProjectItemContent(issue)
	}
	if pr := content.GetPullRequest(); pr != nil {
		return convertPullRequestToMinimalProjectItemContent(pr)
	}
	if draftIssue := content.GetDraftIssue(); draftIssue != nil {
		return convertDraftIssueToMinimalProjectItemContent(draftIssue)
	}

	return nil
}

func convertIssueToMinimalProjectItemContent(issue *github.Issue) *MinimalProjectItemContent {
	m := &MinimalProjectItemContent{
		ID:          issue.GetID(),
		NodeID:      issue.GetNodeID(),
		Number:      issue.GetNumber(),
		Title:       issue.GetTitle(),
		State:       issue.GetState(),
		StateReason: issue.GetStateReason(),
		HTMLURL:     issue.GetHTMLURL(),
		Repository:  issueRepositoryFullName(issue),
		Comments:    issue.GetComments(),
		Draft:       issue.GetDraft(),
		CreatedAt:   formatProjectTimestamp(issue.CreatedAt),
		UpdatedAt:   formatProjectTimestamp(issue.UpdatedAt),
		ClosedAt:    formatProjectTimestamp(issue.ClosedAt),
	}

	if user := issue.GetUser(); user != nil {
		m.Author = user.GetLogin()
	}
	for _, assignee := range issue.Assignees {
		if assignee != nil {
			m.Assignees = append(m.Assignees, assignee.GetLogin())
		}
	}
	for _, label := range issue.Labels {
		if label != nil {
			m.Labels = append(m.Labels, label.GetName())
		}
	}
	if milestone := issue.GetMilestone(); milestone != nil {
		m.Milestone = milestone.GetTitle()
	}

	return m
}

func convertPullRequestToMinimalProjectItemContent(pr *github.PullRequest) *MinimalProjectItemContent {
	m := &MinimalProjectItemContent{
		ID:         pr.GetID(),
		NodeID:     pr.GetNodeID(),
		Number:     pr.GetNumber(),
		Title:      pr.GetTitle(),
		State:      pr.GetState(),
		HTMLURL:    pr.GetHTMLURL(),
		Repository: pullRequestRepositoryFullName(pr),
		Comments:   pr.GetComments(),
		Draft:      pr.GetDraft(),
		Merged:     pr.GetMerged(),
		CreatedAt:  formatProjectTimestamp(pr.CreatedAt),
		UpdatedAt:  formatProjectTimestamp(pr.UpdatedAt),
		ClosedAt:   formatProjectTimestamp(pr.ClosedAt),
		MergedAt:   formatProjectTimestamp(pr.MergedAt),
	}

	if user := pr.GetUser(); user != nil {
		m.Author = user.GetLogin()
	}
	for _, assignee := range pr.Assignees {
		if assignee != nil {
			m.Assignees = append(m.Assignees, assignee.GetLogin())
		}
	}
	for _, label := range pr.Labels {
		if label != nil {
			m.Labels = append(m.Labels, label.GetName())
		}
	}
	if milestone := pr.GetMilestone(); milestone != nil {
		m.Milestone = milestone.GetTitle()
	}

	return m
}

func convertDraftIssueToMinimalProjectItemContent(draftIssue *github.ProjectV2DraftIssue) *MinimalProjectItemContent {
	m := &MinimalProjectItemContent{
		ID:        draftIssue.GetID(),
		NodeID:    draftIssue.GetNodeID(),
		Title:     draftIssue.GetTitle(),
		CreatedAt: formatProjectTimestamp(draftIssue.CreatedAt),
		UpdatedAt: formatProjectTimestamp(draftIssue.UpdatedAt),
	}

	if user := draftIssue.GetUser(); user != nil {
		m.Author = user.GetLogin()
	}

	return m
}

func convertToMinimalProjectItemFields(fields []*github.ProjectV2ItemFieldValue) []MinimalProjectItemFieldValue {
	minimalFields := make([]MinimalProjectItemFieldValue, 0, len(fields))
	for _, field := range fields {
		if field == nil {
			continue
		}
		minimalFields = append(minimalFields, MinimalProjectItemFieldValue{
			ID:       field.GetID(),
			Name:     field.GetName(),
			DataType: field.GetDataType(),
			Value:    minimalProjectFieldValue(field.GetValue()),
		})
	}
	return minimalFields
}

func minimalProjectFieldValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return v
	case []string:
		return v
	case map[string]any:
		return minimalProjectMapValue(v)
	case []any:
		return minimalProjectArrayValue(v)
	case *github.User:
		return v.GetLogin()
	case *github.Label:
		return v.GetName()
	case *github.Repository:
		return v.GetFullName()
	case *github.Milestone:
		return v.GetTitle()
	case *github.PullRequest:
		return minimalProjectPullRequestRefFromPullRequest(v)
	case *github.ProjectV2FieldOption:
		return minimalProjectOptionValue{
			ID:    v.GetID(),
			Name:  projectTextContentString(v.GetName()),
			Color: v.GetColor(),
		}
	case *github.ProjectV2FieldIteration:
		return minimalProjectIterationValue{
			ID:        v.GetID(),
			Title:     projectTextContentString(v.GetTitle()),
			StartDate: v.GetStartDate(),
			Duration:  v.GetDuration(),
		}
	case []*github.User:
		logins := make([]string, 0, len(v))
		for _, user := range v {
			if user != nil {
				logins = append(logins, user.GetLogin())
			}
		}
		return logins
	case []*github.Label:
		names := make([]string, 0, len(v))
		for _, label := range v {
			if label != nil {
				names = append(names, label.GetName())
			}
		}
		return names
	case []*github.PullRequest:
		refs := make([]minimalProjectPullRequestRef, 0, len(v))
		for _, pr := range v {
			if pr != nil {
				refs = append(refs, minimalProjectPullRequestRefFromPullRequest(pr))
			}
		}
		return refs
	default:
		return nil
	}
}

func minimalProjectMapValue(value map[string]any) any {
	if text := minimalProjectTextValue(value); text != "" {
		return text
	}
	if repo := fullNameFromMap(value); repo != "" {
		return repo
	}
	if login := stringFromMap(value, "login"); login != "" {
		return login
	}
	if isPullRequestMap(value) {
		return minimalProjectPullRequestRefFromMap(value)
	}
	if option, ok := minimalProjectOptionFromMap(value); ok {
		return option
	}
	if iteration, ok := minimalProjectIterationFromMap(value); ok {
		return iteration
	}
	if title := stringFromMap(value, "title"); title != "" {
		return title
	}
	if name := stringFromMap(value, "name"); name != "" {
		return name
	}

	compact := make(map[string]any)
	for key, nestedValue := range value {
		minimalValue := minimalProjectFieldValue(nestedValue)
		if shouldKeepMinimalProjectValue(minimalValue) {
			compact[key] = minimalValue
		}
	}
	if len(compact) == 0 {
		return nil
	}
	return compact
}

func minimalProjectArrayValue(values []any) any {
	if refs, ok := minimalProjectPullRequestRefsFromArray(values); ok {
		return refs
	}
	if strings, ok := minimalProjectStringsFromArray(values, "login"); ok {
		return strings
	}
	if strings, ok := minimalProjectStringsFromArray(values, "name"); ok {
		return strings
	}

	compact := make([]any, 0, len(values))
	for _, value := range values {
		minimalValue := minimalProjectFieldValue(value)
		if shouldKeepMinimalProjectValue(minimalValue) {
			compact = append(compact, minimalValue)
		}
	}
	if len(compact) == 0 {
		return nil
	}
	return compact
}

func minimalProjectTextValue(value map[string]any) string {
	if raw := stringFromMap(value, "raw"); raw != "" {
		return raw
	}
	if html := stringFromMap(value, "html"); html != "" {
		return html
	}
	return stringFromMap(value, "text")
}

func minimalProjectOptionFromMap(value map[string]any) (minimalProjectOptionValue, bool) {
	name := textContentStringFromMap(value, "name")
	color := stringFromMap(value, "color")
	if name == "" && color == "" {
		return minimalProjectOptionValue{}, false
	}
	return minimalProjectOptionValue{
		ID:    stringFromMap(value, "id"),
		Name:  name,
		Color: color,
	}, true
}

func minimalProjectIterationFromMap(value map[string]any) (minimalProjectIterationValue, bool) {
	startDate := stringFromMap(value, "start_date")
	duration := intFromAny(value["duration"])
	if startDate == "" && duration == 0 {
		return minimalProjectIterationValue{}, false
	}
	return minimalProjectIterationValue{
		ID:        stringFromMap(value, "id"),
		Title:     textContentStringFromMap(value, "title"),
		StartDate: startDate,
		Duration:  duration,
	}, true
}

// textContentStringFromMap returns a string for a field that may be either a
// plain string or a nested ProjectV2TextContent object (with raw/html/text
// fields), as returned for project option names and iteration titles.
func textContentStringFromMap(value map[string]any, key string) string {
	if s := stringFromMap(value, key); s != "" {
		return s
	}
	if nested, ok := value[key].(map[string]any); ok {
		return minimalProjectTextValue(nested)
	}
	return ""
}

func minimalProjectPullRequestRefsFromArray(values []any) ([]minimalProjectPullRequestRef, bool) {
	refs := make([]minimalProjectPullRequestRef, 0, len(values))
	for _, value := range values {
		switch pr := value.(type) {
		case map[string]any:
			if !isPullRequestMap(pr) {
				return nil, false
			}
			refs = append(refs, minimalProjectPullRequestRefFromMap(pr))
		case *github.PullRequest:
			if pr == nil {
				continue
			}
			refs = append(refs, minimalProjectPullRequestRefFromPullRequest(pr))
		default:
			return nil, false
		}
	}
	return refs, len(refs) > 0
}

func minimalProjectStringsFromArray(values []any, key string) ([]string, bool) {
	strings := make([]string, 0, len(values))
	for _, value := range values {
		switch v := value.(type) {
		case map[string]any:
			stringValue := stringFromMap(v, key)
			if stringValue == "" {
				return nil, false
			}
			strings = append(strings, stringValue)
		case *github.User:
			if key != "login" || v == nil {
				return nil, false
			}
			strings = append(strings, v.GetLogin())
		case *github.Label:
			if key != "name" || v == nil {
				return nil, false
			}
			strings = append(strings, v.GetName())
		default:
			return nil, false
		}
	}
	return strings, len(strings) > 0
}

func minimalProjectPullRequestRefFromPullRequest(pr *github.PullRequest) minimalProjectPullRequestRef {
	if pr == nil {
		return minimalProjectPullRequestRef{}
	}
	return minimalProjectPullRequestRef{
		Number:     pr.GetNumber(),
		Title:      pr.GetTitle(),
		State:      pr.GetState(),
		HTMLURL:    pr.GetHTMLURL(),
		Repository: pullRequestRepositoryFullName(pr),
	}
}

func minimalProjectPullRequestRefFromMap(value map[string]any) minimalProjectPullRequestRef {
	htmlURL := stringFromMap(value, "html_url")
	repository := fullNameFromMapValue(value["repository"])
	if repository == "" {
		repository = branchRepositoryFullNameFromMap(value, "base")
	}
	if repository == "" {
		repository = branchRepositoryFullNameFromMap(value, "head")
	}
	if repository == "" {
		repository = repositoryFromHTMLURL(htmlURL)
	}

	return minimalProjectPullRequestRef{
		Number:     intFromAny(value["number"]),
		Title:      stringFromMap(value, "title"),
		State:      stringFromMap(value, "state"),
		HTMLURL:    htmlURL,
		Repository: repository,
	}
}

func isPullRequestMap(value map[string]any) bool {
	return intFromAny(value["number"]) != 0 && (stringFromMap(value, "html_url") != "" || stringFromMap(value, "state") != "")
}

func branchRepositoryFullNameFromMap(value map[string]any, branchKey string) string {
	branch, ok := value[branchKey].(map[string]any)
	if !ok {
		return ""
	}
	return fullNameFromMapValue(branch["repo"])
}

func shouldKeepMinimalProjectValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return v != ""
	case []any:
		return len(v) > 0
	case []string:
		return len(v) > 0
	case []minimalProjectPullRequestRef:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		return true
	}
}

func issueRepositoryFullName(issue *github.Issue) string {
	if repo := issue.GetRepository(); repo != nil {
		return repo.GetFullName()
	}
	return repositoryFromHTMLURL(issue.GetHTMLURL())
}

func pullRequestRepositoryFullName(pr *github.PullRequest) string {
	if base := pr.GetBase(); base != nil {
		if repo := base.GetRepo(); repo != nil && repo.GetFullName() != "" {
			return repo.GetFullName()
		}
	}
	if head := pr.GetHead(); head != nil {
		if repo := head.GetRepo(); repo != nil && repo.GetFullName() != "" {
			return repo.GetFullName()
		}
	}
	return repositoryFromHTMLURL(pr.GetHTMLURL())
}

func fullNameFromMapValue(value any) string {
	repo, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	return fullNameFromMap(repo)
}

func fullNameFromMap(value map[string]any) string {
	return stringFromMap(value, "full_name")
}

func repositoryFromHTMLURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	return parts[0] + "/" + parts[1]
}

func projectTextContentString(content *github.ProjectV2TextContent) string {
	if content == nil {
		return ""
	}
	if raw := content.GetRaw(); raw != "" {
		return raw
	}
	return content.GetHTML()
}

func formatProjectTimestamp(timestamp *github.Timestamp) string {
	if timestamp == nil || timestamp.IsZero() {
		return ""
	}
	return timestamp.Format(time.RFC3339)
}

func stringFromMap(value map[string]any, key string) string {
	return stringFromAny(value[key])
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		i, err := strconv.Atoi(v)
		if err != nil {
			return 0
		}
		return i
	default:
		return 0
	}
}

func convertToMinimalUser(user *github.User) *MinimalUser {
	if user == nil {
		return nil
	}

	return &MinimalUser{
		Login:      user.GetLogin(),
		ID:         user.GetID(),
		ProfileURL: user.GetHTMLURL(),
		AvatarURL:  user.GetAvatarURL(),
	}
}

// newMinimalCommitFromCore builds a MinimalCommit from the fields that are
// shared between *github.RepositoryCommit and *github.CommitResult. Caller
// is responsible for setting any type-specific extras (stats/files for
// RepositoryCommit, repository for CommitResult).
func newMinimalCommitFromCore(sha, htmlURL string, commit *github.Commit, author, committer *github.User) MinimalCommit {
	minimalCommit := MinimalCommit{
		SHA:     sha,
		HTMLURL: htmlURL,
	}

	if commit != nil {
		minimalCommit.Commit = &MinimalCommitInfo{
			Message: commit.GetMessage(),
		}

		if commit.Author != nil {
			minimalCommit.Commit.Author = &MinimalCommitAuthor{
				Name:  commit.Author.GetName(),
				Email: commit.Author.GetEmail(),
			}
			if commit.Author.Date != nil {
				minimalCommit.Commit.Author.Date = commit.Author.Date.Format(time.RFC3339)
			}
		}

		if commit.Committer != nil {
			minimalCommit.Commit.Committer = &MinimalCommitAuthor{
				Name:  commit.Committer.GetName(),
				Email: commit.Committer.GetEmail(),
			}
			if commit.Committer.Date != nil {
				minimalCommit.Commit.Committer.Date = commit.Committer.Date.Format(time.RFC3339)
			}
		}
	}

	if author != nil {
		minimalCommit.Author = &MinimalUser{
			Login:      author.GetLogin(),
			ID:         author.GetID(),
			ProfileURL: author.GetHTMLURL(),
			AvatarURL:  author.GetAvatarURL(),
		}
	}

	if committer != nil {
		minimalCommit.Committer = &MinimalUser{
			Login:      committer.GetLogin(),
			ID:         committer.GetID(),
			ProfileURL: committer.GetHTMLURL(),
			AvatarURL:  committer.GetAvatarURL(),
		}
	}

	return minimalCommit
}

// commitDetail controls how much per-file information convertToMinimalCommit
// includes in its output.
type commitDetail string

const (
	// commitDetailNone omits Stats and Files entirely.
	commitDetailNone commitDetail = "none"
	// commitDetailStats includes Stats and Files with metadata only
	// (filename, status, additions, deletions, changes) but no patch text.
	commitDetailStats commitDetail = "stats"
	// commitDetailFullPatch additionally includes the unified diff for each file.
	commitDetailFullPatch commitDetail = "full_patch"
)

// parseCommitDetail validates the user-supplied detail value and returns the
// default (stats) when the value is empty.
func parseCommitDetail(s string) (commitDetail, error) {
	switch s {
	case "":
		return commitDetailStats, nil
	case string(commitDetailNone), string(commitDetailStats), string(commitDetailFullPatch):
		return commitDetail(s), nil
	default:
		return "", fmt.Errorf("invalid detail %q: must be one of \"none\", \"stats\", \"full_patch\"", s)
	}
}

func convertToMinimalCommit(commit *github.RepositoryCommit, detail commitDetail) MinimalCommit {
	minimalCommit := newMinimalCommitFromCore(
		commit.GetSHA(),
		commit.GetHTMLURL(),
		commit.Commit,
		commit.Author,
		commit.Committer,
	)

	if detail == commitDetailNone {
		return minimalCommit
	}

	if commit.Stats != nil {
		minimalCommit.Stats = &MinimalCommitStats{
			Additions: commit.Stats.GetAdditions(),
			Deletions: commit.Stats.GetDeletions(),
			Total:     commit.Stats.GetTotal(),
		}
	}

	if len(commit.Files) > 0 {
		minimalCommit.Files = make([]MinimalCommitFile, 0, len(commit.Files))
		for _, file := range commit.Files {
			minimalFile := MinimalCommitFile{
				Filename:  file.GetFilename(),
				Status:    file.GetStatus(),
				Additions: file.GetAdditions(),
				Deletions: file.GetDeletions(),
				Changes:   file.GetChanges(),
			}
			if detail == commitDetailFullPatch {
				minimalFile.Patch = file.GetPatch()
			}
			minimalCommit.Files = append(minimalCommit.Files, minimalFile)
		}
	}

	return minimalCommit
}

// convertCommitResultToMinimalCommit converts a GitHub API commit search
// result, attaching the containing repository so the caller can tell which
// repo each result came from.
func convertCommitResultToMinimalCommit(commit *github.CommitResult) MinimalCommitSearchItem {
	item := MinimalCommitSearchItem{
		MinimalCommit: newMinimalCommitFromCore(
			commit.GetSHA(),
			commit.GetHTMLURL(),
			commit.Commit,
			commit.Author,
			commit.Committer,
		),
	}

	if commit.Repository != nil {
		item.Repository = &MinimalRepoRef{
			FullName: commit.Repository.GetFullName(),
			HTMLURL:  commit.Repository.GetHTMLURL(),
			Private:  commit.Repository.GetPrivate(),
		}
	}

	return item
}

// MinimalPageInfo contains pagination cursor information.
type MinimalPageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	StartCursor     string `json:"startCursor,omitempty"`
	EndCursor       string `json:"endCursor,omitempty"`
}

// MinimalReviewComment is the trimmed output type for PR review comment objects.
type MinimalReviewComment struct {
	Body      string `json:"body,omitempty"`
	Path      string `json:"path"`
	Line      *int   `json:"line,omitempty"`
	Author    string `json:"author,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
	HTMLURL   string `json:"html_url"`
}

// MinimalReviewThread is the trimmed output type for PR review thread objects.
type MinimalReviewThread struct {
	ID          string                 `json:"id"`
	IsResolved  bool                   `json:"is_resolved"`
	IsOutdated  bool                   `json:"is_outdated"`
	IsCollapsed bool                   `json:"is_collapsed"`
	Comments    []MinimalReviewComment `json:"comments"`
	TotalCount  int                    `json:"total_count"`
}

// MinimalReviewThreadsResponse is the trimmed output for a paginated list of PR review threads.
type MinimalReviewThreadsResponse struct {
	ReviewThreads []MinimalReviewThread `json:"review_threads"`
	TotalCount    int                   `json:"totalCount"`
	PageInfo      MinimalPageInfo       `json:"pageInfo"`
}

func convertToMinimalPRFiles(files []*github.CommitFile) []MinimalPRFile {
	result := make([]MinimalPRFile, 0, len(files))
	for _, f := range files {
		result = append(result, MinimalPRFile{
			Filename:         f.GetFilename(),
			Status:           f.GetStatus(),
			Additions:        f.GetAdditions(),
			Deletions:        f.GetDeletions(),
			Changes:          f.GetChanges(),
			Patch:            f.GetPatch(),
			PreviousFilename: f.GetPreviousFilename(),
		})
	}
	return result
}

func convertToMinimalPullRequestCommits(commits []*github.RepositoryCommit) []MinimalPullRequestCommit {
	result := make([]MinimalPullRequestCommit, 0, len(commits))
	for _, commit := range commits {
		if commit == nil {
			continue
		}

		minimalCommit := MinimalPullRequestCommit{
			SHA:     commit.GetSHA(),
			HTMLURL: commit.GetHTMLURL(),
		}

		if commit.Commit != nil {
			minimalCommit.Message = commit.Commit.GetMessage()
			minimalCommit.Author = convertToMinimalCommitAuthor(commit.Commit.Author)
		}

		result = append(result, minimalCommit)
	}
	return result
}

func convertToMinimalCommitAuthor(author *github.CommitAuthor) *MinimalCommitAuthor {
	if author == nil {
		return nil
	}

	minimalAuthor := &MinimalCommitAuthor{
		Name:  author.GetName(),
		Email: author.GetEmail(),
	}
	if author.Date != nil {
		minimalAuthor.Date = author.Date.Format(time.RFC3339)
	}

	return minimalAuthor
}

// convertToMinimalBranch converts a GitHub API Branch to MinimalBranch
func convertToMinimalBranch(branch *github.Branch) MinimalBranch {
	return MinimalBranch{
		Name:      branch.GetName(),
		SHA:       branch.GetCommit().GetSHA(),
		Protected: branch.GetProtected(),
	}
}

func convertToMinimalRelease(release *github.RepositoryRelease) MinimalRelease {
	m := MinimalRelease{
		ID:         release.GetID(),
		TagName:    release.GetTagName(),
		Name:       release.GetName(),
		Body:       release.GetBody(),
		HTMLURL:    release.GetHTMLURL(),
		Prerelease: release.GetPrerelease(),
		Draft:      release.GetDraft(),
		Author:     convertToMinimalUser(release.GetAuthor()),
	}

	if release.PublishedAt != nil {
		m.PublishedAt = release.PublishedAt.Format(time.RFC3339)
	}

	return m
}

func convertToMinimalTag(tag *github.RepositoryTag) MinimalTag {
	m := MinimalTag{
		Name: tag.GetName(),
	}

	if commit := tag.GetCommit(); commit != nil {
		m.SHA = commit.GetSHA()
	}

	return m
}

// MinimalCheckRun is the trimmed output type for check run objects.
type MinimalCheckRun struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Conclusion  string `json:"conclusion,omitempty"`
	HTMLURL     string `json:"html_url,omitempty"`
	DetailsURL  string `json:"details_url,omitempty"`
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// MinimalCheckRunsResult is the trimmed output type for check runs list results.
type MinimalCheckRunsResult struct {
	TotalCount int               `json:"total_count"`
	CheckRuns  []MinimalCheckRun `json:"check_runs"`
}

// MinimalReviewerUser is a user requested to review a pull request.
type MinimalReviewerUser struct {
	Login   string `json:"login"`
	HTMLURL string `json:"html_url,omitempty"`
}

// MinimalReviewerTeam is a team requested to review a pull request.
type MinimalReviewerTeam struct {
	Slug    string `json:"slug"`
	Name    string `json:"name,omitempty"`
	HTMLURL string `json:"html_url,omitempty"`
}

// MinimalPRReviewers contains the list of requested reviewers for a pull request.
type MinimalPRReviewers struct {
	Users []MinimalReviewerUser `json:"users"`
	Teams []MinimalReviewerTeam `json:"teams"`
}

// MinimalCommitStatus represents a single commit status check.
type MinimalCommitStatus struct {
	State       string `json:"state"`
	Context     string `json:"context"`
	Description string `json:"description,omitempty"`
	TargetURL   string `json:"target_url,omitempty"`
}

// MinimalStatusChecks contains a unified view of all status checks for a PR.
type MinimalStatusChecks struct {
	CombinedState string                `json:"combined_state"`
	Statuses      []MinimalCommitStatus `json:"statuses"`
	CheckRuns     []MinimalCheckRun     `json:"check_runs"`
	TotalCount    int                   `json:"total_count"`
}

// convertToMinimalCheckRun converts a GitHub API CheckRun to MinimalCheckRun
func convertToMinimalCheckRun(checkRun *github.CheckRun) MinimalCheckRun {
	minimalCheckRun := MinimalCheckRun{
		ID:         checkRun.GetID(),
		Name:       checkRun.GetName(),
		Status:     checkRun.GetStatus(),
		Conclusion: checkRun.GetConclusion(),
		HTMLURL:    checkRun.GetHTMLURL(),
		DetailsURL: checkRun.GetDetailsURL(),
	}

	if checkRun.StartedAt != nil {
		minimalCheckRun.StartedAt = checkRun.StartedAt.Format("2006-01-02T15:04:05Z")
	}
	if checkRun.CompletedAt != nil {
		minimalCheckRun.CompletedAt = checkRun.CompletedAt.Format("2006-01-02T15:04:05Z")
	}

	return minimalCheckRun
}

func convertToMinimalReviewThreadsResponse(query reviewThreadsQuery) MinimalReviewThreadsResponse {
	threads := query.Repository.PullRequest.ReviewThreads

	minimalThreads := make([]MinimalReviewThread, 0, len(threads.Nodes))
	for _, thread := range threads.Nodes {
		minimalThreads = append(minimalThreads, convertToMinimalReviewThread(thread))
	}

	return MinimalReviewThreadsResponse{
		ReviewThreads: minimalThreads,
		TotalCount:    int(threads.TotalCount),
		PageInfo: MinimalPageInfo{
			HasNextPage:     bool(threads.PageInfo.HasNextPage),
			HasPreviousPage: bool(threads.PageInfo.HasPreviousPage),
			StartCursor:     string(threads.PageInfo.StartCursor),
			EndCursor:       string(threads.PageInfo.EndCursor),
		},
	}
}

func convertToMinimalReviewThread(thread reviewThreadNode) MinimalReviewThread {
	comments := make([]MinimalReviewComment, 0, len(thread.Comments.Nodes))
	for _, c := range thread.Comments.Nodes {
		comments = append(comments, convertToMinimalReviewComment(c))
	}

	return MinimalReviewThread{
		ID:          fmt.Sprintf("%v", thread.ID),
		IsResolved:  bool(thread.IsResolved),
		IsOutdated:  bool(thread.IsOutdated),
		IsCollapsed: bool(thread.IsCollapsed),
		Comments:    comments,
		TotalCount:  int(thread.Comments.TotalCount),
	}
}

func convertToMinimalReviewComment(c reviewCommentNode) MinimalReviewComment {
	m := MinimalReviewComment{
		Body:    string(c.Body),
		Path:    string(c.Path),
		Author:  string(c.Author.Login),
		HTMLURL: c.URL.String(),
	}

	if c.Line != nil {
		line := int(*c.Line)
		m.Line = &line
	}

	if !c.CreatedAt.IsZero() {
		m.CreatedAt = c.CreatedAt.Format(time.RFC3339)
	}
	if !c.UpdatedAt.IsZero() {
		m.UpdatedAt = c.UpdatedAt.Format(time.RFC3339)
	}

	return m
}
