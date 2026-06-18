# Install GitHub MCP Server in OpenAI Codex

## Prerequisites

1. OpenAI Codex (MCP-enabled) installed / available
2. A [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new)

> The remote GitHub MCP server is hosted by GitHub at `https://api.githubcopilot.com/mcp/` and supports Streamable HTTP.

## Remote Configuration

Edit `~/.codex/config.toml` (shared by CLI and IDE extension) and add:

```toml
[mcp_servers.github]
url = "https://api.githubcopilot.com/mcp/"
# Replace with your real PAT (least-privilege scopes). Do NOT commit this.
bearer_token_env_var = "GITHUB_PAT_TOKEN"
```

You can also add it via the Codex CLI:

```cli
codex mcp add github --url https://api.githubcopilot.com/mcp/  
```

<details>
<summary><b>Storing Your PAT Securely</b></summary>
<br>

For security, avoid hardcoding your token. One common approach:

1. Store your token in `.env` file
```
GITHUB_PAT_TOKEN=ghp_your_token_here
```

2. Add to .gitignore
```bash
echo -e ".env" >> .gitignore
```
</details>

## Local Docker Configuration

Use this if you prefer a local, self-hosted instance instead of the remote HTTP server, please refer to the [OpenAI documentation for configuration](https://developers.openai.com/codex/mcp).

## Verification

After starting Codex (CLI or IDE):
1. Run `/mcp` in the TUI or use the IDE MCP panel; confirm `github` shows tools.
2. Ask: "List my GitHub repositories".
3. If tools are missing:
   - Check token validity & scopes.
   - Confirm correct table name: `[mcp_servers.github]`.

## Usage

After setup, Codex can interact with GitHub directly. It will use the default tool set automatically but can be [configured](../../README.md#default-toolset). Try these example prompts:

**Repository Operations:**
- "List my GitHub repositories"
- "Show me recent issues in [owner/repo]"
- "Create a new issue in [owner/repo] titled 'Bug: fix login'"

**Pull Requests:**
- "List open pull requests in [owner/repo]"
- "Show me the diff for PR #123"
- "Add a comment to PR #123: 'LGTM, approved'"

**Actions & Workflows:**
- "Show me recent workflow runs in [owner/repo]"
- "Trigger the 'deploy' workflow in [owner/repo]"

**Gists:**
- "Create a gist with this code snippet"
- "List my gists"

> **Tip**: Use `/mcp` in the Codex UI to see all available GitHub tools and their descriptions.

## Choosing Scopes for Your PAT

Minimal useful scopes (adjust as needed):
- `repo` (general repository operations)
- `workflow` (if you want Actions workflow access)
- `read:org` (if accessing org-level resources)
- `project` (for classic project boards)
- `gist` (if using gist tools)

Use the principle of least privilege: add scopes only when a tool request fails due to permission.

## Troubleshooting

| Issue | Possible Cause | Fix |
|-------|----------------|-----|
| Authentication failed | Missing/incorrect PAT scope | Regenerate PAT; ensure `repo` scope present |
| 401 Unauthorized (remote) | Token expired/revoked | Create new PAT; update `bearer_token_env_var` |
| Server not listed | Wrong table name or syntax error | Use `[mcp_servers.github]`; validate TOML |
| Tools missing / zero tools | Insufficient PAT scopes | Add needed scopes (workflow, gist, etc.) |
| Token in file risks leakage | Committed accidentally | Rotate token; add file to `.gitignore` |

## Security Best Practices
1. Never commit tokens into version control
3. Rotate tokens periodically
4. Restrict scopes up front; expand only when required
5. Remove unused PATs from your GitHub account

## References
- Remote server URL: `https://api.githubcopilot.com/mcp/`
- Release binaries: [GitHub Releases](https://github.com/github/github-mcp-server/releases)
- OpenAI Codex MCP docs: https://developers.openai.com/codex/mcp
- Main project README: [Advanced configuration options](../../README.md)
