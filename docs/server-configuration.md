# Server Configuration Guide

This guide helps you choose the right configuration for your use case and shows you how to apply it. For the complete reference of available toolsets and tools, see the [README](../README.md#tool-configuration).

## Quick Reference
We currently support the following ways in which the GitHub MCP Server can be configured: 

| Configuration | Remote Server | Local Server |
|---------------|---------------|--------------|
| Toolsets | `X-MCP-Toolsets` header or `/x/{toolset}` URL | `--toolsets` flag or `GITHUB_TOOLSETS` env var |
| Individual Tools | `X-MCP-Tools` header | `--tools` flag or `GITHUB_TOOLS` env var |
| Exclude Tools | `X-MCP-Exclude-Tools` header | `--exclude-tools` flag or `GITHUB_EXCLUDE_TOOLS` env var |
| Read-Only Mode | `X-MCP-Readonly` header or `/readonly` URL | `--read-only` flag or `GITHUB_READ_ONLY` env var |
| Dynamic Mode | Not available | `--dynamic-toolsets` flag or `GITHUB_DYNAMIC_TOOLSETS` env var |
| Lockdown Mode | `X-MCP-Lockdown` header | `--lockdown-mode` flag or `GITHUB_LOCKDOWN_MODE` env var |
| PR Author Allowlist | Server `--allowed-pr-authors` flag or `GITHUB_ALLOWED_PR_AUTHORS` env var | `--allowed-pr-authors` flag or `GITHUB_ALLOWED_PR_AUTHORS` env var |
| Insiders Mode | `X-MCP-Insiders` header or `/insiders` URL | `--insiders` flag or `GITHUB_INSIDERS` env var |
| Feature Flags | `X-MCP-Features` header | `--features` flag |
| Scope Filtering | Always enabled | Always enabled |
| Server Name/Title | Not available | `GITHUB_MCP_SERVER_NAME` / `GITHUB_MCP_SERVER_TITLE` env vars or `github-mcp-server-config.json` |

> **Default behavior:** If you don't specify any configuration, the server uses the **default toolsets**: `context`, `issues`, `pull_requests`, `repos`, `users`.

---

## How Configuration Works

All configuration options are **composable**: you can combine toolsets, individual tools, excluded tools, dynamic discovery, read-only mode and lockdown mode in any way that suits your workflow.

Note: **read-only** mode acts as a strict security filter that takes precedence over any other configuration, by disabling write tools even when explicitly requested.

Note: **excluded tools** takes precedence over toolsets and individual tools — listed tools are always excluded, even if their toolset is enabled or they are explicitly added via `--tools` / `X-MCP-Tools`.

Note: **PR author allowlist** restricts mutating pull request tools to existing pull requests authored by the configured GitHub logins. Read-only PR tools and `create_pull_request` are not restricted. `actions_run_trigger` is not restricted by this setting because it targets a ref rather than a pull request number.

---

## Configuration Examples

The examples below use VS Code configuration format to illustrate the concepts. If you're using a different MCP host (Cursor, Claude Desktop, JetBrains, etc.), your configuration might need to look slightly different. See [installation guides](./installation-guides) for host-specific setup.

### Enabling Specific Tools

**Best for:** users who know exactly what they need and want to optimize context usage by loading only the tools they will use. 

**Example:**

<table>
<tr><th>Remote Server</th><th>Local Server</th></tr>
<tr valign="top">
<td>

```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/",
  "headers": {
    "X-MCP-Tools": "get_file_contents,get_me,pull_request_read"
  }
}
```

</td>
<td>

```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--tools=get_file_contents,get_me,pull_request_read"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

</td>
</tr>
</table>

---

### Enabling Specific Toolsets

**Best for:** Users who want to enable multiple related toolsets.

<table>
<tr><th>Remote Server</th><th>Local Server</th></tr>
<tr valign="top">
<td>

```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/",
  "headers": {
    "X-MCP-Toolsets": "issues,pull_requests"
  }
}
```

</td>
<td>

```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--toolsets=issues,pull_requests"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

