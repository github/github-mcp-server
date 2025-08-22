# Install GitHub MCP Server in Rovo Dev CLI

## Prerequisites

1. Rovo Dev CLI installed (latest version)
2. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) with appropriate scopes

## MCP Server Setup

Uses GitHub's hosted server at https://api.githubcopilot.com/mcp/.

### Install steps

1. Run `acli rovodev mcp` to open the MCP configuration for Rovo Dev CLI
2. Add configuration by following example below.
3. Replace `YOUR_GITHUB_PAT` with your actual [GitHub Personal Access Token](https://github.com/settings/tokens)
4. Save the file and restart Rovo Dev CLI with `acli rovodev`

### Example configuration

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
