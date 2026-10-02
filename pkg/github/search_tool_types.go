package github

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"

	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
)

type SearchAccountsInput struct {
	Query   string `json:"query"`
	Sort    string `json:"sort,omitempty"`
	Order   string `json:"order,omitempty"`
	Page    *int   `json:"page,omitempty"`
	PerPage *int   `json:"perPage,omitempty"`
}

type SearchRepositoriesInput struct {
	SearchAccountsInput
	MinimalOutput *bool `json:"minimal_output,omitempty"`
}

type SearchCommitsInput = SearchAccountsInput

type SearchRepositoriesOutput struct {
	Minimal *MinimalSearchRepositoriesResult
	Full    *FullSearchRepositoriesResult
}

func (out SearchRepositoriesOutput) MarshalJSON() ([]byte, error) {
	if out.Minimal != nil {
		return json.Marshal(out.Minimal)
	}
	return json.Marshal(out.Full)
}

func searchRepositoriesOutputSchema() *jsonschema.Schema {
	minimal := repositoryOutputSchema[MinimalSearchRepositoriesResult]()
	options := &jsonschema.ForOptions{TypeSchemas: map[reflect.Type]*jsonschema.Schema{
		reflect.TypeFor[github.Timestamp]():    {Type: "string", Format: "date-time"},
		reflect.TypeFor[github.Team]():         {Type: "object", Ref: "#/$defs/team"},
		reflect.TypeFor[SearchRepository]():    {Type: "object", Ref: "#/$defs/searchRepository"},
		reflect.TypeFor[SearchPropertyValue](): {Ref: "#/$defs/searchPropertyValue"},
	}}
	full, err := jsonschema.For[FullSearchRepositoriesResult](options)
	if err != nil {
		panic(err)
	}
	type repositorySchema SearchRepository
	repository, err := jsonschema.For[repositorySchema](options)
	if err != nil {
		panic(err)
	}
	repository.Type = ""
	repository.Types = []string{"object", "null"}
	full.Defs = map[string]*jsonschema.Schema{
		"searchRepository": repository,
		"team":             repositoryOutputSchema[github.Team]().Defs["team"],
		"searchPropertyValue": {AnyOf: []*jsonschema.Schema{
			{Type: "null"}, {Type: "string"}, {Type: "number"}, {Type: "boolean"},
			{Type: "array", Items: &jsonschema.Schema{Ref: "#/$defs/searchPropertyValue"}},
			{Type: "object", AdditionalProperties: &jsonschema.Schema{Ref: "#/$defs/searchPropertyValue"}},
		}},
	}
	union := repositoryUnionSchema(&jsonschema.Schema{Type: "null"}, minimal, full)
	// Empty items can satisfy both variants. AnyOf preserves that overlap.
	union.AnyOf, union.OneOf = union.OneOf, nil
	return union
}

type FullSearchRepositoriesResult struct {
	Total             *int                `json:"total_count,omitempty"`
	IncompleteResults *bool               `json:"incomplete_results,omitempty"`
	Repositories      []*SearchRepository `json:"items,omitempty"`
}

