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
  - **Required OAuth Scopes**: `repo`
  - **MCP App UI**: `ui://github-mcp-server/pr-write`
  - `base`: Branch to merge into (string, required)
  - `body`: PR description (string, optional)
  - `draft`: Create as draft PR (boolean, optional)
  - `head`: Branch containing changes (string, required)
  - `maintainer_can_modify`: Allow maintainer edits (boolean, optional)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `title`: PR title (string, required)

- **get_me** - Get my user profile
  - **MCP App UI**: `ui://github-mcp-server/get-me`
  - No parameters required

- **issue_write** - Create or update issue/pull request
  - **Required OAuth Scopes**: `repo`
  - **MCP App UI**: `ui://github-mcp-server/issue-write`
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
  - `type`: Type of this issue. Only use if issue types are enabled for this repository. Use list_issue_types tool to get valid type values for this repository or its owner organization. If the repository doesn't support issue types, omit this parameter. (string, optional)

### `remote_mcp_issue_fields`

- **issue_write** - Create or update issue/pull request
  - **Required OAuth Scopes**: `repo`
  - `assignees`: Usernames to assign to this issue (string[], optional)
  - `body`: Issue body content (string, optional)
  - `duplicate_of`: Issue number that this issue is a duplicate of. Only used when state_reason is 'duplicate'. (number, optional)
  - `issue_fields`: Issue field values to set or clear. Each item requires 'field_name' and exactly one of 'value', 'field_option_name', or 'delete: true'. (object[], optional)
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
  - `type`: Type of this issue. Only use if issue types are enabled for this repository. Use list_issue_types tool to get valid type values for this repository or its owner organization. If the repository doesn't support issue types, omit this parameter. (string, optional)

- **list_issue_fields** - List issue fields
  - **Required OAuth Scopes (any of)**: `repo`, `read:org`
  - **Accepted OAuth Scopes**: `admin:org`, `read:org`, `repo`, `write:org`
  - `owner`: The account owner of the repository or organization. The name is not case sensitive. (string, required)
  - `repo`: The name of the repository. When provided, returns fields for this specific repository (inherited from its organization). When omitted, returns org-level fields directly. (string, optional)

- **list_issues** - List issues
  - **Required OAuth Scopes**: `repo`
  - `after`: Cursor for pagination. Use the cursor from the previous response. (string, optional)
  - `direction`: Order direction. If provided, the 'orderBy' also needs to be provided. (string, optional)
  - `field_filters`: Filter by custom issue field values. Each entry takes a field_name and a value; the server looks up the field and coerces the value to its type (single-select option name, text, number, or YYYY-MM-DD date). (object[], optional)
  - `labels`: Filter by labels (string[], optional)
  - `orderBy`: Order issues by field. If provided, the 'direction' also needs to be provided. (string, optional)
  - `owner`: Repository owner (string, required)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)
  - `since`: Filter by date (ISO 8601 timestamp) (string, optional)
  - `state`: Filter by state, by default both open and closed issues are returned when not provided (string, optional)

### `issues_granular`

