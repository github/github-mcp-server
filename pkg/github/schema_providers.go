package github

import (
	"encoding/json"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
)

// This file contains typed input structs that implement SchemaProvider and ResolvedSchemaProvider
// interfaces for high-traffic MCP tools. This provides maximum performance by:
// 1. Avoiding reflection for schema generation
// 2. Pre-resolving schemas to skip the resolution step entirely
//
// Each input struct provides:
// - MCPSchema() - returns the pre-computed JSON schema
// - MCPResolvedSchema() - returns the pre-resolved schema ready for validation

// schemaCache provides thread-safe lazy initialization of resolved schemas
type schemaCache struct {
	once     sync.Once
	resolved *jsonschema.Resolved
}

func (c *schemaCache) get(schema *jsonschema.Schema) *jsonschema.Resolved {
	c.once.Do(func() {
		var err error
		c.resolved, err = schema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
		if err != nil {
			// This should never happen with well-formed schemas
			panic("failed to resolve schema: " + err.Error())
		}
	})
	return c.resolved
}

// ============================================================================
// SearchRepositoriesInput - for search_repositories tool
// ============================================================================

// SearchRepositoriesInput is the typed input for the search_repositories tool.
type SearchRepositoriesInput struct {
	Query         string `json:"query"`
	Sort          string `json:"sort,omitempty"`
	Order         string `json:"order,omitempty"`
	MinimalOutput *bool  `json:"minimal_output,omitempty"`
	Page          int    `json:"page,omitempty"`
	PerPage       int    `json:"perPage,omitempty"`
}

var (
	searchRepositoriesSchema = &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				Description: "Repository search query. Examples: 'machine learning in:name stars:>1000 language:python', 'topic:react', 'user:facebook'. Supports advanced search syntax for precise filtering.",
			},
			"sort": {
				Type:        "string",
				Description: "Sort repositories by field, defaults to best match",
				Enum:        []any{"stars", "forks", "help-wanted-issues", "updated"},
			},
			"order": {
				Type:        "string",
				Description: "Sort order",
				Enum:        []any{"asc", "desc"},
			},
			"minimal_output": {
				Type:        "boolean",
				Description: "Return minimal repository information (default: true). When false, returns full GitHub API repository objects.",
				Default:     json.RawMessage(`true`),
			},
			"page": {
				Type:        "number",
				Description: "Page number for pagination (min 1)",
				Minimum:     jsonschema.Ptr(1.0),
			},
			"perPage": {
				Type:        "number",
				Description: "Results per page for pagination (min 1, max 100)",
				Minimum:     jsonschema.Ptr(1.0),
				Maximum:     jsonschema.Ptr(100.0),
			},
		},
		Required: []string{"query"},
	}
	searchRepositoriesResolvedCache schemaCache
)

// MCPSchema returns the JSON schema for SearchRepositoriesInput.
func (SearchRepositoriesInput) MCPSchema() *jsonschema.Schema {
	return searchRepositoriesSchema
}

// MCPResolvedSchema returns the pre-resolved schema for SearchRepositoriesInput.
func (SearchRepositoriesInput) MCPResolvedSchema() *jsonschema.Resolved {
	return searchRepositoriesResolvedCache.get(searchRepositoriesSchema)
}

// ============================================================================
// SearchCodeInput - for search_code tool
// ============================================================================

// SearchCodeInput is the typed input for the search_code tool.
type SearchCodeInput struct {
	Query   string `json:"query"`
	Sort    string `json:"sort,omitempty"`
	Order   string `json:"order,omitempty"`
	Page    int    `json:"page,omitempty"`
	PerPage int    `json:"perPage,omitempty"`
}

var (
	searchCodeSchema = &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				Description: "Search query using GitHub's powerful code search syntax. Examples: 'content:Skill language:Java org:github', 'NOT is:archived language:Python OR language:go', 'repo:github/github-mcp-server'. Supports exact matching, language filters, path filters, and more.",
			},
			"sort": {
				Type:        "string",
				Description: "Sort field ('indexed' only)",
			},
			"order": {
				Type:        "string",
				Description: "Sort order for results",
				Enum:        []any{"asc", "desc"},
			},
			"page": {
				Type:        "number",
				Description: "Page number for pagination (min 1)",
				Minimum:     jsonschema.Ptr(1.0),
			},
			"perPage": {
				Type:        "number",
				Description: "Results per page for pagination (min 1, max 100)",
				Minimum:     jsonschema.Ptr(1.0),
				Maximum:     jsonschema.Ptr(100.0),
			},
		},
		Required: []string{"query"},
	}
	searchCodeResolvedCache schemaCache
)