</td>
</tr>
</table>

---

### Enabling Toolsets + Tools

**Best for:** Users who want broad functionality from some areas, plus specific tools from others.

Enable entire toolsets, then add individual tools from toolsets you don't want fully enabled.

<table>
<tr><th>Remote Server</th><th>Local Server</th></tr>
<tr valign="top">
<td>

```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/",
  "headers": {
    "X-MCP-Toolsets": "repos,issues",
    "X-MCP-Tools": "get_gist,pull_request_read"
  }
}
```

</td>
<td>

```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--toolsets=repos,issues",
    "--tools=get_gist,pull_request_read"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

</td>
</tr>
</table>

**Result:** All repository and issue tools, plus just the gist tools you need.

---

### Excluding Specific Tools

**Best for:** Users who want to enable a broad toolset but need to exclude specific tools for security, compliance, or to prevent undesired behavior.

Listed tools are removed regardless of any other configuration — even if their toolset is enabled or they are individually added.

<table>
<tr><th>Remote Server</th><th>Local Server</th></tr>
<tr valign="top">
<td>

```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/",
  "headers": {
    "X-MCP-Toolsets": "pull_requests",
    "X-MCP-Exclude-Tools": "create_pull_request,merge_pull_request"
  }
}
```

</td>
<td>

```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--toolsets=pull_requests",
    "--exclude-tools=create_pull_request,merge_pull_request"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

</td>
</tr>
</table>

**Result:** All pull request tools except `create_pull_request` and `merge_pull_request` — the user gets read and review tools only.

---

### Read-Only Mode

**Best for:** Security conscious users who want to ensure the server won't allow operations that modify issues, pull requests, repositories etc.

When active, this mode will disable all tools that are not read-only even if they were requested.

**Example:** 
<table>
<tr><th>Remote Server</th><th>Local Server</th></tr>
<tr valign="top">
<td>

**Option A: Header**
```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/",
  "headers": {
    "X-MCP-Toolsets": "issues,repos,pull_requests",
    "X-MCP-Readonly": "true"
  }
}
```

**Option B: URL path**
```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/x/all/readonly"
}
```

</td>
<td>


```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--toolsets=issues,repos,pull_requests",
    "--read-only"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

</td>
</tr>
</table>

> Even if `issues` toolset contains `create_issue`, it will be excluded in read-only mode.

---

### Dynamic Discovery (Local Only)

**Best for:** Letting the LLM discover and enable toolsets as needed.

Starts with only discovery tools (`enable_toolset`, `list_available_toolsets`, `get_toolset_tools`), then expands on demand.

<table>
<tr><th>Local Server Only</th></tr>
<tr valign="top">
<td>

```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--dynamic-toolsets"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

**With some tools pre-enabled:**
```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--dynamic-toolsets",
    "--tools=get_me,search_code"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

</td>
</tr>
</table>

When both dynamic mode and specific tools are enabled in the server configuration, the server will start with the 3 dynamic tools + the specified tools.

---

### Lockdown Mode

**Best for:** Public repositories where you want to limit content from users without push access.

Lockdown mode ensures the server only surfaces content in public repositories from users with push access to that repository. Private repositories are unaffected, and collaborators retain full access to their own content.

**Example:**
<table>
<tr><th>Remote Server</th><th>Local Server</th></tr>
<tr valign="top">
<td>

```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/",
  "headers": {
    "X-MCP-Lockdown": "true"
  }
}
```

</td>
<td>

```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--lockdown-mode"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

</td>
</tr>
</table>

---

### PR Author Allowlist

**Best for:** Automation workflows that may mutate bot-authored pull requests but should never mutate human-authored pull requests.

When set, mutating pull request tools first fetch the target pull request and check `pr.User.Login`. If the author is not in the allowlist, the tool returns an error before making the mutation. Empty or unset means unrestricted behavior.

