# Install GitHub MCP Server in fx

[fx](https://fx.sh) is an open source coding agent for the terminal. It reads MCP servers from `~/.fx/mcp.json` under an `mcp` key. For general setup information (prerequisites, Docker installation, security best practices), see the [Installation Guides README](./README.md).

## Prerequisites

1. fx installed (`curl -fsSL https://fx.sh/setup.sh | bash` or see [fx installation docs](https://fx.sh/docs/getting-started/installation))
2. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) with appropriate scopes
3. For local installation: [Docker](https://www.docker.com/) installed and running

> [!IMPORTANT]
> fx rejects a literal `Authorization` header in `mcp.json` so credentials do not become ordinary profile data. Use `bearer_token_env` instead, which reads the token from an environment variable at connection time.

## Remote Server (Recommended)

Uses GitHub's hosted server at `https://api.githubcopilot.com/mcp/`. Add it from your terminal:

```sh
fx mcp add --transport http github https://api.githubcopilot.com/mcp/
```

Then open `~/.fx/mcp.json` and add `bearer_token_env` to the saved entry:

```json
{
  "mcp": {
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "bearer_token_env": "GITHUB_PERSONAL_ACCESS_TOKEN"
    }
  }
}
```

Export the token in the shell that starts fx:

```sh
export GITHUB_PERSONAL_ACCESS_TOKEN=your_pat_here
```

fx sends the value as a bearer token on every request. The token is never written to `mcp.json`.

### Limiting toolsets

The GitHub MCP server registers many tools. To keep prompts within your model's context window, filter server side with the `X-MCP-Toolsets` header, which fx sends through `headers`:

```json
{
  "mcp": {
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "bearer_token_env": "GITHUB_PERSONAL_ACCESS_TOKEN",
      "headers": {
        "X-MCP-Toolsets": "repos,issues,pull_requests"
      }
    }
  }
}
```

See the [Server Configuration Guide](../server-configuration.md) and the [main README's toolsets section](../../README.md#available-toolsets).

## Local Server (Docker)

The local GitHub MCP server runs via Docker and requires Docker Desktop (or another Docker runtime) to be installed and running.

```json
{
  "mcp": {
    "github": {
      "type": "local",
      "command": [
        "docker", "run", "-i", "--rm",
        "-e", "GITHUB_PERSONAL_ACCESS_TOKEN",
        "ghcr.io/github/github-mcp-server"
      ],
      "environment": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "your_pat_here"
      }
    }
  }
}
```

To log in with OAuth instead of a token, publish a fixed callback port to loopback:

```json
{
  "mcp": {
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
```

See **[Local Server OAuth Login](../oauth-login.md)** for the native-binary flow, headless fallback, GitHub Enterprise, and bringing your own OAuth or GitHub App.

The first launch of a container or package can exceed the default startup timeout. Raise it for that server with `"startup_timeout_ms": 60000`.

## Verify Installation

1. Connect and inspect the server without opening a session:

   ```sh
   fx mcp list --connect
   ```

   The `github` entry should report `state=ready` along with its negotiated name and tool count.

2. Try a prompt that references the server by name:

   ```
   Use the github MCP server to list my recently merged pull requests.
   ```

## Managing the Server

| Command | Purpose |
| --- | --- |
| `fx mcp list` | List configured servers from `mcp.json` without connecting. |
| `fx mcp list --connect` | Connect and discover before rendering health. |
| `fx mcp remove github` | Remove the server from the profile. |
| `fx mcp path` | Print the path of the file fx reads. |
| `/mcp reload` | Apply a hand edit without restarting an open session. |

## Troubleshooting

- **`401 Unauthorized` from the remote server**: confirm the environment variable named in `bearer_token_env` is exported in the shell that starts fx, and that the PAT is valid and not expired.
- **Server reports `state=failed`**: run `fx mcp list --connect` for the failure line, and see the [Installation Guides README](./README.md) for shared troubleshooting.
- **Context window exceeded**: the GitHub MCP server registers many tools. Filter with the `X-MCP-Toolsets` header shown above.
- **Docker errors on the local server**: ensure Docker is running and the image has been pulled (`docker pull ghcr.io/github/github-mcp-server`).

## Important Notes

- **Configuration key**: fx uses `mcp` (not `mcpServers`).
- **Config location**: `~/.fx/mcp.json`. Run `fx mcp path` to print it.
- **Type discriminator**: `"type": "http"` for the remote server, `"type": "local"` for stdio.
- **Command shape**: `command` is a single array combining the executable and its arguments.
- **Environment variable key**: `environment` (`env` is also accepted).
- **Credentials**: a literal `Authorization` header is rejected. Use `bearer_token_env` for a bearer token, or `header_env` to map a header name to an environment variable.
- **CLI and session commands**: every `fx mcp` subcommand is also available inside a session as `/mcp`, for example `/mcp list`.