// MCPSchema returns the JSON schema for SearchCodeInput.
func (SearchCodeInput) MCPSchema() *jsonschema.Schema {
	return searchCodeSchema
}

// MCPResolvedSchema returns the pre-resolved schema for SearchCodeInput.
func (SearchCodeInput) MCPResolvedSchema() *jsonschema.Resolved {
	return searchCodeResolvedCache.get(searchCodeSchema)
}

// ============================================================================
// SearchUsersInput - for search_users tool
// ============================================================================

// SearchUsersInput is the typed input for the search_users tool.
type SearchUsersInput struct {
	Query   string `json:"query"`
	Sort    string `json:"sort,omitempty"`
	Order   string `json:"order,omitempty"`
	Page    int    `json:"page,omitempty"`
	PerPage int    `json:"perPage,omitempty"`
}

var (
	searchUsersSchema = &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				Description: "User search query. Examples: 'john smith', 'location:seattle', 'followers:>100'. Search is automatically scoped to type:user.",
			},
			"sort": {
				Type:        "string",
				Description: "Sort users by number of followers or repositories, or when the person joined GitHub.",
				Enum:        []any{"followers", "repositories", "joined"},
			},
			"order": {
				Type:        "string",
				Description: "Sort order",
				Enum:        []any{"asc", "desc"},
			},
			"page": {
				Type:        "number",
				Description: "Page number for pagination (min 1)",
				Minimum:     jsonschema.Ptr(1.0),
			},
			"perPage": {
				Type:        "number",
				Description: "Results per page for pagination (min 1, max 100)",
				Minimum:     jsonschema.Ptr(1.0),
				Maximum:     jsonschema.Ptr(100.0),
			},
		},
		Required: []string{"query"},
	}
	searchUsersResolvedCache schemaCache
)

// MCPSchema returns the JSON schema for SearchUsersInput.
func (SearchUsersInput) MCPSchema() *jsonschema.Schema {
	return searchUsersSchema
}

// MCPResolvedSchema returns the pre-resolved schema for SearchUsersInput.
func (SearchUsersInput) MCPResolvedSchema() *jsonschema.Resolved {
	return searchUsersResolvedCache.get(searchUsersSchema)
}

// ============================================================================
// GetFileContentsInput - for get_file_contents tool
// ============================================================================

// GetFileContentsInput is the typed input for the get_file_contents tool.
type GetFileContentsInput struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Path  string `json:"path,omitempty"`
	Ref   string `json:"ref,omitempty"`
	SHA   string `json:"sha,omitempty"`
}

var (
	getFileContentsSchema = &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {
				Type:        "string",
				Description: "Repository owner (username or organization)",
			},
			"repo": {
				Type:        "string",
				Description: "Repository name",
			},
			"path": {
				Type:        "string",
				Description: "Path to file/directory (directories must end with a slash '/')",
				Default:     json.RawMessage(`"/"`),
			},
			"ref": {
				Type:        "string",
				Description: "Accepts optional git refs such as `refs/tags/{tag}`, `refs/heads/{branch}` or `refs/pull/{pr_number}/head`",
			},
			"sha": {
				Type:        "string",
				Description: "Accepts optional commit SHA. If specified, it will be used instead of ref",
			},
		},
		Required: []string{"owner", "repo"},
	}
	getFileContentsResolvedCache schemaCache
)

// MCPSchema returns the JSON schema for GetFileContentsInput.
func (GetFileContentsInput) MCPSchema() *jsonschema.Schema {
	return getFileContentsSchema
}

// MCPResolvedSchema returns the pre-resolved schema for GetFileContentsInput.
func (GetFileContentsInput) MCPResolvedSchema() *jsonschema.Resolved {
	return getFileContentsResolvedCache.get(getFileContentsSchema)
}

// ============================================================================
// ListCommitsInput - for list_commits tool
// ============================================================================