// SearchRepository mirrors the full search response without the upstream
// untyped custom_properties map. Parent/source/template repositories retain
// the same recursive wire contract.
type SearchRepository struct {
	ID                        *int64                         `json:"id,omitempty"`
	NodeID                    *string                        `json:"node_id,omitempty"`
	Owner                     *github.User                   `json:"owner,omitempty"`
	Name                      *string                        `json:"name,omitempty"`
	FullName                  *string                        `json:"full_name,omitempty"`
	Description               *string                        `json:"description,omitempty"`
	Homepage                  *string                        `json:"homepage,omitempty"`
	CodeOfConduct             *github.CodeOfConduct          `json:"code_of_conduct,omitempty"`
	DefaultBranch             *string                        `json:"default_branch,omitempty"`
	MasterBranch              *string                        `json:"master_branch,omitempty"`
	CreatedAt                 *github.Timestamp              `json:"created_at,omitempty"`
	PushedAt                  *github.Timestamp              `json:"pushed_at,omitempty"`
	UpdatedAt                 *github.Timestamp              `json:"updated_at,omitempty"`
	HTMLURL                   *string                        `json:"html_url,omitempty"`
	CloneURL                  *string                        `json:"clone_url,omitempty"`
	GitURL                    *string                        `json:"git_url,omitempty"`
	MirrorURL                 *string                        `json:"mirror_url,omitempty"`
	SSHURL                    *string                        `json:"ssh_url,omitempty"`
	SVNURL                    *string                        `json:"svn_url,omitempty"`
	Language                  *string                        `json:"language,omitempty"`
	Fork                      *bool                          `json:"fork,omitempty"`
	ForksCount                *int                           `json:"forks_count,omitempty"`
	NetworkCount              *int                           `json:"network_count,omitempty"`
	OpenIssuesCount           *int                           `json:"open_issues_count,omitempty"`
	OpenIssues                *int                           `json:"open_issues,omitempty"`
	StargazersCount           *int                           `json:"stargazers_count,omitempty"`
	SubscribersCount          *int                           `json:"subscribers_count,omitempty"`
	WatchersCount             *int                           `json:"watchers_count,omitempty"`
	Watchers                  *int                           `json:"watchers,omitempty"`
	Size                      *int                           `json:"size,omitempty"`
	AutoInit                  *bool                          `json:"auto_init,omitempty"`
	Parent                    *SearchRepository              `json:"parent,omitempty"`
	Source                    *SearchRepository              `json:"source,omitempty"`
	TemplateRepository        *SearchRepository              `json:"template_repository,omitempty"`
	Organization              *github.Organization           `json:"organization,omitempty"`
	Permissions               *github.RepositoryPermissions  `json:"permissions,omitempty"`
	AllowRebaseMerge          *bool                          `json:"allow_rebase_merge,omitempty"`
	AllowUpdateBranch         *bool                          `json:"allow_update_branch,omitempty"`
	AllowSquashMerge          *bool                          `json:"allow_squash_merge,omitempty"`
	AllowMergeCommit          *bool                          `json:"allow_merge_commit,omitempty"`
	AllowAutoMerge            *bool                          `json:"allow_auto_merge,omitempty"`
	AllowForking              *bool                          `json:"allow_forking,omitempty"`
	WebCommitSignoffRequired  *bool                          `json:"web_commit_signoff_required,omitempty"`
	DeleteBranchOnMerge       *bool                          `json:"delete_branch_on_merge,omitempty"`
	UseSquashPRTitleAsDefault *bool                          `json:"use_squash_pr_title_as_default,omitempty"`
	SquashMergeCommitTitle    *string                        `json:"squash_merge_commit_title,omitempty"`
	SquashMergeCommitMessage  *string                        `json:"squash_merge_commit_message,omitempty"`
	MergeCommitTitle          *string                        `json:"merge_commit_title,omitempty"`
	MergeCommitMessage        *string                        `json:"merge_commit_message,omitempty"`
	Topics                    []string                       `json:"topics,omitempty"`
	CustomProperties          map[string]SearchPropertyValue `json:"custom_properties,omitempty"`
	Archived                  *bool                          `json:"archived,omitempty"`
	Disabled                  *bool                          `json:"disabled,omitempty"`
	License                   *github.License                `json:"license,omitempty"`
	Private                   *bool                          `json:"private,omitempty"`
	HasIssues                 *bool                          `json:"has_issues,omitempty"`
	HasWiki                   *bool                          `json:"has_wiki,omitempty"`
	HasPages                  *bool                          `json:"has_pages,omitempty"`
	HasProjects               *bool                          `json:"has_projects,omitempty"`
	HasDownloads              *bool                          `json:"has_downloads,omitempty"`
	HasDiscussions            *bool                          `json:"has_discussions,omitempty"`
	HasPullRequests           *bool                          `json:"has_pull_requests,omitempty"`
	PullRequestCreationPolicy *string                        `json:"pull_request_creation_policy,omitempty"`
	IsTemplate                *bool                          `json:"is_template,omitempty"`
	LicenseTemplate           *string                        `json:"license_template,omitempty"`
	GitignoreTemplate         *string                        `json:"gitignore_template,omitempty"`
	SecurityAndAnalysis       *github.SecurityAndAnalysis    `json:"security_and_analysis,omitempty"`
	TeamID                    *int64                         `json:"team_id,omitempty"`
	URL                       *string                        `json:"url,omitempty"`
	ArchiveURL                *string                        `json:"archive_url,omitempty"`
	AssigneesURL              *string                        `json:"assignees_url,omitempty"`
	BlobsURL                  *string                        `json:"blobs_url,omitempty"`
	BranchesURL               *string                        `json:"branches_url,omitempty"`
	CollaboratorsURL          *string                        `json:"collaborators_url,omitempty"`
	CommentsURL               *string                        `json:"comments_url,omitempty"`
	CommitsURL                *string                        `json:"commits_url,omitempty"`
	CompareURL                *string                        `json:"compare_url,omitempty"`
	ContentsURL               *string                        `json:"contents_url,omitempty"`
	ContributorsURL           *string                        `json:"contributors_url,omitempty"`
	DeploymentsURL            *string                        `json:"deployments_url,omitempty"`
	DownloadsURL              *string                        `json:"downloads_url,omitempty"`
	EventsURL                 *string                        `json:"events_url,omitempty"`
	ForksURL                  *string                        `json:"forks_url,omitempty"`
	GitCommitsURL             *string                        `json:"git_commits_url,omitempty"`
	GitRefsURL                *string                        `json:"git_refs_url,omitempty"`
	GitTagsURL                *string                        `json:"git_tags_url,omitempty"`
	HooksURL                  *string                        `json:"hooks_url,omitempty"`
	IssueCommentURL           *string                        `json:"issue_comment_url,omitempty"`
	IssueEventsURL            *string                        `json:"issue_events_url,omitempty"`
	IssuesURL                 *string                        `json:"issues_url,omitempty"`
	KeysURL                   *string                        `json:"keys_url,omitempty"`
	LabelsURL                 *string                        `json:"labels_url,omitempty"`
	LanguagesURL              *string                        `json:"languages_url,omitempty"`
	MergesURL                 *string                        `json:"merges_url,omitempty"`
	MilestonesURL             *string                        `json:"milestones_url,omitempty"`
	NotificationsURL          *string                        `json:"notifications_url,omitempty"`
	PullsURL                  *string                        `json:"pulls_url,omitempty"`
	ReleasesURL               *string                        `json:"releases_url,omitempty"`
	StargazersURL             *string                        `json:"stargazers_url,omitempty"`
	StatusesURL               *string                        `json:"statuses_url,omitempty"`
	SubscribersURL            *string                        `json:"subscribers_url,omitempty"`
	SubscriptionURL           *string                        `json:"subscription_url,omitempty"`
	TagsURL                   *string                        `json:"tags_url,omitempty"`
	TreesURL                  *string                        `json:"trees_url,omitempty"`
	TeamsURL                  *string                        `json:"teams_url,omitempty"`
	TextMatches               []*github.TextMatch            `json:"text_matches,omitempty"`
	Visibility                *string                        `json:"visibility,omitempty"`
	RoleName                  *string                        `json:"role_name,omitempty"`
}

