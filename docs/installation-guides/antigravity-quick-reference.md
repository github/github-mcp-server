# Antigravity GitHub MCP Quick Reference

Fast copy-paste configurations for Google's Antigravity IDE + the official GitHub remote MCP server (`https://api.githubcopilot.com/mcp/`). 

**Full guide**: See `docs/installation-guides/install-antigravity.md` in the [github-mcp-server](https://github.com/github/github-mcp-server) repository (includes detailed Windows tips, verification commands, security guidance, and Grok-parity recommendations).

## 1. Basic (Minimal — Default Toolsets Only)

```json
{
  "mcpServers": {
    "github": {
      "serverUrl": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer YOUR_GITHUB_PAT"
      }
    }
  }
}
```

## 2. Recommended Rich (Grok-like Experience — Preferred for Most Users)

Curated high-value toolsets for powerful agentic workflows while respecting Antigravity's <50 tool recommendation.

```json
{
  "mcpServers": {
    "github": {
      "serverUrl": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer YOUR_GITHUB_PAT",
        "X-MCP-Toolsets": "context,repos,issues,pull_requests,copilot,actions,discussions,projects,users,labels,orgs,gists,notifications"
      }
    }
  }
}
```

## 3. Maximum Power (Use with Caution)

```json
{
  "mcpServers": {
    "github": {
      "serverUrl": "https://api.githubcopilot.com/mcp/x/all",
      "headers": {
        "Authorization": "Bearer YOUR_GITHUB_PAT"
      }
    }
  }
}
```

> [!WARNING]
> The maximum configuration enables every tool. Start with the Recommended version. Add `"X-MCP-Readonly": "true"` to any config for a safe read-only mode.

## Quick PAT Scopes (for Recommended/Max)

- `repo` (essential)
- `read:org`
- `security_events`
- `gist`
- `project`
- `notifications`

Create at: https://github.com/settings/tokens

## After Editing

1. Save `mcp_config.json`
2. Fully restart Antigravity (or refresh MCP Servers panel)
3. Verify expanded tools in the MCP Servers panel (... → MCP Servers)

## Safety First

- Use a **dedicated PAT** for Antigravity
- Begin with read-only + Recommended config
- See the full guide for security recommendations and Windows merge examples (common when other Google Cloud MCP servers are already present)

**Links**:
- Full Antigravity Guide + Power User section
- [Remote Server docs](https://github.com/github/github-mcp-server/blob/main/docs/remote-server.md) (headers & toolsets table)
- [Server Configuration Guide](https://github.com/github/github-mcp-server/blob/main/docs/server-configuration.md) (recipes)
