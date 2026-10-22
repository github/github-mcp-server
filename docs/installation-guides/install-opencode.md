# Install GitHub MCP Server in OpenCode

[OpenCode](https://opencode.ai) is a terminal-based AI coding agent that exposes MCP servers under the `mcp` key in `opencode.json` (or `opencode.jsonc`). For general setup information (prerequisites, Docker installation, security best practices), see the [Installation Guides README](./README.md).

> [!IMPORTANT]
> OpenCode v2 changed the MCP config shape. Servers live under `mcp.servers`, they connect automatically, and a server is disabled with `"disabled": true` instead of `"enabled": false`. The older `mcp.<name>` shape with an `enabled` field belongs to OpenCode v1, which is covered in [OpenCode v1](#opencode-v1) below. Pick the section for the version you run; check with `opencode --version`.

## Prerequisites

1. OpenCode installed (`brew install sst/tap/opencode` or see [OpenCode install docs](https://opencode.ai/docs/))
2. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) with appropriate scopes
3. For local installation: [Docker](https://www.docker.com/) installed and running

> [!IMPORTANT]
> The OpenCode docs note that the GitHub MCP server can add a lot of tokens to your context. Consider limiting toolsets — for example, by setting `X-MCP-Toolsets` on the remote server or `--toolsets` on the local server — to keep prompts within your model's context window. See the [Server Configuration Guide](../server-configuration.md) and the [main README's toolsets section](../../README.md#available-toolsets).

## OpenCode v2

### Remote Server (Recommended)

Uses GitHub's hosted server at `https://api.githubcopilot.com/mcp/`. The quickest path is the CLI:

```sh
opencode mcp add github --url https://api.githubcopilot.com/mcp/ \
  --header 'Authorization=Bearer {env:GITHUB_PERSONAL_ACCESS_TOKEN}'
```

Quote the `{env:...}` reference so the shell writes it literally rather than expanding it. Add `--global` to make the server available in every project instead of only the current one.

To configure it by hand, edit your [OpenCode config](https://opencode.ai/v2/docs/config) (typically `~/.config/opencode/opencode.json`, or `opencode.json` in your project root) and add the server under `mcp.servers`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "servers": {
      "github": {
        "type": "remote",
        "url": "https://api.githubcopilot.com/mcp/",
        "oauth": false,
        "headers": {
          "Authorization": "Bearer {env:GITHUB_PERSONAL_ACCESS_TOKEN}"
        }
      }
    }
  }
}
```

Replace the header with a literal `Bearer YOUR_GITHUB_PAT` if you would rather not use an environment variable, and set `GITHUB_PERSONAL_ACCESS_TOKEN` in your shell before starting OpenCode if you do.

The `oauth: false` setting disables OpenCode's automatic OAuth discovery and tells it to use the PAT in `Authorization` instead — without this, OpenCode may try the OAuth flow first. Remote servers use OAuth by default in v2.

### Local Server (Docker)

The local GitHub MCP server runs via Docker and requires Docker Desktop (or another Docker runtime) to be installed and running.

Log in with OAuth instead of a token. On github.com the official image already includes the app credentials, so you provide none yourself — the server opens a browser login on first use and keeps the token in memory only. In Docker, publish a fixed callback port to loopback:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "servers": {
      "github": {
        "type": "local",
        "command": [
          "docker", "run", "-i", "--rm",
          "-p", "127.0.0.1:8085:8085",
          "-e", "GITHUB_OAUTH_CALLBACK_PORT",
          "ghcr.io/github/github-mcp-server"
        ],
        "environment": {
          "GITHUB_OAUTH_CALLBACK_PORT": "8085"
        }
      }
    }
  }
}
```

See **[Local Server OAuth Login](../oauth-login.md)** for the native-binary flow (no fixed port), headless/device-code fallback, GitHub Enterprise, and bringing your own OAuth or GitHub App.

To authenticate with a Personal Access Token instead (it takes precedence over OAuth), pass the token through the environment and drop the OAuth port publishing:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "servers": {
      "github": {
        "type": "local",
        "command": [
          "docker", "run", "-i", "--rm",
          "-e", "GITHUB_PERSONAL_ACCESS_TOKEN",
          "ghcr.io/github/github-mcp-server"
        ],
        "environment": {
          "GITHUB_PERSONAL_ACCESS_TOKEN": "{env:GITHUB_PERSONAL_ACCESS_TOKEN}"
        }
      }
    }
  }
}
```

> [!IMPORTANT]
> OpenCode expects `command` as a **single array** combining the executable and its arguments (e.g. `["docker", "run", "-i", ...]`), and the env-var key is `environment` (not `env`). This differs from hosts like Zed and Cursor.

### Verify Installation

1. Restart OpenCode (or start a new session).
2. Check that the server is discovered and connected:
   ```sh
   opencode mcp list
   ```
   A connected server shows as `github  connected`. A remote server that still needs a login shows `needs authentication`.
3. Try a prompt that references the server by name to bias the model toward its tools:
   ```
   Use the github tool to list my recently merged pull requests.
   ```

### Managing the Server

| Command | Purpose |
| --- | --- |
| `opencode mcp list` | List configured MCP servers and their auth/connection status. |
| `opencode mcp add github --url <url>` | Add a remote server to the current project config. |
| `opencode mcp add github -- <command...>` | Add a local server; pass environment variables with repeated `--env` flags. |
| `opencode mcp auth github` | Start the OAuth flow for a server. |
| `opencode mcp logout github` | Clear stored OAuth tokens for the server. |

The `/mcps` menu inside OpenCode can connect, disconnect, and authenticate servers without editing the file.

### Disabling Tools Per-Agent

Because the GitHub MCP server can register a large number of tools, you may want to **hide them globally** and **allow them only for specific agents**. v2 matches permission actions against the normalized `<server>_<tool>` name, rather than using a `tools` block:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "servers": {
      "github": {
        "type": "remote",
        "url": "https://api.githubcopilot.com/mcp/",
        "oauth": false,
        "headers": { "Authorization": "Bearer {env:GITHUB_PERSONAL_ACCESS_TOKEN}" }
      }
    }
  },
  "permissions": [
    {
      "action": "github_*",
      "resource": "*",
      "effect": "deny"
    }
  ]
}
```

