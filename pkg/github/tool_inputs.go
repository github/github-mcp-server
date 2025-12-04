package github

// Tool input types for MCP tools. These structs use json and jsonschema tags
// to define the input schema for each tool. The mcpgen code generator creates
// SchemaProvider implementations for these types, enabling zero-reflection
// schema generation at runtime.
//
// To regenerate after modifying input types, run:
//   go generate ./pkg/github/...

//go:generate mcpgen -type=SearchRepositoriesInput,SearchCodeInput,SearchUsersInput,SearchIssuesInput,SearchPullRequestsInput,GetFileContentsInput,CreateIssueInput,CreatePullRequestInput,ListCommitsInput,GetCommitInput,ListBranchesInput,ListTagsInput,ListReleasesInput,GetReleaseByTagInput,ListWorkflowsInput,ListWorkflowRunsInput,GetWorkflowRunInput,ListNotificationsInput,GetRepositoryTreeInput,CreateBranchInput,CreateOrUpdateFileInput,PushFilesInput,ForkRepositoryInput,CreateRepositoryInput

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

// SearchIssuesInput defines input parameters for the search_issues tool.
type SearchIssuesInput struct {
	Query   string `json:"query" jsonschema:"required,description=Search query using GitHub issues search syntax"`
	Owner   string `json:"owner" jsonschema:"description=Repository owner to scope the search"`
	Repo    string `json:"repo" jsonschema:"description=Repository name to scope the search"`
	Sort    string `json:"sort" jsonschema:"description=Sort by field,enum=comments|reactions|reactions-+1|reactions--1|reactions-smile|reactions-thinking_face|reactions-heart|reactions-tada|interactions|created|updated"`
	Order   string `json:"order" jsonschema:"description=Sort order (asc/desc),enum=asc|desc"`
	Page    int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// SearchPullRequestsInput defines input parameters for the search_pull_requests tool.
type SearchPullRequestsInput struct {
	Query   string `json:"query" jsonschema:"required,description=Search query using GitHub pull requests search syntax"`
	Owner   string `json:"owner" jsonschema:"description=Repository owner to scope the search"`
	Repo    string `json:"repo" jsonschema:"description=Repository name to scope the search"`
	Sort    string `json:"sort" jsonschema:"description=Sort by field,enum=comments|reactions|reactions-+1|reactions--1|reactions-smile|reactions-thinking_face|reactions-heart|reactions-tada|interactions|created|updated"`
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

// ListCommitsInput defines input parameters for listing commits.
type ListCommitsInput struct {
	Owner   string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo    string `json:"repo" jsonschema:"required,description=Repository name"`
	SHA     string `json:"sha" jsonschema:"description=SHA or branch to start listing commits from"`
	Author  string `json:"author" jsonschema:"description=Filter commits by author username or email"`
	Page    int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// GetCommitInput defines input parameters for getting a commit.
type GetCommitInput struct {
	Owner       string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo        string `json:"repo" jsonschema:"required,description=Repository name"`
	SHA         string `json:"sha" jsonschema:"required,description=Commit SHA or branch name or tag name"`
	IncludeDiff bool   `json:"include_diff" jsonschema:"description=Whether to include file diffs and stats in the response"`
	Page        int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage     int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// ListBranchesInput defines input parameters for listing branches.
type ListBranchesInput struct {
	Owner   string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo    string `json:"repo" jsonschema:"required,description=Repository name"`
	Page    int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// ListTagsInput defines input parameters for listing tags.
type ListTagsInput struct {
	Owner   string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo    string `json:"repo" jsonschema:"required,description=Repository name"`
	Page    int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// ListReleasesInput defines input parameters for listing releases.
type ListReleasesInput struct {
	Owner   string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo    string `json:"repo" jsonschema:"required,description=Repository name"`
	Page    int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// GetReleaseByTagInput defines input parameters for getting a release by tag.
type GetReleaseByTagInput struct {
	Owner string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo  string `json:"repo" jsonschema:"required,description=Repository name"`
	Tag   string `json:"tag" jsonschema:"required,description=Tag name (e.g. 'v1.0.0')"`
}

// ListWorkflowsInput defines input parameters for listing workflows.
type ListWorkflowsInput struct {
	Owner   string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo    string `json:"repo" jsonschema:"required,description=Repository name"`
	Page    int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// ListWorkflowRunsInput defines input parameters for listing workflow runs.
type ListWorkflowRunsInput struct {
	Owner      string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo       string `json:"repo" jsonschema:"required,description=Repository name"`
	WorkflowID string `json:"workflow_id" jsonschema:"required,description=The workflow ID or workflow file name"`
	Branch     string `json:"branch" jsonschema:"description=Filter by branch name"`
	Actor      string `json:"actor" jsonschema:"description=Filter by actor (user who triggered the workflow)"`
	Status     string `json:"status" jsonschema:"description=Filter by status,enum=queued|in_progress|completed|requested|waiting"`
	Event      string `json:"event" jsonschema:"description=Filter by event type,enum=push|pull_request|workflow_dispatch|schedule|release"`
	Page       int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage    int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// GetWorkflowRunInput defines input parameters for getting a workflow run.
type GetWorkflowRunInput struct {
	Owner string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo  string `json:"repo" jsonschema:"required,description=Repository name"`
	RunID int    `json:"run_id" jsonschema:"required,description=The unique identifier of the workflow run"`
}

// ListNotificationsInput defines input parameters for listing notifications.
type ListNotificationsInput struct {
	Filter  string `json:"filter" jsonschema:"description=Filter notifications,enum=default|include_read_notifications|only_participating"`
	Since   string `json:"since" jsonschema:"description=Only show notifications updated after the given time (ISO 8601 format)"`
	Before  string `json:"before" jsonschema:"description=Only show notifications updated before the given time (ISO 8601 format)"`
	Owner   string `json:"owner" jsonschema:"description=Repository owner to filter notifications"`
	Repo    string `json:"repo" jsonschema:"description=Repository name to filter notifications"`
	Page    int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// GetRepositoryTreeInput defines input parameters for getting repository tree.
type GetRepositoryTreeInput struct {
	Owner      string `json:"owner" jsonschema:"required,description=Repository owner (username or organization)"`
	Repo       string `json:"repo" jsonschema:"required,description=Repository name"`
	TreeSHA    string `json:"tree_sha" jsonschema:"description=The SHA1 value or ref (branch or tag) name of the tree"`
	Recursive  bool   `json:"recursive" jsonschema:"description=Recursively fetch the tree"`
	PathFilter string `json:"path_filter" jsonschema:"description=Optional path prefix to filter the tree results"`
}

// CreateBranchInput defines input parameters for creating a branch.
type CreateBranchInput struct {
	Owner   string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo    string `json:"repo" jsonschema:"required,description=Repository name"`
	Branch  string `json:"branch" jsonschema:"required,description=Name for the new branch"`
	FromRef string `json:"from_ref" jsonschema:"description=The ref to create the branch from (defaults to the default branch)"`
}

// CreateOrUpdateFileInput defines input parameters for creating or updating a file.
type CreateOrUpdateFileInput struct {
	Owner   string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo    string `json:"repo" jsonschema:"required,description=Repository name"`
	Path    string `json:"path" jsonschema:"required,description=Path where to create/update the file"`
	Content string `json:"content" jsonschema:"required,description=Content of the file"`
	Message string `json:"message" jsonschema:"required,description=Commit message"`
	Branch  string `json:"branch" jsonschema:"required,description=Branch to create/update the file in"`
	SHA     string `json:"sha" jsonschema:"description=SHA of the file being replaced (required for updates)"`
}

// PushFilesInput defines input parameters for pushing multiple files.
type PushFilesInput struct {
	Owner   string          `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo    string          `json:"repo" jsonschema:"required,description=Repository name"`
	Branch  string          `json:"branch" jsonschema:"required,description=Branch to push to"`
	Message string          `json:"message" jsonschema:"required,description=Commit message"`
	Files   []FileOperation `json:"files" jsonschema:"required,description=List of file operations to perform"`
}

// FileOperation represents a file operation for push_files.
type FileOperation struct {
	Path    string `json:"path" jsonschema:"required,description=Path to the file"`
	Content string `json:"content" jsonschema:"description=Content of the file (for create/update)"`
}

// ForkRepositoryInput defines input parameters for forking a repository.
type ForkRepositoryInput struct {
	Owner        string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo         string `json:"repo" jsonschema:"required,description=Repository name"`
	Organization string `json:"organization" jsonschema:"description=Organization to fork to (defaults to authenticated user)"`
}

// CreateRepositoryInput defines input parameters for creating a repository.
type CreateRepositoryInput struct {
	Name        string `json:"name" jsonschema:"required,description=Repository name"`
	Description string `json:"description" jsonschema:"description=Repository description"`
	Private     bool   `json:"private" jsonschema:"description=Whether the repository is private"`
	AutoInit    bool   `json:"auto_init" jsonschema:"description=Initialize with a README"`
}
