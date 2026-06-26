# Install GitHub MCP Server in OpenCode

[OpenCode](https://opencode.ai) is a terminal-based AI coding agent that exposes MCP servers under the `mcp` key in `opencode.json` (or `opencode.jsonc`). For general setup information (prerequisites, Docker installation, security best practices), see the [Installation Guides README](./README.md).

## Prerequisites

1. OpenCode installed (`brew install sst/tap/opencode` or see [OpenCode install docs](https://opencode.ai/docs/))
2. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) with appropriate scopes
3. For local installation: [Docker](https://www.docker.com/) installed and running

> [!IMPORTANT]
> The OpenCode docs note that the GitHub MCP server can add a lot of tokens to your context. Consider limiting toolsets — for example, by setting `X-MCP-Toolsets` on the remote server or `--toolsets` on the local server — to keep prompts within your model's context window. See the [Server Configuration Guide](../server-configuration.md) and the [main README's toolsets section](../../README.md#available-toolsets).

## Remote Server (Recommended)

Uses GitHub's hosted server at `https://api.githubcopilot.com/mcp/`. Edit your [OpenCode config](https://opencode.ai/docs/config/) (typically `~/.config/opencode/opencode.json`, or `opencode.json` in your project root) and add the following under `mcp`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "github": {
      "type": "remote",
      "url": "https://api.githubcopilot.com/mcp/",
      "enabled": true,
      "oauth": false,
      "headers": {
        "Authorization": "Bearer YOUR_GITHUB_PAT"
      }
    }
  }
}
```

Replace `YOUR_GITHUB_PAT` with your [GitHub Personal Access Token](https://github.com/settings/tokens). The `oauth: false` setting disables OpenCode's automatic OAuth discovery and tells it to use the PAT in `Authorization` instead — without this, OpenCode may try the OAuth flow first.

### Using an environment variable for the PAT

OpenCode supports environment-variable interpolation in config values via `{env:VAR_NAME}`. To avoid putting your PAT directly in `opencode.json`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "github": {
      "type": "remote",
      "url": "https://api.githubcopilot.com/mcp/",
      "enabled": true,
      "oauth": false,
      "headers": {
        "Authorization": "Bearer {env:GITHUB_PERSONAL_ACCESS_TOKEN}"
      }
    }
  }
}
```

Set `GITHUB_PERSONAL_ACCESS_TOKEN` in your shell environment before starting OpenCode.

## Local Server (Docker)

The local GitHub MCP server runs via Docker and requires Docker Desktop (or another Docker runtime) to be installed and running.

Log in with OAuth instead of a token. On github.com the official image already includes the app credentials, so you provide none yourself — the server opens a browser login on first use and keeps the token in memory only. In Docker, publish a fixed callback port to loopback:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "github": {
      "type": "local",
      "command": [
        "docker", "run", "-i", "--rm",
        "-p", "127.0.0.1:8085:8085",
        "-e", "GITHUB_OAUTH_CALLBACK_PORT",
        "ghcr.io/github/github-mcp-server"
      ],
      "enabled": true,
      "environment": {
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
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "github": {
      "type": "local",
      "command": [
        "docker", "run", "-i", "--rm",
        "-e", "GITHUB_PERSONAL_ACCESS_TOKEN",
        "ghcr.io/github/github-mcp-server"
      ],
      "enabled": true,
      "environment": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "YOUR_GITHUB_PAT"
      }
    }
  }
}
```

> [!IMPORTANT]
> OpenCode expects `command` as a **single array** combining the executable and its arguments (e.g. `["docker", "run", "-i", ...]`), and the env-var key is `environment` (not `env`). This differs from hosts like Zed and Cursor.

## Verify Installation

1. Restart OpenCode (or start a new session).
2. Check that the server is discovered:
   ```sh
   opencode mcp list
   ```
3. Try a prompt that references the server by name to bias the model toward its tools:
   ```
   Use the github tool to list my recently merged pull requests.
   ```

## Managing the Server

OpenCode exposes a few useful subcommands for MCP servers:

| Command | Purpose |
| --- | --- |
| `opencode mcp list` | List configured MCP servers and their auth/connection status. |
| `opencode mcp debug github` | Show auth status, test HTTP connectivity, and walk through OAuth discovery for the `github` server. |
| `opencode mcp auth github` | Trigger an OAuth flow manually (only relevant if `oauth` is not set to `false`). |
| `opencode mcp logout github` | Clear stored OAuth tokens for the server. |

## Disabling Tools Per-Agent

Because the GitHub MCP server can register a large number of tools, you may want to **disable them globally** and **re-enable them only for specific agents**. OpenCode uses the `<server-name>_*` glob pattern to match all tools from a server:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "github": {
      "type": "remote",
      "url": "https://api.githubcopilot.com/mcp/",
      "enabled": true,
      "oauth": false,
      "headers": { "Authorization": "Bearer {env:GITHUB_PERSONAL_ACCESS_TOKEN}" }
    }
  },
  "tools": {
    "github_*": false
  },
  "agent": {
    "github-helper": {
      "tools": { "github_*": true }
    }
  }
}
```

This pattern is recommended by the [OpenCode MCP docs](https://opencode.ai/docs/mcp-servers/) for servers with many tools.

## Troubleshooting

- **`401 Unauthorized` from the remote server**: confirm your PAT is valid and not expired. If you set `oauth: false`, OpenCode will not attempt an OAuth fallback — the `Authorization` header must be correct.
- **Server marked failed in `opencode mcp list`**: run `opencode mcp debug github` to see the exact connectivity and auth diagnostics.
- **Tools missing from prompts**: check that `enabled: true` is set on the server and that you have not disabled `github_*` in your `tools` block without re-enabling it for the current agent.
- **Context window exceeded**: the GitHub MCP server can register many tools. Use server-side toolset filtering (`X-MCP-Toolsets` header) to register only the toolsets you need.
- **Docker errors on the local server**: ensure Docker Desktop is running and the `ghcr.io/github/github-mcp-server` image has been pulled (`docker pull ghcr.io/github/github-mcp-server`).

## Important Notes

- **Configuration key**: OpenCode uses `mcp` (not `mcpServers` or `context_servers`).
- **Type discriminator**: every entry must include `"type": "local"` or `"type": "remote"`.
- **Command shape**: `command` is a single array combining the executable and its arguments.
- **Environment variable key**: `environment` (not `env`).
- **OAuth**: enabled by default for remote servers. Set `"oauth": false` when using PAT-in-`Authorization`, otherwise OpenCode may try OAuth first.
- **Env interpolation**: use `{env:VAR_NAME}` in string values to read from the shell environment instead of hard-coding secrets.
