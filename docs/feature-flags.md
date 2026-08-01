# Feature Flags

Feature flags let you opt into experimental tool behavior on top of the default
GitHub MCP Server surface. Insiders Mode turns on a curated subset of these
flags automatically — see [Insiders Features](./insiders-features.md) for that
specific set.

For background on how flags resolve at request time, see the [resolution
section in the Insiders docs](./insiders-features.md#how-feature-flags-are-resolved).

## Enabling a flag

| Method | Remote Server | Local Server |
|--------|---------------|--------------|
| Header | `X-MCP-Features: <flag>,<flag>` | N/A |
| CLI flag | N/A | `--features=<flag>,<flag>` |
| Environment variable | N/A | `GITHUB_FEATURES=<flag>,<flag>` |

Only flags listed in
[`AllowedFeatureFlags`](../pkg/github/feature_flags.go) can be enabled by
end users. Insiders-only flags are not user-toggleable.

---

## Tools affected by each flag

The list below is regenerated from the Go source. For each user-controllable
feature flag, it lists every tool whose **inventory or input schema** differs
from the default — either because the flag introduces a new tool, or because
it selects a flag-aware variant of an existing tool. Flags that only affect
runtime behavior (such as output formatting) won't appear here.

<!-- START AUTOMATED FEATURE FLAG TOOLS -->

### `remote_mcp_ui_apps`

- **create_pull_request** - Open new pull request
  - **所需 OAuth Scopes**：`repo`
  - **MCP App UI**：`ui://github-mcp-server/pr-write`
  - `base`: Branch to merge into (string, 必需)
  - `body`: PR description (string, 可选)
  - `draft`: Create as draft PR (boolean, 可选)
  - `head`: Branch containing changes (string, 必需)
  - `maintainer_can_modify`: Allow maintainer edits (boolean, 可选)
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)
  - `reviewers`: GitHub usernames or ORG/team-slug team reviewers to request reviews from (string[], 可选)
  - `title`: PR title (string, 必需)

- **get_me** - Get my user profile
  - **MCP App UI**：`ui://github-mcp-server/get-me`
  - 无需参数

- **issue_write** - Create or update issue/pull request
  - **所需 OAuth Scopes**：`repo`
  - **MCP App UI**：`ui://github-mcp-server/issue-write`
  - `assignees`: Usernames to assign to this issue (string[], 可选)
  - `body`: Issue body content (string, 可选)
  - `duplicate_of`: Issue number that this issue is a duplicate of. Only used when state_reason is 'duplicate'. (number, 可选)
  - `issue_fields`: Issue field values to set or clear. Each item requires 'field_name' and exactly one of 'value', 'field_option_name', or 'delete: true'. (object[], 可选)
  - `issue_number`: Issue number to update (number, 可选)
  - `labels`: Labels to apply to this issue (string[], 可选)
  - `method`: Write operation to perform on a single issue.
    Options are:
    - 'create' - creates a new issue.
    - 'update' - updates an existing issue.
     (string, 必需)
  - `milestone`: Milestone number (number, 可选)
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)
  - `state`: New state (string, 可选)
  - `state_reason`: Reason for the state change. Ignored unless state is changed. (string, 可选)
  - `title`: Issue title (string, 可选)
  - `type`: Type of this issue. Only use if issue types are enabled for this repository. Use list_issue_types tool to get valid type values for this repository or its owner organization. If the repository doesn't support issue types, omit this parameter. (string, 可选)

- **ui_get** - Get UI data
  - **所需 OAuth Scopes（任一）**：`repo`, `read:org`
  - **可接受 OAuth Scopes**：`admin:org`, `read:org`, `repo`, `write:org`
  - `method`: The type of data to fetch (string, 必需)
  - `owner`: Repository owner (required for all methods) (string, 必需)
  - `repo`: Repository name (required for labels, assignees, milestones, branches, issue fields, reviewers) (string, 可选)

