# Tool Authentication Requirements

This document provides a comprehensive reference for the authentication requirements of each tool in the GitHub MCP Server. It covers both OAuth scopes (for classic personal access tokens and OAuth apps) and fine-grained permissions (for fine-grained personal access tokens).

## Quick Reference

- **OAuth Scopes**: Used by OAuth apps and classic Personal Access Tokens (PATs)
- **Fine-Grained Permissions**: Used by fine-grained Personal Access Tokens

For OAuth scopes documentation, see: [Scopes for OAuth Apps](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps)

For fine-grained permission documentation, see: [Permissions for Fine-Grained PATs](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens)

## Scope and Permission Hierarchy

### OAuth Scope Hierarchy

Some OAuth scopes include access to other scopes. If you have a parent scope, you automatically have access to all child scopes:

| Parent Scope | Includes |
|-------------|----------|
| `repo` | `repo:status`, `repo_deployment`, `public_repo`, `repo:invite`, `security_events` |
| `user` | `read:user`, `user:email`, `user:follow` |
| `admin:org` | `write:org`, `read:org` |
| `write:org` | `read:org` |
| `admin:repo_hook` | `write:repo_hook`, `read:repo_hook` |
| `write:repo_hook` | `read:repo_hook` |
| `admin:public_key` | `write:public_key`, `read:public_key` |
| `write:public_key` | `read:public_key` |
| `admin:gpg_key` | `write:gpg_key`, `read:gpg_key` |
| `write:gpg_key` | `read:gpg_key` |
| `project` | `read:project` |
| `write:packages` | `read:packages` |

### Fine-Grained Permission Levels

Fine-grained permissions have three access levels:

| Level | Description |
|-------|-------------|
| `read` | Read-only access to the resource |
| `write` | Read and write access to the resource |
| `admin` | Full administrative access to the resource |

Write access typically includes read access, and admin access typically includes both read and write access.

---

## Tools by Category

### Repository Tools

| Tool | OAuth Scope | Fine-Grained Permission |
|------|-------------|------------------------|
| `get_file_contents` | `repo` | `contents:read` |
| `create_or_update_file` | `repo` | `contents:write` |
| `delete_file` | `repo` | `contents:write` |
| `push_files` | `repo` | `contents:write` |
| `create_repository` | `repo` | `administration:write` |
| `fork_repository` | `repo` | `contents:read`, `administration:write` |
| `create_branch` | `repo` | `contents:write` |
| `list_branches` | `repo` | `contents:read` |
| `list_commits` | `repo` | `contents:read` |
| `get_commit` | `repo` | `contents:read` |
| `list_tags` | `repo` | `contents:read` |
| `get_tag` | `repo` | `contents:read` |
| `list_releases` | `repo` | `contents:read` |
| `get_latest_release` | `repo` | `contents:read` |
| `get_release_by_tag` | `repo` | `contents:read` |
| `star_repository` | `public_repo` | `starring:write` |
| `unstar_repository` | `public_repo` | `starring:write` |
| `list_starred_repositories` | *(none)* | `starring:read` |
| `get_repository_tree` | `repo` | `contents:read` |

### Issue Tools

| Tool | OAuth Scope | Fine-Grained Permission |
|------|-------------|------------------------|
| `list_issues` | `repo` | `issues:read` |
| `get_issue` | `repo` | `issues:read` |
| `create_issue` | `repo` | `issues:write` |
| `update_issue` | `repo` | `issues:write` |
| `add_issue_comment` | `repo` | `issues:write` |
| `list_issue_comments` | `repo` | `issues:read` |
| `search_issues` | `repo` | `issues:read` |
| `list_issue_types` | `read:org` | `issues:read` |
| `assign_copilot_to_issue` | `repo` | `issues:write` |

### Pull Request Tools