- **add_sub_issue** - Add Sub-Issue
  - **Required OAuth Scopes**: `repo`
  - `issue_number`: The parent issue number (number, required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `replace_parent`: If true, reparent the sub-issue if it already has a parent (boolean, optional)
  - `repo`: Repository name (string, required)
  - `sub_issue_id`: The ID of the sub-issue to add. ID is not the same as issue number (number, required)

- **create_issue** - Create Issue
  - **Required OAuth Scopes**: `repo`
  - `body`: Issue body content (optional) (string, optional)
  - `owner`: Repository owner (username or organization) (string, required)
  - `repo`: Repository name (string, required)
  - `title`: Issue title (string, required)

- **remove_sub_issue** - Remove Sub-Issue
  - **Required OAuth Scopes**: `repo`
  - `issue_number`: The parent issue number (number, required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `repo`: Repository name (string, required)
  - `sub_issue_id`: The ID of the sub-issue to remove. ID is not the same as issue number (number, required)

- **reprioritize_sub_issue** - Reprioritize Sub-Issue
  - **Required OAuth Scopes**: `repo`
  - `after_id`: The ID of the sub-issue to place this after (either after_id OR before_id should be specified) (number, optional)
  - `before_id`: The ID of the sub-issue to place this before (either after_id OR before_id should be specified) (number, optional)
  - `issue_number`: The parent issue number (number, required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `repo`: Repository name (string, required)
  - `sub_issue_id`: The ID of the sub-issue to reorder. ID is not the same as issue number (number, required)

- **set_issue_fields** - Set Issue Fields
  - **Required OAuth Scopes**: `repo`
  - `fields`: Array of issue field values to set. Each element must have a 'field_id' (string, the GraphQL node ID of the field) and exactly one value field: 'text_value' for text fields, 'number_value' for number fields, 'date_value' (ISO 8601 date string) for date fields, or 'single_select_option_id' (the GraphQL node ID of the option) for single select fields. Set 'delete' to true to remove a field value. (object[], required)
  - `issue_number`: The issue number to update (number, required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `repo`: Repository name (string, required)

- **update_issue_assignees** - Update Issue Assignees
  - **Required OAuth Scopes**: `repo`
  - `assignees`: GitHub usernames to assign to this issue (string[], required)
  - `issue_number`: The issue number to update (number, required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `repo`: Repository name (string, required)

- **update_issue_body** - Update Issue Body
  - **Required OAuth Scopes**: `repo`
  - `body`: The new body content for the issue (string, required)
  - `issue_number`: The issue number to update (number, required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `repo`: Repository name (string, required)

- **update_issue_labels** - Update Issue Labels
  - **Required OAuth Scopes**: `repo`
  - `issue_number`: The issue number to update (number, required)
  - `labels`: Labels to apply to this issue. ([], required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `repo`: Repository name (string, required)

- **update_issue_milestone** - Update Issue Milestone
  - **Required OAuth Scopes**: `repo`
  - `issue_number`: The issue number to update (number, required)
  - `milestone`: The milestone number to set on the issue (integer, required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `repo`: Repository name (string, required)

- **update_issue_state** - Update Issue State
  - **Required OAuth Scopes**: `repo`
  - `issue_number`: The issue number to update (number, required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `repo`: Repository name (string, required)
  - `state`: The new state for the issue (string, required)
  - `state_reason`: The reason for the state change (only for closed state) (string, optional)

- **update_issue_title** - Update Issue Title
  - **Required OAuth Scopes**: `repo`
  - `issue_number`: The issue number to update (number, required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `repo`: Repository name (string, required)
  - `title`: The new title for the issue (string, required)

- **update_issue_type** - Update Issue Type
  - **Required OAuth Scopes**: `repo`
  - `confidence`: How confident you are in this choice. Use 'high' for clear signal or explicit user request, 'medium' for reasonable inference with some ambiguity, 'low' for best guess with limited signal. (string, optional)
  - `is_suggestion`: If true, this issue type change is sent to the API as a suggestion (suggest:true) rather than an applied value. Whether the type is applied or recorded as a proposal is determined by the API. (boolean, optional)
  - `issue_number`: The issue number to update (number, required)
  - `issue_type`: The issue type to set (string, required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `rationale`: One concise sentence explaining what specifically about the issue led you to choose this type. State the concrete signal (e.g. 'Reports a crash when saving' → bug, 'Asks for dark mode support' → feature). (string, optional)
  - `repo`: Repository name (string, required)

### `pull_requests_granular`

- **add_pull_request_review_comment** - Add Pull Request Review Comment
  - **Required OAuth Scopes**: `repo`
  - `body`: The comment body (string, required)
  - `line`: The line number in the diff to comment on (optional) (number, optional)
  - `owner`: Repository owner (username or organization) (string, required)
  - `path`: The relative path of the file to comment on (string, required)
  - `pullNumber`: The pull request number (number, required)
  - `repo`: Repository name (string, required)
  - `side`: The side of the diff to comment on (optional) (string, optional)
  - `startLine`: The start line of a multi-line comment (optional) (number, optional)
  - `startSide`: The start side of a multi-line comment (optional) (string, optional)
  - `subjectType`: The subject type of the comment (string, required)

- **create_pull_request_review** - Create Pull Request Review
  - **Required OAuth Scopes**: `repo`
  - `body`: The review body text (optional) (string, optional)
  - `commitID`: The SHA of the commit to review (optional, defaults to latest) (string, optional)
  - `event`: The review action to perform. If omitted, creates a pending review. (string, optional)
  - `owner`: Repository owner (username or organization) (string, required)
  - `pullNumber`: The pull request number (number, required)
  - `repo`: Repository name (string, required)

- **delete_pending_pull_request_review** - Delete Pending Pull Request Review
  - **Required OAuth Scopes**: `repo`
  - `owner`: Repository owner (username or organization) (string, required)
  - `pullNumber`: The pull request number (number, required)
  - `repo`: Repository name (string, required)

- **request_pull_request_reviewers** - Request Pull Request Reviewers
  - **Required OAuth Scopes**: `repo`
  - `owner`: Repository owner (username or organization) (string, required)
  - `pullNumber`: The pull request number (number, required)
  - `repo`: Repository name (string, required)
  - `reviewers`: GitHub usernames or ORG/team-slug team reviewers to request reviews from (string[], required)

- **resolve_review_thread** - Resolve Review Thread
  - **Required OAuth Scopes**: `repo`
  - `threadID`: The node ID of the review thread to resolve (e.g., PRRT_kwDOxxx) (string, required)

- **submit_pending_pull_request_review** - Submit Pending Pull Request Review
  - **Required OAuth Scopes**: `repo`
  - `body`: The review body text (optional) (string, optional)
  - `event`: The review action to perform (string, required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `pullNumber`: The pull request number (number, required)
  - `repo`: Repository name (string, required)

- **unresolve_review_thread** - Unresolve Review Thread
  - **Required OAuth Scopes**: `repo`
  - `threadID`: The node ID of the review thread to unresolve (e.g., PRRT_kwDOxxx) (string, required)

- **update_pull_request_body** - Update Pull Request Body
  - **Required OAuth Scopes**: `repo`
  - `body`: The new body content for the pull request (string, required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `pullNumber`: The pull request number (number, required)
  - `repo`: Repository name (string, required)

- **update_pull_request_draft_state** - Update Pull Request Draft State
  - **Required OAuth Scopes**: `repo`
  - `draft`: Set to true to convert to draft, false to mark as ready for review (boolean, required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `pullNumber`: The pull request number (number, required)
  - `repo`: Repository name (string, required)

- **update_pull_request_state** - Update Pull Request State
  - **Required OAuth Scopes**: `repo`
  - `owner`: Repository owner (username or organization) (string, required)
  - `pullNumber`: The pull request number (number, required)
  - `repo`: Repository name (string, required)
  - `state`: The new state for the pull request (string, required)

- **update_pull_request_title** - Update Pull Request Title
  - **Required OAuth Scopes**: `repo`
  - `owner`: Repository owner (username or organization) (string, required)
  - `pullNumber`: The pull request number (number, required)
  - `repo`: Repository name (string, required)
  - `title`: The new title for the pull request (string, required)

### `file_blame`

- **get_file_blame** - Get file blame information
  - **Required OAuth Scopes**: `repo`
  - `after`: Cursor for pagination. Use the cursor from the previous response. (string, optional)
  - `end_line`: Optional 1-based ending line of the window of interest. Must be >= start_line when both are provided. (number, optional)
  - `owner`: Repository owner (username or organization) (string, required)
  - `path`: Path to the file in the repository, relative to the repository root (string, required)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `ref`: Git reference (branch, tag, or commit SHA). Defaults to the repository's default branch (HEAD). (string, optional)
  - `repo`: Repository name (string, required)
  - `start_line`: Optional 1-based starting line of the window of interest. Only ranges overlapping [start_line, end_line] are returned, clamped to the window. (number, optional)

<!-- END AUTOMATED FEATURE FLAG TOOLS -->