- **update_pull_request** - Edit pull request
  - **所需 OAuth Scopes**：`repo`
  - **MCP App UI**：`ui://github-mcp-server/pr-edit`
  - `base`: New base branch name (string, 可选)
  - `body`: New description (string, 可选)
  - `draft`: Mark pull request as draft (true) or ready for review (false) (boolean, 可选)
  - `maintainer_can_modify`: Allow maintainer edits (boolean, 可选)
  - `owner`: Repository owner (string, 必需)
  - `pullNumber`: Pull request number to update (number, 必需)
  - `repo`: Repository name (string, 必需)
  - `reviewers`: GitHub usernames or ORG/team-slug team reviewers to request reviews from (string[], 可选)
  - `state`: New state (string, 可选)
  - `title`: New title (string, 可选)

### `issues_granular`

- **add_issue_comment_reaction** - Add Reaction to Issue or Pull Request Comment
  - **所需 OAuth Scopes**：`repo`
  - `comment_id`: The issue or pull request comment ID (number, 必需)
  - `content`: The emoji reaction type (string, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `repo`: Repository name (string, 必需)

- **add_issue_reaction** - Add Reaction to Issue or Pull Request
  - **所需 OAuth Scopes**：`repo`
  - `content`: The emoji reaction type (string, 必需)
  - `issue_number`: The issue number (number, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `repo`: Repository name (string, 必需)

- **add_sub_issue** - Add Sub-Issue
  - **所需 OAuth Scopes**：`repo`
  - `issue_number`: The parent issue number (number, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `replace_parent`: If true, reparent the sub-issue if it already has a parent (boolean, 可选)
  - `repo`: Repository name (string, 必需)
  - `sub_issue_id`: The ID of the sub-issue to add. ID is not the same as issue number (number, 必需)

- **create_issue** - Create Issue
  - **所需 OAuth Scopes**：`repo`
  - `body`: Issue body content (optional) (string, 可选)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `repo`: Repository name (string, 必需)
  - `title`: Issue title (string, 必需)

- **remove_sub_issue** - Remove Sub-Issue
  - **所需 OAuth Scopes**：`repo`
  - `issue_number`: The parent issue number (number, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `repo`: Repository name (string, 必需)
  - `sub_issue_id`: The ID of the sub-issue to remove. ID is not the same as issue number (number, 必需)

- **reprioritize_sub_issue** - Reprioritize Sub-Issue
  - **所需 OAuth Scopes**：`repo`
  - `after_id`: The ID of the sub-issue to place this after (either after_id OR before_id should be specified) (number, 可选)
  - `before_id`: The ID of the sub-issue to place this before (either after_id OR before_id should be specified) (number, 可选)
  - `issue_number`: The parent issue number (number, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `repo`: Repository name (string, 必需)
  - `sub_issue_id`: The ID of the sub-issue to reorder. ID is not the same as issue number (number, 必需)

- **set_issue_fields** - Set Issue Fields
  - **所需 OAuth Scopes**：`repo`
  - `fields`: Array of issue field values to set. Each element must have a 'field_id' (string, the GraphQL node ID of the field) and exactly one value field: 'text_value' for text fields, 'number_value' for number fields, 'date_value' (ISO 8601 date string) for date fields, or 'single_select_option_id' (the GraphQL node ID of the option) for single select fields. Set 'delete' to true to remove a field value. (object[], 必需)
  - `issue_number`: The issue number to update (number, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `repo`: Repository name (string, 必需)

- **update_issue_assignees** - Update Issue Assignees
  - **所需 OAuth Scopes**：`repo`
  - `assignees`: GitHub usernames to assign to this issue. ([], 必需)
  - `issue_number`: The issue number to update (number, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `repo`: Repository name (string, 必需)

- **update_issue_body** - Update Issue Body
  - **所需 OAuth Scopes**：`repo`
  - `body`: The new body content for the issue (string, 必需)
  - `issue_number`: The issue number to update (number, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `repo`: Repository name (string, 必需)

- **update_issue_labels** - Update Issue Labels
  - **所需 OAuth Scopes**：`repo`
  - `issue_number`: The issue number to update (number, 必需)
  - `labels`: Labels to apply to this issue. ([], 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `repo`: Repository name (string, 必需)

- **update_issue_milestone** - Update Issue Milestone
  - **所需 OAuth Scopes**：`repo`
  - `issue_number`: The issue number to update (number, 必需)
  - `milestone`: The milestone number to set on the issue (integer, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `repo`: Repository name (string, 必需)

- **update_issue_state** - Update Issue State
  - **所需 OAuth Scopes**：`repo`
  - `confidence`: How confident you are in this choice. Use 'HIGH' for clear signal or explicit user request, 'MEDIUM' for reasonable inference with some ambiguity, 'LOW' for best guess with limited signal. (string, 可选)
  - `duplicate_of`: The issue number of the canonical issue this issue duplicates. Only valid when state_reason is 'duplicate'. Required when is_suggestion is true and state_reason is 'duplicate'. The issue number is resolved to a database ID before being sent to the API. (number, 可选)
  - `is_suggestion`: If true, this state change is sent to the API as a suggestion (suggest:true) rather than an applied change. Whether the change is applied or recorded as a proposal is determined by the API. (boolean, 可选)
  - `issue_number`: The issue number to update (number, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `rationale`: One concise sentence explaining what specifically about the issue led you to choose this state. State the concrete signal (e.g. 'The reported crash is fixed in v2.1' → completed). (string, 可选)
  - `repo`: Repository name (string, 必需)
  - `state`: The new state for the issue (string, 必需)
  - `state_reason`: The reason for the state change (only for closed state) (string, 可选)

- **update_issue_title** - Update Issue Title
  - **所需 OAuth Scopes**：`repo`
  - `issue_number`: The issue number to update (number, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `repo`: Repository name (string, 必需)
  - `title`: The new title for the issue (string, 必需)

- **update_issue_type** - Update Issue Type
  - **所需 OAuth Scopes**：`repo`
  - `confidence`: How confident you are in this choice. Use 'HIGH' for clear signal or explicit user request, 'MEDIUM' for reasonable inference with some ambiguity, 'LOW' for best guess with limited signal. (string, 可选)
  - `is_suggestion`: If true, this issue type change is sent to the API as a suggestion (suggest:true) rather than an applied value. Whether the type is applied or recorded as a proposal is determined by the API. (boolean, 可选)
  - `issue_number`: The issue number to update (number, 必需)
  - `issue_type`: The issue type to set (string, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `rationale`: One concise sentence explaining what specifically about the issue led you to choose this type. State the concrete signal (e.g. 'Reports a crash when saving' → bug, 'Asks for dark mode support' → feature). (string, 可选)
  - `repo`: Repository name (string, 必需)

### `pull_requests_granular`

- **add_pull_request_review_comment** - Add Pull Request Review Comment
  - **所需 OAuth Scopes**：`repo`
  - `body`: The comment body (string, 必需)
  - `line`: The line number in the diff to comment on (optional) (number, 可选)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `path`: The relative path of the file to comment on (string, 必需)
  - `pullNumber`: The pull request number (number, 必需)
  - `repo`: Repository name (string, 必需)
  - `side`: The side of the diff to comment on (optional) (string, 可选)
  - `startLine`: The start line of a multi-line comment (optional) (number, 可选)
  - `startSide`: The start side of a multi-line comment (optional) (string, 可选)
  - `subjectType`: The subject type of the comment (string, 必需)

- **add_pull_request_review_comment_reaction** - Add Pull Request Review Comment Reaction
  - **所需 OAuth Scopes**：`repo`
  - `comment_id`: The numeric pull request review comment ID. Use the number from a #discussion_r... anchor, not the GraphQL thread node ID (PRRT_...). (number, 必需)
  - `content`: The emoji reaction type (string, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `repo`: Repository name (string, 必需)

- **create_pull_request_review** - Create Pull Request Review
  - **所需 OAuth Scopes**：`repo`
  - `body`: The review body text (optional) (string, 可选)
  - `commitID`: The SHA of the commit to review (optional, defaults to latest) (string, 可选)
  - `event`: The review action to perform. If omitted, creates a pending review. (string, 可选)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `pullNumber`: The pull request number (number, 必需)
  - `repo`: Repository name (string, 必需)

- **delete_pending_pull_request_review** - Delete Pending Pull Request Review
  - **所需 OAuth Scopes**：`repo`
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `pullNumber`: The pull request number (number, 必需)
  - `repo`: Repository name (string, 必需)

- **request_pull_request_reviewers** - Request Pull Request Reviewers
  - **所需 OAuth Scopes**：`repo`
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `pullNumber`: The pull request number (number, 必需)
  - `repo`: Repository name (string, 必需)
  - `reviewers`: GitHub usernames or ORG/team-slug team reviewers to request reviews from (string[], 必需)

- **resolve_review_thread** - Resolve Review Thread
  - **所需 OAuth Scopes**：`repo`
  - `threadID`: The node ID of the review thread to resolve (e.g., PRRT_kwDOxxx) (string, 必需)

- **submit_pending_pull_request_review** - Submit Pending Pull Request Review
  - **所需 OAuth Scopes**：`repo`
  - `body`: The review body text (optional) (string, 可选)
  - `event`: The review action to perform (string, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `pullNumber`: The pull request number (number, 必需)
  - `repo`: Repository name (string, 必需)

- **unresolve_review_thread** - Unresolve Review Thread
  - **所需 OAuth Scopes**：`repo`
  - `threadID`: The node ID of the review thread to unresolve (e.g., PRRT_kwDOxxx) (string, 必需)

- **update_pull_request_body** - Update Pull Request Body
  - **所需 OAuth Scopes**：`repo`
  - `body`: The new body content for the pull request (string, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `pullNumber`: The pull request number (number, 必需)
  - `repo`: Repository name (string, 必需)

- **update_pull_request_draft_state** - Update Pull Request Draft State
  - **所需 OAuth Scopes**：`repo`
  - `draft`: Set to true to convert to draft, false to mark as ready for review (boolean, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `pullNumber`: The pull request number (number, 必需)
  - `repo`: Repository name (string, 必需)

- **update_pull_request_state** - Update Pull Request State
  - **所需 OAuth Scopes**：`repo`
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `pullNumber`: The pull request number (number, 必需)
  - `repo`: Repository name (string, 必需)
  - `state`: The new state for the pull request (string, 必需)

- **update_pull_request_title** - Update Pull Request Title
  - **所需 OAuth Scopes**：`repo`
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `pullNumber`: The pull request number (number, 必需)
  - `repo`: Repository name (string, 必需)
  - `title`: The new title for the pull request (string, 必需)

### `file_blame`

- **get_file_blame** - Get file blame information
  - **所需 OAuth Scopes**：`repo`
  - `after`: Cursor for pagination. Use the cursor from the previous response. (string, 可选)
  - `end_line`: Optional 1-based ending line of the window of interest. Must be >= start_line when both are provided. (number, 可选)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `path`: Path to the file in the repository, relative to the repository root (string, 必需)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `ref`: Git reference (branch, tag, or commit SHA). Defaults to the repository's default branch (HEAD). (string, 可选)
  - `repo`: Repository name (string, 必需)
  - `start_line`: Optional 1-based starting line of the window of interest. Only ranges overlapping [start_line, end_line] are returned, clamped to the window. (number, 可选)

### `issue_dependencies`

- **issue_dependency_read** - Read issue dependencies
  - **所需 OAuth Scopes**：`repo`
  - `issue_number`: The number of the issue (number, 必需)
  - `method`: The read operation to perform on a single issue's dependencies.
    Options are:
    1. get_blocked_by - List the issues that block this issue (this issue is blocked by them).
    2. get_blocking - List the issues that this issue blocks.
     (string, 必需)
  - `owner`: The owner of the repository (string, 必需)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `repo`: The name of the repository (string, 必需)

- **issue_dependency_write** - Change issue dependency
  - **所需 OAuth Scopes**：`repo`
  - `issue_number`: The number of the subject issue (number, 必需)
  - `method`: The action to perform.
    Options are:
    - 'add' - create the dependency relationship.
    - 'remove' - delete the dependency relationship. (string, 必需)
  - `owner`: The owner of the subject issue's repository (string, 必需)
  - `related_issue_number`: The number of the related issue to link or unlink (number, 必需)
  - `related_owner`: The owner of the related issue's repository. Defaults to 'owner' when omitted. (string, 可选)
  - `related_repo`: The name of the related issue's repository. Defaults to 'repo' when omitted. (string, 可选)
  - `repo`: The name of the subject issue's repository (string, 必需)
  - `type`: The relationship direction relative to the subject issue.
    Options are:
    - 'blocked_by' - the subject issue is blocked by the related issue.
    - 'blocking' - the subject issue blocks the related issue. (string, 必需)

<!-- END AUTOMATED FEATURE FLAG TOOLS -->
