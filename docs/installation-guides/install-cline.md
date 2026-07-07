# Install GitHub MCP Server in Cline

[Cline](https://github.com/cline/cline) is an AI coding assistant that runs in VS Code-compatible editors (VS Code, Cursor, Windsurf, etc.). For general setup information (prerequisites, Docker installation, security best practices), see the [Installation Guides README](./README.md).

## Remote Server

Cline stores MCP settings in `cline_mcp_settings.json`. To edit it, click the Cline icon in your editor's sidebar, open the menu in the top right corner of the Cline panel, and select **"MCP Servers"**. You can add a remote server through the **"Remote Servers"** tab, or click **"Configure MCP Servers"** to edit the JSON directly.

```json
{
  "mcpServers": {
    "github": {
      "url": "https://api.githubcopilot.com/mcp/",
      "type": "streamableHttp",
      "disabled": false,
      "headers": {
        "Authorization": "Bearer <YOUR_GITHUB_PAT>"
      },
      "autoApprove": []
    }
  }
}
```

Replace `YOUR_GITHUB_PAT` with your [GitHub Personal Access Token](https://github.com/settings/tokens). To customize toolsets, add server-side headers like `X-MCP-Toolsets` or `X-MCP-Readonly` to the `headers` object — see [Server Configuration Guide](../server-configuration.md).

> **Important:** The transport type must be `"streamableHttp"` (camelCase, no hyphen). Using `"streamable-http"` or omitting the type will cause Cline to fall back to SSE, resulting in a `405` error.

## Local Server (Docker)

1. Click the Cline icon in your editor's sidebar (or open the command palette and search for "Cline"), then click the **MCP Servers** icon (server stack icon at the top of the Cline panel), and click **"Configure MCP Servers"** to open `cline_mcp_settings.json`.
2. Add one of the configurations below. The OAuth option needs no token; for the PAT option, replace `YOUR_GITHUB_PAT` with your [GitHub Personal Access Token](https://github.com/settings/tokens).

Log in with OAuth instead of a token. On github.com the official image already includes the app credentials, so you provide none yourself — the server opens a browser login on first use and keeps the token in memory only. In Docker, publish a fixed callback port to loopback:

```json
{
  "mcpServers": {
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
  "mcpServers": {
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

## Troubleshooting

- **SSE error 405 with remote server**: Ensure `"type"` is set to `"streamableHttp"` (camelCase, no hyphen) in `cline_mcp_settings.json`. Using `"streamable-http"` or omitting `"type"` causes Cline to fall back to SSE, which this server does not support.
- **Authentication failures**: Verify your PAT has the required scopes
- **Docker issues**: Ensure Docker Desktop is installed and running
