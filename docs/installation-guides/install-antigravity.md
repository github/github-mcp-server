# Installing GitHub MCP Server in Antigravity

This guide covers setting up the GitHub MCP Server in Google's Antigravity IDE.

## Prerequisites

- Antigravity IDE installed (latest version)
- GitHub Personal Access Token with appropriate scopes

## Installation Methods

### Option 1: Remote Server (Recommended)

Uses GitHub's hosted server at `https://api.githubcopilot.com/mcp/`.

> [!NOTE]
> We recommend this manual configuration method because the "official" installation via the Antigravity MCP Store currently has known issues (often resulting in Docker errors). This direct remote connection is more reliable.

#### Step 1: Access MCP Configuration

1. Open Antigravity
2. Click the "..." (Additional Options) menu in the Agent panel
3. Select "MCP Servers"
4. Click "Manage MCP Servers"
5. Click "View raw config"

This will open your `mcp_config.json` file at:
- **Windows**: `C:\Users\<USERNAME>\.gemini\antigravity\mcp_config.json`
- **macOS/Linux**: `~/.gemini\antigravity\mcp_config.json`

#### Step 2: Add Configuration

Add the following to your `mcp_config.json`:

```json
{
  "mcpServers": {
    "github": {
      "serverUrl": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer YOUR_GITHUB_PAT"
      }
    }
  }
}
```

**Important**: Note that Antigravity uses `serverUrl` instead of `url` for HTTP-based MCP servers.

#### Step 3: Configure Your Token

Replace `YOUR_GITHUB_PAT` with your actual GitHub Personal Access Token.

Create a token here: https://github.com/settings/tokens

Recommended scopes:
- `repo` - Full control of private repositories
- `read:org` - Read org and team membership
- `read:user` - Read user profile data

#### Step 4: Restart Antigravity

Close and reopen Antigravity for the changes to take effect.

#### Step 5: Verify Installation

1. Open the MCP Servers panel (... menu → MCP Servers)
2. You should see "github" with a list of available tools
3. You can now use GitHub tools in your conversations

> [!NOTE]
> The status indicator in the MCP Servers panel might not immediately turn green in some versions, but the tools will still function if configured correctly.

### Power User Configuration: Unlocking a Rich GitHub Experience (Grok-like Integration)

The basic remote configuration above uses GitHub's default toolsets (`context`, `repos`, `issues`, `pull_requests`, `users`). This provides a solid starting point but a relatively narrow surface.

For a dramatically more capable experience — comparable to the powerful GitHub integration that powers xAI's Grok (via its `grok_com_github` MCP server with approximately 42 tools) — use the remote server's rich customization options.

**Why go beyond the defaults?**
- Enable Copilot code reviews (`request_copilot_review`)
- Full pull request lifecycle, advanced commenting, and review workflows
- Powerful code search (`search_code`), cross-repository operations, and automation
- Issues, discussions, projects, Actions, and more for end-to-end agentic workflows
- Antigravity users gain the same "GitHub superpowers" that make Grok exceptionally effective at repository management, code review, and developer tasks

Antigravity recommends keeping total enabled tools under 50 for best performance and relevance. Grok achieves excellent results with a rich but curated ~42-tool surface.

#### Ready-to-Use Configuration Examples

**A. Basic (current minimal setup — default toolsets only)**

```json
{
  "mcpServers": {
    "github": {
      "serverUrl": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer YOUR_GITHUB_PAT"
      }
    }
  }
}
```

**B. Recommended for most Antigravity power users (rich, curated, Grok-like)**

This enables a broad but practical set of high-value toolsets while staying well under the recommended tool limit:

```json
{
  "mcpServers": {
    "github": {
      "serverUrl": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer YOUR_GITHUB_PAT",
        "X-MCP-Toolsets": "context,repos,issues,pull_requests,copilot,actions,discussions,projects,users,labels,orgs,gists,notifications"
      }
    }
  }
}
```

**C. Maximum power (use with caution)**

