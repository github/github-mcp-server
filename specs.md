# github-mcp-plus — Specs

## Overview

A personal GitHub MCP server for use in AI agent development workflows. Built as a fork of the [official GitHub MCP server](https://github.com/github/github-mcp-server). Supports github.com and self-hosted GitHub Enterprise instances, including non-SSL environments.

---

## Base

- **Fork of**: https://github.com/github/github-mcp-server
- **Language**: Go
- **MCP SDK**: `modelcontextprotocol/go-sdk`
- **GitHub Client**: `google/go-github` (REST) + `shurcooL/githubv4` (GraphQL)

We inherit the official server's authentication, enterprise support, transport layer, tool registration pipeline, and toolset filtering. Our changes are additive and minimal.

---

## Configuration

All configuration via environment variables. Inherits official server env vars and adds one.

| Variable | Required | Default | Description |
|---|---|---|---|
| `GITHUB_PERSONAL_ACCESS_TOKEN` | Yes | — | Personal Access Token for authentication |
| `GITHUB_GH_HOST` | No | `github.com` | GitHub host for Enterprise. E.g. `github.mycompany.com` |
| `GITHUB_SKIP_SSL_VERIFY` | No | `false` | Set `true` to disable TLS certificate verification. For private GHE instances without valid certs. |

---

## What We Change in the Official Server

### 1. Add `GITHUB_SKIP_SSL_VERIFY` support
Inject a custom `http.Transport` with `InsecureSkipVerify: true` when the env var is set. Plugs into the existing transport chain before the bearer token transport.

### 2. Add URL parsing
A pre-processing utility that accepts a GitHub URL in place of discrete parameters and extracts `owner`, `repo`, and resource number/path from it.

- Handles github.com and GHE host URLs
- Supported URL shapes:
  - Repo: `https://{host}/{owner}/{repo}`
  - Issue: `https://{host}/{owner}/{repo}/issues/{number}`
  - PR: `https://{host}/{owner}/{repo}/pull/{number}`
  - File: `https://{host}/{owner}/{repo}/blob/{ref}/{path}`
- Any tool parameter named `url` triggers parsing; extracted values override or populate `owner`, `repo`, `number`/`path`/`ref`
- Applied at the tool handler level, not as HTTP middleware (MCP is not HTTP-routed)

### 3. Add `get_repository` tool
The official server has no direct "fetch repo by owner/repo" tool (only search). We add one.

---

## Tools

### Inherited (no changes)

These tools are used as-is from the official server. No modifications.

| Tool | Method | Description |
|---|---|---|
| `list_branches` | REST | List branches in a repo |
| `list_issues` | GraphQL | List issues with native GitHub filters |
| `list_pull_requests` | REST | List PRs with native GitHub filters |
| `get_file_contents` | REST | Read file content at a ref |
| `pull_request_read` | GraphQL | Unified PR read tool (see methods below) |
| `issue_read` | GraphQL | Unified issue read tool (see methods below) |

**`pull_request_read` methods used:**
- `get_comments` — PR conversation-level comments
- `get_review_comments` — Inline review comments, includes `diff_hunk`, `isResolved`, `isOutdated`
- `get_reviews` — Review summaries (approved / changes requested / commented / dismissed)

**`issue_read` methods used:**
- `get_comments` — Issue comments

---

### New Tools

#### `get_repository`

Get metadata for a single repository.

**Input:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `owner` | string | Yes* | Repository owner (user or org) |
| `repo` | string | Yes* | Repository name |
| `url` | string | No | GitHub repo URL — parsed to populate `owner` and `repo` |

*Not required if `url` is provided.

**Output:**

```
name, full_name, description, visibility, default_branch,
is_fork, is_archived, is_template,
stargazers_count, forks_count, open_issues_count,
topics[], clone_url, ssh_url,
created_at, updated_at, pushed_at
```

---

## Pagination

All list tools support:

| Parameter | Type | Default | Description |
|---|---|---|---|
| `per_page` | int | 30 | Results per page (max 100) |
| `page` | int | 1 | Page number (REST-based tools) |
| `after` | string | — | Cursor for GraphQL-based tools |

Responses include page info:
```json
{
  "has_next_page": true,
  "has_previous_page": false,
  "start_cursor": "...",
  "end_cursor": "..."
}
```

Pagination is the agent's responsibility — the server returns one page per call.

---

## URL Parsing

Any tool parameter named `url` is parsed before the tool executes. The URL must match the configured host (`GITHUB_GH_HOST` or `github.com`).

**Examples:**

```
# Repo URL
url = "https://github.com/facebook/react"
→ owner = "facebook", repo = "react"

# Issue URL
url = "https://github.com/facebook/react/issues/30072"
→ owner = "facebook", repo = "react", number = 30072

# PR URL
url = "https://github.com/facebook/react/pull/31000"
→ owner = "facebook", repo = "react", number = 31000

# File URL
url = "https://github.com/facebook/react/blob/main/packages/react/index.js"
→ owner = "facebook", repo = "react", ref = "main", path = "packages/react/index.js"
```

Explicitly provided `owner`/`repo`/`number` parameters take precedence over URL-parsed values.

---

## V2 Features (Future)

Client-side filtering applied after fetching, before returning to the agent.

| Tool | Filter Additions |
|---|---|
| `issue_read(get_comments)` | `author`, body keyword/regex |
| `pull_request_read(get_comments)` | `author`, body keyword/regex |
| `pull_request_read(get_review_comments)` | `author`, file path glob, body keyword/regex |
| `pull_request_read(get_reviews)` | `reviewer`, `state` |

---

## Out of Scope (V1)

- Write operations (create/update issues, PRs, comments, files)
- GitHub Actions / Workflows
- Notifications
- GraphQL Search API
- Discussions
- Projects
- Security scanning alerts
- Releases, Deployments, Pages
- User / org / team management
- Webhooks
