# OAuth Authentication (stdio mode)

The GitHub MCP Server supports OAuth 2.1 authentication for stdio mode, allowing users to authenticate via their browser instead of manually creating Personal Access Tokens.

## How It Works

When no `GITHUB_PERSONAL_ACCESS_TOKEN` is configured and OAuth credentials are available, the server starts without a token. On the first tool call, it triggers the OAuth flow:

1. **PKCE flow** (primary): A local callback server starts, your browser opens to GitHub's authorization page, and the token is received via callback. If the browser cannot open (e.g., Docker), the authorization URL is shown via [MCP URL elicitation](https://modelcontextprotocol.io/specification/2025-11-25/client/elicitation).

2. **Device flow** (fallback): If the callback server cannot start (e.g., Docker without port binding), the server falls back to GitHub's [device flow](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps#device-flow). A code is displayed that you enter at [github.com/login/device](https://github.com/login/device).

### Authentication Priority

| Priority | Source | Notes |
|----------|--------|-------|
| 1 (highest) | `GITHUB_PERSONAL_ACCESS_TOKEN` | PAT is used directly, OAuth is skipped |
| 2 | `GITHUB_OAUTH_CLIENT_ID` (env/flag) | Explicit OAuth credentials |
| 3 | Built-in credentials | Baked into official releases via build flags |

## Docker Setup (Recommended)

Docker is the standard distribution method. The recommended setup uses PKCE with a bound port:

```json
{
  "mcpServers": {
    "github": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "GITHUB_OAUTH_CLIENT_ID",
        "-e", "GITHUB_OAUTH_CLIENT_SECRET",
        "-e", "GITHUB_OAUTH_CALLBACK_PORT=8085",
        "-p", "127.0.0.1:8085:8085",
        "ghcr.io/github/github-mcp-server"
      ],
      "env": {
        "GITHUB_OAUTH_CLIENT_ID": "your_client_id",
        "GITHUB_OAUTH_CLIENT_SECRET": "your_client_secret"
      }
    }
  }
}
```

> **Security**: Always bind to `127.0.0.1` (not `0.0.0.0`) to restrict the callback to localhost.

### Docker Without Port Binding (Device Flow)

If you cannot bind a port, the server falls back to device flow:

```json
{
  "mcpServers": {
    "github": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "GITHUB_OAUTH_CLIENT_ID",
        "-e", "GITHUB_OAUTH_CLIENT_SECRET",
        "ghcr.io/github/github-mcp-server"
      ],
      "env": {
        "GITHUB_OAUTH_CLIENT_ID": "your_client_id",
        "GITHUB_OAUTH_CLIENT_SECRET": "your_client_secret"
      }
    }
  }
}
```

## Native Binary Setup

For native binaries, PKCE works automatically with a random port:

```bash
export GITHUB_OAUTH_CLIENT_ID="your_client_id"
export GITHUB_OAUTH_CLIENT_SECRET="your_client_secret"
./github-mcp-server stdio
```

The browser opens automatically. No port configuration needed.

## Creating a GitHub OAuth App

1. Go to **GitHub Settings** → **Developer settings** → **OAuth Apps**
2. Click **New OAuth App**
3. Fill in:
   - **Application name**: e.g., "GitHub MCP Server"
   - **Homepage URL**: `https://github.com/github/github-mcp-server`
   - **Authorization callback URL**: `http://localhost:8085/callback` (match your `--oauth-callback-port`)
4. Click **Register application**
5. Copy the **Client ID** and generate a **Client Secret**

> **Note**: The callback URL must be registered even for device flow, though it won't be used.

## Configuration Reference

| Environment Variable | Flag | Description |
|---------------------|------|-------------|
| `GITHUB_OAUTH_CLIENT_ID` | `--oauth-client-id` | OAuth client ID |
| `GITHUB_OAUTH_CLIENT_SECRET` | `--oauth-client-secret` | OAuth client secret |
| `GITHUB_OAUTH_CALLBACK_PORT` | `--oauth-callback-port` | Fixed callback port (0 = random) |
| `GITHUB_OAUTH_SCOPES` | `--oauth-scopes` | Override automatic scope selection |

## Security Design

### PKCE (Proof Key for Code Exchange)
All authorization code flows use PKCE with S256 challenge, preventing authorization code interception even if an attacker can observe the callback.

### Fixed Port Considerations
Docker requires a fixed callback port for port mapping. This is acceptable because:
- **PKCE verifier** is generated per-flow and never leaves the process — an attacker who intercepts the callback cannot exchange the code
- **State parameter** prevents CSRF — the callback validates state match
- **Callback server binds to 127.0.0.1** — not accessible from outside the host
- **Short-lived** — the server shuts down immediately after receiving the callback

### Token Handling
- Tokens are stored **in memory only** — never written to disk
- OAuth token takes precedence over PAT if both become available
- The server requests only the scopes needed by the configured tools

### URL Elicitation Security
When the browser cannot auto-open, the authorization URL is shown via MCP URL-mode elicitation. This is secure because:
- URL elicitation presents the URL to the user without exposing it to the LLM context
- The MCP client shows the full URL for user inspection before navigation
- Credentials flow directly between the user's browser and GitHub — never through the MCP channel

### Device Flow as Fallback
Device flow is more susceptible to social engineering than PKCE (the device code could theoretically be phished), which is why PKCE is always attempted first. Device flow is only used when a callback server cannot be started.