See the OpenCode [permissions docs](https://opencode.ai/v2/docs/permissions) for per-agent overrides.

### Keeping a Server Configured but Disconnected

Servers connect automatically in v2. To keep one configured without connecting it, replace `enabled` with `disabled`:

```json
{
  "mcp": {
    "servers": {
      "github": {
        "type": "remote",
        "url": "https://api.githubcopilot.com/mcp/",
        "disabled": true
      }
    }
  }
}
```

### Timeouts

v2 sets defaults under `mcp.timeout`, and a server's own `timeout` object overrides the matching keys:

```json
{
  "mcp": {
    "timeout": {
      "startup": 45000,
      "catalog": 30000
    },
    "servers": {
      "github": {
        "type": "remote",
        "url": "https://api.githubcopilot.com/mcp/",
        "timeout": {
          "catalog": 60000
        }
      }
    }
  }
}
```

`startup` covers the transport connection and `initialize`, `catalog` covers listing tools, prompts, and resources, and `execution` covers tool calls (default 12 hours). Raise `catalog` if `opencode mcp list` reports the GitHub server as failed on a slow connection.

## OpenCode v1

On OpenCode v1, servers sit directly under `mcp` and a server is enabled with `enabled: true`. The same remote configuration looks like this:

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

Note the three differences from v2: no `servers` level, an `enabled` field rather than `disabled`, and OAuth options in camelCase (`clientId`, `clientSecret`, `scope`).

On v1, tools are hidden with a `tools` block and re-enabled per agent:

```json
{
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

## Troubleshooting

- **`401 Unauthorized` from the remote server**: confirm your PAT is valid and not expired. If you set `oauth: false`, OpenCode will not attempt an OAuth fallback — the `Authorization` header must be correct.
- **Server marked failed in `opencode mcp list`**: check the config shape for your OpenCode version first. A v2 config read by v1 (or the reverse) usually shows up as a server that never connects, because the `disabled`/`enabled` field or the `servers` level is not recognised.
- **Tools missing from prompts**: check that the server is not `disabled`, and that you have not denied `github_*` in `permissions` (v2) or in `tools` (v1) without re-allowing it for the current agent.
- **Context window exceeded**: the GitHub MCP server can register many tools. Use server-side toolset filtering (`X-MCP-Toolsets` header) to register only the toolsets you need.
- **Docker errors on the local server**: ensure Docker Desktop is running and the `ghcr.io/github/github-mcp-server` image has been pulled (`docker pull ghcr.io/github/github-mcp-server`).

## Important Notes

- **Configuration key**: OpenCode uses `mcp` (not `mcpServers` or `context_servers`).
- **Server level**: v2 nests servers under `mcp.servers`; v1 places them directly under `mcp`.
- **Type discriminator**: every entry must include `"type": "local"` or `"type": "remote"`.
- **Command shape**: `command` is a single array combining the executable and its arguments.
- **Environment variable key**: `environment` (not `env`), in both versions.
- **Enable/disable**: v2 servers connect automatically and are disabled with `"disabled": true`; v1 uses `"enabled": true` / `"enabled": false`.
- **OAuth**: enabled by default for remote servers. Set `"oauth": false` when using PAT-in-`Authorization`, otherwise OpenCode may try OAuth first. v2 OAuth fields are snake_case (`client_id`, `client_secret`, `callback_port`, `redirect_uri`, `auth_server_metadata_url`).
- **Env interpolation**: use `{env:VAR_NAME}` in string values to read from the shell environment instead of hard-coding secrets.
