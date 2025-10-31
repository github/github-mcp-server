# Install GitHub MCP Server in OpenAI Codex

## Prerequisites

1. OpenAI Codex (MCP-enabled) installed / available
2. A [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new)
3. (Optional for local hosting) [Docker](https://www.docker.com/) installed and running

<details>
<summary><b>Storing Your PAT Securely</b></summary>
<br>

For security, avoid hardcoding your token. One common approach:

1. Store your token in `.env` file
```
GITHUB_PAT=ghp_your_token_here
```

2. Add to .gitignore
```bash
echo -e ".env\n.codex/config.toml" >> .gitignore
```

</details>

> The remote GitHub MCP server is hosted by GitHub at `https://api.githubcopilot.com/mcp/` and supports Streamable HTTP.

## Quick Remote Configuration (Recommended)

Edit `~/.codex/config.toml` (shared by CLI and IDE extension) and add:

```toml
[mcp_servers.github]
url = "https://api.githubcopilot.com/mcp/"
# Replace with your real PAT (least-privilege scopes). Do NOT commit this.
bearer_token = "ghp_your_token_here"

# Optional server-level timeouts (adjust if tools are long running)
startup_timeout_sec = 30
tool_timeout_sec = 300
```

PAT scopes: start with `repo`; add `workflow`, `read:org`, `project`, `gist` only when needed.

If you prefer not to store the token directly in the file, you can:
1. Use the collapsible "Storing Your PAT Securely" section above for `.env` file approach.
2. Use the Codex IDE extension's config UI to paste it instead of editing the file manually.
3. Regenerate and rotate frequently. (Codex config does not yet support an environment-variable reference for `bearer_token`.)

## Local Docker Configuration (STDIO)

Use this if you prefer a local, self-hosted instance instead of the remote HTTP server.

> The npm package `@modelcontextprotocol/server-github` is deprecated (April 2025). Use the official Docker image `ghcr.io/github/github-mcp-server`.

Add to `config.toml`:

```toml
[mcp_servers.github]
command = "docker"
args = [
  "run", "-i", "--rm",
  "-e", "GITHUB_PERSONAL_ACCESS_TOKEN",
  "ghcr.io/github/github-mcp-server"
]
startup_timeout_sec = 30
tool_timeout_sec = 300

[mcp_servers.github.env]
GITHUB_PERSONAL_ACCESS_TOKEN = "ghp_your_token_here"
```

Or use the Codex CLI to add it (see "Add via Codex CLI" section below).

### Binary (Alternative Local STDIO)
Build the server (or download a release):

```bash
go build -o github-mcp-server ./cmd/github-mcp-server
```

`config.toml` entry:
```toml
[mcp_servers.github]
command = "./github-mcp-server"
args = ["stdio"]
startup_timeout_sec = 30
tool_timeout_sec = 300

[mcp_servers.github.env]
GITHUB_PERSONAL_ACCESS_TOKEN = "ghp_your_token_here"
```

Or use the Codex CLI to add it (see "Add via Codex CLI" section below).

## Add via Codex CLI (STDIO)

The Codex CLI supports adding STDIO MCP servers with `codex mcp add`. This launches the server command when Codex starts. Remote (streamable HTTP) servers like the hosted GitHub MCP (`url = "https://api.githubcopilot.com/mcp/"`) must currently be added by editing `~/.codex/config.toml` directly (the CLI add flow does not set `url`).

### Binary (stdio) via CLI
```bash
export GITHUB_PERSONAL_ACCESS_TOKEN=ghp_your_token_here
codex mcp add github -- ./github-mcp-server stdio
```

With an inline env flag (alternative):
```bash
codex mcp add github --env GITHUB_PERSONAL_ACCESS_TOKEN=$GITHUB_PERSONAL_ACCESS_TOKEN -- ./github-mcp-server stdio
```

### Docker (stdio) via CLI
```bash
export GITHUB_PERSONAL_ACCESS_TOKEN=ghp_your_token_here
codex mcp add github -- docker run -i --rm -e GITHUB_PERSONAL_ACCESS_TOKEN ghcr.io/github/github-mcp-server
```

Inline env form:
```bash
codex mcp add github --env GITHUB_PERSONAL_ACCESS_TOKEN=$GITHUB_PERSONAL_ACCESS_TOKEN -- docker run -i --rm -e GITHUB_PERSONAL_ACCESS_TOKEN ghcr.io/github/github-mcp-server
```

### Verify (CLI)
After adding:
```bash
codex                # start Codex TUI
/mcp                 # list active servers
```
You should see `github` in the list with tools available. If not:
- Re-run with the PAT exported
- Check that the binary is executable: `chmod +x ./github-mcp-server`
- Confirm Docker can pull the image: `docker pull ghcr.io/github/github-mcp-server`

### Removing / Updating
```bash
codex mcp remove github
codex mcp list
```
Then re-add with updated scopes or a rotated token.

## Configuration Details

- **File path**: `~/.codex/config.toml`
- **Scope**: Global configuration shared by both CLI and IDE extension
- **Format**: Must be valid TOML (use a linter to verify)
- **Editor access**: 
  - CLI: Edit directly or use `codex mcp add` commands
  - IDE: Click gear icon → MCP settings → Open config.toml

## Verification

After starting Codex (CLI or IDE):
1. Run `/mcp` in the TUI or use the IDE MCP panel; confirm `github` shows tools.
2. Ask: "List my GitHub repositories".
3. If tools are missing:
   - Check token validity & scopes.
   - Confirm correct table name: `[mcp_servers.github]`.
   - For stdio: ensure env var `GITHUB_PERSONAL_ACCESS_TOKEN` is set before launching Codex.
4. For long-running operations, consider increasing `tool_timeout_sec`.

## Usage

After setup, Codex can interact with GitHub directly. Try these example prompts:

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

> **Tip**: Use `/mcp` in the Codex TUI to see all available GitHub tools and their descriptions.

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
| 401 Unauthorized (remote) | Token expired/revoked | Create new PAT; update `bearer_token` |
| Server not listed | Wrong table name or syntax error | Use `[mcp_servers.github]`; validate TOML |
| Tools missing / zero tools | Insufficient PAT scopes | Add needed scopes (workflow, gist, etc.) |
| Docker run fails | Image pull or local Docker issue | `docker pull ghcr.io/github/github-mcp-server`; verify Docker running |
| Stdio exits immediately | `GITHUB_PERSONAL_ACCESS_TOKEN` not set | Add env table or export var before launch |
| Timeouts on large operations | Default too low | Increase `tool_timeout_sec` (e.g. 600) |
| Token in file risks leakage | Committed accidentally | Rotate token; add file to `.gitignore` |

### Debug Tips
- Mask token: `printf '%s\n' "$GITHUB_PERSONAL_ACCESS_TOKEN" | head -c 4`
- Validate TOML: `python -c 'import tomllib; tomllib.load(open("$HOME/.codex/config.toml","rb"))'`
- Inspect tools: use `/mcp` then expand server details.
- Manual Docker test:
  ```bash
  docker run -i --rm -e GITHUB_PERSONAL_ACCESS_TOKEN=$GITHUB_PERSONAL_ACCESS_TOKEN ghcr.io/github/github-mcp-server
  ```

## Security Best Practices
1. Never commit tokens into version control
2. Prefer environment variables or secret managers
3. Rotate tokens periodically
4. Restrict scopes up front; expand only when required
5. Remove unused PATs from your GitHub account

## Important Notes

- **npm package deprecation**: The npm package `@modelcontextprotocol/server-github` is deprecated as of April 2025. Use the official Docker image `ghcr.io/github/github-mcp-server` or build from source.
- **Remote server**: GitHub's hosted MCP server at `https://api.githubcopilot.com/mcp/` is the recommended setup for most users (no Docker needed).
- **Token security**: Never commit `config.toml` with embedded tokens to version control. Use `.gitignore` and rotate tokens regularly.
- **CLI help**: Run `codex mcp --help` to see all available MCP management commands.
- **Configuration sharing**: The `~/.codex/config.toml` file is shared between Codex CLI and IDE extension—configure once, use everywhere.
- **Advanced features**: See the [main README](../../README.md) for toolsets, read-only mode, and dynamic tool discovery options.

## References
- Remote server URL: `https://api.githubcopilot.com/mcp/`
- Docker image: `ghcr.io/github/github-mcp-server`
- Release binaries: [GitHub Releases](https://github.com/github/github-mcp-server/releases)
- OpenAI Codex MCP docs: https://developers.openai.com/codex/mcp
- Main project README: [Advanced configuration options](../../README.md)

## Notes on Host Configuration
Codex MCP server tables live under `[mcp_servers.<name>]`. Supported keys relevant here:
- STDIO: `command`, `args`, `env`, `startup_timeout_sec`, `tool_timeout_sec`
- Streamable HTTP: `url`, `bearer_token`, `startup_timeout_sec`, `tool_timeout_sec`

We intentionally omit OAuth configuration because it requires the experimental RMCP client and is not applicable to the GitHub MCP server in this guide.
