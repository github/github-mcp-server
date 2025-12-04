package github

// Tool input types for MCP tools. These structs use json and jsonschema tags
// to define the input schema for each tool. The mcpgen code generator creates
// SchemaProvider implementations for these types, enabling zero-reflection
// schema generation at runtime.
//
// To regenerate after modifying input types, run:
//   go generate ./pkg/github/...

//go:generate mcpgen -type=SearchRepositoriesInput,SearchCodeInput,SearchUsersInput,SearchIssuesInput,SearchPullRequestsInput,GetFileContentsInput,CreateIssueInput,CreatePullRequestInput,ListCommitsInput,GetCommitInput,ListBranchesInput,ListTagsInput,ListReleasesInput,GetReleaseByTagInput,ListWorkflowsInput,ListWorkflowRunsInput,GetWorkflowRunInput,ListNotificationsInput,GetRepositoryTreeInput,CreateBranchInput,CreateOrUpdateFileInput,PushFilesInput,ForkRepositoryInput,CreateRepositoryInput,GetMeInput,ListGistsInput,GetGistInput,CreateGistInput,UpdateGistInput,ListLabelsInput,GetLabelInput,ListDiscussionsInput,GetDiscussionInput,GetDiscussionCommentsInput,ListDiscussionCategoriesInput,ListProjectsInput,GetProjectInput,ListProjectItemsInput,GetProjectItemInput,AddProjectItemInput,UpdateProjectItemInput,DeleteProjectItemInput,ListPullRequestsInput,PullRequestReadInput,UpdatePullRequestInput,MergePullRequestInput,UpdatePullRequestBranchInput,RequestCopilotReviewInput,IssueReadInput,IssueWriteInput,ListIssuesInput,ListIssueTypesInput,AddIssueCommentInput,StarRepositoryInput,UnstarRepositoryInput,ListStarredRepositoriesInput,ListCodeScanningAlertsInput,GetCodeScanningAlertInput,ListSecretScanningAlertsInput,GetSecretScanningAlertInput,ListDependabotAlertsInput,GetDependabotAlertInput,GetLatestReleaseInput,GetTagInput,ListWorkflowJobsInput,GetJobLogsInput,GetWorkflowRunLogsInput,GetWorkflowRunUsageInput,ListWorkflowRunArtifactsInput,DownloadWorkflowRunArtifactInput,CancelWorkflowRunInput,RerunWorkflowRunInput,RerunFailedJobsInput,DeleteWorkflowRunLogsInput,RunWorkflowInput,GetNotificationDetailsInput,ManageNotificationSubscriptionInput,ManageRepositoryNotificationSubscriptionInput,MarkAllNotificationsReadInput,DismissNotificationInput,GetTeamsInput,GetTeamMembersInput,DeleteFileInput

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

// GetMeInput defines input parameters for getting authenticated user info.
type GetMeInput struct{}