```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--toolsets=pull_requests,actions",
    "--allowed-pr-authors=renovate[bot],github-actions[bot]"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

Known limitations: `actions_run_trigger` operates on refs, not pull request numbers, so it is not gated by this setting. Review-thread resolve and unresolve tools take only opaque thread IDs and are not gated by the PR author allowlist. The allowlist checks `pr.User.Login`; PRs from forks authored by allowed bots still pass. Enabling the allowlist adds one API call before a mutating PR operation when the handler does not already have the pull request.

---

### Insiders Mode

**Best for:** Users who want early access to experimental features and new tools before they reach general availability.

Insiders Mode unlocks experimental features, such as [MCP Apps](#mcp-apps) support. We created this mode to have a way to roll out experimental features and collect feedback. So if you are using Insiders, please don't hesitate to share your feedback with us! Features in Insiders Mode may change, evolve, or be removed based on user feedback.

<table>
<tr><th>Remote Server</th><th>Local Server</th></tr>
<tr valign="top">
<td>

**Option A: URL path**
```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/insiders"
}
```

**Option B: Header**
```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/",
  "headers": {
    "X-MCP-Insiders": "true"
  }
}
```

</td>
<td>

```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--insiders"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

</td>
</tr>
</table>

See [Insiders Features](./insiders-features.md) for a full list of what's available in Insiders Mode.

---

### MCP Apps

[MCP Apps](https://modelcontextprotocol.io/docs/extensions/apps) is an extension to the Model Context Protocol that enables servers to deliver interactive user interfaces to end users. Instead of returning plain text that the LLM must interpret and relay, tools can render forms, profiles, and dashboards right in the chat.

MCP Apps is enabled by [Insiders Mode](#insiders-mode), or independently via the `remote_mcp_ui_apps` feature flag.

**Supported tools:**

| Tool | Description |
|------|-------------|
| `get_me` | Displays your GitHub user profile with avatar, bio, and stats in a rich card |
| `issue_write` | Opens an interactive form to create or update issues |
| `create_pull_request` | Provides a full PR creation form to create a pull request (or a draft pull request) |

**Client requirements:** MCP Apps requires a host that supports the [MCP Apps extension](https://modelcontextprotocol.io/docs/extensions/apps). Currently tested with VS Code (`chat.mcp.apps.enabled` setting).

<table>
<tr><th>Remote Server</th><th>Local Server</th></tr>
<tr valign="top">
<td>

```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/",
  "headers": {
    "X-MCP-Features": "remote_mcp_ui_apps"
  }
}
```

</td>
<td>

```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--features=remote_mcp_ui_apps"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

</td>
</tr>
</table>

---

### Scope Filtering

**Automatic feature:** The server handles OAuth scopes differently depending on authentication type:

- **Classic PATs** (`ghp_` prefix): Tools are filtered at startup based on token scopes—you only see tools you have permission to use
- **OAuth** (remote server): Uses scope challenges—when a tool needs a scope you haven't granted, you're prompted to authorize it
- **Other tokens**: No filtering—all tools shown, API enforces permissions

This happens transparently—no configuration needed. If scope detection fails for a classic PAT (e.g., network issues), the server logs a warning and continues with all tools available.

See [Scope Filtering](./scope-filtering.md) for details on how filtering works with different token types.

---

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Server fails to start | Invalid tool name in `--tools` or `X-MCP-Tools` | Check tool name spelling; use exact names from [Tools list](../README.md#tools) |
| Write tools not working | Read-only mode enabled | Remove `--read-only` flag or `X-MCP-Readonly` header |
| Tools missing | Toolset not enabled | Add the required toolset or specific tool |
| Dynamic tools not available | Using remote server | Dynamic mode is available in the local MCP server only |

---

## Useful links

- [README: Tool Configuration](../README.md#tool-configuration)
- [README: Available Toolsets](../README.md#available-toolsets) — Complete list of toolsets
- [README: Tools](../README.md#tools) — Complete list of individual tools
- [Remote Server Documentation](./remote-server.md) — Remote-specific options and headers
- [Installation Guides](./installation-guides) — Host-specific setup instructions
