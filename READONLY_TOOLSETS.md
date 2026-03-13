# Read-Only Mode Toolsets

When running the GitHub MCP Server with `--read-only` flag or `GITHUB_READ_ONLY=true`, only tools marked as read-only are available.

## Available Toolsets in Read-Only Mode

### context
**Description:** Tools that provide context about the current user and GitHub context
**Read-only tools:**
- `get_me` - Get authenticated user details
- `get_context` - Get current GitHub context information
- `get_token_scopes` - Get OAuth token scopes

### repos (repositories)
**Description:** GitHub Repository related tools
**Read-only tools:**
- `get_file_contents` - Read file contents from repositories
- `list_branches` - List repository branches
- `list_commits` - List commits
- `list_tags` - List repository tags
- `list_releases` - List releases
- `get_latest_release` - Get latest release
- `get_release_by_tag` - Get specific release
- `search_repositories` - Search for repositories
- `search_code` - Search code across repositories

### git
**Description:** Low-level Git operations
**Read-only tools:**
- `get_commit` - Get commit details
- `get_tag` - Get tag information

### issues
**Description:** GitHub Issues related tools
**Read-only tools:**
- `list_issues` - List issues
- `issue_read` (get) - Get issue details
- `issue_read` (get_comments) - Get issue comments
- `issue_read` (get_labels) - Get issue labels
- `search_issues` - Search issues

### pull_requests
**Description:** GitHub Pull Request related tools
**Read-only tools:**
- `list_pull_requests` - List pull requests
- `pull_request_read` (get) - Get PR details
- `pull_request_read` (get_diff) - Get PR diff
- `pull_request_read` (get_status) - Get PR status
- `pull_request_read` (get_files) - Get PR files
- `pull_request_read` (get_review_comments) - Get review comments
- `pull_request_read` (get_reviews) - Get reviews
- `pull_request_read` (get_comments) - Get PR comments
- `pull_request_read` (get_check_runs) - Get check runs
- `search_pull_requests` - Search pull requests

### users
**Description:** GitHub User related tools
**Read-only tools:**
- `search_users` - Search for users
- `get_team_members` - Get team member list
- `get_teams` - Get user's teams

### actions
**Description:** GitHub Actions workflows and CI/CD operations
**Read-only tools:**
- `actions_list` - List workflow runs
- `actions_get` - Get workflow run details
- `get_job_logs` - Get job logs

### code_security
**Description:** Code security tools (Code Scanning)
**Read-only tools:**
- `get_code_scanning_alert` - Get code scanning alert
- `list_code_scanning_alerts` - List code scanning alerts

### dependabot
**Description:** Dependabot tools
**Read-only tools:**
- `get_dependabot_alert` - Get Dependabot alert
- `list_dependabot_alerts` - List Dependabot alerts

### notifications
**Description:** GitHub Notifications
**Read-only tools:**
- `list_notifications` - List notifications
- `get_notification_details` - Get notification details

### discussions
**Description:** GitHub Discussions
**Read-only tools:**
- `get_discussion` - Get discussion details
- `get_discussion_comments` - Get discussion comments
- `list_discussions` - List discussions
- `search_discussions` - Search discussions

### gists
**Description:** GitHub Gist tools
**Read-only tools:**
- `get_gist` - Get gist details
- `list_gists` - List gists

### security_advisories
**Description:** Security advisories
**Read-only tools:**
- `get_global_security_advisory` - Get global security advisory
- `list_global_security_advisories` - List global security advisories

### projects
**Description:** GitHub Projects
**Read-only tools:**
- `get_project` - Get project details
- `list_projects` - List projects

### stargazers
**Description:** GitHub Stargazers
**Read-only tools:**
- `list_stargazers` - List repository stargazers

### dynamic
**Description:** Toolset discovery and management
**Read-only tools:**
- `list_available_toolsets` - List all available toolsets
- `get_toolset_tools` - Get tools in a specific toolset
- `enable_toolset` - Enable additional toolsets (read-only operation)

### labels
**Description:** Repository labels
**Read-only tools:**
- `get_label` - Get label details
- `list_labels` - List repository labels

## Toolsets with NO Read-Only Tools

These toolsets are completely unavailable in read-only mode:
- `orgs` - Organization management (all write operations)
- `secret_protection` - Secret scanning (all write operations)
- `copilot` - Copilot operations (all write operations)

## Usage

Enable specific read-only toolsets:
```bash
docker run -e GITHUB_PERSONAL_ACCESS_TOKEN=token \
  -e GITHUB_READ_ONLY=true \
  -e GITHUB_TOOLSETS=repos,issues,pull_requests \
  herterkairos/github-mcp-server
```

Or enable all toolsets in read-only mode:
```bash
docker run -e GITHUB_PERSONAL_ACCESS_TOKEN=token \
  -e GITHUB_READ_ONLY=true \
  herterkairos/github-mcp-server
```
