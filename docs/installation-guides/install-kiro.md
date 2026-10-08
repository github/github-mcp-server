# Install GitHub MCP Server in Kiro

## Prerequisites

1. [Kiro](https://kiro.dev) installed (latest version)
2. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) with appropriate scopes
3. For local installation: [Docker](https://www.docker.com/) installed and running

## Remote Server Setup (Recommended)

[![Add to Kiro](https://kiro.dev/images/add-to-kiro.svg)](https://kiro.dev/launch/mcp/add?name=github&config=%7B%22type%22%3A%22http%22%2C%22url%22%3A%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2F%22%7D)

Uses GitHub's hosted server at https://api.githubcopilot.com/mcp/. Clicking the install button opens a confirmation dialog in Kiro that shows the server name, URL, and full config before anything is written. The GitHub server currently requires a Personal Access Token, so add the `Authorization` header shown below after installing.

### Install steps

1. Click the install button above and confirm in the dialog, or open your MCP config file (see [Configuration Files](#configuration-files)) and enter the code block below.
2. Replace `YOUR_GITHUB_PAT` with your actual [GitHub Personal Access Token](https://github.com/settings/tokens).
3. Save the file. Kiro reconnects the server automatically.

### Streamable HTTP Configuration

```json
{
  "mcpServers": {
    "github": {
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer YOUR_GITHUB_PAT"
      }
    }
  }
}
```

## Local Server Setup

The local GitHub MCP server runs via Docker and requires Docker Desktop to be installed and running.

### Install steps

1. Open your MCP config file (see [Configuration Files](#configuration-files)) and enter the code block below.
2. Replace `YOUR_GITHUB_PAT` with your actual [GitHub Personal Access Token](https://github.com/settings/tokens).
3. Save the file. Kiro reconnects the server automatically.

### Docker Configuration

```json
{
  "mcpServers": {
    "github": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "-e",
        "GITHUB_PERSONAL_ACCESS_TOKEN",
        "ghcr.io/github/github-mcp-server"
      ],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "YOUR_GITHUB_PAT"
      }
    }
  }
}
```

> **Important**: The npm package `@modelcontextprotocol/server-github` is no longer supported as of April 2025. Use the official Docker image `ghcr.io/github/github-mcp-server` instead.

## Configuration Files

Kiro reads MCP servers from two locations (see the [Kiro MCP documentation](https://kiro.dev/docs/mcp/)):

- **Workspace (this project only)**: `.kiro/settings/mcp.json` in the project root
- **User (all projects)**: `~/.kiro/settings/mcp.json`

Open either from the command palette: **Kiro: Open workspace MCP config (JSON)** or **Kiro: Open user MCP config (JSON)**.

## Verify Installation

1. Open the Kiro panel and check the MCP servers tab for a connected "github" server.
2. In chat, check the available tools.
3. Test with: "List my GitHub repositories".

## Troubleshooting

### Remote Server Issues

- **Authentication failures**: Verify your PAT has the correct scopes and has not expired.
- **Connection errors**: Check firewall/proxy settings.

### Local Server Issues

- **Docker errors**: Ensure Docker Desktop is running.
- **Image pull failures**: Try `docker logout ghcr.io` then retry.
- **Docker not found**: Install Docker Desktop and ensure it's running.

### General Issues

- **MCP not loading**: Check the MCP servers tab in the Kiro panel for the server status and error messages (Output tab → "Kiro - MCP Logs").
- **Invalid JSON**: Validate that the JSON format is correct.

## Important Notes

- **Docker image**: `ghcr.io/github/github-mcp-server` (official and supported)
- **npm package**: `@modelcontextprotocol/server-github` (deprecated as of April 2025 - no longer functional)
- **Kiro specifics**: Supports both workspace and user configurations, uses the `mcpServers` key, and supports remote HTTP MCP servers.
