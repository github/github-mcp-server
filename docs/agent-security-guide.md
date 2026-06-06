# Agent Security & Rate Limiting Guide

The GitHub MCP Server exposes dozens of tools, including destructive operations like `delete_file`, write operations like `push_files` and `merge_pull_request`, and resource creation like `create_repository`. By default, every enabled tool is available to the connected agent at all times. There are no built-in per-tool rate limits.

This guide covers practical ways to limit what an AI agent can do when using the GitHub MCP Server in production workflows — without relying solely on coarse-grained PAT scopes.

## Risks in Agent Workflows

Agents running in a loop or responding to prompt injection can:

- Create dozens of repositories, issues, or pull requests
- Delete files or trigger destructive GitHub Actions operations
- Merge pull requests or push commits without human review
- Exhaust GitHub API rate limits through rapid tool calls

PAT scopes alone are often too broad for per-tool control. A `repo`-scoped token grants write access to every repository tool, not just the ones you want the agent to use.

## Defense in Depth

Combine multiple layers for the strongest protection:

| Layer | What it controls | Best for |
|-------|------------------|----------|
| [Server configuration](#built-in-server-controls) | Which tools are registered at startup | All deployments |
| [PAT scopes](#authentication-and-token-scopes) | What the GitHub API allows | Local server, any host |
| [Organization policies](policies-and-governance.md) | Who can connect and with what credentials | Enterprise teams |
| [MCP enforcement proxy](#mcp-enforcement-proxies) | Per-tool blocks and rate limits at runtime | Production agent workflows |

## Built-in Server Controls

The GitHub MCP Server ships with configuration options that restrict tool access before any call reaches the GitHub API. See the [Server Configuration Guide](server-configuration.md) for host-specific examples.

### Read-only mode

The simplest safeguard. Disables all write tools regardless of other configuration.

```bash
github-mcp-server stdio --read-only
```

Remote server equivalent: `X-MCP-Readonly: true` header or `/readonly` URL path.

### Toolset and tool allowlists

Enable only the toolsets or individual tools your workflow needs:

```bash
github-mcp-server stdio \
  --toolsets=context,repos,issues,pull_requests \
  --tools=get_file_contents,issue_read,pull_request_read
```

This reduces context size for the LLM and prevents access to tools you did not explicitly enable.

### Exclude specific tools

When you need a broad toolset but want to block high-risk tools, use `--exclude-tools`:

```bash
github-mcp-server stdio \
  --toolsets=repos,pull_requests \
  --exclude-tools=delete_file,merge_pull_request,push_files,create_repository
```

Excluded tools take precedence over toolsets and individual tool allowlists.

Tools annotated with `DestructiveHint` in the server source are the highest-risk operations. As of this writing, they are: `delete_file`, `actions_run_trigger`, `delete_pending_pull_request_review`, `discussion_comment_write`, `projects_write`, and `remove_sub_issue`. Block these via `--exclude-tools` or your enforcement proxy even when other write tools are allowed.

### Lockdown mode

Limits content surfaced from public repositories to collaborators with push access. Useful when agents browse public repos but should not act on unverified external content.

```bash
github-mcp-server stdio --lockdown-mode
```

### Recommended profiles

| Use case | Configuration |
|----------|---------------|
| Code review assistant | `--read-only` or `--toolsets=context,repos,pull_requests` with `--exclude-tools=merge_pull_request` |
| Issue triage bot | `--toolsets=context,issues` with `--exclude-tools=issue_write` |
| PR authoring agent | `--toolsets=context,repos,pull_requests` with `--exclude-tools=merge_pull_request,delete_file` |
| Full automation (trusted) | Default toolsets + MCP enforcement proxy for rate limits |

## Authentication and Token Scopes

### Prefer fine-grained PATs

Fine-grained PATs limit access to specific repositories and permissions. Use the minimum permissions required:

- **Contents: Read-only** for agents that only browse code
- **Issues: Read and write** only when the agent should create or update issues
- **Pull requests: Read and write** only when the agent should open PRs

See [PAT Scope Filtering](scope-filtering.md) for how classic PAT scopes filter available tools at startup.

### Separate tokens per environment

Use different tokens for development, staging, and production agent workflows. Rotate tokens on a regular schedule and store them in platform credential managers, not source code.

### OAuth over long-lived PATs

When your host supports it, prefer OAuth authentication via the [remote server](remote-server.md). OAuth uses scope challenges so permissions are granted incrementally as tools are used, rather than upfront with a broad PAT.

## MCP Enforcement Proxies

For production agent workflows, server configuration alone cannot enforce runtime rate limits or block a tool that was enabled at startup. An **MCP enforcement proxy** sits between the host application and the MCP server, inspecting each tool call before it reaches GitHub.

Proxies can:

- Block specific tools entirely (e.g., `delete_file`)
- Rate-limit write operations (e.g., 30 calls per hour across all write tools)
- Apply per-tool limits (e.g., 5 `create_repository` calls per hour)
- Log tool invocations for audit and alerting

Place the proxy in your MCP server configuration so the host connects to the proxy instead of directly to the GitHub MCP Server:

```json
{
  "servers": {
    "github": {
      "command": "your-mcp-proxy",
      "args": ["--policy", "/path/to/security-policy.yaml", "--", "docker", "run", "-i", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN", "ghcr.io/github/github-mcp-server"]
    }
  }
}
```

The exact configuration depends on your proxy. Several open-source MCP proxies support policy-based tool filtering and rate limiting.

## Sample Security Policy

This repository includes a [recommended security policy template](examples/recommended-security-policy.yaml) with suggested defaults:

- **Blocked tools:** all tools annotated with `DestructiveHint` in the server (`delete_file`, `actions_run_trigger`, `delete_pending_pull_request_review`, `discussion_comment_write`, `projects_write`, and `remove_sub_issue`)
- **Write rate limit:** 30 invocations per hour across write tools
- **Repository creation limit:** 5 per hour
- **Merge limit:** 10 per hour

The policy file is a reference template for MCP enforcement proxies. The GitHub MCP Server does not read this file directly — configure your proxy to load it, or translate the rules into your proxy's native format.

Adapt the template to your workflow. A read-only code review agent needs fewer write allowances than a PR authoring agent.

## Monitoring and Response

Even with controls in place, monitor agent activity:

- Review GitHub audit logs for unexpected API activity from your token or app
- Set up alerts on GitHub API rate limit headers (`X-RateLimit-Remaining`)
- Watch for bursts of repository, issue, or PR creation
- Require human approval for merge and delete operations in high-risk workflows

## Related Documentation

- [Server Configuration Guide](server-configuration.md) — toolsets, exclude-tools, read-only mode
- [Policies & Governance](policies-and-governance.md) — organization-level controls
- [Scope Filtering](scope-filtering.md) — how PAT scopes filter tools
- [Host Integration Guide](host-integration.md) — architecture for embedding the server