// ListCommitsInput is the typed input for the list_commits tool.
type ListCommitsInput struct {
	Owner   string `json:"owner"`
	Repo    string `json:"repo"`
	SHA     string `json:"sha,omitempty"`
	Author  string `json:"author,omitempty"`
	Page    int    `json:"page,omitempty"`
	PerPage int    `json:"perPage,omitempty"`
}

var (
	listCommitsSchema = &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {
				Type:        "string",
				Description: "Repository owner",
			},
			"repo": {
				Type:        "string",
				Description: "Repository name",
			},
			"sha": {
				Type:        "string",
				Description: "Commit SHA, branch or tag name to list commits of. If not provided, uses the default branch of the repository. If a commit SHA is provided, will list commits up to that SHA.",
			},
			"author": {
				Type:        "string",
				Description: "Author username or email address to filter commits by",
			},
			"page": {
				Type:        "number",
				Description: "Page number for pagination (min 1)",
				Minimum:     jsonschema.Ptr(1.0),
			},
			"perPage": {
				Type:        "number",
				Description: "Results per page for pagination (min 1, max 100)",
				Minimum:     jsonschema.Ptr(1.0),
				Maximum:     jsonschema.Ptr(100.0),
			},
		},
		Required: []string{"owner", "repo"},
	}
	listCommitsResolvedCache schemaCache
)

// MCPSchema returns the JSON schema for ListCommitsInput.
func (ListCommitsInput) MCPSchema() *jsonschema.Schema {
	return listCommitsSchema
}

// MCPResolvedSchema returns the pre-resolved schema for ListCommitsInput.
func (ListCommitsInput) MCPResolvedSchema() *jsonschema.Resolved {
	return listCommitsResolvedCache.get(listCommitsSchema)
}

// ============================================================================
// GetCommitInput - for get_commit tool
// ============================================================================

// GetCommitInput is the typed input for the get_commit tool.
type GetCommitInput struct {
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	SHA         string `json:"sha"`
	IncludeDiff *bool  `json:"include_diff,omitempty"`
	Page        int    `json:"page,omitempty"`
	PerPage     int    `json:"perPage,omitempty"`
}

var (
	getCommitSchema = &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {
				Type:        "string",
				Description: "Repository owner",
			},
			"repo": {
				Type:        "string",
				Description: "Repository name",
			},
			"sha": {
				Type:        "string",
				Description: "Commit SHA, branch name, or tag name",
			},
			"include_diff": {
				Type:        "boolean",
				Description: "Whether to include file diffs and stats in the response. Default is true.",
				Default:     json.RawMessage(`true`),
			},
			"page": {
				Type:        "number",
				Description: "Page number for pagination (min 1)",
				Minimum:     jsonschema.Ptr(1.0),
			},
			"perPage": {
				Type:        "number",
				Description: "Results per page for pagination (min 1, max 100)",
				Minimum:     jsonschema.Ptr(1.0),
				Maximum:     jsonschema.Ptr(100.0),
			},
		},
		Required: []string{"owner", "repo", "sha"},
	}
	getCommitResolvedCache schemaCache
)

// MCPSchema returns the JSON schema for GetCommitInput.
func (GetCommitInput) MCPSchema() *jsonschema.Schema {
	return getCommitSchema
}

// MCPResolvedSchema returns the pre-resolved schema for GetCommitInput.
func (GetCommitInput) MCPResolvedSchema() *jsonschema.Resolved {
	return getCommitResolvedCache.get(getCommitSchema)
}

// ============================================================================
// ListIssuesInput - for list_issues tool
// ============================================================================

