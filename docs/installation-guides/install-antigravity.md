# Installing GitHub MCP Server in Antigravity

This guide covers setting up the GitHub MCP Server in Google's Antigravity IDE.

## Prerequisites

- Antigravity IDE installed (latest version)
- GitHub Personal Access Token with appropriate scopes

## Installation Methods

### Option 1: Remote Server (Recommended)

Uses GitHub's hosted server at `https://api.githubcopilot.com/mcp/`.

> [!NOTE]
> We recommend this manual configuration method because the "official" installation via the Antigravity MCP Store currently has known issues (often resulting in Docker errors). This direct remote connection is more reliable.

#### Step 1: Access MCP Configuration

1. Open Antigravity
2. Click the "..." (Additional Options) menu in the Agent panel
3. Select "MCP Servers"
4. Click "Manage MCP Servers"
5. Click "View raw config"

This will open your `mcp_config.json` file at:
- **Windows**: `C:\Users\<USERNAME>\.gemini\antigravity\mcp_config.json`
- **macOS/Linux**: `~/.gemini/antigravity/mcp_config.json`

#### Step 2: Add Configuration

Add the following to your `mcp_config.json`:

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

**Important**: Note that Antigravity uses `serverUrl` instead of `url` for HTTP-based MCP servers.

#### Step 3: Configure Your Token

Replace `YOUR_GITHUB_PAT` with your actual GitHub Personal Access Token.

Create a token here: https://github.com/settings/tokens

Recommended scopes:
- `repo` - Full control of private repositories
- `read:org` - Read org and team membership
- `read:user` - Read user profile data

#### Step 4: Restart Antigravity

Close and reopen Antigravity for the changes to take effect.

#### Step 5: Verify Installation

1. Open the MCP Servers panel (... menu → MCP Servers)
2. You should see "github" with a list of available tools
3. You can now use GitHub tools in your conversations

> [!NOTE]
> The status indicator in the MCP Servers panel might not immediately turn green in some versions, but the tools will still function if configured correctly.

### Option 2: Local Docker Server

If you prefer running the server locally with Docker:

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

**Requirements**:
- Docker Desktop installed and running
- Docker must be in your system PATH

## Troubleshooting

### "Error: serverUrl or command must be specified"

Make sure you're using `serverUrl` (not `url`) for the remote server configuration. Antigravity requires `serverUrl` for HTTP-based MCP servers.

### Server not appearing in MCP list

- Verify JSON syntax in your config file
- Check that your PAT hasn't expired
- Restart Antigravity completely

### Tools not working

- Ensure your PAT has the correct scopes
- Check the MCP Servers panel for error messages
- Verify internet connection for remote server

## Available Tools

Once installed, you'll have access to tools like:
- `create_repository` - Create new GitHub repositories
- `push_files` - Push files to repositories
- `search_repositories` - Search for repositories
- `create_or_update_file` - Manage file content
- `get_file_contents` - Read file content
- And many more...

For a complete list of available tools and features, see the [main README](../../README.md).

## Differences from Other IDEs

- **Configuration key**: Antigravity uses `serverUrl` instead of `url` for HTTP servers
- **Config location**: `.gemini/antigravity/mcp_config.json` instead of `.cursor/mcp.json`
- **Tool limits**: Antigravity recommends keeping total enabled tools under 50 for optimal performance

## Next Steps

- Explore the [Server Configuration Guide](../server-configuration.md) for advanced options
- Check out [toolsets documentation](../../README.md#available-toolsets) to customize available tools
- See the [Remote Server Documentation](../remote-server.md) for more details
