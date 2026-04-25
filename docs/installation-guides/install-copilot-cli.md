# Install GitHub MCP Server in Copilot CLI

The GitHub MCP server comes pre-installed in Copilot CLI, with read-only tools enabled by default.

## Built-in Server

To verify the server is available, from an active Copilot CLI session:

```bash
/mcp show github-mcp-server
```

### Per-Session Customization

Use CLI flags to customize the server for a session:

```bash
# Enable an additional toolset
copilot --add-github-mcp-toolset discussions

# Enable multiple additional toolsets
copilot --add-github-mcp-toolset discussions --add-github-mcp-toolset stargazers

# Enable all toolsets
copilot --enable-all-github-mcp-tools

# Enable a specific tool
copilot --add-github-mcp-tool list_discussions

# Disable the built-in server entirely
copilot --disable-builtin-mcps
```

Run `copilot --help` for all available flags. For the list of toolsets, see [Available toolsets](../../README.md#available-toolsets); for the list of tools, see [Tools](../../README.md#tools).

## Custom Configuration

You can configure the GitHub MCP server in Copilot CLI using either the interactive command or by manually editing the configuration file.

> **Server naming:** Name your server `github-mcp-server` to replace the built-in server, or use a different name (e.g., `github`) to run alongside it.

### Prerequisites

1. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) with appropriate scopes
2. For local server: [Docker](https://www.docker.com/) installed and running

<details>
<summary><b>Storing Your PAT Securely</b></summary>
<br>

To set your PAT as an environment variable:

```bash
# Add to your shell profile (~/.bashrc, ~/.zshrc, etc.)
export GITHUB_PERSONAL_ACCESS_TOKEN=your_token_here
```

</details>

### Method 1: Interactive Setup (Recommended)

From an active Copilot CLI session, run the interactive command:

```bash
/mcp add
```

Follow the prompts to configure the server.

### Method 2: Manual Setup

Create or edit the configuration file `~/.copilot/mcp-config.json` and add one of the following configurations:

#### Remote Server

Connect to the hosted MCP server:

```json
{
  "mcpServers": {
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer ${GITHUB_PERSONAL_ACCESS_TOKEN}"
      }
    }
  }
}
```

For additional options like toolsets and read-only mode, see the [remote server documentation](../remote-server.md#optional-headers).

#### Local Docker

With Docker running, you can run the GitHub MCP server in a container:

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
        "GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_PERSONAL_ACCESS_TOKEN}"
      }
    }
  }
}
```

#### Binary

You can download the latest binary release from the [GitHub releases page](https://github.com/github/github-mcp-server/releases) or build it from source by running:

```bash
go build -o github-mcp-server ./cmd/github-mcp-server
```

Then configure (replace `/path/to/binary` with the actual path):

```json
{
  "mcpServers": {
    "github": {
      "command": "/path/to/binary",
      "args": ["stdio"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_PERSONAL_ACCESS_TOKEN}"
      }
    }
  }
}
```

## Verification

1. Restart Copilot CLI
2. Run `/mcp show` to list configured servers
3. Try: "List my GitHub repositories"

## Troubleshooting

### Local Server Issues

- **Docker errors**: Ensure Docker Desktop is running
- **Image pull failures**: Try `docker logout ghcr.io` then retry

### Authentication Issues

- **Invalid PAT**: Verify your GitHub PAT has correct scopes:
  - `repo` - Repository operations
  - `read:packages` - Docker image access (if using Docker)
- **Token expired**: Generate a new GitHub PAT

### Configuration Issues

- **Invalid JSON**: Validate your configuration:
  ```bash
  cat ~/.copilot/mcp-config.json | jq .
  ```

## References

- [Copilot CLI Documentation](https://docs.github.com/en/copilot/concepts/agents/about-copilot-cli)
