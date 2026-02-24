# Install GitHub MCP Server in AdaL CLI

## Prerequisites

1. AdaL CLI installed — `npm install -g @sylphai/adal-cli` (requires Node.js 20+)
2. [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) with appropriate scopes

## Option 1 — Shortcut (Easiest)

AdaL CLI has a pre-configured shortcut for GitHub. Set your token before starting AdaL, then add the server:

```bash
# macOS / Linux
export GITHUB_TOKEN="ghp_xxxx"

# Windows (PowerShell)
$env:GITHUB_TOKEN="ghp_xxxx"
```

Then inside your AdaL session:

```bash
/mcp add github
```

AdaL reads the `GITHUB_TOKEN` environment variable at startup. See [AdaL CLI MCP docs](https://docs.sylph.ai/features/mcp-support-proposed) for more info.

## Option 2 — Remote GitHub MCP Server

Connect directly to GitHub's hosted MCP server at `https://api.githubcopilot.com/mcp/` using a PAT header.

<details>
<summary>AdaL CLI Remote Server Connection</summary>

```bash
/mcp add github --transport http --url https://api.githubcopilot.com/mcp/ --header "Authorization:Bearer YOUR_GITHUB_PAT"
```

Replace `YOUR_GITHUB_PAT` with your actual [GitHub Personal Access Token](https://github.com/settings/tokens).

</details>

## Option 3 — Local GitHub MCP Server (Docker)

Run the server locally using Docker.

### Prerequisites

- [Docker](https://www.docker.com/) installed and running
- [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) with appropriate scopes

<details>
<summary>AdaL CLI Local Server Connection</summary>

```bash
/mcp add github --command docker --args "run,-i,--rm,-e,GITHUB_PERSONAL_ACCESS_TOKEN,ghcr.io/github/github-mcp-server" --env "GITHUB_PERSONAL_ACCESS_TOKEN=YOUR_GITHUB_PAT"
```

Replace `YOUR_GITHUB_PAT` with your actual [GitHub Personal Access Token](https://github.com/settings/tokens).

</details>

## Verify the Connection

After adding the server, test it inside AdaL:

```
/mcp
```

Select the `github` server → **Test Connection**. You should see the available GitHub tools listed.

## Troubleshooting

- **"Connection failed"** — Verify your PAT is valid and has the required scopes (`repo`, etc.)
- **Token not read (shortcut)** — Make sure `GITHUB_TOKEN` is set *before* starting AdaL; restart required if set after launch
- **Docker errors** — Ensure Docker is running: `docker ps`
- **Token not found (local)** — Confirm your PAT is correctly set in the `--env` flag

For more details, see the main [README.md](../../README.md).