```json
{
  "mcpServers": {
    "github": {
      "serverUrl": "https://api.githubcopilot.com/mcp/x/all",
      "headers": {
        "Authorization": "Bearer YOUR_GITHUB_PAT"
      }
    }
  }
}
```

> [!WARNING]
> Configuration C enables every available tool. While powerful, it may exceed Antigravity's recommended <50 tool guideline and can impact performance or relevance. Start with B (Recommended) for the best balance. You can always add `X-MCP-Readonly: "true"` for a safe read-only variant of any config.

#### Recommended PAT Scopes for Rich Setups

For the Recommended (B) or Maximum (C) configurations, create a dedicated PAT with these scopes (https://github.com/settings/tokens/new):

| Scope                  | Purpose                                      | Required for                  |
|------------------------|----------------------------------------------|-------------------------------|
| `repo`                 | Full repository read/write access            | Most tools (core)             |
| `read:org`             | Organization and team membership             | orgs, teams, some context     |
| `security_events`      | Code scanning, Dependabot, secret scanning   | code_security, dependabot     |
| `gist`                 | Gist creation and management                 | gists toolset                 |
| `project`              | GitHub Projects (classic + new)              | projects toolset              |
| `notifications`        | Notifications access                         | notifications toolset         |
| `read:user` / `user`   | User profile data                            | context, users                |

Start with the minimum you need and expand. Use a dedicated token (never your primary development PAT) and rotate it periodically.

#### Windows & PowerShell Tips

- **Exact path on this machine**: `C:\Users\pico\.gemini\antigravity\mcp_config.json` (verified on a real Antigravity installation).
- **Safe merging**: If the file already contains other servers (common — e.g. `notebooks` and `visualization` entries from Google Cloud tools), simply add the `"github"` key **inside** the existing `"mcpServers"` object. Do not overwrite the whole file.
- **Editor recommendation**: Use VS Code (or any editor with JSON validation). It provides syntax highlighting, auto-formatting (right-click → Format Document), and real-time error detection.
- **Validation command** (PowerShell):
  ```powershell
  Get-Content "$env:USERPROFILE\.gemini\antigravity\mcp_config.json" | ConvertFrom-Json | Out-Null
  Write-Host "JSON is valid"
  ```
- After editing, save, then fully restart Antigravity (or use the MCP Servers panel refresh if available).

#### Verification — Confirming Your Rich Tool Surface

1. Restart Antigravity completely after editing.
2. Open the MCP Servers panel.
3. Expand the "github" server entry.
4. You should now see a significantly longer list of tools (basic config ≈ default 5 toolsets; Recommended B typically 25–35+ tools; Maximum C approaches the full ~42-tool surface used by agents like Grok).

**Immediate test prompts** (type in the Agent chat):
- "List open pull requests assigned to me"
- "Search the organization for files containing 'TODO' or 'FIXME'"
- "Create a draft pull request from my current branch with a clear description"
- "Request a Copilot review on pull request #123 in owner/repo"
- "Find and summarize the most recent issues mentioning 'antigravity' or 'MCP'"
- "Show me my recent notifications and any open security alerts"

Successful execution of Copilot review requests, advanced searches, and cross-feature operations confirms you have achieved Grok-parity richness.

#### Security & Governance Considerations

> [!WARNING]
> Giving an AI coding IDE broad write access (issues, PRs, code, Actions, etc.) is extremely powerful but carries risk. Treat the GitHub MCP integration with the same care as any high-privilege automation account.

**Recommendations**:
- Use a **dedicated PAT** created specifically for Antigravity (easy to revoke/rotate).
- Begin with the **Recommended (B)** config + `X-MCP-Readonly: "true"` while you evaluate behavior.
- Consider `X-MCP-Lockdown: "true"` when working with public repositories.
- Regularly audit token scopes and usage in your GitHub settings.
- Never share or commit the config file containing your PAT.
- Review the [policies and governance documentation](https://github.com/github/github-mcp-server/blob/main/docs/policies-and-governance.md) in the GitHub MCP Server repo.

For the most secure starting point, combine the basic remote URL with `X-MCP-Readonly` and a minimal toolset list.

#### Further Reading (Essential)

- [Remote Server Documentation](../remote-server.md) — Full headers reference (`X-MCP-Toolsets`, `X-MCP-Readonly`, `X-MCP-Insiders`, etc.), URL path patterns (`/x/all`, `/x/copilot`, `/readonly`), and the complete toolsets table.
- [Server Configuration Guide](../server-configuration.md) — Composable recipes, precedence rules, exclusions, and troubleshooting for headers vs. path modifiers.
- [Main README](../../README.md#available-toolsets) — Definitive list of all toolsets and individual tools.

### Option 2: Local Docker Server

If you prefer running the server locally with Docker:

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

**Requirements**:
- Docker Desktop installed and running
- Docker must be in your system PATH

## Troubleshooting

### "Error: serverUrl or command must be specified"

Make sure you're using `serverUrl` (not `url`) for the remote server configuration. Antigravity requires `serverUrl` for HTTP-based MCP servers.

### Server not appearing in MCP list

- Verify JSON syntax in your config file
- Check that your PAT hasn't expired
- Restart Antigravity completely

### Tools not working

- Ensure your PAT has the correct scopes
- Check the MCP Servers panel for error messages
- Verify internet connection for remote server

### Advanced Configuration Gotchas

- Header names are case-sensitive in practice — use exact casing: `X-MCP-Toolsets`, `X-MCP-Readonly`, `X-MCP-Insiders`.
- To combine multiple toolsets, **use the `X-MCP-Toolsets` header** (not URL path). URL paths like `/x/repos` are for single toolsets only.
- Antigravity requires a full restart (or MCP panel refresh) after any change to headers or `serverUrl`.
- Invalid tool names in `X-MCP-Tools` will prevent the server from starting (toolsets silently ignore unknowns).
- When merging with an existing `mcp_config.json` (e.g. one already containing Google Cloud notebooks/visualization entries), preserve the outer structure and only add the `"github"` entry.

## Available Tools

Once installed, you'll have access to tools like:
- `create_repository` - Create new GitHub repositories
- `push_files` - Push files to repositories
- `search_repositories` - Search for repositories
- `create_or_update_file` - Manage file content
- `get_file_contents` - Read file content
- And many more...

**With the basic configuration** you receive the default toolsets.  
**With the Recommended rich configuration (above)** Antigravity users unlock a tool surface on par with high-capability agents such as xAI Grok's `grok_com_github` integration (approximately 42 tools). This includes `request_copilot_review`, `search_code`, full PR review/comment tools, `fork_repository`, advanced issue/PR search, and the complete set of repository, issues, and collaboration capabilities.

For a complete list of available tools and features, see the [main README](../../README.md).

## Differences from Other IDEs

- **Configuration key**: Antigravity uses `serverUrl` instead of `url` for HTTP servers (confirmed across official Antigravity MCP documentation).
- **Config location**: `.gemini/antigravity/mcp_config.json` instead of `.cursor/mcp.json`
- **Tool limits**: Antigravity recommends keeping total enabled tools under 50 for optimal performance. This makes curated rich configurations (example B above) ideal — delivering Grok-like power without overwhelming the agent or hitting limits.
- **Access method**: "View raw config" via the Agent panel ... menu (very convenient for quick edits).

## Next Steps

- Explore the powerful options in the new **Power User Configuration** section above for a Grok-parity experience.
- Explore the [Server Configuration Guide](../server-configuration.md) for advanced options.
- Check out [toolsets documentation](../../README.md#available-toolsets) to customize available tools.
- See the [Remote Server Documentation](../remote-server.md) for more details on headers and URL patterns.
- Consider starting with a read-only rich config while you explore capabilities.

---

*This enhanced guide builds directly on the excellent foundational work in the original Antigravity installation instructions. It adds the advanced remote customization details that let Antigravity users achieve the same rich, full-featured GitHub MCP integration experience available to other high-capability agents.*
