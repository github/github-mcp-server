# Install GitHub MCP Server in Google Gemini CLI

## Prerequisites

1. Google Gemini CLI installed (see [official Gemini CLI documentation](https://github.com/google-gemini/gemini-cli))
2. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) with appropriate scopes
3. For local installation: [Docker](https://www.docker.com/) installed and running

<details>
<summary><b>Storing Your PAT Securely</b></summary>
<br>

For security, avoid hardcoding your token. Create or update `~/.gemini/.env` (where `~` is your home or project directory) with your PAT:

```bash
# ~/.gemini/.env
GITHUB_MCP_PAT=your_token_here
```

</details>

## GitHub MCP Server Configuration

MCP servers for Gemini CLI are configured in its settings JSON under an `mcpServers` key.

- **Global configuration**: `~/.gemini/settings.json` where `~` is your home directory
- **Project-specific**: `.gemini/settings.json` in your project directory

After securely storing your PAT, you can add the GitHub MCP server configuration to your settings file using one of the methods below. You may need to restart the Gemini CLI for changes to take effect.

> **Note:** For the most up-to-date configuration options, see the [main README.md](../../README.md).

### Method 1: Gemini Extension (Recommended)

The simplest way is to use GitHub's hosted MCP server via our gemini extension.

`gemini extensions install https://github.com/github/github-mcp-server`

> [!NOTE]
> You will still need to have a personal access token with the appropriate scopes called `GITHUB_MCP_PAT` in your environment.

### Method 2: Remote Server

You can also connect to the hosted MCP server directly. After securely storing your PAT, configure Gemini CLI with:

```json
// ~/.gemini/settings.json
{
    "mcpServers": {
        "github": {
            "httpUrl": "https://api.githubcopilot.com/mcp/",
            "headers": {
                "Authorization": "Bearer $GITHUB_MCP_PAT"
            }
        }
    }
}
```

### Method 3: Local Docker

With docker running, you can run the GitHub MCP server in a container:

```json
// ~/.gemini/settings.json
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
                "GITHUB_PERSONAL_ACCESS_TOKEN": "$GITHUB_MCP_PAT"
            }
        }
    }
}
```

### Method 4: Binary

You can download the latest binary release from the [GitHub releases page](https://github.com/github/github-mcp-server/releases) or build it from source by running `go build -o github-mcp-server ./cmd/github-mcp-server`.

Then, replacing `/path/to/binary` with the actual path to your binary, configure Gemini CLI with:

```json
// ~/.gemini/settings.json
{
    "mcpServers": {
        "github": {
            "command": "/path/to/binary",
            "args": ["stdio"],
            "env": {
                "GITHUB_PERSONAL_ACCESS_TOKEN": "$GITHUB_MCP_PAT"
            }
        }
    }
}
```

## Verification

To verify that the GitHub MCP server has been configured, start Gemini CLI in your terminal with `gemini`, then:

1. **Check MCP server status**:

    ```
    /mcp list
    ```

    ```
    ℹConfigured MCP servers:

    🟢 github - Ready (96 tools, 2 prompts)
        Tools:
        - github__add_comment_to_pending_review
        - github__add_issue_comment
        - github__add_sub_issue
        ...
    ```

2. **Test with a prompt**
    ```
    List my GitHub repositories
    ```

## Additional Configuration

You can find more MCP configuration options for Gemini CLI here: [MCP Configuration Structure](https://google-gemini.github.io/gemini-cli/docs/tools/mcp-server.html#configuration-structure). For example, bypassing tool confirmations or excluding specific tools.

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
    cat ~/.gemini/settings.json | jq .
    ```
- **MCP connection issues**: Check logs for connection errors:
    ```bash
    gemini --debug "test command"
    ```

## References

- Gemini CLI Docs > [MCP Configuration Structure](https://google-gemini.github.io/gemini-cli/docs/tools/mcp-server.html#configuration-structure)