| Tool | OAuth Scope | Fine-Grained Permission |
|------|-------------|------------------------|
| `list_pull_requests` | `repo` | `pull_requests:read` |
| `get_pull_request` | `repo` | `pull_requests:read` |
| `create_pull_request` | `repo` | `pull_requests:write` |
| `update_pull_request` | `repo` | `pull_requests:write` |
| `merge_pull_request` | `repo` | `contents:write`, `pull_requests:write` |
| `list_pull_request_commits` | `repo` | `pull_requests:read` |
| `get_pull_request_diff` | `repo` | `pull_requests:read` |
| `get_pull_request_files` | `repo` | `pull_requests:read` |
| `update_pull_request_branch` | `repo` | `contents:write`, `pull_requests:write` |
| `list_pull_request_reviews` | `repo` | `pull_requests:read` |
| `create_pull_request_review` | `repo` | `pull_requests:write` |
| `add_pull_request_review_comment` | `repo` | `pull_requests:write` |
| `request_copilot_review` | `repo` | `pull_requests:write` |
| `get_pull_request_review` | `repo` | `pull_requests:read` |
| `get_pull_request_comments` | `repo` | `pull_requests:read` |
| `create_pending_pull_request_review` | `repo` | `pull_requests:write` |
| `submit_pending_pull_request_review` | `repo` | `pull_requests:write` |
| `delete_pending_pull_request_review` | `repo` | `pull_requests:write` |

### Git Tools

| Tool | OAuth Scope | Fine-Grained Permission |
|------|-------------|------------------------|
| `create_git_tag` | `repo` | `contents:write` |
| `create_tree` | `repo` | `contents:write` |

### Actions Tools

| Tool | OAuth Scope | Fine-Grained Permission |
|------|-------------|------------------------|
| `list_workflows` | `repo` | `actions:read` |
| `list_workflow_runs` | `repo` | `actions:read` |
| `get_workflow_run` | `repo` | `actions:read` |
| `get_workflow_run_logs` | `repo` | `actions:read` |
| `run_workflow` | `repo` | `actions:write` |
| `cancel_workflow_run` | `repo` | `actions:write` |
| `rerun_workflow` | `repo` | `actions:write` |
| `rerun_failed_jobs` | `repo` | `actions:write` |
| `list_workflow_jobs` | `repo` | `actions:read` |
| `get_job_logs` | `repo` | `actions:read` |
| `list_workflow_run_artifacts` | `repo` | `actions:read` |
| `download_workflow_run_artifact` | `repo` | `actions:read` |
| `get_workflow_run_usage` | `repo` | `actions:read` |

### Label Tools

| Tool | OAuth Scope | Fine-Grained Permission |
|------|-------------|------------------------|
| `list_labels` | `repo` | `issues:read` or `pull_requests:read` |
| `get_label` | `repo` | `issues:read` or `pull_requests:read` |
| `label_write` | `repo` | `issues:write` or `pull_requests:write` |

### Notification Tools

| Tool | OAuth Scope | Fine-Grained Permission |
|------|-------------|------------------------|
| `list_notifications` | `notifications` | *N/A - Requires classic token* |
| `get_notification_details` | `notifications` | *N/A - Requires classic token* |
| `dismiss_notification` | `notifications` | *N/A - Requires classic token* |
| `mark_all_notifications_read` | `notifications` | *N/A - Requires classic token* |
| `manage_notification_subscription` | `notifications` | *N/A - Requires classic token* |
| `manage_repository_notification_subscription` | `notifications` | *N/A - Requires classic token* |

> **Note**: Notification endpoints are not available with fine-grained PATs. Use a classic PAT with the `notifications` scope.

### Discussion Tools

| Tool | OAuth Scope | Fine-Grained Permission |
|------|-------------|------------------------|
| `list_discussions` | `repo` | `discussions:read` |
| `get_discussion` | `repo` | `discussions:read` |
| `list_discussion_categories` | `repo` | `discussions:read` |
| `get_discussion_comments` | `repo` | `discussions:read` |

### Project Tools

| Tool | OAuth Scope | Fine-Grained Permission |
|------|-------------|------------------------|
| `list_projects` | `read:project` | `organization_projects:read` |
| `get_project` | `read:project` | `organization_projects:read` |
| `list_project_items` | `read:project` | `organization_projects:read` |
| `get_project_item` | `read:project` | `organization_projects:read` |
| `list_project_fields` | `read:project` | `organization_projects:read` |
| `update_project_item` | `project` | `organization_projects:write` |
| `create_project_draft` | `project` | `organization_projects:write` |
| `add_project_item` | `project` | `organization_projects:write` |
| `delete_project_item` | `project` | `organization_projects:write` |

### Gist Tools

| Tool | OAuth Scope | Fine-Grained Permission |
|------|-------------|------------------------|
| `list_gists` | *(none)* | `gists:read` |
| `get_gist` | *(none)* | `gists:read` |
| `create_gist` | `gist` | `gists:write` |
| `update_gist` | `gist` | `gists:write` |

