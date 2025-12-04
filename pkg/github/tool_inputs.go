package github

// Tool input types for MCP tools. These structs use json and jsonschema tags
// to define the input schema for each tool. The mcpgen code generator creates
// SchemaProvider implementations for these types, enabling zero-reflection
// schema generation at runtime.
//
// To regenerate after modifying input types, run:
//   go generate ./pkg/github/...

//go:generate mcpgen -type=SearchRepositoriesInput,SearchCodeInput,SearchUsersInput,GetFileContentsInput,CreateIssueInput,CreatePullRequestInput

// SearchRepositoriesInput defines input parameters for the search_repositories tool.
type SearchRepositoriesInput struct {
	Query         string `json:"query" jsonschema:"required,description=Repository search query using GitHub search syntax"`
	Sort          string `json:"sort" jsonschema:"description=Sort repositories by field - defaults to best match,enum=stars|forks|help-wanted-issues|updated"`
	Order         string `json:"order" jsonschema:"description=Sort order (asc/desc),enum=asc|desc"`
	MinimalOutput bool   `json:"minimal_output" jsonschema:"description=Return minimal repository information - when false returns full GitHub API objects"`
	Page          int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage       int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// SearchCodeInput defines input parameters for the search_code tool.
type SearchCodeInput struct {
	Query   string `json:"query" jsonschema:"required,description=Search query using GitHub code search syntax"`
	Sort    string `json:"sort" jsonschema:"description=Sort field (indexed only)"`
	Order   string `json:"order" jsonschema:"description=Sort order for results (asc/desc),enum=asc|desc"`
	Page    int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// SearchUsersInput defines input parameters for the search_users tool.
type SearchUsersInput struct {
	Query   string `json:"query" jsonschema:"required,description=User search query using GitHub search syntax"`
	Sort    string `json:"sort" jsonschema:"description=Sort users by followers/repositories/joined,enum=followers|repositories|joined"`
	Order   string `json:"order" jsonschema:"description=Sort order (asc/desc),enum=asc|desc"`
	Page    int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// GetFileContentsInput defines input parameters for the get_file_contents tool.
type GetFileContentsInput struct {
	Owner string `json:"owner" jsonschema:"required,description=Repository owner (username or organization)"`
	Repo  string `json:"repo" jsonschema:"required,description=Repository name"`
	Path  string `json:"path" jsonschema:"description=Path to file or directory"`
	Ref   string `json:"ref" jsonschema:"description=Git ref such as branch name or tag name or commit SHA"`
	SHA   string `json:"sha" jsonschema:"description=Commit SHA (overrides ref if specified)"`
}

// CreateIssueInput defines input parameters for creating a new issue.
type CreateIssueInput struct {
	Owner     string   `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo      string   `json:"repo" jsonschema:"required,description=Repository name"`
	Title     string   `json:"title" jsonschema:"required,description=Issue title"`
	Body      string   `json:"body" jsonschema:"description=Issue body content"`
	Assignees []string `json:"assignees" jsonschema:"description=Usernames to assign to this issue"`
	Labels    []string `json:"labels" jsonschema:"description=Labels to add to this issue"`
	Milestone int      `json:"milestone" jsonschema:"description=Milestone number to assign"`
}

// CreatePullRequestInput defines input parameters for creating a pull request.
type CreatePullRequestInput struct {
	Owner               string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo                string `json:"repo" jsonschema:"required,description=Repository name"`
	Title               string `json:"title" jsonschema:"required,description=Pull request title"`
	Body                string `json:"body" jsonschema:"description=Pull request body/description"`
	Head                string `json:"head" jsonschema:"required,description=Branch where your changes are implemented"`
	Base                string `json:"base" jsonschema:"required,description=Branch you want the changes pulled into"`
	Draft               bool   `json:"draft" jsonschema:"description=Create as a draft pull request"`
	MaintainerCanModify bool   `json:"maintainer_can_modify" jsonschema:"description=Whether maintainers can modify the pull request"`
}
