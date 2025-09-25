# Install GitHub MCP Server in Claude Applications

## Claude Code CLI

### Prerequisites
- Claude Code CLI installed
- [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new)
- For local setup: [Docker](https://www.docker.com/) installed and running
- Open Claude Code inside the directory for your project (recommended for best experience and clear scope of configuration)

<details>
<summary><b>Storing Your PAT Securely</b></summary>
<br>

For security, avoid hardcoding your token. One common approach:

1. Store your token in `.env` file
```
GITHUB_PAT=your_token_here
```

2. Add to .gitignore
```bash
echo -e ".env\n.mcp.json" >> .gitignore
```

</details>

### Remote Server Setup (Streamable HTTP)

1. Run the following command in the Claude Code CLI
```bash
claude mcp add --transport http github https://api.githubcopilot.com/mcp -H "Authorization: Bearer YOUR_GITHUB_PAT"
```

With an environment variable:
```bash
claude mcp add --transport http github https://api.githubcopilot.com/mcp -H "Authorization: Bearer $(grep GITHUB_PAT .env | cut -d '=' -f2)"
```
2. Restart Claude Code
3. Run `claude mcp list` to see if the GitHub server is configured

### Local Server Setup (Docker required)

### With Docker
1. Run the following command in the Claude Code CLI:
```bash
claude mcp add github -e GITHUB_PERSONAL_ACCESS_TOKEN=YOUR_GITHUB_PAT -- docker run -i --rm -e GITHUB_PERSONAL_ACCESS_TOKEN ghcr.io/github/github-mcp-server
```

With an environment variable:
```bash
claude mcp add github -e GITHUB_PERSONAL_ACCESS_TOKEN=$(grep GITHUB_PAT .env | cut -d '=' -f2) -- docker run -i --rm -e GITHUB_PERSONAL_ACCESS_TOKEN ghcr.io/github/github-mcp-server
```
2. Restart Claude Code
3. Run `claude mcp list` to see if the GitHub server is configured

### With a Binary (no Docker)

1. Download [release binary](https://github.com/github/github-mcp-server/releases)
2. Add to your `PATH`
3. Run:
```bash
claude mcp add-json github '{"command": "github-mcp-server", "args": ["stdio"], "env": {"GITHUB_PERSONAL_ACCESS_TOKEN": "YOUR_GITHUB_PAT"}}'
```
2. Restart Claude Code
3. Run `claude mcp list` to see if the GitHub server is configured

### Verification
```bash
claude mcp list
claude mcp get github
```

---

## Claude Desktop

> ⚠️ **Note**: Some users have reported compatibility issues with Claude Desktop and Docker-based MCP servers. We're investigating. If you experience issues, try using another MCP host, while we look into it!

### Prerequisites
- Claude Desktop installed (latest version)
- [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new)
- For local setup: [Docker](https://www.docker.com/) installed and running
- For remote setup: npm (Node.js) installed

### Configuration File Location
- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux**: `~/.config/Claude/claude_desktop_config.json`

### Remote Server Setup (Recommended - No Docker Required)

This method uses the GitHub MCP remote server without requiring Docker:

Add this configuration to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": [
        "mcp-remote",
        "https://api.githubcopilot.com/mcp",
        "--header",
        "Authorization: Bearer ${GITHUB_PERSONAL_ACCESS_TOKEN}"
      ],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "YOUR_GITHUB_PAT"
      }
    }
  }
}
```

> **Note**: Replace `YOUR_GITHUB_PAT` with your actual GitHub Personal Access Token. Ensure your token has the necessary `repo` scope permissions.

### Local Server Setup (Docker - Alternative Method)

Add this codeblock to your `claude_desktop_config.json`:

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

### Manual Setup Steps
1. Open Claude Desktop
2. Go to Settings → Developer → Edit Config
3. Choose one of the two methods above (Remote Server recommended, or Docker for local setup)
4. Paste the corresponding configuration in your `claude_desktop_config.json` file
5. If you're navigating to the configuration file outside of the app:
   - **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
   - **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
   - **Linux**: `~/.config/Claude/claude_desktop_config.json`
6. Replace `YOUR_GITHUB_PAT` with your actual GitHub Personal Access Token
7. Save the file
8. Restart Claude Desktop

---

## Troubleshooting

**Authentication Failed:**
- Verify PAT has `repo` scope
- Check token hasn't expired
- Ensure the token is correctly placed in your configuration

**Remote Server (Claude Desktop with npx mcp-remote):**
- Verify URL: `https://api.githubcopilot.com/mcp`
- Ensure npm is installed and accessible

**Remote Server (Claude Code):**
- Verify URL: `https://api.githubcopilot.com/mcp`

**Docker Issues (Local Setup Only):**
- Ensure Docker Desktop is running
- Try: `docker pull ghcr.io/github/github-mcp-server`
- If pull fails: `docker logout ghcr.io` then retry

**Server Not Starting / Tools Not Showing:**
- Run `claude mcp list` to view currently configured MCP servers
- Validate JSON syntax
- If using an environment variable to store your PAT, make sure you're properly sourcing your PAT using the environment variable
- Restart Claude Code and check `/mcp` command
- Delete the GitHub server by running `claude mcp remove github` and repeating the setup process with a different method
- Make sure you're running Claude Code within the project you're currently working on to ensure the MCP configuration is properly scoped to your project
- Check logs:
  - Claude Code: Use `/mcp` command
  - Claude Desktop: `ls ~/Library/Logs/Claude/` and `cat ~/Library/Logs/Claude/mcp-server-*.log` (macOS) or `%APPDATA%\Claude\logs\` (Windows)

---

## Important Notes

- The npm package `@modelcontextprotocol/server-github` is deprecated as of April 2025
- Remote server requires Streamable HTTP support (check your Claude version)
- Configuration scopes for Claude Code:
  - `-s user`: Available across all projects
  - `-s project`: Shared via `.mcp.json` file
  - Default: `local` (current project only)
