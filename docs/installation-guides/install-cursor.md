# Install GitHub MCP Server in Cursor

## Prerequisites
1. Cursor IDE installed (latest version)
2. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) with appropriate scopes
3. For local installation: [Docker](https://www.docker.com/) installed and running

## Remote Server Setup (Recommended)

The remote GitHub MCP server is hosted by GitHub at `https://api.githubcopilot.com/mcp/` and supports Streamable HTTP protocol. Cursor currently supports remote servers with PAT authentication.

### Streamable HTTP Configuration
As of Cursor v0.48.0, Cursor supports Streamable HTTP servers directly:

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

**Note**: You may need to update to the latest version, if the current version doesn't support direct Streamable HTTP

## Local Server Setup

### Docker Installation (Required)
> **Important**: The npm package `@modelcontextprotocol/server-github` is no longer supported as of April 2025. Use the official Docker image `ghcr.io/github/github-mcp-server` instead.

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

## Installation Steps

### Via Cursor Settings UI
1. Open Cursor
2. Navigate to **Settings** → **Tools & Integrations** → **MCP**
3. Click **"+ Add new global MCP server"**
4. This opens `~/.cursor/mcp.json` in the editor
5. Add your chosen configuration from above
6. Save the file
7. Restart Cursor

### Manual Configuration
1. Create or edit the configuration file:
   - **Global (all projects)**: `~/.cursor/mcp.json`
   - **Project-specific**: `.cursor/mcp.json` in project root
2. Add your chosen configuration
3. Save the file
4. Restart Cursor completely

### Token Security
- Create PATs with minimum required scopes:
  - `repo` - For repository operations
  - `read:packages` - For Docker image pull (local setup)
  - Additional scopes based on tools you need
- Use separate PATs for different projects
- Regularly rotate tokens
- Never commit configuration files to version control

## Configuration Details

- **File paths**: 
  - Global: `~/.cursor/mcp.json`
  - Project: `.cursor/mcp.json`
- **Scope**: Both global and project-specific configurations supported
- **Format**: Must be valid JSON (use a linter to verify)

## Verification

After installation:
1. Restart Cursor completely
2. Open Settings → Tools & Integrations → MCP
3. Look for green dot next to your server name
4. In chat/composer, check "Available Tools"
5. Test with: "List my GitHub repositories"

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
