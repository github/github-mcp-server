# Install GitHub MCP Server in Google Gemini CLI

## Prerequisites
1. Google Gemini CLI installed (see [official Gemini CLI documentation](https://github.com/google-gemini/gemini-cli))
2. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) with appropriate scopes
3. For local installation: [Docker](https://www.docker.com/) or [Podman](https://podman.io) installed and running

## GitHub MCP Server Configuration

### Method 1: Remote Server (Recommended)

The simplest way is to use GitHub's hosted MCP server:

```json
{
  "mcpServers": {
    "github": {
      "httpUrl": "https://api.githubcopilot.com/mcp/",
      "trust": true,
      "headers": {
        "Authorization": "Bearer YOUR_GITHUB_PAT"
      }
    }
  }
}
```

Create or update `~/.gemini/.env` with your environment variables:
```bash
# ~/.gemini/.env
GITHUB_PAT=ghp_sample_sample_sample
GOOGLE_CLOUD_PROJECT=my-gcp-project
GEMINI_API_KEY=AIzSamplelBQGwHw-62R598eI-HFOWd4Ol847g
```

### Method 2: Local Docker Setup (Alternative)

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

### Binary Configuration (Alternative)

If you prefer not to use Docker, you can build from source:

```bash
# Clone and build the server
git clone https://github.com/github/github-mcp-server.git
cd github-mcp-server
go build -o github-mcp-server ./cmd/github-mcp-server
```

Then configure:

```json
{
  "mcpServers": {
    "github": {
      "command": "/path/to/github-mcp-server",
      "args": ["stdio"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "YOUR_GITHUB_PAT"
      }
    }
  }
}
```

## Installation Steps

### Configuration File Location

Gemini CLI uses a settings JSON file to configure MCP servers:

- **Global configuration**: `~/.gemini/settings.json`
- **Project-specific**: `.gemini/settings.json` in your project directory

### Setup Steps

1. Create or edit your settings file with your chosen configuration from above
2. Replace `YOUR_GITHUB_PAT` with your actual [GitHub Personal Access Token](https://github.com/settings/tokens)
3. Save the file
4. Restart Gemini CLI if it was running

### Using Environment Variables (Recommended)

For better security, use environment variables:

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
        "GITHUB_PERSONAL_ACCESS_TOKEN": "$GITHUB_PAT"
      }
    }
  }
}
```

Then add the environment variable to `~/.gemini/.env`:
```bash
GITHUB_PAT=your_github_pat
```

## Configuration Options

### Toolset Configuration

Enable specific GitHub API capabilities:

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
        "-e",
        "GITHUB_TOOLSETS",
        "ghcr.io/github/github-mcp-server"
      ],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "$GITHUB_PAT",
        "GITHUB_TOOLSETS": "repos,issues,pull_requests,actions"
      }
    }
  }
}
```

### Read-Only Mode

For security, run the server in read-only mode:

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
        "-e",
        "GITHUB_READ_ONLY",
        "ghcr.io/github/github-mcp-server"
      ],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "$GITHUB_PAT",
        "GITHUB_READ_ONLY": "1"
      }
    }
  }
}
```

### GitHub Enterprise Support

For GitHub Enterprise Server or Enterprise Cloud with data residency:

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
        "-e",
        "GITHUB_HOST",
        "ghcr.io/github/github-mcp-server"
      ],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "$GITHUB_PAT",
        "GITHUB_HOST": "https://your-github-enterprise.com"
      }
    }
  }
}
```

## Verification

After configuration, verify the installation:

1. **Check MCP server status**:
   ```bash
   gemini --prompt "/mcp list"
   ```

2. **List available tools**:
   ```bash
   gemini --prompt "/tools"
   ```

3. **Test with a simple command**:
   ```bash
   gemini "List my GitHub repositories"
   ```

## Usage Examples

Once configured, use natural language commands:

```bash
# Repository operations
gemini "Show me the latest commits in microsoft/vscode"

# Issue management  
gemini "Create an issue titled 'Bug report' in my-org/my-repo"

# Pull request operations
gemini "Review the latest pull request in my repository"

# Code analysis
gemini "Analyze the security alerts in my repositories"
```

## Troubleshooting

### Local Server Issues
- **Docker errors**: Ensure Docker Desktop is running
  ```bash
  docker --version
  ```
- **Image pull failures**: Try `docker logout ghcr.io` then retry
- **Docker not found**: Install Docker Desktop and ensure it's running

### Authentication Issues  
- **Invalid PAT**: Verify your GitHub PAT has correct scopes:
  - `repo` - Repository operations
  - `read:packages` - Docker image access (if using Docker)
- **Token expired**: Generate a new GitHub PAT

### Configuration Issues
- **Invalid JSON**: Validate your configuration:
  ```bash
  cat ~/.gemini/config.json | jq .
  ```
- **MCP connection issues**: Check logs for connection errors:
  ```bash
  gemini --debug "test command"
  ```

### Debug Mode

Enable debug mode for detailed logging:

```bash
gemini --debug "Your command here"
```

## Important Notes

- **Official repository**: [github/github-mcp-server](https://github.com/github/github-mcp-server)
- **Docker image**: `ghcr.io/github/github-mcp-server` (official and supported)
- **Gemini CLI specifics**: Uses `mcpServers` key, supports both global and project configurations
- **Remote server method**: Preferred approach using GitHub's hosted MCP server at `https://api.githubcopilot.com/mcp/`