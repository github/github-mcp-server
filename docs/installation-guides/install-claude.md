# Install GitHub MCP Server in Claude Applications

This guide covers installation of the GitHub MCP server for Claude Code CLI, Claude Desktop, and Claude Web applications.

## Claude Web (claude.ai)

Claude Web supports remote MCP servers through the Integrations built-in feature.

### Prerequisites

1. Claude Pro, Team, or Enterprise account (Integrations not available on free plan)
2. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new)

### Installation

**Note**: As of July 2025, the remote GitHub MCP Server has known compatibility issues with Claude Web. While Claude Web supports remote MCP servers from other providers (like Atlassian, Zapier, Notion), the GitHub MCP Server integration may not work reliably.

For other remote MCP servers that do work with Claude Web:

1. Go to [claude.ai](https://claude.ai) and log in
2. Click your profile icon → **Settings**
3. Navigate to **Integrations** section
4. Click **+ Add integration** or **Add More**
5. Enter the remote server URL
6. Follow the OAuth authentication flow when prompted

**Alternative**: Use Claude Desktop or Claude Code CLI for reliable GitHub MCP Server integration.

---

## Claude Code CLI

Claude Code CLI provides command-line access to Claude with MCP server integration.

### Prerequisites

1. Claude Code CLI installed
2. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new)
3. [Docker](https://www.docker.com/) installed and running

### Installation

Run the following command to add the GitHub MCP server using Docker:

```bash
claude mcp add github -- docker run -i --rm -e GITHUB_PERSONAL_ACCESS_TOKEN ghcr.io/github/github-mcp-server
```

Then set the environment variable:
```bash
claude mcp update github -e GITHUB_PERSONAL_ACCESS_TOKEN=your_github_pat
```

Or as a single command with the token inline:
```bash
claude mcp add-json github '{"command": "docker", "args": ["run", "-i", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN", "ghcr.io/github/github-mcp-server"], "env": {"GITHUB_PERSONAL_ACCESS_TOKEN": "your_github_pat"}}'
```

**Important**: The npm package `@modelcontextprotocol/server-github` is no longer supported as of April 2025. Use the official Docker image `ghcr.io/github/github-mcp-server` instead.

### Configuration Options

- Use `-s user` to add the server to your user configuration (available across all projects)
- Use `-s project` to add the server to project-specific configuration (shared via `.mcp.json`)
- Default scope is `local` (available only to you in the current project)

### Verification

Run the following command to verify the installation:
```bash
claude mcp list
```

---

## Claude Desktop

Claude Desktop provides a graphical interface for interacting with the GitHub MCP Server.

### Prerequisites

1. Claude Desktop installed
2. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new)
3. [Docker](https://www.docker.com/) installed and running

### Configuration File Location

- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux**: `~/.config/Claude/claude_desktop_config.json` (unofficial support)

### Installation

Add the following to your `claude_desktop_config.json`:

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
        "GITHUB_PERSONAL_ACCESS_TOKEN": "your_github_pat"
      }
    }
  }
}
```

**Important**: The npm package `@modelcontextprotocol/server-github` is no longer supported as of April 2025. Use the official Docker image `ghcr.io/github/github-mcp-server` instead.

### Using Environment Variables

Claude Desktop supports environment variable references. You can use:

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

Then set the environment variable in your system before starting Claude Desktop.

### Installation Steps

1. Open Claude Desktop
2. Go to Settings (from the Claude menu) → Developer → Edit Config
3. Add your chosen configuration
4. Save the file
5. Restart Claude Desktop

### Verification

After restarting, you should see:
- An MCP icon in the Claude Desktop interface
- The GitHub server listed as "running" in Developer settings

---

## Troubleshooting

### Claude Web
- Currently experiencing compatibility issues with the GitHub MCP Server
- Try other remote MCP servers (Atlassian, Zapier, Notion) which work reliably
- Use Claude Desktop or Claude Code CLI as alternatives for GitHub integration

### Claude Code CLI
- Verify the command syntax is correct (note the single quotes around the JSON)
- Ensure Docker is running: `docker --version`
- Use `/mcp` command within Claude Code to check server status

### Claude Desktop
- Check logs at:
  - **macOS**: `~/Library/Logs/Claude/`
  - **Windows**: `%APPDATA%\Claude\logs\`
- Look for `mcp-server-github.log` for server-specific errors
- Ensure configuration file is valid JSON
- Try running the Docker command manually in terminal to diagnose issues

### Common Issues
- **Invalid JSON**: Validate your configuration at [jsonlint.com](https://jsonlint.com)
- **PAT issues**: Ensure your GitHub PAT has required scopes
- **Docker not found**: Install Docker Desktop and ensure it's running
- **Docker image pull fails**: Try `docker logout ghcr.io` then retry

---

## Security Best Practices

- **Protect configuration files**: Set appropriate file permissions
- **Use environment variables** when possible instead of hardcoding tokens
- **Limit PAT scope** to only necessary permissions
- **Regularly rotate** your GitHub Personal Access Tokens
- **Never commit** configuration files containing tokens to version control

---

## Additional Resources

- [Model Context Protocol Documentation](https://modelcontextprotocol.io)
- [Claude Code MCP Documentation](https://docs.anthropic.com/en/docs/claude-code/mcp)
- [Claude Web Integrations Support](https://support.anthropic.com/en/articles/11175166-about-custom-integrations-using-remote-mcp)