// Custom property values are JSON unions, not arbitrary Go values.
type SearchPropertyValue struct {
	String *string
	Number *json.Number
	Bool   *bool
	Array  []SearchPropertyValue
	Object map[string]SearchPropertyValue
}

func (value SearchPropertyValue) MarshalJSON() ([]byte, error) {
	switch {
	case value.String != nil:
		return json.Marshal(value.String)
	case value.Number != nil:
		return json.Marshal(value.Number)
	case value.Bool != nil:
		return json.Marshal(value.Bool)
	case value.Array != nil:
		return json.Marshal(value.Array)
	case value.Object != nil:
		return json.Marshal(value.Object)
	default:
		return []byte("null"), nil
	}
}

func (value *SearchPropertyValue) UnmarshalJSON(raw []byte) error {
	*value = SearchPropertyValue{}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return io.ErrUnexpectedEOF
	}
	switch raw[0] {
	case '"':
		return json.Unmarshal(raw, &value.String)
	case '[':
		return json.Unmarshal(raw, &value.Array)
	case '{':
		return json.Unmarshal(raw, &value.Object)
	case 't', 'f':
		return json.Unmarshal(raw, &value.Bool)
	case 'n':
		return json.Unmarshal(raw, new(*string))
	default:
		return json.Unmarshal(raw, &value.Number)
	}
}

func normalizeSearchArguments(minimalOutput bool) func(json.RawMessage) (json.RawMessage, error) {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var args map[string]any
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		if _, err := RequiredParam[string](args, "query"); err != nil {
			return nil, err
		}
		for _, field := range []string{"sort", "order"} {
			if _, err := OptionalParam[string](args, field); err != nil {
				return nil, err
			}
		}
		if _, err := OptionalPaginationParams(args); err != nil {
			return nil, err
		}
		if minimalOutput {
			if _, err := OptionalBoolParamWithDefault(args, "minimal_output", true); err != nil {
				return nil, err
			}
		}
		return normalizeRepositoryArguments(raw)
	}
}
