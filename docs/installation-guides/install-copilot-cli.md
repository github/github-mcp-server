# Install GitHub MCP Server in Copilot CLI

## Prerequisites

1. Copilot CLI installed (see [official Copilot CLI documentation](https://docs.github.com/en/copilot/concepts/agents/about-copilot-cli))
2. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) with appropriate scopes
3. For local installation: [Docker](https://www.docker.com/) installed and running

<details>
<summary><b>Storing Your PAT Securely</b></summary>
<br>

To set your PAT as an environment variable:

```bash
# Add to your shell profile (~/.bashrc, ~/.zshrc, etc.)
export GITHUB_PERSONAL_ACCESS_TOKEN=your_token_here
```

</details>

## GitHub MCP Server Configuration

You can configure the GitHub MCP server in Copilot CLI using either the interactive command or by manually editing the configuration file.

### Method 1: Interactive Setup (Recommended)

Use the Copilot CLI to interactively add the MCP server:

```bash
/mcp add
```

Follow the prompts to configure the GitHub MCP server.

### Method 2: Manual Configuration

Create or edit the configuration file `~/.copilot/mcp-config.json` and add one of the following configurations:

#### Remote Server

Connect to the hosted MCP server:

```json
{
  "mcpServers": {
    "github": {
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer ${GITHUB_PERSONAL_ACCESS_TOKEN}"
      }
    }
  }
}
```

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

You can download the latest binary release from the [GitHub releases page](https://github.com/github/github-mcp-server/releases) or build it from source by running `go build -o github-mcp-server ./cmd/github-mcp-server`.

Then, replacing `/path/to/binary` with the actual path to your binary, configure Copilot CLI with:

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

To verify that the GitHub MCP server has been configured:

1. Start or restart Copilot CLI
2. The GitHub tools should be available for use in your conversations

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
    cat ~/.copilot/mcp-config.json | jq .
    ```

## References

- [Copilot CLI Documentation](https://docs.github.com/en/copilot/concepts/agents/about-copilot-cli)