// ListGistsInput defines input parameters for listing gists.
type ListGistsInput struct {
	Username string `json:"username" jsonschema:"description=GitHub username (omit for authenticated user's gists)"`
	Since    string `json:"since" jsonschema:"description=Only gists updated after this time (ISO 8601 timestamp)"`
	Page     int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage  int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// GetGistInput defines input parameters for getting a gist.
type GetGistInput struct {
	GistID string `json:"gist_id" jsonschema:"required,description=The ID of the gist to retrieve"`
}

// CreateGistInput defines input parameters for creating a gist.
type CreateGistInput struct {
	Description string            `json:"description" jsonschema:"description=Description of the gist"`
	Public      bool              `json:"public" jsonschema:"description=Whether the gist is public"`
	Files       map[string]string `json:"files" jsonschema:"required,description=Map of filename to file content"`
}

// UpdateGistInput defines input parameters for updating a gist.
type UpdateGistInput struct {
	GistID      string            `json:"gist_id" jsonschema:"required,description=The ID of the gist to update"`
	Description string            `json:"description" jsonschema:"description=New description for the gist"`
	Files       map[string]string `json:"files" jsonschema:"description=Map of filename to new file content"`
}

// ListLabelsInput defines input parameters for listing labels.
type ListLabelsInput struct {
	Owner   string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo    string `json:"repo" jsonschema:"required,description=Repository name"`
	Page    int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// GetLabelInput defines input parameters for getting a label.
type GetLabelInput struct {
	Owner string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo  string `json:"repo" jsonschema:"required,description=Repository name"`
	Name  string `json:"name" jsonschema:"required,description=Label name"`
}

// ListDiscussionsInput defines input parameters for listing discussions.
type ListDiscussionsInput struct {
	Owner      string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo       string `json:"repo" jsonschema:"required,description=Repository name"`
	CategoryID string `json:"category_id" jsonschema:"description=Filter by category ID"`
	OrderBy    string `json:"order_by" jsonschema:"description=Field to order by,enum=CREATED_AT|UPDATED_AT"`
	Direction  string `json:"direction" jsonschema:"description=Order direction,enum=ASC|DESC"`
	First      int    `json:"first" jsonschema:"description=Number of discussions to return"`
	After      string `json:"after" jsonschema:"description=Cursor for pagination"`
}

// GetDiscussionInput defines input parameters for getting a discussion.
type GetDiscussionInput struct {
	Owner            string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo             string `json:"repo" jsonschema:"required,description=Repository name"`
	DiscussionNumber int    `json:"discussion_number" jsonschema:"required,description=Discussion number"`
}

// GetDiscussionCommentsInput defines input parameters for getting discussion comments.
type GetDiscussionCommentsInput struct {
	Owner            string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo             string `json:"repo" jsonschema:"required,description=Repository name"`
	DiscussionNumber int    `json:"discussion_number" jsonschema:"required,description=Discussion number"`
	First            int    `json:"first" jsonschema:"description=Number of comments to return"`
	After            string `json:"after" jsonschema:"description=Cursor for pagination"`
}

// ListDiscussionCategoriesInput defines input parameters for listing discussion categories.
type ListDiscussionCategoriesInput struct {
	Owner string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo  string `json:"repo" jsonschema:"required,description=Repository name"`
	First int    `json:"first" jsonschema:"description=Number of categories to return"`
	After string `json:"after" jsonschema:"description=Cursor for pagination"`
}

// ListProjectsInput defines input parameters for listing projects.
type ListProjectsInput struct {
	OwnerType string `json:"owner_type" jsonschema:"required,description=Owner type,enum=user|org"`
	Owner     string `json:"owner" jsonschema:"required,description=Owner name (user handle or org name)"`
	Query     string `json:"query" jsonschema:"description=Filter projects by title and state (e.g. 'roadmap is:open')"`
	PerPage   int    `json:"per_page" jsonschema:"description=Results per page"`
	After     string `json:"after" jsonschema:"description=Forward pagination cursor"`
	Before    string `json:"before" jsonschema:"description=Backward pagination cursor"`
}

// GetProjectInput defines input parameters for getting a project.
type GetProjectInput struct {
	Owner     string `json:"owner" jsonschema:"required,description=Owner of the project"`
	ProjectID int    `json:"project_id" jsonschema:"required,description=Project number"`
}

// ListProjectItemsInput defines input parameters for listing project items.
type ListProjectItemsInput struct {
	Owner     string `json:"owner" jsonschema:"required,description=Owner of the project"`
	ProjectID int    `json:"project_id" jsonschema:"required,description=Project number"`
	First     int    `json:"first" jsonschema:"description=Number of items to return"`
	After     string `json:"after" jsonschema:"description=Cursor for pagination"`
}

// GetProjectItemInput defines input parameters for getting a project item.
type GetProjectItemInput struct {
	Owner     string `json:"owner" jsonschema:"required,description=Owner of the project"`
	ProjectID int    `json:"project_id" jsonschema:"required,description=Project number"`
	ItemID    string `json:"item_id" jsonschema:"required,description=Item ID"`
}

// AddProjectItemInput defines input parameters for adding a project item.
type AddProjectItemInput struct {
	Owner     string `json:"owner" jsonschema:"required,description=Owner of the project"`
	ProjectID int    `json:"project_id" jsonschema:"required,description=Project number"`
	ContentID string `json:"content_id" jsonschema:"required,description=ID of the issue or PR to add"`
}

// UpdateProjectItemInput defines input parameters for updating a project item.
type UpdateProjectItemInput struct {
	Owner     string `json:"owner" jsonschema:"required,description=Owner of the project"`
	ProjectID int    `json:"project_id" jsonschema:"required,description=Project number"`
	ItemID    string `json:"item_id" jsonschema:"required,description=Item ID to update"`
	FieldID   string `json:"field_id" jsonschema:"required,description=Field ID to update"`
	Value     any    `json:"value" jsonschema:"required,description=New value for the field"`
}

// DeleteProjectItemInput defines input parameters for deleting a project item.
type DeleteProjectItemInput struct {
	Owner     string `json:"owner" jsonschema:"required,description=Owner of the project"`
	ProjectID int    `json:"project_id" jsonschema:"required,description=Project number"`
	ItemID    string `json:"item_id" jsonschema:"required,description=Item ID to delete"`
}

// ListPullRequestsInput defines input parameters for listing pull requests.
type ListPullRequestsInput struct {
	Owner     string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo      string `json:"repo" jsonschema:"required,description=Repository name"`
	State     string `json:"state" jsonschema:"description=Filter by state,enum=open|closed|all"`
	Head      string `json:"head" jsonschema:"description=Filter by head user/org and branch"`
	Base      string `json:"base" jsonschema:"description=Filter by base branch"`
	Sort      string `json:"sort" jsonschema:"description=Sort by,enum=created|updated|popularity|long-running"`
	Direction string `json:"direction" jsonschema:"description=Sort direction,enum=asc|desc"`
	Page      int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage   int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// PullRequestReadInput defines input parameters for reading pull request info.
type PullRequestReadInput struct {
	Method     string `json:"method" jsonschema:"required,description=Read operation to perform,enum=get|get_diff|get_status|get_files|get_review_comments|get_reviews|get_comments"`
	Owner      string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo       string `json:"repo" jsonschema:"required,description=Repository name"`
	PullNumber int    `json:"pullNumber" jsonschema:"required,description=Pull request number"`
	Page       int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage    int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// UpdatePullRequestInput defines input parameters for updating a pull request.
type UpdatePullRequestInput struct {
	Owner               string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo                string `json:"repo" jsonschema:"required,description=Repository name"`
	PullNumber          int    `json:"pull_number" jsonschema:"required,description=Pull request number"`
	Title               string `json:"title" jsonschema:"description=New title"`
	Body                string `json:"body" jsonschema:"description=New body"`
	State               string `json:"state" jsonschema:"description=New state,enum=open|closed"`
	Base                string `json:"base" jsonschema:"description=New base branch"`
	MaintainerCanModify bool   `json:"maintainer_can_modify" jsonschema:"description=Allow maintainer modifications"`
}

// MergePullRequestInput defines input parameters for merging a pull request.
type MergePullRequestInput struct {
	Owner         string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo          string `json:"repo" jsonschema:"required,description=Repository name"`
	PullNumber    int    `json:"pull_number" jsonschema:"required,description=Pull request number"`
	CommitTitle   string `json:"commit_title" jsonschema:"description=Title for the merge commit"`
	CommitMessage string `json:"commit_message" jsonschema:"description=Message for the merge commit"`
	MergeMethod   string `json:"merge_method" jsonschema:"description=Merge method,enum=merge|squash|rebase"`
}

// UpdatePullRequestBranchInput defines input parameters for updating a pull request branch.
type UpdatePullRequestBranchInput struct {
	Owner           string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo            string `json:"repo" jsonschema:"required,description=Repository name"`
	PullNumber      int    `json:"pull_number" jsonschema:"required,description=Pull request number"`
	ExpectedHeadSHA string `json:"expected_head_sha" jsonschema:"description=Expected SHA of the head ref"`
}

// RequestCopilotReviewInput defines input parameters for requesting Copilot review.
type RequestCopilotReviewInput struct {
	Owner      string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo       string `json:"repo" jsonschema:"required,description=Repository name"`
	PullNumber int    `json:"pull_number" jsonschema:"required,description=Pull request number"`
}

// IssueReadInput defines input parameters for reading issue info.
type IssueReadInput struct {
	Method      string `json:"method" jsonschema:"required,description=Read operation to perform,enum=get|get_comments|get_sub_issues|get_labels"`
	Owner       string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo        string `json:"repo" jsonschema:"required,description=Repository name"`
	IssueNumber int    `json:"issue_number" jsonschema:"required,description=Issue number"`
	Page        int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage     int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// IssueWriteInput defines input parameters for writing issue info.
type IssueWriteInput struct {
	Method      string   `json:"method" jsonschema:"required,description=Write operation to perform,enum=create|update"`
	Owner       string   `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo        string   `json:"repo" jsonschema:"required,description=Repository name"`
	IssueNumber int      `json:"issue_number" jsonschema:"description=Issue number (for update)"`
	Title       string   `json:"title" jsonschema:"description=Issue title"`
	Body        string   `json:"body" jsonschema:"description=Issue body"`
	Assignees   []string `json:"assignees" jsonschema:"description=Usernames to assign"`
	Labels      []string `json:"labels" jsonschema:"description=Labels to add"`
	Milestone   int      `json:"milestone" jsonschema:"description=Milestone number"`
	State       string   `json:"state" jsonschema:"description=Issue state,enum=open|closed"`
	StateReason string   `json:"state_reason" jsonschema:"description=Reason for closing,enum=completed|not_planned|duplicate"`
}

// ListIssuesInput defines input parameters for listing issues.
type ListIssuesInput struct {
	Owner     string   `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo      string   `json:"repo" jsonschema:"required,description=Repository name"`
	State     string   `json:"state" jsonschema:"description=Filter by state,enum=OPEN|CLOSED"`
	Labels    []string `json:"labels" jsonschema:"description=Filter by labels"`
	OrderBy   string   `json:"orderBy" jsonschema:"description=Order by field,enum=CREATED_AT|UPDATED_AT|COMMENTS"`
	Direction string   `json:"direction" jsonschema:"description=Order direction,enum=ASC|DESC"`
	Since     string   `json:"since" jsonschema:"description=Filter by date (ISO 8601 timestamp)"`
	PerPage   int      `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
	After     string   `json:"after" jsonschema:"description=Cursor for pagination"`
}

// ListIssueTypesInput defines input parameters for listing issue types.
type ListIssueTypesInput struct {
	Owner string `json:"owner" jsonschema:"required,description=Organization owner"`
}

// AddIssueCommentInput defines input parameters for adding an issue comment.
type AddIssueCommentInput struct {
	Owner       string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo        string `json:"repo" jsonschema:"required,description=Repository name"`
	IssueNumber int    `json:"issue_number" jsonschema:"required,description=Issue number"`
	Body        string `json:"body" jsonschema:"required,description=Comment body"`
}

// StarRepositoryInput defines input parameters for starring a repository.
type StarRepositoryInput struct {
	Owner string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo  string `json:"repo" jsonschema:"required,description=Repository name"`
}

// UnstarRepositoryInput defines input parameters for unstarring a repository.
type UnstarRepositoryInput struct {
	Owner string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo  string `json:"repo" jsonschema:"required,description=Repository name"`
}

// ListStarredRepositoriesInput defines input parameters for listing starred repositories.
type ListStarredRepositoriesInput struct {
	Username string `json:"username" jsonschema:"description=GitHub username (defaults to authenticated user)"`
	Sort     string `json:"sort" jsonschema:"description=Sort by,enum=created|updated"`
	Order    string `json:"order" jsonschema:"description=Sort direction,enum=asc|desc"`
	Page     int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage  int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// ListCodeScanningAlertsInput defines input parameters for listing code scanning alerts.
type ListCodeScanningAlertsInput struct {
	Owner    string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo     string `json:"repo" jsonschema:"required,description=Repository name"`
	Ref      string `json:"ref" jsonschema:"description=Git ref to filter results"`
	State    string `json:"state" jsonschema:"description=Filter by state,enum=open|closed|dismissed|fixed"`
	Severity string `json:"severity" jsonschema:"description=Filter by severity,enum=critical|high|medium|low|warning|note|error"`
	ToolName string `json:"tool_name" jsonschema:"description=Filter by tool name"`
}

// GetCodeScanningAlertInput defines input parameters for getting a code scanning alert.
type GetCodeScanningAlertInput struct {
	Owner       string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo        string `json:"repo" jsonschema:"required,description=Repository name"`
	AlertNumber int    `json:"alertNumber" jsonschema:"required,description=Alert number"`
}

// ListSecretScanningAlertsInput defines input parameters for listing secret scanning alerts.
type ListSecretScanningAlertsInput struct {
	Owner      string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo       string `json:"repo" jsonschema:"required,description=Repository name"`
	State      string `json:"state" jsonschema:"description=Filter by state,enum=open|resolved"`
	SecretType string `json:"secret_type" jsonschema:"description=Filter by secret type"`
	Resolution string `json:"resolution" jsonschema:"description=Filter by resolution,enum=false_positive|wont_fix|revoked|pattern_edited|pattern_deleted|used_in_tests"`
}

// GetSecretScanningAlertInput defines input parameters for getting a secret scanning alert.
type GetSecretScanningAlertInput struct {
	Owner       string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo        string `json:"repo" jsonschema:"required,description=Repository name"`
	AlertNumber int    `json:"alertNumber" jsonschema:"required,description=Alert number"`
}

// ListDependabotAlertsInput defines input parameters for listing Dependabot alerts.
type ListDependabotAlertsInput struct {
	Owner    string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo     string `json:"repo" jsonschema:"required,description=Repository name"`
	State    string `json:"state" jsonschema:"description=Filter by state,enum=auto_dismissed|dismissed|fixed|open"`
	Severity string `json:"severity" jsonschema:"description=Filter by severity,enum=low|medium|high|critical"`
	Page     int    `json:"page" jsonschema:"description=Page number for pagination"`
	PerPage  int    `json:"perPage" jsonschema:"description=Results per page"`
}

// GetDependabotAlertInput defines input parameters for getting a Dependabot alert.
type GetDependabotAlertInput struct {
	Owner       string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo        string `json:"repo" jsonschema:"required,description=Repository name"`
	AlertNumber int    `json:"alert_number" jsonschema:"required,description=Alert number"`
}

// GetLatestReleaseInput defines input parameters for getting the latest release.
type GetLatestReleaseInput struct {
	Owner string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo  string `json:"repo" jsonschema:"required,description=Repository name"`
}

// GetTagInput defines input parameters for getting a tag.
type GetTagInput struct {
	Owner string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo  string `json:"repo" jsonschema:"required,description=Repository name"`
	Tag   string `json:"tag" jsonschema:"required,description=Tag name"`
}

// ListWorkflowJobsInput defines input parameters for listing workflow jobs.
type ListWorkflowJobsInput struct {
	Owner   string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo    string `json:"repo" jsonschema:"required,description=Repository name"`
	RunID   int    `json:"run_id" jsonschema:"required,description=Workflow run ID"`
	Filter  string `json:"filter" jsonschema:"description=Filter jobs,enum=latest|all"`
	Page    int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// GetJobLogsInput defines input parameters for getting job logs.
type GetJobLogsInput struct {
	Owner         string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo          string `json:"repo" jsonschema:"required,description=Repository name"`
	JobID         int    `json:"job_id" jsonschema:"description=Job ID for single job logs"`
	RunID         int    `json:"run_id" jsonschema:"description=Run ID for failed jobs logs"`
	FailedOnly    bool   `json:"failed_only" jsonschema:"description=Get logs for failed jobs only"`
	ReturnContent bool   `json:"return_content" jsonschema:"description=Return actual log content instead of URLs"`
	TailLines     int    `json:"tail_lines" jsonschema:"description=Number of lines from end of log"`
}

// GetWorkflowRunLogsInput defines input parameters for getting workflow run logs.
type GetWorkflowRunLogsInput struct {
	Owner string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo  string `json:"repo" jsonschema:"required,description=Repository name"`
	RunID int    `json:"run_id" jsonschema:"required,description=Workflow run ID"`
}

// GetWorkflowRunUsageInput defines input parameters for getting workflow run usage.
type GetWorkflowRunUsageInput struct {
	Owner string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo  string `json:"repo" jsonschema:"required,description=Repository name"`
	RunID int    `json:"run_id" jsonschema:"required,description=Workflow run ID"`
}

// ListWorkflowRunArtifactsInput defines input parameters for listing workflow run artifacts.
type ListWorkflowRunArtifactsInput struct {
	Owner   string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo    string `json:"repo" jsonschema:"required,description=Repository name"`
	RunID   int    `json:"run_id" jsonschema:"required,description=Workflow run ID"`
	Page    int    `json:"page" jsonschema:"description=Page number for pagination (min 1)"`
	PerPage int    `json:"perPage" jsonschema:"description=Results per page for pagination (min 1 and max 100)"`
}

// DownloadWorkflowRunArtifactInput defines input parameters for downloading an artifact.
type DownloadWorkflowRunArtifactInput struct {
	Owner      string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo       string `json:"repo" jsonschema:"required,description=Repository name"`
	ArtifactID int    `json:"artifact_id" jsonschema:"required,description=Artifact ID"`
}

// CancelWorkflowRunInput defines input parameters for canceling a workflow run.
type CancelWorkflowRunInput struct {
	Owner string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo  string `json:"repo" jsonschema:"required,description=Repository name"`
	RunID int    `json:"run_id" jsonschema:"required,description=Workflow run ID"`
}

// RerunWorkflowRunInput defines input parameters for rerunning a workflow run.
type RerunWorkflowRunInput struct {
	Owner string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo  string `json:"repo" jsonschema:"required,description=Repository name"`
	RunID int    `json:"run_id" jsonschema:"required,description=Workflow run ID"`
}

// RerunFailedJobsInput defines input parameters for rerunning failed jobs.
type RerunFailedJobsInput struct {
	Owner string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo  string `json:"repo" jsonschema:"required,description=Repository name"`
	RunID int    `json:"run_id" jsonschema:"required,description=Workflow run ID"`
}

// DeleteWorkflowRunLogsInput defines input parameters for deleting workflow run logs.
type DeleteWorkflowRunLogsInput struct {
	Owner string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo  string `json:"repo" jsonschema:"required,description=Repository name"`
	RunID int    `json:"run_id" jsonschema:"required,description=Workflow run ID"`
}

// RunWorkflowInput defines input parameters for running a workflow.
type RunWorkflowInput struct {
	Owner      string         `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo       string         `json:"repo" jsonschema:"required,description=Repository name"`
	WorkflowID string         `json:"workflow_id" jsonschema:"required,description=Workflow ID or filename"`
	Ref        string         `json:"ref" jsonschema:"required,description=Git ref to run workflow from"`
	Inputs     map[string]any `json:"inputs" jsonschema:"description=Workflow inputs"`
}

// GetNotificationDetailsInput defines input parameters for getting notification details.
type GetNotificationDetailsInput struct {
	NotificationID string `json:"notification_id" jsonschema:"required,description=Notification thread ID"`
}

// ManageNotificationSubscriptionInput defines input parameters for managing notification subscription.
type ManageNotificationSubscriptionInput struct {
	NotificationID string `json:"notification_id" jsonschema:"required,description=Notification thread ID"`
	Action         string `json:"action" jsonschema:"required,description=Action to perform,enum=get|set|delete"`
	Ignored        bool   `json:"ignored" jsonschema:"description=Whether to ignore notifications"`
}

// ManageRepositoryNotificationSubscriptionInput defines input parameters for managing repo notification subscription.
type ManageRepositoryNotificationSubscriptionInput struct {
	Owner   string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo    string `json:"repo" jsonschema:"required,description=Repository name"`
	Action  string `json:"action" jsonschema:"required,description=Action to perform,enum=get|set|delete"`
	Ignored bool   `json:"ignored" jsonschema:"description=Whether to ignore notifications"`
}

// MarkAllNotificationsReadInput defines input parameters for marking all notifications read.
type MarkAllNotificationsReadInput struct {
	LastReadAt string `json:"last_read_at" jsonschema:"description=Timestamp of last read notification (ISO 8601)"`
	Read       bool   `json:"read" jsonschema:"description=Whether to mark as read"`
	Owner      string `json:"owner" jsonschema:"description=Repository owner (for repo-specific marking)"`
	Repo       string `json:"repo" jsonschema:"description=Repository name (for repo-specific marking)"`
}

// DismissNotificationInput defines input parameters for dismissing a notification.
type DismissNotificationInput struct {
	ThreadID string `json:"thread_id" jsonschema:"required,description=Notification thread ID"`
}

// GetTeamsInput defines input parameters for getting teams.
type GetTeamsInput struct {
	Org     string `json:"org" jsonschema:"required,description=Organization name"`
	Page    int    `json:"page" jsonschema:"description=Page number for pagination"`
	PerPage int    `json:"perPage" jsonschema:"description=Results per page"`
}

// GetTeamMembersInput defines input parameters for getting team members.
type GetTeamMembersInput struct {
	Org      string `json:"org" jsonschema:"required,description=Organization name"`
	TeamSlug string `json:"team_slug" jsonschema:"required,description=Team slug"`
	Role     string `json:"role" jsonschema:"description=Filter by role,enum=member|maintainer|all"`
	Page     int    `json:"page" jsonschema:"description=Page number for pagination"`
	PerPage  int    `json:"perPage" jsonschema:"description=Results per page"`
}

// DeleteFileInput defines input parameters for deleting a file.
type DeleteFileInput struct {
	Owner   string `json:"owner" jsonschema:"required,description=Repository owner"`
	Repo    string `json:"repo" jsonschema:"required,description=Repository name"`
	Path    string `json:"path" jsonschema:"required,description=Path to the file to delete"`
	Message string `json:"message" jsonschema:"required,description=Commit message"`
	Branch  string `json:"branch" jsonschema:"required,description=Branch containing the file"`
	SHA     string `json:"sha" jsonschema:"required,description=SHA of the file being deleted"`
}
