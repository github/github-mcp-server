For full details on the GitHub MCP Server, see the [README](README.md)

<!-- START AUTOMATED TOOLS -->
<details>

<summary>Actions</summary>

- **cancel_workflow_run** - Cancel workflow run
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `run_id`: The unique identifier of the workflow run (number, required)

- **delete_workflow_run_logs** - Delete workflow logs
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `run_id`: The unique identifier of the workflow run (number, required)

- **download_workflow_run_artifact** - Download workflow artifact
  - `artifact_id`: The unique identifier of the artifact (number, required)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

- **get_job_logs** - Get job logs
  - `failed_only`: When true, gets logs for all failed jobs in run_id (boolean, optional)
  - `job_id`: The unique identifier of the workflow job (required for single job logs) (number, optional)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `return_content`: Returns actual log content instead of URLs (boolean, optional)
  - `run_id`: Workflow run ID (required when using failed_only) (number, optional)
  - `tail_lines`: Number of lines to return from the end of the log (number, optional)

- **get_workflow_run** - Get workflow run
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `run_id`: The unique identifier of the workflow run (number, required)

- **get_workflow_run_logs** - Get workflow run logs
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `run_id`: The unique identifier of the workflow run (number, required)

- **get_workflow_run_usage** - Get workflow usage
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `run_id`: The unique identifier of the workflow run (number, required)

- **list_workflow_jobs** - List workflow jobs
  - `filter`: Filters jobs by their completed_at timestamp (string, optional)
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)
  - `run_id`: The unique identifier of the workflow run (number, required)

- **list_workflow_run_artifacts** - List workflow artifacts
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)
  - `run_id`: The unique identifier of the workflow run (number, required)

- **list_workflow_runs** - List workflow runs
  - `actor`: Returns someone's workflow runs. Use the login for the user who created the workflow run. (string, optional)
  - `branch`: Returns workflow runs associated with a branch. Use the name of the branch. (string, optional)
  - `event`: Returns workflow runs for a specific event type (string, optional)
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)
  - `status`: Returns workflow runs with the check run status (string, optional)
  - `workflow_id`: The workflow ID or workflow file name (string, required)

- **list_workflows** - List workflows
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)

- **rerun_failed_jobs** - Rerun failed jobs
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `run_id`: The unique identifier of the workflow run (number, required)

- **rerun_workflow_run** - Rerun workflow run
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `run_id`: The unique identifier of the workflow run (number, required)

- **run_workflow** - Run workflow
  - `inputs`: Inputs the workflow accepts (object, optional)
  - `owner`: Repository owner (string, required)
  - `ref`: The git reference for the workflow. The reference can be a branch or tag name. (string, required)
  - `repo`: Repository name (string, required)
  - `workflow_id`: The workflow ID (numeric) or workflow file name (e.g., main.yml, ci.yaml) (string, required)

</details>

<details>

<summary>Code Security</summary>

- **get_code_scanning_alert** - Get code scanning alert
  - `alertNumber`: The number of the alert. (number, required)
  - `owner`: The owner of the repository. (string, required)
  - `repo`: The name of the repository. (string, required)

- **list_code_scanning_alerts** - List code scanning alerts
  - `owner`: The owner of the repository. (string, required)
  - `ref`: The Git reference for the results you want to list. (string, optional)
  - `repo`: The name of the repository. (string, required)
  - `severity`: Filter code scanning alerts by severity (string, optional)
  - `state`: Filter code scanning alerts by state. Defaults to open (string, optional)
  - `tool_name`: The name of the tool used for code scanning. (string, optional)

</details>

<details>

<summary>Context</summary>

- **get_me** - Get my user profile
  - No parameters required

- **get_team_members** - Get team members
  - `org`: Organization login (owner) that contains the team. (string, required)
  - `team_slug`: Team slug (string, required)

- **get_teams** - Get teams
  - `user`: Username to get teams for. If not provided, uses the authenticated user. (string, optional)

</details>

<details>

<summary>Dependabot</summary>

- **get_dependabot_alert** - Get dependabot alert
  - `alertNumber`: The number of the alert. (number, required)
  - `owner`: The owner of the repository. (string, required)
  - `repo`: The name of the repository. (string, required)

- **list_dependabot_alerts** - List dependabot alerts
  - `owner`: The owner of the repository. (string, required)
  - `repo`: The name of the repository. (string, required)
  - `severity`: Filter dependabot alerts by severity (string, optional)
  - `state`: Filter dependabot alerts by state. Defaults to open (string, optional)

