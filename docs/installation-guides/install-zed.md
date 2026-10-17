# Install GitHub MCP Server in Zed

[Zed](https://zed.dev) is a high-performance multiplayer code editor with native MCP support. Zed exposes MCP servers under the `context_servers` settings key. For general setup information (prerequisites, Docker installation, security best practices), see the [Installation Guides README](./README.md).

## Prerequisites

1. Zed installed (latest version — Zed v0.224.0+ recommended for the modern `agent.tool_permissions` settings shape)
2. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) with appropriate scopes
3. For local installation: [Docker](https://www.docker.com/) installed and running

## Installation Methods

There are two ways to install the GitHub MCP server in Zed:

- **Option A — Zed Extension (easiest):** a community-maintained [GitHub MCP extension](https://zed.dev/extensions/mcp-server-github) is available in the Zed extension gallery. Install it from the Agent Panel's top-right menu → "View Server Extensions", or from the command palette via the `zed: extensions` action. After installation, Zed pops up a modal asking for your GitHub Personal Access Token.
- **Option B — Custom Server (recommended for the official remote endpoint):** add the configuration manually to `settings.json` to use either GitHub's hosted remote server or the official Docker image directly. The rest of this guide covers Option B.

## Remote Server (Recommended)

Uses GitHub's hosted server at `https://api.githubcopilot.com/mcp/`. Open your Zed [settings file](https://zed.dev/docs/configuring-zed.html#settings-files) (Command Palette → `zed: open settings`) and add the configuration below under `context_servers`.

```json
{
  "context_servers": {
    "github": {
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer YOUR_GITHUB_PAT"
      }
    }
  }
}
```

Replace `YOUR_GITHUB_PAT` with your [GitHub Personal Access Token](https://github.com/settings/tokens). To customize toolsets, add server-side headers like `X-MCP-Toolsets` or `X-MCP-Readonly` to the `headers` object — see the [Server Configuration Guide](../server-configuration.md).

> [!NOTE]
> If you omit the `Authorization` header, Zed will attempt the standard MCP OAuth flow on first use. The GitHub MCP server does not currently advertise OAuth for non-Copilot hosts, so a Personal Access Token in the `Authorization` header is the supported path.

## Local Server (Docker)

The local GitHub MCP server runs via Docker and requires Docker Desktop (or another Docker runtime) to be installed and running.

Log in with OAuth instead of a token. On github.com the official image already includes the app credentials, so you provide none yourself — the server opens a browser login on first use and keeps the token in memory only. In Docker, publish a fixed callback port to loopback:

```json
{
  "context_servers": {
    "github": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-p", "127.0.0.1:8085:8085",
        "-e", "GITHUB_OAUTH_CALLBACK_PORT",
        "ghcr.io/github/github-mcp-server"
      ],
      "env": {
        "GITHUB_OAUTH_CALLBACK_PORT": "8085"
      }
    }
  }
}
```

See **[Local Server OAuth Login](../oauth-login.md)** for the native-binary flow (no fixed port), headless/device-code fallback, GitHub Enterprise, and bringing your own OAuth or GitHub App.

To authenticate with a Personal Access Token instead (it takes precedence over OAuth):

```json
{
  "context_servers": {
    "github": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "GITHUB_PERSONAL_ACCESS_TOKEN",
        "ghcr.io/github/github-mcp-server"
      ],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "YOUR_GITHUB_PAT"
      }
    }
  }
}
```

> [!IMPORTANT]
> Zed expects `command` as a **string** plus a separate `args` array, not a single array combining both. This differs from hosts like OpenCode and Claude Desktop.

## Verify Installation

1. Open the Agent Panel and click into its Settings view (or run `agent: open settings`).
2. Find `github` in the context servers list. A green indicator dot with the tooltip "Server is active" confirms a working configuration. Other colors and tooltip messages indicate startup or auth errors.
3. Try a prompt that should invoke a tool — for example, `List my recent GitHub pull requests`. Zed will prompt for tool approval before the first call unless your `agent.tool_permissions.default` is set to `"allow"`.

## Tool Permissions (Optional)

Zed v0.224.0+ controls tool approval via `agent.tool_permissions`. Approve a specific GitHub MCP tool without per-call prompts by using the `mcp:<server>:<tool_name>` key format:

```json
{
  "agent": {
    "tool_permissions": {
      "default": "confirm",
      "rules": [
        { "tool": "mcp:github:list_pull_requests", "permission": "allow" },
        { "tool": "mcp:github:list_issues", "permission": "allow" }
      ]
    }
  }
}
```

See the [Zed tool permissions docs](https://zed.dev/docs/ai/tool-permissions.html) for the full schema.

## Troubleshooting

- **Server indicator stays red / "Server is not running"**: check the Agent Panel's settings view for the per-server error string. Most common cause is invalid JSON in `settings.json` — Zed surfaces JSON parse errors in the editor itself.
- **`401 Unauthorized`**: verify your PAT has not expired and includes the scopes for the tools you intend to call. The remote endpoint will reject requests with no `Authorization` header (no anonymous access).
- **Tools missing from prompts**: confirm the Agent profile in use has not disabled the server. If you're using a [custom profile](https://zed.dev/docs/ai/agent-panel.html#custom-profiles), make sure `enable_all_context_servers` is `true` or that `github` is explicitly listed.
- **Docker errors on the local server**: ensure Docker Desktop is running and the `ghcr.io/github/github-mcp-server` image has been pulled at least once. Try `docker pull ghcr.io/github/github-mcp-server` from a terminal.

## Important Notes

- **Configuration key**: Zed uses `context_servers` (not `mcpServers`).
- **Command shape**: `command` is a string + separate `args` array.
- **OAuth**: omitting `Authorization` triggers Zed's MCP OAuth flow, but the GitHub MCP server's PAT-based auth is the supported path today.
- **External agents**: MCP servers configured in `context_servers` are forwarded to [external agents](https://zed.dev/docs/ai/external-agents.html) via the Agent Client Protocol.
