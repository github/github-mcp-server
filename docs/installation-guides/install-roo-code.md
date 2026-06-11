# Install GitHub MCP Server in Roo Code

[Roo Code](https://github.com/RooCodeInc/Roo-Code) is an AI coding assistant that runs in VS Code-compatible editors (VS Code, Cursor, Windsurf, etc.). For general setup information (prerequisites, Docker installation, security best practices), see the [Installation Guides README](./README.md).

## Remote Server

### Step-by-step setup

1. Click the **Roo Code icon** in your editor's sidebar to open the Roo Code pane
2. Click the **gear icon** (⚙️) in the top navigation of the Roo Code pane, then click on **"MCP Servers"** icon on the left.
3. Scroll to the bottom and click **"Edit Global MCP"** (for all projects) or **"Edit Project MCP"** (for the current project only)
4. Add the configuration below to the opened file (`mcp_settings.json` or `.roo/mcp.json`)
5. Replace `YOUR_GITHUB_PAT` with your [GitHub Personal Access Token](https://github.com/settings/tokens)
6. Save the file — the server should connect automatically

```json
{
  "mcpServers": {
    "github": {
      "type": "streamable-http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer YOUR_GITHUB_PAT"
      }
    }
  }
}
```

> **Important:** The `type` must be `"streamable-http"` (with hyphen). Using `"http"` or omitting the type will fail.

To customize toolsets, add server-side headers like `X-MCP-Toolsets` or `X-MCP-Readonly` to the `headers` object — see [Server Configuration Guide](../server-configuration.md).

## Local Server (Docker)

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

- **Connection failures**: Ensure `type` is `streamable-http`, not `http`
- **Authentication failures**: Verify PAT is prefixed with `Bearer ` in the `Authorization` header
- **Docker issues**: Ensure Docker Desktop is running