</details>

<details>

<summary>Discussions</summary>

- **get_discussion** - Get discussion
  - `discussionNumber`: Discussion Number (number, required)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

- **get_discussion_comments** - Get discussion comments
  - `after`: Cursor for pagination. Use the endCursor from the previous page's PageInfo for GraphQL APIs. (string, optional)
  - `discussionNumber`: Discussion Number (number, required)
  - `owner`: Repository owner (string, required)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)

- **list_discussion_categories** - List discussion categories
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name. If not provided, discussion categories will be queried at the organisation level. (string, optional)

- **list_discussions** - List discussions
  - `after`: Cursor for pagination. Use the endCursor from the previous page's PageInfo for GraphQL APIs. (string, optional)
  - `category`: Optional filter by discussion category ID. If provided, only discussions with this category are listed. (string, optional)
  - `direction`: Order direction. (string, optional)
  - `orderBy`: Order discussions by field. If provided, the 'direction' also needs to be provided. (string, optional)
  - `owner`: Repository owner (string, required)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name. If not provided, discussions will be queried at the organisation level. (string, optional)

</details>

<details>

<summary>Gists</summary>

- **create_gist** - Create Gist
  - `content`: Content for simple single-file gist creation (string, required)
  - `description`: Description of the gist (string, optional)
  - `filename`: Filename for simple single-file gist creation (string, required)
  - `public`: Whether the gist is public (boolean, optional)

- **get_gist** - Get Gist Content
  - `gist_id`: The ID of the gist (string, required)

