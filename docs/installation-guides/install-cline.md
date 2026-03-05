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
2. Add the configuration below, replacing `YOUR_GITHUB_PAT` with your [GitHub Personal Access Token](https://github.com/settings/tokens).

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
