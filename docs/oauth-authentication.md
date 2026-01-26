# OAuth Authentication

The GitHub MCP Server supports OAuth authentication for stdio mode, enabling interactive authentication when no Personal Access Token (PAT) is configured.

## Overview

OAuth authentication allows users to authenticate with GitHub through their browser without pre-configuring a token. This is useful for:

- **End users** who want authentication to "just work" without configuration
- **Interactive sessions** where users want to authenticate on-demand
- **Docker deployments** where tokens shouldn't be baked into images
- **Multi-user scenarios** where each user authenticates individually

## How It Works

Official releases of the GitHub MCP Server include built-in OAuth credentials, providing a seamless authentication experience. When you run the server without a PAT configured, it will automatically prompt for OAuth authentication when a tool requires it.

### Authentication Priority

The server uses the following priority for authentication:

1. **Personal Access Token** (GITHUB_PERSONAL_ACCESS_TOKEN) - Highest priority, explicit user choice
2. **Explicit OAuth configuration** (--oauth-client-id flag/env) - Developer/power user override
3. **Built-in OAuth credentials** - Default for official releases, "just works"
4. **No authentication** - Warning displayed, tools will fail when called

## Quick Start

For most users, simply run the server without any configuration:

```bash
# Official releases include built-in OAuth - just run and authenticate when prompted
./github-mcp-server stdio
```

The server will prompt for browser-based authentication when you first call a tool that requires GitHub access.

## Developer Configuration

Developers building from source or wanting to use their own OAuth app can provide credentials explicitly.

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `GITHUB_OAUTH_CLIENT_ID` | OAuth app client ID | For custom OAuth apps |
| `GITHUB_OAUTH_CLIENT_SECRET` | OAuth app client secret | Recommended |

### Command Line Flags

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--oauth-client-id` | `GITHUB_OAUTH_CLIENT_ID` | Override OAuth app client ID |
| `--oauth-client-secret` | `GITHUB_OAUTH_CLIENT_SECRET` | Override OAuth app client secret |
| `--oauth-callback-port` | `GITHUB_OAUTH_CALLBACK_PORT` | Fixed port for OAuth callback (required for Docker with `-p` flag) |
| `--oauth-scopes` | `GITHUB_OAUTH_SCOPES` | Custom OAuth scopes (comma-separated) |

### Building with Custom OAuth Credentials

When building from source, you can bake in your own OAuth credentials:

```bash
# Build with custom OAuth credentials
go build -ldflags="-X github.com/github/github-mcp-server/internal/buildinfo.OAuthClientID=your-client-id \
  -X github.com/github/github-mcp-server/internal/buildinfo.OAuthClientSecret=your-client-secret" \
  ./cmd/github-mcp-server

# Or use environment variables during development
export GITHUB_OAUTH_CLIENT_ID="your-client-id"
export GITHUB_OAUTH_CLIENT_SECRET="your-client-secret"
./github-mcp-server stdio
```

For Docker builds:

```bash
docker build \
  --build-arg OAUTH_CLIENT_ID="your-client-id" \
  --build-arg OAUTH_CLIENT_SECRET="your-client-secret" \
  -t github-mcp-server .
```

## Authentication Flows

The server automatically selects the appropriate OAuth flow based on the environment:

### 1. PKCE Flow (Browser-based)

Used for local binary execution where a browser can be opened:

1. Server starts a local callback server
2. Browser opens to GitHub authorization page
3. User authorizes the application
4. GitHub redirects to local callback with authorization code
5. Server exchanges code for access token

### 2. Device Flow (Docker/Headless)

Used when running in Docker or when a browser cannot be opened:

1. Server requests a device code from GitHub
2. User is shown a URL and code to enter
3. User visits `github.com/login/device` and enters the code
4. Server polls GitHub until authorization is complete
5. Access token is retrieved

## Usage Examples

### Local Binary (Official Release)

```bash
# Official releases have built-in OAuth - just run!
./github-mcp-server stdio
# Authentication will be prompted when a tool is called
```

### Local Binary (Custom OAuth)

```bash
# Override with your own OAuth app
export GITHUB_OAUTH_CLIENT_ID="your-client-id"
export GITHUB_OAUTH_CLIENT_SECRET="your-client-secret"
./github-mcp-server stdio
```

### Docker (Official Image)

```bash
# Official images have built-in OAuth - just run!
docker run -i --rm ghcr.io/github/github-mcp-server stdio
```

### Docker (with Custom OAuth)

```bash
docker run -i --rm \
  -e GITHUB_OAUTH_CLIENT_ID="your-client-id" \
  -e GITHUB_OAUTH_CLIENT_SECRET="your-client-secret" \
  ghcr.io/github/github-mcp-server stdio
```

### Docker (with PKCE Flow via port mapping)

```bash
docker run -i --rm \
  --network=host \
  ghcr.io/github/github-mcp-server stdio --oauth-callback-port=8085
```

### VS Code MCP Configuration

```jsonc
{
  "mcpServers": {
    "github": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "ghcr.io/github/github-mcp-server",
        "stdio"
      ],
      "type": "stdio"
    }
  }
}
```

## Creating an OAuth App

1. Go to **GitHub Settings** → **Developer settings** → **OAuth Apps**
2. Click **New OAuth App**
3. Fill in the details:
   - **Application name**: Your app name (e.g., "GitHub MCP Server")
   - **Homepage URL**: Your homepage or `https://github.com/github/github-mcp-server`
   - **Authorization callback URL**: `http://localhost:8085/callback` (or your chosen port)
4. Click **Register application**
5. Copy the **Client ID**
6. Generate and copy the **Client Secret**

## Scope Computation

The server automatically computes the required OAuth scopes based on enabled tools:

- If `--toolsets` or `--tools` are specified, only scopes for those tools are requested
- If no tools are specified, default scopes are used: `repo`, `user`, `gist`, `notifications`, `read:org`, `project`
- Custom scopes can be specified with `--oauth-scopes`

## Security Considerations

1. **Client Secret**: While optional for public OAuth apps, using a client secret is recommended for better security
2. **Token Storage**: OAuth tokens are stored in memory only and not persisted to disk
3. **Scope Minimization**: Request only the scopes needed for your use case
4. **PKCE**: The PKCE flow provides protection against authorization code interception attacks

## Troubleshooting

### "redirect_uri not associated with this client"

Ensure the callback port matches your OAuth app's registered callback URL. Use `--oauth-callback-port` to specify the exact port.

### Browser doesn't open automatically

The server will fall back to displaying the authorization URL. In Docker, the device flow is used automatically.

### Token not being used

Verify that `GITHUB_PERSONAL_ACCESS_TOKEN` is not set, as it takes precedence over OAuth.
