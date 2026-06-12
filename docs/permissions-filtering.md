# Fine-Grained Permission Filtering

The GitHub MCP Server records the **fine-grained permission** each tool needs. This lets a consumer hide tools that the caller's token (a fine-grained PAT or GitHub App installation token) is not authorized to use — mirroring the way [PAT scope filtering](./scope-filtering.md) works for classic-PAT OAuth scopes.

> **Note:** This subsystem is **dormant in the OSS server**. The OSS server has no source for a caller's granted permissions, so it never hides tools on this basis. The permission requirements are declarative metadata published here as the public source of truth; a consumer such as the [remote MCP server](./remote-server.md) supplies the granted-permission set to activate filtering.

## How It Works

Each tool may declare a `RequiredPermissions` requirement built from the typed permission catalog in [`pkg/permissions`](../pkg/permissions). A requirement is an **OR of AND-sets** of `permission:level` pairs:

- A single-endpoint tool collapses to one pair, e.g. `issues:write`.
- A tool that calls multiple endpoints **ANDs** their requirements together.
- Where an endpoint accepts one of several permissions, those alternatives are **ORed**.

Permission levels form an ordered lattice: `read < write < admin`. A grant of `write` therefore satisfies a `read` requirement.

A tool with **no** declared requirement (the zero value) is **ungated** and always shown.

## Permission Catalog

The catalog of permission names, their resource scope (repository, organization, or account), and their valid levels is **generated** from the public [`github/rest-api-description`](https://github.com/github/rest-api-description) `app-permissions` schema. It is regenerated with:

```bash
go generate ./pkg/permissions
```

This is the only source consulted — the catalog contains exclusively public data that also appears in the REST API documentation and the `X-Accepted-GitHub-Permissions` response header. Enterprise permissions are excluded, since they are not relevant to repository/organization MCP tooling.

## Token Types

| Token type | Permission handling |
|------------|---------------------|
| **Fine-grained PAT** (`github_pat_`) | A consumer can hide tools whose required permission the token lacks; the GitHub API still enforces permissions at call time |
| **GitHub App** (`ghs_`) | Same as fine-grained PAT — permissions come from the app installation |
| **Classic PAT** (`ghp_`) | Uses OAuth [scope filtering](./scope-filtering.md) instead |

## Inspecting Tool Requirements

List the per-tool fine-grained permission requirements with the CLI:

```bash
script/list-permissions            # human-readable text
script/list-permissions --format json
script/list-permissions --format summary
```

The generated table below is produced by `script/generate-docs` and lists every tool that currently declares a requirement.

## Tool Permission Requirements

<!-- START AUTOMATED PERMISSIONS -->
| Toolset | Tool | Required Permissions (fine-grained) |
|---------|------|-------------------------------------|
| `actions` | `actions_get` | `actions:read` |
| `actions` | `actions_list` | `actions:read` |
| `actions` | `actions_run_trigger` | `actions:write` |
| `actions` | `get_job_logs` | `actions:read` |
| `code_security` | `get_code_scanning_alert` | `security_events:read` |
| `code_security` | `list_code_scanning_alerts` | `security_events:read` |
| `context` | `get_team_members` | `members:read` |
| `context` | `get_teams` | `members:read` |
| `dependabot` | `get_dependabot_alert` | `vulnerability_alerts:read` |
| `dependabot` | `list_dependabot_alerts` | `vulnerability_alerts:read` |
| `discussions` | `discussion_comment_write` | `discussions:write` |
| `discussions` | `get_discussion_comments` | `discussions:read` |
| `discussions` | `get_discussion` | `discussions:read` |
| `discussions` | `list_discussion_categories` | `discussions:read` |
| `discussions` | `list_discussions` | `discussions:read` |
| `git` | `get_repository_tree` | `contents:read` |
| `issues` | `add_issue_comment` | `issues:write` |
| `issues` | `get_label` | `issues:read` |
| `issues` | `issue_read` | `issues:read` |
| `issues` | `issue_write` | `issues:write` |
| `issues` | `list_issues` | `issues:read` |
| `issues` | `sub_issue_write` | `issues:write` |
| `labels` | `get_label` | `issues:read` |
| `labels` | `label_write` | `issues:write` |
| `labels` | `list_label` | `issues:read` |
| `pull_requests` | `add_comment_to_pending_review` | `pull_requests:write` |
| `pull_requests` | `add_reply_to_pull_request_comment` | `pull_requests:write` |
| `pull_requests` | `create_pull_request` | `pull_requests:write` |
| `pull_requests` | `list_pull_requests` | `pull_requests:read` |
| `pull_requests` | `merge_pull_request` | `contents:write AND pull_requests:write` |
| `pull_requests` | `pull_request_read` | `pull_requests:read` |
| `pull_requests` | `pull_request_review_write` | `pull_requests:write` |
| `pull_requests` | `update_pull_request_branch` | `contents:write AND pull_requests:write` |
| `pull_requests` | `update_pull_request` | `pull_requests:write` |
| `repos` | `create_branch` | `contents:write` |
| `repos` | `create_or_update_file` | `contents:write` |
| `repos` | `delete_file` | `contents:write` |
| `repos` | `get_commit` | `contents:read` |
| `repos` | `get_file_contents` | `contents:read` |
| `repos` | `get_latest_release` | `contents:read` |
| `repos` | `get_release_by_tag` | `contents:read` |
| `repos` | `get_tag` | `contents:read` |
| `repos` | `list_branches` | `contents:read` |
| `repos` | `list_commits` | `contents:read` |
| `repos` | `list_releases` | `contents:read` |
| `repos` | `list_repository_collaborators` | `administration:read` |
| `repos` | `list_tags` | `contents:read` |
| `repos` | `push_files` | `contents:write` |
| `secret_protection` | `get_secret_scanning_alert` | `secret_scanning_alerts:read` |
| `secret_protection` | `list_secret_scanning_alerts` | `secret_scanning_alerts:read` |
| `stargazers` | `list_starred_repositories` | `starring:read` |
| `stargazers` | `star_repository` | `starring:write` |
| `stargazers` | `unstar_repository` | `starring:write` |
<!-- END AUTOMATED PERMISSIONS -->

## Related Documentation

- [PAT Scope Filtering](./scope-filtering.md)
- [Server Configuration Guide](./server-configuration.md)
- [GitHub fine-grained PAT permissions](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
