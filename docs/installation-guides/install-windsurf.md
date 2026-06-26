# Install GitHub MCP Server in Windsurf

## Prerequisites
1. Windsurf IDE installed (latest version)
2. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) with appropriate scopes
3. For local installation: [Docker](https://www.docker.com/) installed and running

## Remote Server Setup (Recommended)

The remote GitHub MCP server is hosted by GitHub at `https://api.githubcopilot.com/mcp/` and supports Streamable HTTP protocol. Windsurf currently supports PAT authentication only.

### Streamable HTTP Configuration
Windsurf supports Streamable HTTP servers with a `serverUrl` field:

```json
{
  "mcpServers": {
    "github": {
      "serverUrl": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer YOUR_GITHUB_PAT"
      }
    }
  }
}
```

## Local Server Setup

### Docker Installation (Required)
**Important**: The npm package `@modelcontextprotocol/server-github` is no longer supported as of April 2025. Use the official Docker image `ghcr.io/github/github-mcp-server` instead.

Log in with OAuth instead of a token. On github.com the official image already includes the app credentials, so you provide none yourself — the server opens a browser login on first use and keeps the token in memory only. In Docker, publish a fixed callback port to loopback:

```json
{
  "mcpServers": {
    "github": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "-p",
        "127.0.0.1:8085:8085",
        "-e",
        "GITHUB_OAUTH_CALLBACK_PORT",
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

### Via Plugin Store
1. Open Windsurf and navigate to Cascade
2. Click the **Plugins** icon or **hammer icon** (🔨)
3. Search for "GitHub MCP Server"
4. Click **Install** and enter your PAT when prompted
5. Click **Refresh** (🔄)

### Manual Configuration
1. Click the hammer icon (🔨) in Cascade
2. Click **Configure** to open `~/.codeium/windsurf/mcp_config.json`
3. Add your chosen configuration from above
4. Save the file
5. Click **Refresh** (🔄) in the MCP toolbar

## Configuration Details

- **File path**: `~/.codeium/windsurf/mcp_config.json`
- **Scope**: Global configuration only (no per-project support)
- **Format**: Must be valid JSON (use a linter to verify)

## Verification

After installation:
1. Look for "1 available MCP server" in the MCP toolbar
2. Click the hammer icon to see available GitHub tools
3. Test with: "List my GitHub repositories"
4. Check for green dot next to the server name

## Troubleshooting

### Remote Server Issues
- **Authentication failures**: Verify PAT has correct scopes and hasn't expired
- **Connection errors**: Check firewall/proxy settings for HTTPS connections
- **Streamable HTTP not working**: Ensure you're using the correct `serverUrl` field format

### Local Server Issues
- **Docker errors**: Ensure Docker Desktop is running
- **Image pull failures**: Try `docker logout ghcr.io` then retry
- **Docker not found**: Install Docker Desktop and ensure it's running

### General Issues
- **Invalid JSON**: Validate with [jsonlint.com](https://jsonlint.com)
- **Tools not appearing**: Restart Windsurf completely
- **Check logs**: `~/.codeium/windsurf/logs/`

## Important Notes

- **Official repository**: [github/github-mcp-server](https://github.com/github/github-mcp-server)
- **Remote server URL**: `https://api.githubcopilot.com/mcp/`
- **Docker image**: `ghcr.io/github/github-mcp-server` (official and supported)
- **npm package**: `@modelcontextprotocol/server-github` (deprecated as of April 2025 - no longer functional)
- **Windsurf limitations**: No environment variable interpolation, global config only
