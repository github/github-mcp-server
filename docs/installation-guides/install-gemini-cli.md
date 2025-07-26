# Install GitHub MCP Server in Google Gemini CLI

## Prerequisites
1. Google Gemini CLI installed (see [Installation Options](#installation-options))
2. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) with appropriate scopes
3. For local installation: [Docker](https://www.docker.com/) installed and running

## Installation Options

### Option 1: npm (Global Installation)
```bash
npm install -g @google/gemini-cli
```

### Option 2: npx (No Installation Required)
```bash
npx https://github.com/google-gemini/gemini-cli
```

### Option 3: Homebrew
```bash
brew install gemini-cli
```

## Authentication Setup

Before using Gemini CLI, you need to authenticate with Google:

### Using Gemini API Key (Recommended)
1. Get your API key from [Google AI Studio](https://makersuite.google.com/app/apikey)
2. Set it as an environment variable:
   ```bash
   export GEMINI_API_KEY=your_api_key_here
   ```

### Using Vertex AI
1. Configure Google Cloud credentials for Vertex AI
2. Set the project ID:
   ```bash
   export GOOGLE_CLOUD_PROJECT=your_project_id
   ```

## Local Server Setup

**Important**: The npm package `@modelcontextprotocol/server-github` is no longer supported as of April 2025. Use the official Docker image `ghcr.io/github/github-mcp-server` instead.

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

- **Global configuration**: `~/.gemini-cli/settings.json`
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

Then set the environment variable:
```bash
export GITHUB_PAT=your_github_pat
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
   gemini-cli mcp status
   ```

2. **List available tools**:
   ```bash
   gemini-cli mcp list-tools
   ```

3. **Test with a simple command**:
   ```bash
   gemini-cli "List my GitHub repositories"
   ```

## Usage Examples

Once configured, use natural language commands:

```bash
# Repository operations
gemini-cli "Show me the latest commits in microsoft/vscode"

# Issue management  
gemini-cli "Create an issue titled 'Bug report' in my-org/my-repo"

# Pull request operations
gemini-cli "Review the latest pull request in my repository"

# Code analysis
gemini-cli "Analyze the security alerts in my repositories"
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
  cat ~/.gemini-cli/settings.json | jq .
  ```
- **MCP connection issues**: Check logs for connection errors:
  ```bash
  gemini-cli mcp logs github
  ```

### Debug Mode

Enable debug mode for detailed logging:

```bash
export GEMINI_CLI_DEBUG=1
gemini-cli "Your command here"
```

## Important Notes

- **Official repository**: [github/github-mcp-server](https://github.com/github/github-mcp-server)
- **Docker image**: `ghcr.io/github/github-mcp-server` (official and supported)
- **npm package**: `@modelcontextprotocol/server-github` (deprecated as of April 2025 - no longer functional)
- **Gemini CLI specifics**: Uses `mcpServers` key, supports both global and project configurations