// ListIssuesInput is the typed input for the list_issues tool.
type ListIssuesInput struct {
	Owner     string   `json:"owner"`
	Repo      string   `json:"repo"`
	State     string   `json:"state,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	OrderBy   string   `json:"orderBy,omitempty"`
	Direction string   `json:"direction,omitempty"`
	Since     string   `json:"since,omitempty"`
	PerPage   int      `json:"perPage,omitempty"`
	After     string   `json:"after,omitempty"`
}

var (
	listIssuesSchema = &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {
				Type:        "string",
				Description: "Repository owner",
			},
			"repo": {
				Type:        "string",
				Description: "Repository name",
			},
			"state": {
				Type:        "string",
				Description: "Filter by state, by default both open and closed issues are returned when not provided",
				Enum:        []any{"OPEN", "CLOSED"},
			},
			"labels": {
				Type:        "array",
				Description: "Filter by labels",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
			"orderBy": {
				Type:        "string",
				Description: "Order issues by field. If provided, the 'direction' also needs to be provided.",
				Enum:        []any{"CREATED_AT", "UPDATED_AT", "COMMENTS"},
			},
			"direction": {
				Type:        "string",
				Description: "Order direction. If provided, the 'orderBy' also needs to be provided.",
				Enum:        []any{"ASC", "DESC"},
			},
			"since": {
				Type:        "string",
				Description: "Filter by date (ISO 8601 timestamp)",
			},
			"perPage": {
				Type:        "number",
				Description: "Results per page for pagination (min 1, max 100)",
				Minimum:     jsonschema.Ptr(1.0),
				Maximum:     jsonschema.Ptr(100.0),
			},
			"after": {
				Type:        "string",
				Description: "Cursor for pagination. Use the endCursor from the previous page's PageInfo for GraphQL APIs.",
			},
		},
		Required: []string{"owner", "repo"},
	}
	listIssuesResolvedCache schemaCache
)

// MCPSchema returns the JSON schema for ListIssuesInput.
func (ListIssuesInput) MCPSchema() *jsonschema.Schema {
	return listIssuesSchema
}

// MCPResolvedSchema returns the pre-resolved schema for ListIssuesInput.
func (ListIssuesInput) MCPResolvedSchema() *jsonschema.Resolved {
	return listIssuesResolvedCache.get(listIssuesSchema)
}

// ============================================================================
// IssueReadInput - for issue_read tool
// ============================================================================

// IssueReadInput is the typed input for the issue_read tool.
type IssueReadInput struct {
	Method      string `json:"method"`
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	IssueNumber int    `json:"issue_number"`
	Page        int    `json:"page,omitempty"`
	PerPage     int    `json:"perPage,omitempty"`
}

var (
	issueReadSchema = &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"method": {
				Type: "string",
				Description: `The read operation to perform on a single issue.
Options are:
1. get - Get details of a specific issue.
2. get_comments - Get issue comments.
3. get_sub_issues - Get sub-issues of the issue.
4. get_labels - Get labels assigned to the issue.
`,
				Enum: []any{"get", "get_comments", "get_sub_issues", "get_labels"},
			},
			"owner": {
				Type:        "string",
				Description: "The owner of the repository",
			},
			"repo": {
				Type:        "string",
				Description: "The name of the repository",
			},
			"issue_number": {
				Type:        "number",
				Description: "The number of the issue",
			},
			"page": {
				Type:        "number",
				Description: "Page number for pagination (min 1)",
				Minimum:     jsonschema.Ptr(1.0),
			},
			"perPage": {
				Type:        "number",
				Description: "Results per page for pagination (min 1, max 100)",
				Minimum:     jsonschema.Ptr(1.0),
				Maximum:     jsonschema.Ptr(100.0),
			},
		},
		Required: []string{"method", "owner", "repo", "issue_number"},
	}
	issueReadResolvedCache schemaCache
)

// MCPSchema returns the JSON schema for IssueReadInput.
func (IssueReadInput) MCPSchema() *jsonschema.Schema {
	return issueReadSchema
}

// MCPResolvedSchema returns the pre-resolved schema for IssueReadInput.
func (IssueReadInput) MCPResolvedSchema() *jsonschema.Resolved {
	return issueReadResolvedCache.get(issueReadSchema)
}

// ============================================================================
// ListPullRequestsInput - for list_pull_requests tool
// ============================================================================

// ListPullRequestsInput is the typed input for the list_pull_requests tool.
type ListPullRequestsInput struct {
	Owner     string `json:"owner"`
	Repo      string `json:"repo"`
	State     string `json:"state,omitempty"`
	Head      string `json:"head,omitempty"`
	Base      string `json:"base,omitempty"`
	Sort      string `json:"sort,omitempty"`
	Direction string `json:"direction,omitempty"`
	Page      int    `json:"page,omitempty"`
	PerPage   int    `json:"perPage,omitempty"`
}

var (
	listPullRequestsSchema = &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {
				Type:        "string",
				Description: "Repository owner",
			},
			"repo": {
				Type:        "string",
				Description: "Repository name",
			},
			"state": {
				Type:        "string",
				Description: "Filter by state",
				Enum:        []any{"open", "closed", "all"},
			},
			"head": {
				Type:        "string",
				Description: "Filter by head user/org and branch",
			},
			"base": {
				Type:        "string",
				Description: "Filter by base branch",
			},
			"sort": {
				Type:        "string",
				Description: "Sort by",
				Enum:        []any{"created", "updated", "popularity", "long-running"},
			},
			"direction": {
				Type:        "string",
				Description: "Sort direction",
				Enum:        []any{"asc", "desc"},
			},
			"page": {
				Type:        "number",
				Description: "Page number for pagination (min 1)",
				Minimum:     jsonschema.Ptr(1.0),
			},
			"perPage": {
				Type:        "number",
				Description: "Results per page for pagination (min 1, max 100)",
				Minimum:     jsonschema.Ptr(1.0),
				Maximum:     jsonschema.Ptr(100.0),
			},
		},
		Required: []string{"owner", "repo"},
	}
	listPullRequestsResolvedCache schemaCache
)

// MCPSchema returns the JSON schema for ListPullRequestsInput.
func (ListPullRequestsInput) MCPSchema() *jsonschema.Schema {
	return listPullRequestsSchema
}

// MCPResolvedSchema returns the pre-resolved schema for ListPullRequestsInput.
func (ListPullRequestsInput) MCPResolvedSchema() *jsonschema.Resolved {
	return listPullRequestsResolvedCache.get(listPullRequestsSchema)
}

// ============================================================================
// PullRequestReadInput - for pull_request_read tool
// ============================================================================

// PullRequestReadInput is the typed input for the pull_request_read tool.
type PullRequestReadInput struct {
	Method     string `json:"method"`
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	PullNumber int    `json:"pullNumber"`
	Page       int    `json:"page,omitempty"`
	PerPage    int    `json:"perPage,omitempty"`
}

var (
	pullRequestReadSchema = &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"method": {
				Type: "string",
				Description: `Action to specify what pull request data needs to be retrieved from GitHub. 
Possible options: 
 1. get - Get details of a specific pull request.
 2. get_diff - Get the diff of a pull request.
 3. get_status - Get status of a head commit in a pull request. This reflects status of builds and checks.
 4. get_files - Get the list of files changed in a pull request. Use with pagination parameters to control the number of results returned.
 5. get_review_comments - Get the review comments on a pull request. They are comments made on a portion of the unified diff during a pull request review. Use with pagination parameters to control the number of results returned.
 6. get_reviews - Get the reviews on a pull request. When asked for review comments, use get_review_comments method.
 7. get_comments - Get comments on a pull request. Use this if user doesn't specifically want review comments. Use with pagination parameters to control the number of results returned.
`,
				Enum: []any{"get", "get_diff", "get_status", "get_files", "get_review_comments", "get_reviews", "get_comments"},
			},
			"owner": {
				Type:        "string",
				Description: "Repository owner",
			},
			"repo": {
				Type:        "string",
				Description: "Repository name",
			},
			"pullNumber": {
				Type:        "number",
				Description: "Pull request number",
			},
			"page": {
				Type:        "number",
				Description: "Page number for pagination (min 1)",
				Minimum:     jsonschema.Ptr(1.0),
			},
			"perPage": {
				Type:        "number",
				Description: "Results per page for pagination (min 1, max 100)",
				Minimum:     jsonschema.Ptr(1.0),
				Maximum:     jsonschema.Ptr(100.0),
			},
		},
		Required: []string{"method", "owner", "repo", "pullNumber"},
	}
	pullRequestReadResolvedCache schemaCache
)

// MCPSchema returns the JSON schema for PullRequestReadInput.
func (PullRequestReadInput) MCPSchema() *jsonschema.Schema {
	return pullRequestReadSchema
}

// MCPResolvedSchema returns the pre-resolved schema for PullRequestReadInput.
func (PullRequestReadInput) MCPResolvedSchema() *jsonschema.Resolved {
	return pullRequestReadResolvedCache.get(pullRequestReadSchema)
}
