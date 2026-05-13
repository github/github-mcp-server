# Skill Resources for Progressive Tool Discovery

This document describes how the GitHub MCP Server uses **skill resources** to enable progressive tool discovery — letting clients load only the tools and guidance relevant to the current task, rather than exposing everything at once.

## Background: From Server Instructions to Skills

Earlier versions of this server injected a single large system prompt (server instructions) covering all tools into every MCP session. This approach had drawbacks:

- **Context waste**: Every session loaded guidance for all tools, regardless of what the user needed.
- **No progressive disclosure**: Clients couldn't selectively load tool subsets based on the task at hand.
- **Weak tool alignment**: Bold markdown references (e.g., `**search_repositories**`) didn't reliably trigger models to call the actual MCP tools.

This version **replaces server instructions with skill resources** — structured SKILL.md documents served via `skill://` URIs. Each skill covers a specific workflow, lists exactly which tools it needs, and provides targeted guidance. Clients that support skills can progressively discover and load only what's relevant.

For the broader design rationale, see:
- [Progressive Tool Discovery](https://github.com/SamMorrowDrums/mcpi/blob/main/docs/progressive-tool-discovery.md)
- [Skills Mechanism](https://github.com/SamMorrowDrums/mcpi-ext/blob/main/docs/skills.md)
- [Server Developer Guide](https://github.com/SamMorrowDrums/mcpi-ext/blob/main/docs/server-developer-guide.md)
- [Skills-as-Groups Proposal](https://github.com/modelcontextprotocol/experimental-ext-grouping/pull/13)

## How It Works

### Skill Structure

Each skill is registered as an MCP resource at `skill://github/{name}/SKILL.md` with the MIME type `text/markdown`. The content is a Markdown document with YAML frontmatter:

```yaml
---
name: review-pr
description: Conduct a thorough code review of a pull request. Use when reviewing someone else's PR, checking code changes, leaving review comments, approving or requesting changes.
allowed-tools:
  - pull_request_read
  - get_file_contents
  - search_code
  - create_pull_request_review
  - add_pull_request_review_comment
  - add_comment_to_pending_review
  - submit_pending_pull_request_review
  - delete_pending_pull_request_review
  - add_reply_to_pull_request_comment
  - resolve_review_thread
  - unresolve_review_thread
---

# Review Pull Request

You are reviewing someone else's PR. Be thorough, constructive, and decisive.

## Available Tools
- `pull_request_read` — get diff, files, status, review comments, check runs
- `get_file_contents` / `search_code` — read context beyond the diff
...
```

### Frontmatter Fields

| Field | Purpose |
|-------|---------|
| `name` | Skill identifier, used in the URI path |
| `description` | What the skill does AND when to use it — this is the primary mechanism clients use to decide whether to load a skill |
| `allowed-tools` | MCP tool names this skill gates. When a client loads the skill, these tools become active |

### Client Flow

1. Client calls `resources/list` → sees all `skill://github/*/SKILL.md` resources with descriptions.
2. Client reads the description to decide if a skill is relevant to the current task.
3. Client calls `resources/read` on the skill URI → gets the full SKILL.md content.
4. Client activates the tools listed in `allowed-tools` and uses the body as workflow guidance.

### Implementation

Skills are defined in `pkg/github/skill_resources.go` as `skillDefinition` structs:

```go
type skillDefinition struct {
    name         string
    description  string
    allowedTools []string
    body         string
}
```

The `buildSkillContent()` function assembles the YAML frontmatter and body into a SKILL.md document. `RegisterSkillResources()` registers all skills as MCP resources at server startup.

## Available Skills

The server defines 27 workflow-oriented skills. Skills map to **user intents** (what the user wants to accomplish), not API domains. Tools overlap across skills based on workflow needs.

### PR Workflows

| Skill | Description | Key Tools |
|-------|-------------|-----------|
| `create-pr` | Create a well-structured pull request | `create_pull_request`, `get_file_contents`, `push_files`, `request_pull_request_reviewers` |
| `review-pr` | Review someone else's PR | `pull_request_read`, `create_pull_request_review`, `add_pull_request_review_comment`, `submit_pending_pull_request_review` |
| `self-review-pr` | Self-check your own PR before requesting review | `pull_request_read`, `actions_get`, `get_job_logs`, `update_pull_request` |
| `address-pr-feedback` | Handle review comments and push fixes | `pull_request_read`, `add_reply_to_pull_request_comment`, `resolve_review_thread`, `push_files` |
| `merge-pr` | Get a PR to merge-ready state and merge it | `pull_request_read`, `merge_pull_request`, `update_pull_request_branch`, `update_pull_request_draft_state` |

### Issue Workflows

| Skill | Description | Key Tools |
|-------|-------------|-----------|
| `triage-issues` | Categorize, deduplicate, and prioritize issues | `list_issues`, `search_issues`, `list_issue_types`, `update_issue_labels`, `update_issue_state` |
| `create-issue` | Create well-structured, searchable issues | `create_issue`, `search_issues`, `list_issue_types`, `get_file_contents` |
| `manage-sub-issues` | Break down large issues into sub-tasks | `issue_read`, `create_issue`, `add_sub_issue`, `reprioritize_sub_issue` |

### CI/CD

| Skill | Description | Key Tools |
|-------|-------------|-----------|
| `debug-ci` | Investigate failing GitHub Actions workflows | `actions_get`, `get_job_logs`, `actions_list`, `get_file_contents` |
| `trigger-workflow` | Run, rerun, or cancel workflow runs | `actions_run_trigger`, `actions_get`, `actions_list` |

### Security

| Skill | Description | Key Tools |
|-------|-------------|-----------|
| `security-audit` | Review code scanning, secret, and dependency alerts | `list_code_scanning_alerts`, `list_secret_scanning_alerts`, `list_dependabot_alerts`, `search_code` |
| `fix-dependabot` | Handle vulnerable dependency alerts | `list_dependabot_alerts`, `get_dependabot_alert`, `search_pull_requests` |
| `research-vulnerability` | Query the GitHub Advisory Database | `list_global_security_advisories`, `get_global_security_advisory` |

### Code Exploration

| Skill | Description | Key Tools |
|-------|-------------|-----------|
| `explore-repo` | Understand an unfamiliar codebase | `get_repository_tree`, `get_file_contents`, `search_code`, `list_commits` |
| `search-code` | Find code patterns across GitHub | `search_code`, `search_repositories`, `get_file_contents` |
| `trace-history` | Understand why code changed via commits and PRs | `list_commits`, `get_commit`, `search_pull_requests`, `pull_request_read` |

### Project Management

| Skill | Description | Key Tools |
|-------|-------------|-----------|
| `manage-project` | Track and update GitHub Projects (v2) items | `projects_list`, `projects_get`, `projects_write` |
| `handle-notifications` | Process GitHub notifications efficiently | `list_notifications`, `get_notification_details`, `dismiss_notification` |
| `prepare-release` | Compile release notes from commits and PRs | `list_releases`, `get_latest_release`, `list_commits`, `search_pull_requests` |

### Repository Management

| Skill | Description | Key Tools |
|-------|-------------|-----------|
| `manage-repo` | Create repos, manage branches, push files | `create_repository`, `fork_repository`, `create_branch`, `push_files` |
| `manage-labels` | Set up and maintain a label scheme | `list_labels`, `label_write`, `search_issues` |

### Collaboration

| Skill | Description | Key Tools |
|-------|-------------|-----------|
| `contribute-oss` | Fork, branch, and submit PRs to external repos | `fork_repository`, `create_branch`, `push_files`, `create_pull_request` |
| `browse-discussions` | Read and explore GitHub Discussions | `list_discussions`, `get_discussion`, `get_discussion_comments` |
| `delegate-to-copilot` | Assign Copilot to issues and request reviews | `assign_copilot_to_issue`, `request_copilot_review` |

### Discovery

| Skill | Description | Key Tools |
|-------|-------------|-----------|
| `get-context` | Understand the current user and permissions | `get_me`, `get_teams`, `get_team_members` |
| `discover-github` | Search for users, orgs, and repositories | `search_users`, `search_orgs`, `search_repositories`, `star_repository` |
| `share-snippet` | Create and manage code snippets via Gists | `create_gist`, `update_gist`, `list_gists`, `get_gist` |

## Design Decisions

### Workflow-Oriented, Not Toolset-Oriented

Skills map to what users want to accomplish, not GitHub API domains. For example, there's no "pull requests" skill — instead there are `review-pr`, `self-review-pr`, `create-pr`, `address-pr-feedback`, and `merge-pr`, each with different tools and different guidance. Tools overlap across skills based on workflow needs.

### Identity-Aware

Different skills exist for different perspectives on the same resource. `review-pr` assumes you're reviewing someone else's work (be thorough, submit with a verdict), while `self-review-pr` assumes you're checking your own work (catch debug code, verify CI, polish description).

### Trigger-Friendly Descriptions

Following the [agent skills specification](https://agentskills.io/specification), each description includes "Use when..." clauses listing specific user intents. The description is the primary mechanism clients use to decide whether to load a skill.

### All Toolsets Enabled by Default

All toolsets have `Default: true`, so every tool is registered out of the box. Skills handle progressive disclosure on the client side — the server exposes the full tool surface and lets skills control what's visible for each workflow.

## Test Coverage

Tests in `pkg/github/skill_resources_test.go` ensure:

- **`TestAllSkillsCoverAllToolsets`**: Every registered MCP tool appears in at least one skill's `allowedTools`. Adding a new tool without including it in a skill fails CI.
- **`TestBuildSkillContent`**: Verifies YAML frontmatter assembly.
- **`TestSkillResourceURIs`**: Checks for duplicate URIs/names and ensures every skill has non-empty description, tools, and body.
- **`TestRegisterSkillResources`**: Verifies registration completes and the correct number of skills exist.

## Related Projects

- [mcpi-ext](https://github.com/SamMorrowDrums/mcpi-ext) — MCP client extensions that implement progressive tool discovery via skills
- [Server Developer Guide](https://github.com/SamMorrowDrums/mcpi-ext/blob/main/docs/server-developer-guide.md) — Guide for MCP server developers implementing skill:// resources
- [Progressive Tool Discovery](https://github.com/SamMorrowDrums/mcpi/blob/main/docs/progressive-tool-discovery.md) — Design rationale for progressive disclosure
- [Skills-as-Groups Proposal](https://github.com/modelcontextprotocol/experimental-ext-grouping/pull/13) — Proposed MCP spec extension for skills
