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
| Scope Filtering | Always enabled | Always enabled |

> **Default behavior:** If you don't specify any configuration, the server uses the **default toolsets**: `context`, `issues`, `pull_requests`, `repos`, `users`.

---

## How Configuration Works

All configuration options are **composable**: you can combine toolsets, individual tools, excluded tools, dynamic discovery, read-only mode and lockdown mode in any way that suits your workflow.

Note: **read-only** mode acts as a strict security filter that takes precedence over any other configuration, by disabling write tools even when explicitly requested.

Note: **excluded tools** takes precedence over toolsets and individual tools — listed tools are always excluded, even if their toolset is enabled or they are explicitly added via `--tools` / `X-MCP-Tools`.

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
