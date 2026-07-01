# Local Server GitHub App Authentication

The local GitHub MCP Server can authenticate directly as a GitHub App
installation instead of a Personal Access Token (PAT) or browser-based OAuth
login. This works for both the `stdio` and `http` commands. The server signs a
short-lived JWT with your app's private key, exchanges it for an installation
access token, and refreshes that installation token automatically before it
expires.

> This guide covers installation authentication using `GITHUB_APP_ID`,
> `GITHUB_APP_INSTALLATION_ID`, and a private key. For browser-based user login
> via OAuth, see [Local Server OAuth Login](oauth-login.md). The remote server
> has its own auth model; see [Remote Server](remote-server.md).

## Contents

- [How it works](#how-it-works)
- [GitHub App setup](#github-app-setup)
- [Quick start](#quick-start)
- [Configuration reference](#configuration-reference)
- [Running in Docker](#running-in-docker)
- [Using the `http` command](#using-the-http-command)
- [GitHub Enterprise Server and ghe.com](#github-enterprise-server-and-ghecom)
- [Troubleshooting](#troubleshooting)

## How it works

When all required app environment variables are present, the local server
authenticates as the configured GitHub App installation instead of as a user:

1. It signs a JWT with the app's private key.
2. It exchanges that JWT for an installation access token.
3. It reuses that installation token for GitHub API calls and refreshes it
   automatically before it expires.

Because this is installation authentication, access is controlled by the app's
granted permissions and repository selection — not by PAT scopes or the OAuth
scopes described in [Local Server OAuth Login](oauth-login.md). No browser
callback, OAuth app, or device-code flow is involved.

## GitHub App setup

Before starting the server, create or choose a GitHub App and install it on the
account or organization that owns the repositories you want to access.

You will need:

- The **App ID**
- The **Installation ID**
- A **private key** for that app

Make sure the installation has the repository access and permissions your
workflows need. For example, repository-writing tools need write permissions on
the relevant resources; read-only setups can grant narrower access.

> GitHub App installation auth does **not** require an OAuth callback URL. If
> you only want installation auth, you do not need to enable device flow or
> browser login in the app settings.

## Quick start

**Native binary (`stdio`)** using a private key file:

```bash
export GITHUB_APP_ID=12345
export GITHUB_APP_INSTALLATION_ID=67890
export GITHUB_APP_PRIVATE_KEY_PATH=/path/to/private-key.pem

github-mcp-server stdio
```

Using an inline private key (for hosts that only accept environment variables):

```bash
export GITHUB_APP_ID=12345
export GITHUB_APP_INSTALLATION_ID=67890
export GITHUB_APP_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----\nMIIE...\n-----END RSA PRIVATE KEY-----"

github-mcp-server stdio
```

Once these variables are set, `GITHUB_PERSONAL_ACCESS_TOKEN` is not required.

## Configuration reference

GitHub App auth is configured with environment variables. The same variables
apply to both `stdio` and `http`.

| Environment variable | Description |
|----------------------|-------------|
| `GITHUB_APP_ID` | GitHub App ID |
| `GITHUB_APP_INSTALLATION_ID` | Installation ID of the app installation the server should act as |
| `GITHUB_APP_PRIVATE_KEY` | PEM-encoded private key inline. If your environment variable system cannot preserve real newlines, use literal `\n` escapes. |
| `GITHUB_APP_PRIVATE_KEY_PATH` | Path to a PEM private key file |
| `GITHUB_HOST` | Optional GitHub Enterprise Server / `ghe.com` host. Omit for github.com. |

Rules:

- Set **exactly one** of `GITHUB_APP_PRIVATE_KEY` or
  `GITHUB_APP_PRIVATE_KEY_PATH`.
- `GITHUB_APP_ID`, `GITHUB_APP_INSTALLATION_ID`, and a private key are all
  required together.
- If none of these variables are set, the server falls back to PAT or OAuth
  auth depending on how you start it.

## Running in Docker

GitHub App auth does **not** use the browser callback flow, so Docker does
**not** need the OAuth callback port mapping required by
[Local Server OAuth Login](oauth-login.md). You only need to provide the app
credentials and, if you use a key file, mount it into the container.

```bash
docker run -i --rm \
  -e GITHUB_APP_ID=12345 \
  -e GITHUB_APP_INSTALLATION_ID=67890 \
  -e GITHUB_APP_PRIVATE_KEY_PATH=/key/private-key.pem \
  -v /path/to/private-key.pem:/key/private-key.pem:ro \
  ghcr.io/github/github-mcp-server
```

VS Code (`.vscode/mcp.json`) with an inline private key:

```json
{
  "servers": {
    "github": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "GITHUB_APP_ID",
        "-e", "GITHUB_APP_INSTALLATION_ID",
        "-e", "GITHUB_APP_PRIVATE_KEY",
        "ghcr.io/github/github-mcp-server"
      ],
      "env": {
        "GITHUB_APP_ID": "12345",
        "GITHUB_APP_INSTALLATION_ID": "67890",
        "GITHUB_APP_PRIVATE_KEY": "-----BEGIN RSA PRIVATE KEY-----\nMIIE...\n-----END RSA PRIVATE KEY-----"
      }
    }
  }
}
```

## Using the `http` command

The same environment variables work for the local HTTP server:

```bash
export GITHUB_APP_ID=12345
export GITHUB_APP_INSTALLATION_ID=67890
export GITHUB_APP_PRIVATE_KEY_PATH=/path/to/private-key.pem

github-mcp-server http --port 8082
```

This is useful when you want a long-running local MCP HTTP endpoint but still
want installation-scoped, automatically refreshed credentials.

## GitHub Enterprise Server and ghe.com

GitHub App auth works against GitHub Enterprise Server and `ghe.com` too, as
long as:

- the app is registered on that same host,
- the installation exists on that host, and
- you set `GITHUB_HOST` / `--gh-host` to that host.

Example:

```bash
export GITHUB_HOST=https://github.example.com
export GITHUB_APP_ID=12345
export GITHUB_APP_INSTALLATION_ID=67890
export GITHUB_APP_PRIVATE_KEY_PATH=/path/to/private-key.pem

github-mcp-server stdio
```

The server derives the REST API base URL from `GITHUB_HOST`, so installation
tokens are requested from the correct instance.

## Troubleshooting

- **"incomplete GitHub App auth config"** — set `GITHUB_APP_ID`,
  `GITHUB_APP_INSTALLATION_ID`, and one private key source together.
- **"GITHUB_APP_PRIVATE_KEY and GITHUB_APP_PRIVATE_KEY_PATH are mutually
  exclusive"** — keep only one.
- **403 or 404 from GitHub** — the app is often not installed on the target
  repo/org, or it lacks the needed permissions.
- **Private key parsing or JWT errors** — make sure the private key matches the
  configured app and is valid PEM.
- **Unexpected fallback to PAT/OAuth** — confirm the app variables are present
  in the actual server process, not just in your shell.
