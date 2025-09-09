# Install GitHub MCP Server in Cursor

## Prerequisites

1. Cursor IDE installed (latest version)
2. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) with appropriate scopes
3. For local installation: [Docker](https://www.docker.com/) installed and running

## Remote Server Setup (Recommended)

[![Install MCP Server](https://cursor.com/deeplink/mcp-install-dark.svg)](https://cursor.com/en/install-mcp?name=github&config=eyJ1cmwiOiJodHRwczovL2FwaS5naXRodWJjb3BpbG90LmNvbS9tY3AvIiwiaGVhZGVycyI6eyJBdXRob3JpemF0aW9uIjoiQmVhcmVyIFlPVVJfR0lUSFVCX1BBVCJ9fQ%3D%3D)

Uses GitHub's hosted server at https://api.githubcopilot.com/mcp/. Requires Cursor v0.48.0+ for Streamable HTTP support. While Cursor supports OAuth for some MCP servers, the GitHub server currently requires a Personal Access Token.

### Install steps

1. Click the install button above and follow the flow, or go directly to your global MCP configuration file at `~/.cursor/mcp.json` and enter the code block below
2. In Tools & Integrations > MCP tools, click the pencil icon next to "github"
3. Replace `YOUR_GITHUB_PAT` with your actual [GitHub Personal Access Token](https://github.com/settings/tokens)
4. Save the file
5. Restart Cursor

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

[![Install MCP Server](https://cursor.com/deeplink/mcp-install-dark.svg)](https://cursor.com/en/install-mcp?name=github&config=eyJjb21tYW5kIjoiZG9ja2VyIHJ1biAtaSAtLXJtIC1lIEdJVEhVQl9QRVJTT05BTF9BQ0NFU1NfVE9LRU4gZ2hjci5pby9naXRodWIvZ2l0aHViLW1jcC1zZXJ2ZXIiLCJlbnYiOnsiR0lUSFVCX1BFUlNPTkFMX0FDQ0VTU19UT0tFTiI6IllPVVJfR0lUSFVCX1BBVCJ9fQ%3D%3D)

The local GitHub MCP server runs via Docker and requires Docker Desktop to be installed and running.

### Install steps

1. Click the install button above and follow the flow, or go directly to your global MCP configuration file at `~/.cursor/mcp.json` and enter the code block below
2. In Tools & Integrations > MCP tools, click the pencil icon next to "github"
3. Replace `YOUR_GITHUB_PAT` with your actual [GitHub Personal Access Token](https://github.com/settings/tokens)
4. Save the file
5. Restart Cursor

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

- **Global (all projects)**: `~/.cursor/mcp.json`
- **Project-specific**: `.cursor/mcp.json` in project root

## Verify Installation

1. Restart Cursor completely
2. Check for green dot in Settings → Tools & Integrations → MCP Tools
3. In chat/composer, check "Available Tools"
4. Test with: "List my GitHub repositories"

## Troubleshooting

### Remote Server Issues

- **Streamable HTTP not working**: Ensure you're using Cursor v0.48.0 or later
- **Authentication failures**: Verify PAT has correct scopes
- **Connection errors**: Check firewall/proxy settings

### Local Server Issues

- **Docker errors**: Ensure Docker Desktop is running
- **Image pull failures**: Try `docker logout ghcr.io` then retry
- **Docker not found**: Install Docker Desktop and ensure it's running

### General Issues

- **MCP not loading**: Restart Cursor completely after configuration
- **Invalid JSON**: Validate that json format is correct
- **Tools not appearing**: Check server shows green dot in MCP settings
- **Check logs**: Look for MCP-related errors in Cursor logs

## Important Notes

- **Docker image**: `ghcr.io/github/github-mcp-server` (official and supported)
- **npm package**: `@modelcontextprotocol/server-github` (deprecated as of April 2025 - no longer functional)
- **Cursor specifics**: Supports both project and global configurations, uses `mcpServers` key
