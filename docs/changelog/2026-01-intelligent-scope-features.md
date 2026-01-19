---
title: "Intelligent Scope Features"
date: 2026-01
description: "OAuth scope challenges, automatic PAT filtering, and comprehensive scope documentation for smarter authentication"
category: feature
---

# Intelligent Scope Features

GitHub MCP Server now intelligently handles OAuth scopes—filtering tools based on your permissions and enabling dynamic scope requests when needed.

## What's New

### OAuth Scope Challenges (Remote Server)

The remote server now implements [MCP scope challenge handling](https://modelcontextprotocol.io/specification/2025-11-05/basic/authorization#scope-challenge-handling). Instead of failing when you lack a required scope, it requests additional permissions dynamically—start with minimal permissions and expand them as needed.

### PAT Scope Filtering

For classic Personal Access Tokens (`ghp_` prefix), tools are automatically filtered based on your token's scopes. The server discovers your scopes at startup and hides tools you can't use.

**Example:** If your PAT only has `repo` and `gist` scopes, tools requiring `admin:org`, `project`, or `notifications` are hidden.

### Server-to-Server Token Handling (Remote Server)

For server-to-server tokens (like `GITHUB_TOKEN` in Actions), the remote server hides user-context tools like `get_me` that don't apply without a human user.

### Documented OAuth Scopes

Every MCP tool now documents its required and accepted OAuth scopes in the README and tool metadata.

### New `list-scopes` Command

Discover what scopes your toolsets need:

```bash
github-mcp-server list-scopes --output=summary
github-mcp-server list-scopes --toolsets=all --output=json
```

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
| **Fine-grained PAT** (`github_pat_`) | No filtering — fine-grained permissions, not OAuth scopes |
| **GitHub App** (`ghs_`) | No filtering — fine-grained permissions, not OAuth scopes |
| **Server-to-Server** (`GITHUB_TOKEN`) | User tools hidden — no user context available |

## Getting Started

**OAuth users:** No action required—scope challenges work automatically.

**PAT users:** Run `list-scopes` to discover required scopes, create a PAT at [github.com/settings/tokens](https://github.com/settings/tokens), and start the server.

## Related Documentation

- [PAT Scope Filtering Guide](https://github.com/github/github-mcp-server/blob/v0.29.0/docs/scope-filtering.md)
- [OAuth Authentication Guide](https://github.com/github/github-mcp-server/blob/v0.29.0/docs/oauth-authentication.md)
- [Server Configuration](https://github.com/github/github-mcp-server/blob/v0.29.0/docs/server-configuration.md)

## Feedback

Share your experience in the [Scope filtering/challenging discussion](https://github.com/github/github-mcp-server/discussions/1802).

## What's Not Included

**Fine-grained permissions** — Fine-grained PATs (`github_pat_`) and GitHub Apps (`ghs_`) use repository-based permissions rather than OAuth scopes. They don't return `X-OAuth-Scopes` headers, so scope filtering and scope challenges don't apply. The API enforces permissions at call time.