### Search Tools

| Tool | OAuth Scope | Fine-Grained Permission |
|------|-------------|------------------------|
| `search_code` | `repo` | `contents:read` |
| `search_issues` | `repo` | `issues:read` |
| `search_users` | `repo` | `metadata:read` |
| `search_repositories` | `repo` | `metadata:read` |

### Security Tools

#### Code Scanning

| Tool | OAuth Scope | Fine-Grained Permission |
|------|-------------|------------------------|
| `list_code_scanning_alerts` | `security_events` | `code_scanning_alerts:read` |
| `get_code_scanning_alert` | `security_events` | `code_scanning_alerts:read` |
| `update_code_scanning_alert` | `security_events` | `code_scanning_alerts:write` |

#### Secret Scanning

| Tool | OAuth Scope | Fine-Grained Permission |
|------|-------------|------------------------|
| `list_secret_scanning_alerts` | `security_events` | `secret_scanning_alerts:read` |
| `get_secret_scanning_alert` | `security_events` | `secret_scanning_alerts:read` |

#### Dependabot

| Tool | OAuth Scope | Fine-Grained Permission |
|------|-------------|------------------------|
| `list_dependabot_alerts` | `repo` | `dependabot_alerts:read` |
| `get_dependabot_alert` | `repo` | `dependabot_alerts:read` |
| `update_dependabot_alert` | `repo` | `dependabot_alerts:write` |

#### Security Advisories

| Tool | OAuth Scope | Fine-Grained Permission |
|------|-------------|------------------------|
| `list_repository_security_advisories` | `repo` | `repository_security_advisories:read` |
| `get_global_security_advisory` | *(none)* | *(none - public data)* |
| `list_global_security_advisories` | *(none)* | *(none - public data)* |

### Context Tools

| Tool | OAuth Scope | Fine-Grained Permission |
|------|-------------|------------------------|
| `get_me` | *(none)* | `metadata:read` |
| `list_teams` | `read:org` | `members:read` |
| `get_team_members` | `read:org` | `members:read` |

### Dynamic Tools (Meta-tools)

These tools are internal to the MCP server and don't call GitHub APIs:

| Tool | OAuth Scope | Fine-Grained Permission |
|------|-------------|------------------------|
| `enable_toolset` | *(none)* | *(none)* |
| `list_available_toolsets` | *(none)* | *(none)* |
| `get_toolset_tools` | *(none)* | *(none)* |

---

## Minimum Required Scopes by Use Case

### Read-Only Access

If you only need to read data (no modifications):

**OAuth Scopes:**
- `repo` - For private repositories
- `public_repo` - For public repositories only
- `read:org` - For organization and team information
- `read:project` - For project boards

**Fine-Grained Permissions:**
- `contents:read`
- `issues:read`
- `pull_requests:read`
- `actions:read`
- `metadata:read`

### Full Development Workflow

For a typical development workflow (read, write, manage PRs and issues):

**OAuth Scopes:**
- `repo` - Covers most repository operations
- `notifications` - If using notification tools
- `project` - If using project boards

**Fine-Grained Permissions:**
- `contents:write`
- `issues:write`
- `pull_requests:write`
- `actions:write`
- `metadata:read`

### Security Scanning

For security-related tools:

**OAuth Scopes:**
- `security_events` - For code scanning and secret scanning
- `repo` - For Dependabot alerts (included in `repo`)

**Fine-Grained Permissions:**
- `code_scanning_alerts:read` or `write`
- `secret_scanning_alerts:read`
- `dependabot_alerts:read` or `write`

---

## Notes

1. **Metadata Permission**: The `metadata:read` permission is automatically granted for all repositories that a fine-grained PAT has access to.

2. **Private vs Public Repositories**: The `repo` scope covers both public and private repositories. Use `public_repo` if you only need access to public repositories with OAuth apps.

3. **Organization Permissions**: Some tools require organization-level permissions (`read:org`, `write:org`, or `admin:org`), which are separate from repository permissions.

4. **Notification Limitations**: Notification endpoints are not available with fine-grained PATs. You must use a classic PAT with the `notifications` scope for notification tools.

5. **Copilot Tools**: The `assign_copilot_to_issue` and `request_copilot_review` tools require `repo` scope and work with repositories where Copilot is enabled.
