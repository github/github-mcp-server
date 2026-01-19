---
title: "Intelligent Scope Features"
date: 2026-01
description: "OAuth scope challenges, automatic PAT filtering, and comprehensive scope documentation for smarter authentication"
category: feature
---

# Intelligent Scope Features

GitHub MCP Server now provides intelligent handling of OAuth scopes across all authentication methods—automatically filtering tools based on your permissions and enabling dynamic scope requests when needed.

## What's New

### OAuth Scope Challenges (Remote Server)

When using the remote MCP server with OAuth authentication (like VS Code's GitHub Copilot Chat), the server now implements the [MCP step-up authentication specification](https://spec.modelcontextprotocol.io/). Instead of failing when you lack a required scope, the server can request additional permissions dynamically.

**How it works:**
1. You attempt to use a tool that requires a scope you haven't granted
2. The server returns a `401` with a `WWW-Authenticate` header indicating the missing scope
3. Your MCP client (if supported) prompts you to authorize the additional scope
4. After granting permission, the operation completes automatically

This means you can start with minimal permissions and expand them naturally as you use more features—no upfront "grant all permissions" prompts.

### PAT Scope Filtering (Local Server)

For users running the local server with a classic Personal Access Token (`ghp_` prefix), tools are now automatically filtered based on your token's scopes. At startup, the server discovers your token's scopes and hides tools you can't use.

**Benefits:**
- **Reduced clutter** — Only see tools your token supports
- **No failed calls** — Tools requiring unavailable scopes are hidden proactively
- **Clear expectations** — Your tool list matches your actual capabilities

**Example:** If your PAT only has `repo` and `gist` scopes, tools requiring `admin:org`, `project`, or `notifications` will be hidden from your tool list.

### Server-to-Server Token Handling (Remote Server)

When using server-to-server tokens (like the `GITHUB_TOKEN` in GitHub Actions), the remote server now intelligently hides user-context tools that don't make sense without a human user.

**Tools hidden for S2S tokens:**
- `get_me` — No user to query
- Other user-specific context tools

This ensures automated workflows see only the tools they can actually use, rather than failing when they attempt to call user-context APIs.

### Documented OAuth Scopes

Every MCP tool now includes explicit OAuth scope documentation:

- **Required Scopes** — The minimum scope(s) needed to use the tool
- **Accepted Scopes** — All scopes that satisfy the requirement (including parent scopes)

This information is visible in:
- **README.md** — Each tool's documentation shows required and accepted scopes
- **Tool metadata** — Available programmatically via the MCP protocol

**Example from README:**
```
### create_issue
Creates a new issue in a GitHub repository.
Required scopes: repo
Accepted scopes: repo
```

### New `list-scopes` Command

A new CLI command helps you understand what scopes your configured toolsets need:

```bash
# See scopes for default toolsets
github-mcp-server list-scopes --output=summary

# Output:
# Required OAuth scopes for enabled tools:
#   read:org
#   repo
# Total: 2 unique scope(s)

# All toolsets with detailed output
github-mcp-server list-scopes --toolsets=all --output=text

# JSON for automation
github-mcp-server list-scopes --output=json
```

Use this to:
- **Create minimal PATs** — Know exactly what scopes to grant
- **Audit permissions** — Understand what each toolset requires
- **CI/CD setup** — Generate scope lists programmatically

## Scope Hierarchy

The server understands GitHub's scope hierarchy, so parent scopes satisfy child scope requirements:

| Parent Scope | Covers |
|-------------|--------|
| `repo` | `public_repo`, `security_events` |
| `admin:org` | `write:org`, `read:org` |
| `project` | `read:project` |
| `write:org` | `read:org` |

If a tool requires `read:org` and your token has `admin:org`, the tool is available.

## Authentication Comparison

| Authentication Method | Scope Handling |
|----------------------|----------------|
| **OAuth** (remote server) | Scope challenges — request permissions on-demand |
| **Classic PAT** (`ghp_`) | Automatic filtering — hide unavailable tools |
| **Fine-grained PAT** (`github_pat_`) | No filtering — API enforces permissions at call time |
| **GitHub App** (`ghs_`) | No filtering — permissions based on app installation |
| **Server-to-Server** (`GITHUB_TOKEN`) | User tools hidden — no user context available |

## Getting Started

### For Remote Server (OAuth) Users

No action required! Scope challenges work automatically with supporting MCP clients like VS Code. You'll be prompted for additional permissions as needed.

### For Local Server (PAT) Users

1. **Discover required scopes:**
   ```bash
   github-mcp-server list-scopes --toolsets=repos,issues,pull_requests --output=summary
   ```

2. **Create a PAT with those scopes** at [github.com/settings/tokens](https://github.com/settings/tokens)

3. **Start the server** — tools not supported by your token will be automatically hidden

### Checking Your Current Scopes

```bash
curl -sI -H "Authorization: Bearer $GITHUB_PERSONAL_ACCESS_TOKEN" \
  https://api.github.com/user | grep -i x-oauth-scopes
```

## Related Documentation

- [PAT Scope Filtering Guide](../scope-filtering.md)
- [OAuth Authentication Guide](../oauth-authentication.md)
- [Server Configuration](../server-configuration.md)

## Feedback

Share your experience and report issues in the [Scope filtering/challenging discussion](https://github.com/github/github-mcp-server/discussions/1802).

## Key PRs

**github-mcp-server:**
- [#1679](https://github.com/github/github-mcp-server/pull/1679) — Add OAuth scope metadata to all MCP tools
- [#1741](https://github.com/github/github-mcp-server/pull/1741) — Add PAT scope filtering for stdio server
- [#1750](https://github.com/github/github-mcp-server/pull/1750) — Add `list-scopes` command using inventory architecture
- [#1650](https://github.com/github/github-mcp-server/pull/1650) — OAuth scopes customization and documentation

**github-mcp-server-remote:**
- [#503](https://github.com/github/github-mcp-server-remote/pull/503) — Dynamic scope challenges implementation
- [#609](https://github.com/github/github-mcp-server-remote/pull/609) — Dynamic OAuth scopes based on route and headers
- [#618](https://github.com/github/github-mcp-server-remote/pull/618) — Initialize tool scope map for scope challenge middleware

## What's Not Included

**Fine-grained PAT support** — Fine-grained Personal Access Tokens use a different permission model based on repository access rather than OAuth scopes. They don't return an `X-OAuth-Scopes` header, so scope filtering and scope challenges don't apply. The GitHub API enforces permissions at call time, and you'll receive clear error messages if an operation isn't permitted.