- **list_gists** - List Gists
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `since`: Only gists updated after this time (ISO 8601 timestamp) (string, optional)
  - `username`: GitHub username (omit for authenticated user's gists) (string, optional)

- **update_gist** - Update Gist
  - `content`: Content for the file (string, required)
  - `description`: Updated description of the gist (string, optional)
  - `filename`: Filename to update or create (string, required)
  - `gist_id`: ID of the gist to update (string, required)

</details>

<details>

<summary>Git</summary>

- **get_repository_tree** - Get repository tree
  - `owner`: Repository owner (username or organization) (string, required)
  - `path_filter`: Optional path prefix to filter the tree results (e.g., 'src/' to only show files in the src directory) (string, optional)
  - `recursive`: Setting this parameter to true returns the objects or subtrees referenced by the tree. Default is false (boolean, optional)
  - `repo`: Repository name (string, required)
  - `tree_sha`: The SHA1 value or ref (branch or tag) name of the tree. Defaults to the repository's default branch (string, optional)

</details>

<details>

<summary>Issues</summary>

- **add_issue_comment** - Add comment to issue
  - `body`: Comment content (string, required)
  - `issue_number`: Issue number to comment on (number, required)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

- **assign_copilot_to_issue** - Assign Copilot to issue
  - `issueNumber`: Issue number (number, required)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

- **get_label** - Get a specific label from a repository.
  - `name`: Label name. (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `repo`: Repository name (string, required)

- **issue_read** - Get issue details
  - `issue_number`: The number of the issue (number, required)
  - `method`: The read operation to perform on a single issue. 
Options are: 
1. get - Get details of a specific issue.
2. get_comments - Get issue comments.
3. get_sub_issues - Get sub-issues of the issue.
4. get_labels - Get labels assigned to the issue.
 (string, required)
  - `owner`: The owner of the repository (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: The name of the repository (string, required)

- **issue_write** - Create or update issue.
  - `assignees`: Usernames to assign to this issue (string[], optional)
  - `body`: Issue body content (string, optional)
  - `duplicate_of`: Issue number that this issue is a duplicate of. Only used when state_reason is 'duplicate'. (number, optional)
  - `issue_number`: Issue number to update (number, optional)
  - `labels`: Labels to apply to this issue (string[], optional)
  - `method`: Write operation to perform on a single issue.
Options are: 
- 'create' - creates a new issue. 
- 'update' - updates an existing issue.
 (string, required)
  - `milestone`: Milestone number (number, optional)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `state`: New state (string, optional)
  - `state_reason`: Reason for the state change. Ignored unless state is changed. (string, optional)
  - `title`: Issue title (string, optional)
  - `type`: Type of this issue. Only use if the repository has issue types configured. Use list_issue_types tool to get valid type values for the organization. If the repository doesn't support issue types, omit this parameter. (string, optional)

- **list_issue_types** - List available issue types
  - `owner`: The organization owner of the repository (string, required)

- **list_issues** - List issues
  - `after`: Cursor for pagination. Use the endCursor from the previous page's PageInfo for GraphQL APIs. (string, optional)
  - `direction`: Order direction. If provided, the 'orderBy' also needs to be provided. (string, optional)
  - `labels`: Filter by labels (string[], optional)
  - `orderBy`: Order issues by field. If provided, the 'direction' also needs to be provided. (string, optional)
  - `owner`: Repository owner (string, required)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)
  - `since`: Filter by date (ISO 8601 timestamp) (string, optional)
  - `state`: Filter by state, by default both open and closed issues are returned when not provided (string, optional)

- **search_issues** - Search issues
  - `order`: Sort order (string, optional)
  - `owner`: Optional repository owner. If provided with repo, only issues for this repository are listed. (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `query`: Search query using GitHub issues search syntax (string, required)
  - `repo`: Optional repository name. If provided with owner, only issues for this repository are listed. (string, optional)
  - `sort`: Sort field by number of matches of categories, defaults to best match (string, optional)

- **sub_issue_write** - Change sub-issue
  - `after_id`: The ID of the sub-issue to be prioritized after (either after_id OR before_id should be specified) (number, optional)
  - `before_id`: The ID of the sub-issue to be prioritized before (either after_id OR before_id should be specified) (number, optional)
  - `issue_number`: The number of the parent issue (number, required)
  - `method`: The action to perform on a single sub-issue
Options are:
- 'add' - add a sub-issue to a parent issue in a GitHub repository.
- 'remove' - remove a sub-issue from a parent issue in a GitHub repository.
- 'reprioritize' - change the order of sub-issues within a parent issue in a GitHub repository. Use either 'after_id' or 'before_id' to specify the new position.
				 (string, required)
  - `owner`: Repository owner (string, required)
  - `replace_parent`: When true, replaces the sub-issue's current parent issue. Use with 'add' method only. (boolean, optional)
  - `repo`: Repository name (string, required)
  - `sub_issue_id`: The ID of the sub-issue to add. ID is not the same as issue number (number, required)

</details>

<details>

<summary>Labels</summary>

- **get_label** - Get a specific label from a repository.
  - `name`: Label name. (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `repo`: Repository name (string, required)

- **label_write** - Write operations on repository labels.
  - `color`: Label color as 6-character hex code without '#' prefix (e.g., 'f29513'). Required for 'create', optional for 'update'. (string, optional)
  - `description`: Label description text. Optional for 'create' and 'update'. (string, optional)
  - `method`: Operation to perform: 'create', 'update', or 'delete' (string, required)
  - `name`: Label name - required for all operations (string, required)
  - `new_name`: New name for the label (used only with 'update' method to rename) (string, optional)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `repo`: Repository name (string, required)

- **list_label** - List labels from a repository
  - `owner`: Repository owner (username or organization name) - required for all operations (string, required)
  - `repo`: Repository name - required for all operations (string, required)

</details>

<details>

<summary>Notifications</summary>

- **dismiss_notification** - Dismiss notification
  - `state`: The new state of the notification (read/done) (string, optional)
  - `threadID`: The ID of the notification thread (string, required)

- **get_notification_details** - Get notification details
  - `notificationID`: The ID of the notification (string, required)

- **list_notifications** - List notifications
  - `before`: Only show notifications updated before the given time (ISO 8601 format) (string, optional)
  - `filter`: Filter notifications to, use default unless specified. Read notifications are ones that have already been acknowledged by the user. Participating notifications are those that the user is directly involved in, such as issues or pull requests they have commented on or created. (string, optional)
  - `owner`: Optional repository owner. If provided with repo, only notifications for this repository are listed. (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Optional repository name. If provided with owner, only notifications for this repository are listed. (string, optional)
  - `since`: Only show notifications updated after the given time (ISO 8601 format) (string, optional)

- **manage_notification_subscription** - Manage notification subscription
  - `action`: Action to perform: ignore, watch, or delete the notification subscription. (string, required)
  - `notificationID`: The ID of the notification thread. (string, required)

- **manage_repository_notification_subscription** - Manage repository notification subscription
  - `action`: Action to perform: ignore, watch, or delete the repository notification subscription. (string, required)
  - `owner`: The account owner of the repository. (string, required)
  - `repo`: The name of the repository. (string, required)

- **mark_all_notifications_read** - Mark all notifications as read
  - `lastReadAt`: Describes the last point that notifications were checked (optional). Default: Now (string, optional)
  - `owner`: Optional repository owner. If provided with repo, only notifications for this repository are marked as read. (string, optional)
  - `repo`: Optional repository name. If provided with owner, only notifications for this repository are marked as read. (string, optional)

</details>

<details>

<summary>Organizations</summary>

- **search_orgs** - Search organizations
  - `order`: Sort order (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `query`: Organization search query. Examples: 'microsoft', 'location:california', 'created:>=2025-01-01'. Search is automatically scoped to type:org. (string, required)
  - `sort`: Sort field by category (string, optional)

</details>

<details>

<summary>Projects</summary>

- **add_project_item** - Add project item
  - `item_id`: The numeric ID of the issue or pull request to add to the project. (number, required)
  - `item_type`: The item's type, either issue or pull_request. (string, required)
  - `owner`: If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive. (string, required)
  - `owner_type`: Owner type (string, required)
  - `project_number`: The project's number. (number, required)

- **delete_project_item** - Delete project item
  - `item_id`: The internal project item ID to delete from the project (not the issue or pull request ID). (number, required)
  - `owner`: If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive. (string, required)
  - `owner_type`: Owner type (string, required)
  - `project_number`: The project's number. (number, required)

- **get_project** - Get project
  - `owner`: If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive. (string, required)
  - `owner_type`: Owner type (string, required)
  - `project_number`: The project's number (number, required)

- **get_project_field** - Get project field
  - `field_id`: The field's id. (number, required)
  - `owner`: If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive. (string, required)
  - `owner_type`: Owner type (string, required)
  - `project_number`: The project's number. (number, required)

- **get_project_item** - Get project item
  - `fields`: Specific list of field IDs to include in the response (e.g. ["102589", "985201", "169875"]). If not provided, only the title field is included. (string[], optional)
  - `item_id`: The item's ID. (number, required)
  - `owner`: If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive. (string, required)
  - `owner_type`: Owner type (string, required)
  - `project_number`: The project's number. (number, required)

- **list_project_fields** - List project fields
  - `after`: Forward pagination cursor from previous pageInfo.nextCursor. (string, optional)
  - `before`: Backward pagination cursor from previous pageInfo.prevCursor (rare). (string, optional)
  - `owner`: If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive. (string, required)
  - `owner_type`: Owner type (string, required)
  - `per_page`: Results per page (max 50) (number, optional)
  - `project_number`: The project's number. (number, required)

- **list_project_items** - List project items
  - `after`: Forward pagination cursor from previous pageInfo.nextCursor. (string, optional)
  - `before`: Backward pagination cursor from previous pageInfo.prevCursor (rare). (string, optional)
  - `fields`: Field IDs to include (e.g. ["102589", "985201"]). CRITICAL: Always provide to get field values. Without this, only titles returned. (string[], optional)
  - `owner`: If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive. (string, required)
  - `owner_type`: Owner type (string, required)
  - `per_page`: Results per page (max 50) (number, optional)
  - `project_number`: The project's number. (number, required)
  - `query`: Query string for advanced filtering of project items using GitHub's project filtering syntax. (string, optional)

- **list_projects** - List projects
  - `after`: Forward pagination cursor from previous pageInfo.nextCursor. (string, optional)
  - `before`: Backward pagination cursor from previous pageInfo.prevCursor (rare). (string, optional)
  - `owner`: If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive. (string, required)
  - `owner_type`: Owner type (string, required)
  - `per_page`: Results per page (max 50) (number, optional)
  - `query`: Filter projects by title text and open/closed state; permitted qualifiers: is:open, is:closed; examples: "roadmap is:open", "is:open feature planning". (string, optional)

- **update_project_item** - Update project item
  - `item_id`: The unique identifier of the project item. This is not the issue or pull request ID. (number, required)
  - `owner`: If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive. (string, required)
  - `owner_type`: Owner type (string, required)
  - `project_number`: The project's number. (number, required)
  - `updated_field`: Object consisting of the ID of the project field to update and the new value for the field. To clear the field, set value to null. Example: {"id": 123456, "value": "New Value"} (object, required)

</details>

<details>

<summary>Pull Requests</summary>

- **add_comment_to_pending_review** - Add review comment to the requester's latest pending pull request review
  - `body`: The text of the review comment (string, required)
  - `line`: The line of the blob in the pull request diff that the comment applies to. For multi-line comments, the last line of the range (number, optional)
  - `owner`: Repository owner (string, required)
  - `path`: The relative path to the file that necessitates a comment (string, required)
  - `pullNumber`: Pull request number (number, required)
  - `repo`: Repository name (string, required)
  - `side`: The side of the diff to comment on. LEFT indicates the previous state, RIGHT indicates the new state (string, optional)
  - `startLine`: For multi-line comments, the first line of the range that the comment applies to (number, optional)
  - `startSide`: For multi-line comments, the starting side of the diff that the comment applies to. LEFT indicates the previous state, RIGHT indicates the new state (string, optional)
  - `subjectType`: The level at which the comment is targeted (string, required)

- **create_pull_request** - Open new pull request
  - `base`: Branch to merge into (string, required)
  - `body`: PR description (string, optional)
  - `draft`: Create as draft PR (boolean, optional)
  - `head`: Branch containing changes (string, required)
  - `maintainer_can_modify`: Allow maintainer edits (boolean, optional)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `title`: PR title (string, required)

- **list_pull_requests** - List pull requests
  - `base`: Filter by base branch (string, optional)
  - `direction`: Sort direction (string, optional)
  - `head`: Filter by head user/org and branch (string, optional)
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)
  - `sort`: Sort by (string, optional)
  - `state`: Filter by state (string, optional)

- **merge_pull_request** - Merge pull request
  - `commit_message`: Extra detail for merge commit (string, optional)
  - `commit_title`: Title for merge commit (string, optional)
  - `merge_method`: Merge method (string, optional)
  - `owner`: Repository owner (string, required)
  - `pullNumber`: Pull request number (number, required)
  - `repo`: Repository name (string, required)

- **pull_request_read** - Get details for a single pull request
  - `method`: Action to specify what pull request data needs to be retrieved from GitHub. 
Possible options: 
 1. get - Get details of a specific pull request.
 2. get_diff - Get the diff of a pull request.
 3. get_status - Get status of a head commit in a pull request. This reflects status of builds and checks.
 4. get_files - Get the list of files changed in a pull request. Use with pagination parameters to control the number of results returned.
 5. get_review_comments - Get the review comments on a pull request. They are comments made on a portion of the unified diff during a pull request review. Use with pagination parameters to control the number of results returned.
 6. get_reviews - Get the reviews on a pull request. When asked for review comments, use get_review_comments method.
 7. get_comments - Get comments on a pull request. Use this if user doesn't specifically want review comments. Use with pagination parameters to control the number of results returned.
 (string, required)
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `pullNumber`: Pull request number (number, required)
  - `repo`: Repository name (string, required)

- **pull_request_review_write** - Write operations (create, submit, delete) on pull request reviews.
  - `body`: Review comment text (string, optional)
  - `commitID`: SHA of commit to review (string, optional)
  - `event`: Review action to perform. (string, optional)
  - `method`: The write operation to perform on pull request review. (string, required)
  - `owner`: Repository owner (string, required)
  - `pullNumber`: Pull request number (number, required)
  - `repo`: Repository name (string, required)

- **request_copilot_review** - Request Copilot review
  - `owner`: Repository owner (string, required)
  - `pullNumber`: Pull request number (number, required)
  - `repo`: Repository name (string, required)

- **search_pull_requests** - Search pull requests
  - `order`: Sort order (string, optional)
  - `owner`: Optional repository owner. If provided with repo, only pull requests for this repository are listed. (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `query`: Search query using GitHub pull request search syntax (string, required)
  - `repo`: Optional repository name. If provided with owner, only pull requests for this repository are listed. (string, optional)
  - `sort`: Sort field by number of matches of categories, defaults to best match (string, optional)

- **update_pull_request** - Edit pull request
  - `base`: New base branch name (string, optional)
  - `body`: New description (string, optional)
  - `draft`: Mark pull request as draft (true) or ready for review (false) (boolean, optional)
  - `maintainer_can_modify`: Allow maintainer edits (boolean, optional)
  - `owner`: Repository owner (string, required)
  - `pullNumber`: Pull request number to update (number, required)
  - `repo`: Repository name (string, required)
  - `reviewers`: GitHub usernames to request reviews from (string[], optional)
  - `state`: New state (string, optional)
  - `title`: New title (string, optional)

- **update_pull_request_branch** - Update pull request branch
  - `expectedHeadSha`: The expected SHA of the pull request's HEAD ref (string, optional)
  - `owner`: Repository owner (string, required)
  - `pullNumber`: Pull request number (number, required)
  - `repo`: Repository name (string, required)

</details>

<details>

<summary>Repositories</summary>

- **create_branch** - Create branch
  - `branch`: Name for new branch (string, required)
  - `from_branch`: Source branch (defaults to repo default) (string, optional)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

- **create_or_update_file** - Create or update file
  - `branch`: Branch to create/update the file in (string, required)
  - `content`: Content of the file (string, required)
  - `message`: Commit message (string, required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `path`: Path where to create/update the file (string, required)
  - `repo`: Repository name (string, required)
  - `sha`: Required if updating an existing file. The blob SHA of the file being replaced. (string, optional)

- **create_repository** - Create repository
  - `autoInit`: Initialize with README (boolean, optional)
  - `description`: Repository description (string, optional)
  - `name`: Repository name (string, required)
  - `organization`: Organization to create the repository in (omit to create in your personal account) (string, optional)
  - `private`: Whether repo should be private (boolean, optional)

- **delete_file** - Delete file
  - `branch`: Branch to delete the file from (string, required)
  - `message`: Commit message (string, required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `path`: Path to the file to delete (string, required)
  - `repo`: Repository name (string, required)

- **fork_repository** - Fork repository
  - `organization`: Organization to fork to (string, optional)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

- **get_commit** - Get commit details
  - `include_diff`: Whether to include file diffs and stats in the response. Default is true. (boolean, optional)
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)
  - `sha`: Commit SHA, branch name, or tag name (string, required)

- **get_file_contents** - Get file or directory contents
  - `owner`: Repository owner (username or organization) (string, required)
  - `path`: Path to file/directory (directories must end with a slash '/') (string, optional)
  - `ref`: Accepts optional git refs such as `refs/tags/{tag}`, `refs/heads/{branch}` or `refs/pull/{pr_number}/head` (string, optional)
  - `repo`: Repository name (string, required)
  - `sha`: Accepts optional commit SHA. If specified, it will be used instead of ref (string, optional)

- **get_latest_release** - Get latest release
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

- **get_release_by_tag** - Get a release by tag name
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `tag`: Tag name (e.g., 'v1.0.0') (string, required)

- **get_tag** - Get tag details
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `tag`: Tag name (string, required)

- **list_branches** - List branches
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)

- **list_commits** - List commits
  - `author`: Author username or email address to filter commits by (string, optional)
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)
  - `sha`: Commit SHA, branch or tag name to list commits of. If not provided, uses the default branch of the repository. If a commit SHA is provided, will list commits up to that SHA. (string, optional)

- **list_releases** - List releases
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)

- **list_tags** - List tags
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)

- **push_files** - Push files to repository
  - `branch`: Branch to push to (string, required)
  - `files`: Array of file objects to push, each object with path (string) and content (string) (object[], required)
  - `message`: Commit message (string, required)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

- **search_code** - Search code
  - `order`: Sort order for results (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `query`: Search query using GitHub's powerful code search syntax. Examples: 'content:Skill language:Java org:github', 'NOT is:archived language:Python OR language:go', 'repo:github/github-mcp-server'. Supports exact matching, language filters, path filters, and more. (string, required)
  - `sort`: Sort field ('indexed' only) (string, optional)

- **search_repositories** - Search repositories
  - `minimal_output`: Return minimal repository information (default: true). When false, returns full GitHub API repository objects. (boolean, optional)
  - `order`: Sort order (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `query`: Repository search query. Examples: 'machine learning in:name stars:>1000 language:python', 'topic:react', 'user:facebook'. Supports advanced search syntax for precise filtering. (string, required)
  - `sort`: Sort repositories by field, defaults to best match (string, optional)

</details>

<details>

<summary>Secret Protection</summary>

- **get_secret_scanning_alert** - Get secret scanning alert
  - `alertNumber`: The number of the alert. (number, required)
  - `owner`: The owner of the repository. (string, required)
  - `repo`: The name of the repository. (string, required)

- **list_secret_scanning_alerts** - List secret scanning alerts
  - `owner`: The owner of the repository. (string, required)
  - `repo`: The name of the repository. (string, required)
  - `resolution`: Filter by resolution (string, optional)
  - `secret_type`: A comma-separated list of secret types to return. All default secret patterns are returned. To return generic patterns, pass the token name(s) in the parameter. (string, optional)
  - `state`: Filter by state (string, optional)

</details>

<details>

<summary>Security Advisories</summary>

- **get_global_security_advisory** - Get a global security advisory
  - `ghsaId`: GitHub Security Advisory ID (format: GHSA-xxxx-xxxx-xxxx). (string, required)

- **list_global_security_advisories** - List global security advisories
  - `affects`: Filter advisories by affected package or version (e.g. "package1,package2@1.0.0"). (string, optional)
  - `cveId`: Filter by CVE ID. (string, optional)
  - `cwes`: Filter by Common Weakness Enumeration IDs (e.g. ["79", "284", "22"]). (string[], optional)
  - `ecosystem`: Filter by package ecosystem. (string, optional)
  - `ghsaId`: Filter by GitHub Security Advisory ID (format: GHSA-xxxx-xxxx-xxxx). (string, optional)
  - `isWithdrawn`: Whether to only return withdrawn advisories. (boolean, optional)
  - `modified`: Filter by publish or update date or date range (ISO 8601 date or range). (string, optional)
  - `published`: Filter by publish date or date range (ISO 8601 date or range). (string, optional)
  - `severity`: Filter by severity. (string, optional)
  - `type`: Advisory type. (string, optional)
  - `updated`: Filter by update date or date range (ISO 8601 date or range). (string, optional)

- **list_org_repository_security_advisories** - List org repository security advisories
  - `direction`: Sort direction. (string, optional)
  - `org`: The organization login. (string, required)
  - `sort`: Sort field. (string, optional)
  - `state`: Filter by advisory state. (string, optional)

- **list_repository_security_advisories** - List repository security advisories
  - `direction`: Sort direction. (string, optional)
  - `owner`: The owner of the repository. (string, required)
  - `repo`: The name of the repository. (string, required)
  - `sort`: Sort field. (string, optional)
  - `state`: Filter by advisory state. (string, optional)

</details>

<details>

<summary>Stargazers</summary>

- **list_starred_repositories** - List starred repositories
  - `direction`: The direction to sort the results by. (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `sort`: How to sort the results. Can be either 'created' (when the repository was starred) or 'updated' (when the repository was last pushed to). (string, optional)
  - `username`: Username to list starred repositories for. Defaults to the authenticated user. (string, optional)

- **star_repository** - Star repository
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

- **unstar_repository** - Unstar repository
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

</details>

<details>

<summary>Users</summary>

- **search_users** - Search users
  - `order`: Sort order (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `query`: User search query. Examples: 'john smith', 'location:seattle', 'followers:>100'. Search is automatically scoped to type:user. (string, required)
  - `sort`: Sort users by number of followers or repositories, or when the person joined GitHub. (string, optional)

</details>
<!-- END AUTOMATED TOOLS -->

### Additional Tools in Remote GitHub MCP Server

<details>

<summary>Copilot</summary>

-   **create_pull_request_with_copilot** - Perform task with GitHub Copilot coding agent
    -   `owner`: Repository owner. You can guess the owner, but confirm it with the user before proceeding. (string, required)
    -   `repo`: Repository name. You can guess the repository name, but confirm it with the user before proceeding. (string, required)
    -   `problem_statement`: Detailed description of the task to be performed (e.g., 'Implement a feature that does X', 'Fix bug Y', etc.) (string, required)
    -   `title`: Title for the pull request that will be created (string, required)
    -   `base_ref`: Git reference (e.g., branch) that the agent will start its work from. If not specified, defaults to the repository's default branch (string, optional)

</details>

<details>

<summary>Copilot Spaces</summary>

-   **get_copilot_space** - Get Copilot Space
    -   `owner`: The owner of the space. (string, required)
    -   `name`: The name of the space. (string, required)

-   **list_copilot_spaces** - List Copilot Spaces
</details>

<details>

<summary>GitHub Support Docs Search</summary>

-   **github_support_docs_search** - Retrieve documentation relevant to answer GitHub product and support questions. Support topics include: GitHub Actions Workflows, Authentication, GitHub Support Inquiries, Pull Request Practices, Repository Maintenance, GitHub Pages, GitHub Packages, GitHub Discussions, Copilot Spaces
    -   `query`: Input from the user about the question they need answered. This is the latest raw unedited user message. You should ALWAYS leave the user message as it is, you should never modify it. (string, required)
</details>