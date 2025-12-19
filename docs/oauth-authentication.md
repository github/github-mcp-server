# OAuth Device Flow Authentication

The GitHub MCP Server supports OAuth device flow authentication as an alternative to Personal Access Tokens (PATs). This provides a streamlined authentication experience where users can authenticate directly through the MCP server without pre-configuring tokens.

## Table of Contents

- [Overview](#overview)
- [How It Works](#how-it-works)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
  - [Using Default OAuth App](#using-default-oauth-app)
  - [Using Custom OAuth Apps](#using-custom-oauth-apps)
  - [Using GitHub Apps](#using-github-apps)
  - [GitHub Enterprise Server (GHES)](#github-enterprise-server-ghes)
  - [GitHub Enterprise Cloud (GHEC)](#github-enterprise-cloud-ghec)
- [CLI Flags](#cli-flags)
- [Environment Variables](#environment-variables)
- [Scopes and Permissions](#scopes-and-permissions)
- [Security Considerations](#security-considerations)
- [Troubleshooting](#troubleshooting)
- [Comparison with PAT Authentication](#comparison-with-pat-authentication)

## Overview

OAuth device flow authentication allows users to authenticate with GitHub by:
1. Starting the authentication process through the MCP server
2. Receiving a user code and verification URL
3. Visiting the URL in a browser and entering the code
4. Automatically completing authentication once approved

This method eliminates the need to manually create and configure Personal Access Tokens, making it easier for users to get started with the GitHub MCP Server.

The device flow authentication works with both **OAuth Apps** and **GitHub Apps**. OAuth Apps use scope-based permissions that can be customized via the `--oauth-scopes` flag, while GitHub Apps use fine-grained permissions that are controlled by the app's configuration in GitHub settings.

## How It Works

The OAuth device flow follows these steps:

1. **User requests authentication**: When the server starts without a token, only the `auth_login` tool is available. The user (or their AI agent) calls this tool to initiate authentication.

2. **Server requests device code**: The server requests a device code from GitHub's OAuth device flow endpoint (`https://github.com/login/device/code` or your enterprise equivalent).

3. **User receives verification URL**: The server displays a verification URL and user code to the user via MCP's URL elicitation feature (if supported by the client).

4. **User authorizes in browser**: The user opens the verification URL in their browser, enters the code, and authorizes the application.

5. **Server polls for token**: While the user is authorizing, the server polls GitHub's token endpoint (`https://github.com/login/oauth/access_token`) until the user completes authorization or the request expires.

6. **Authentication completes**: Once the user authorizes the app, the server receives an access token and automatically registers all GitHub tools.

The entire process is handled by a single `auth_login` tool call that blocks until authentication completes or fails.

## Getting Started

### Quick Start (No Configuration Needed)

The simplest way to use OAuth authentication is to start the server without providing a token:

**Docker:**
```json
{
  "github": {
    "command": "docker",
    "args": ["run", "-i", "--rm", "ghcr.io/github/github-mcp-server", "stdio"]
  }
}
```

**Binary:**
```json
{
  "github": {
    "command": "/path/to/github-mcp-server",
    "args": ["stdio"]
  }
}
```

When the server starts without a `GITHUB_PERSONAL_ACCESS_TOKEN`, it will automatically enter authentication mode. Your AI agent can then call the `auth_login` tool to initiate the OAuth flow.

### Backward Compatibility

OAuth authentication is completely optional. If you provide a `GITHUB_PERSONAL_ACCESS_TOKEN`, the server will use it and skip OAuth authentication entirely. This means existing configurations continue to work without modification.

```json
{
  "github": {
    "command": "docker",
    "args": ["run", "-i", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN", "ghcr.io/github/github-mcp-server"],
    "env": {
      "GITHUB_PERSONAL_ACCESS_TOKEN": "ghp_your_token_here"
    }
  }
}
```

## Configuration

### Using Default OAuth App

The GitHub MCP Server includes a default OAuth App registered and managed by GitHub. This OAuth App works with:
- GitHub.com (https://github.com)
- GitHub Enterprise Cloud with data residency (ghe.com)
- GitHub Enterprise Server (GHES) 3.0+

**No additional configuration is required** to use the default OAuth App. Simply omit the PAT token when starting the server.

The default OAuth App client ID is embedded in the source code (which is safe per OAuth 2.0 specifications for public clients) and requires no client secret.

### Using Custom OAuth Apps

For enterprise scenarios, you may want to use your own OAuth App. This is useful when:
- You want to customize the app name and branding
- You need to restrict access to specific organizations
- You want to use a confidential client (with client secret)
- Your organization's policies require using organization-owned OAuth Apps

#### Creating a Custom OAuth App

1. **Navigate to OAuth App settings:**
   - For personal apps: https://github.com/settings/developers
   - For organization apps: https://github.com/organizations/YOUR_ORG/settings/applications

2. **Create a new OAuth App:**
   - Click "New OAuth App"
   - **Application name**: "GitHub MCP Server (Custom)"
   - **Homepage URL**: https://github.com/github/github-mcp-server (or your fork)
   - **Authorization callback URL**: Not used for device flow, but required. Use: `http://localhost:8080/callback`
   - **Enable Device Flow**: Make sure this is checked

3. **Configure the scopes**: The OAuth App will request the scopes defined in the server (see [Scopes and Permissions](#scopes-and-permissions)).

4. **Get your credentials:**
   - **Client ID**: Copy the client ID (e.g., `Ov23liAbcdefg1234567`)
   - **Client Secret** (optional): Generate a client secret if you want to use a confidential client

#### Using Your Custom OAuth App

Provide the client ID (and optionally client secret) using CLI flags or environment variables:

**CLI Flags:**
```json
{
  "github": {
    "command": "/path/to/github-mcp-server",
    "args": [
      "stdio",
      "--oauth-client-id", "Ov23liYourClientID",
      "--oauth-client-secret", "your_client_secret_if_needed"
    ]
  }
}
```

**Environment Variables:**
```json
{
  "github": {
    "command": "docker",
    "args": ["run", "-i", "--rm", "-e", "GITHUB_OAUTH_CLIENT_ID", "-e", "GITHUB_OAUTH_CLIENT_SECRET", "ghcr.io/github/github-mcp-server"],
    "env": {
      "GITHUB_OAUTH_CLIENT_ID": "Ov23liYourClientID",
      "GITHUB_OAUTH_CLIENT_SECRET": "your_client_secret_if_needed"
    }
  }
}
```

### Using GitHub Apps

The GitHub MCP Server also supports authentication via GitHub Apps using the device flow. GitHub Apps provide more granular permissions and better security controls compared to OAuth Apps.

**Key Differences:**

- **Permissions Model**: GitHub Apps use fine-grained permissions instead of OAuth scopes. The `--oauth-scopes` flag does not apply when using GitHub Apps, as permissions are controlled by the GitHub App's configuration in GitHub's settings.
- **App-Controlled Access**: When authenticating via a GitHub App, the available repositories, organizations, and resources are determined by the app's installation and permissions, not by the scopes requested during authentication.
- **Installation-Based**: GitHub Apps must be installed on organizations or repositories before users can authenticate through them.

#### Creating a GitHub App

1. **Navigate to GitHub App settings:**
   - For personal apps: https://github.com/settings/apps
   - For organization apps: https://github.com/organizations/YOUR_ORG/settings/apps

2. **Create a new GitHub App:**
   - Click "New GitHub App"
   - **GitHub App name**: "GitHub MCP Server (Custom)"
   - **Homepage URL**: https://github.com/github/github-mcp-server
   - **Callback URL**: Not used for device flow, but required. Use: `http://localhost:8080/callback`
   - **Request user authorization (OAuth) during installation**: Uncheck this
   - **Enable Device Flow**: Make sure this is checked
   - **Webhook**: Can be set to inactive if not needed

3. **Configure permissions**: Set the repository and organization permissions based on your needs (equivalent to the scopes in OAuth Apps).

4. **Get your credentials:**
   - **Client ID**: Copy the client ID from the app settings
   - **Client Secret**: Generate a client secret if needed

5. **Install the GitHub App**: Install the app on the organizations or repositories where you want to use it.

#### Using Your GitHub App

Use the same `--oauth-client-id` and `--oauth-client-secret` flags with your GitHub App's credentials:

```bash
github-mcp-server stdio --oauth-client-id Iv1.your_github_app_client_id
```

**Note**: When using GitHub Apps, the `--oauth-scopes` flag is ignored. Access and permissions are controlled by the GitHub App's configuration and installation settings.

### GitHub Enterprise Server (GHES)

To use OAuth device flow with GitHub Enterprise Server:

1. **Ensure GHES supports device flow**: Device flow is available in GHES 3.0 and later.

2. **Create an OAuth App** on your GHES instance:
   - Navigate to: `https://YOUR_GHES_HOSTNAME/settings/developers`
   - Follow the steps in [Creating a Custom OAuth App](#creating-a-custom-oauth-app)

3. **Configure the server** with your GHES hostname:

```json
{
  "github": {
    "command": "docker",
    "args": [
      "run", "-i", "--rm",
      "-e", "GITHUB_HOST",
      "-e", "GITHUB_OAUTH_CLIENT_ID",
      "ghcr.io/github/github-mcp-server"
    ],
    "env": {
      "GITHUB_HOST": "https://github.yourcompany.com",
      "GITHUB_OAUTH_CLIENT_ID": "your_ghes_oauth_client_id"
    }
  }
}
```

**Important**: Always prefix GHES hostnames with `https://` to ensure proper URL construction.

#### Example: Full GHES Configuration

```json
{
  "github": {
    "command": "/path/to/github-mcp-server",
    "args": ["stdio", "--gh-host", "https://github.yourcompany.com", "--oauth-client-id", "Ov23liGHESClientID"]
  }
}
```

### GitHub Enterprise Cloud (GHEC)

GitHub Enterprise Cloud with data residency (ghe.com) works with the default OAuth App. However, you may want to create a custom OAuth App for organization-specific branding or policies.

1. **Create an OAuth App** on your GHEC organization:
   - Navigate to: `https://YOUR_SUBDOMAIN.ghe.com/settings/developers` or your organization settings
   - Follow the steps in [Creating a Custom OAuth App](#creating-a-custom-oauth-app)

2. **Configure the server** with your GHEC hostname:

```json
{
  "github": {
    "command": "docker",
    "args": [
      "run", "-i", "--rm",
      "-e", "GITHUB_HOST",
      "-e", "GITHUB_OAUTH_CLIENT_ID",
      "ghcr.io/github/github-mcp-server"
    ],
    "env": {
      "GITHUB_HOST": "https://yourcompany.ghe.com",
      "GITHUB_OAUTH_CLIENT_ID": "your_ghec_oauth_client_id"
    }
  }
}
```

## CLI Flags

The GitHub MCP Server provides the following CLI flags for OAuth configuration:

| Flag | Description | Default | Example |
|------|-------------|---------|---------|
| `--oauth-client-id` | OAuth App client ID for device flow authentication | Default GitHub MCP OAuth App | `--oauth-client-id Ov23liYourClientID` |
| `--oauth-client-secret` | OAuth App client secret (optional, for confidential clients) | None | `--oauth-client-secret your_client_secret` |
| `--oauth-scopes` | Comma-separated list of OAuth scopes to request | Default scopes (see below) | `--oauth-scopes repo,read:org,gist` |
| `--gh-host` | GitHub hostname for API requests and OAuth endpoints | `github.com` | `--gh-host https://github.yourcompany.com` |

### Usage Examples

**Minimal configuration (uses defaults):**
```bash
github-mcp-server stdio
```

**Custom OAuth App:**
```bash
github-mcp-server stdio --oauth-client-id Ov23liYourClientID
```

**Limiting scopes (minimal permissions):**
```bash
github-mcp-server stdio --oauth-scopes repo,read:org,gist
```

**GHES with custom OAuth App:**
```bash
github-mcp-server stdio --gh-host https://github.yourcompany.com --oauth-client-id Ov23liGHESClientID
```

**Confidential client with secret:**
```bash
github-mcp-server stdio --oauth-client-id Ov23liYourClientID --oauth-client-secret your_secret
```

**Custom scopes with custom OAuth App:**
```bash
github-mcp-server stdio --oauth-client-id Ov23liYourClientID --oauth-scopes repo,read:org,user:email
```

## Environment Variables

All CLI flags can also be configured via environment variables with the `GITHUB_` prefix:

| Environment Variable | Equivalent CLI Flag | Example |
|---------------------|---------------------|---------|
| `GITHUB_OAUTH_CLIENT_ID` | `--oauth-client-id` | `export GITHUB_OAUTH_CLIENT_ID=Ov23liYourClientID` |
| `GITHUB_OAUTH_CLIENT_SECRET` | `--oauth-client-secret` | `export GITHUB_OAUTH_CLIENT_SECRET=your_secret` |
| `GITHUB_OAUTH_SCOPES` | `--oauth-scopes` | `export GITHUB_OAUTH_SCOPES=repo,read:org,gist` |
| `GITHUB_HOST` | `--gh-host` | `export GITHUB_HOST=https://github.yourcompany.com` |
| `GITHUB_PERSONAL_ACCESS_TOKEN` | N/A (disables OAuth) | `export GITHUB_PERSONAL_ACCESS_TOKEN=ghp_token` |

**Note**: If `GITHUB_PERSONAL_ACCESS_TOKEN` is set, the server will use PAT authentication and skip OAuth device flow entirely.

### Environment Variable Priority

The server uses the following priority for configuration:
1. CLI flags (highest priority)
2. Environment variables
3. Default values (lowest priority)

## Scopes and Permissions

The OAuth device flow requests the following scopes by default (defined in `DefaultOAuthScopes`):

| Scope | Description | Required For |
|-------|-------------|--------------|
| `repo` | Full control of private repositories | Repository operations, issues, PRs |
| `repo:status` | Access commit status | CI/CD workflow monitoring |
| `repo_deployment` | Access deployment status | Deployment operations |
| `public_repo` | Access public repositories | Public repository operations |
| `gist` | Create and manage gists | Gist operations |
| `notifications` | Access notifications | Notification tools |
| `user` | Read and write user profile data | User information |
| `user:email` | Access user email addresses | User contact information |
| `user:follow` | Follow and unfollow users | Social features |
| `read:org` | Read organization data | Organization and team access |
| `read:gpg_key` | Read GPG keys | Signature verification |
| `project` | Read and write project data | Project board management |

These scopes form a superset of the `gh` CLI minimal scopes (`repo`, `read:org`, `gist`) to support all GitHub MCP tools while following least-privilege principles.

### Customizing Scopes

You can customize the requested scopes using the `--oauth-scopes` CLI flag or `GITHUB_OAUTH_SCOPES` environment variable:

**CLI Flag:**
```bash
github-mcp-server stdio --oauth-scopes repo,read:org,gist
```

**Environment Variable:**
```bash
export GITHUB_OAUTH_SCOPES=repo,read:org,user:email
github-mcp-server stdio
```

**Docker with environment variable:**
```json
{
  "github": {
    "command": "docker",
    "args": ["run", "-i", "--rm", "-e", "GITHUB_OAUTH_SCOPES", "ghcr.io/github/github-mcp-server"],
    "env": {
      "GITHUB_OAUTH_SCOPES": "repo,read:org,gist"
    }
  }
}
```

#### Minimal Recommended Scopes

For basic functionality, you can use a minimal set of scopes:

```bash
--oauth-scopes repo,read:org,gist
```

This provides:
- `repo`: Access to repositories, issues, and PRs
- `read:org`: Read organization and team information
- `gist`: Manage gists

**Note**: Some MCP tools may not function properly with reduced scopes. Review the scopes table above to understand which scopes are required for specific functionality.

#### Common Scope Combinations

**Read-only operations:**
```bash
--oauth-scopes repo,read:org,read:user
```

**Full repository access with notifications:**
```bash
--oauth-scopes repo,read:org,notifications,user:email
```

**Enterprise with minimal scopes:**
```bash
--oauth-scopes repo,read:org,read:gpg_key
```

## Security Considerations

### Token Storage

- **In-memory only**: Access tokens obtained via OAuth device flow are stored only in memory and are never written to disk.
- **Ephemeral sessions**: Tokens are lost when the server process terminates (e.g., when a Docker container is stopped with `--rm`).
- **No persistent storage**: This design prioritizes security over convenience, making it ideal for short-lived sessions.

### Public vs. Confidential Clients

- **Public clients**: The default OAuth App is a public client (no client secret). The client ID can be safely embedded in source code per OAuth 2.0 specifications.
- **Confidential clients**: If you provide a client secret via `--oauth-client-secret`, the app becomes a confidential client with additional security but requires secure secret storage.

### User Authorization

- **Explicit consent**: Users must explicitly authorize the OAuth App in their browser, providing full visibility into what access is being granted.
- **Revocable access**: Users can revoke access at any time from their GitHub settings: https://github.com/settings/applications

### Best Practices

1. **Use organization-owned OAuth Apps** for enterprise deployments to maintain control
2. **Regularly review authorized applications** in GitHub settings
3. **Use confidential clients** (with secrets) only when you can securely store the secret
4. **Prefer ephemeral tokens** (OAuth) over long-lived PATs when possible
5. **Enable organization policies** to restrict which OAuth Apps can access your organization's data

## Troubleshooting

### Common Issues

#### "Device code expired"

**Cause**: The user didn't complete authorization within the expiration time (typically 15 minutes).

**Solution**: Call `auth_login` again to start a new authentication flow.

#### "Authorization was denied by the user"

**Cause**: The user clicked "Cancel" or "Deny" during the authorization flow.

**Solution**: Call `auth_login` again and ensure the user completes the authorization.

#### "Failed to start authentication"

**Possible causes**:
- Network connectivity issues
- Invalid OAuth App configuration
- GitHub service issues

**Solutions**:
- Check network connectivity to GitHub
- Verify your OAuth client ID is correct
- Check GitHub's status page: https://www.githubstatus.com

#### OAuth not working with GHES

**Possible causes**:
- GHES version doesn't support device flow (requires GHES 3.0+)
- OAuth App not properly configured on GHES
- Incorrect hostname format

**Solutions**:
- Verify GHES version: `https://YOUR_GHES_HOSTNAME/api/v3/meta`
- Ensure OAuth App has device flow enabled in GHES settings
- Use full URL with scheme: `--gh-host https://github.yourcompany.com`

#### "Elicitation failed" message in logs

**Cause**: The MCP client doesn't support URL elicitation (an optional MCP feature).

**Effect**: The authentication flow still works, but the user won't see a clickable link. They'll need to manually copy/paste the verification URL.

**Solution**: No action needed - this is expected for some MCP clients.

### Debug Logging

Enable debug logging to troubleshoot authentication issues:

```json
{
  "github": {
    "command": "/path/to/github-mcp-server",
    "args": ["stdio", "--log-file", "/tmp/github-mcp-server.log"]
  }
}
```

Check the log file for detailed information about the authentication flow.

## Comparison with PAT Authentication

| Feature | OAuth Device Flow | Personal Access Token |
|---------|------------------|---------------------|
| **Initial Setup** | No pre-configuration needed | Must manually create token |
| **Token Storage** | In-memory only | Stored in config files |
| **Token Lifetime** | Session-based (ephemeral) | Long-lived (until revoked) |
| **Security** | Explicit browser authorization | Token visible in config |
| **User Experience** | Interactive flow | Copy/paste token |
| **Enterprise Control** | Organization OAuth App policies | Token-level restrictions |
| **Offline Use** | Requires initial online auth | Works offline after setup |
| **Multiple Clients** | Each client needs separate auth | Same token can be reused |

### When to Use Each Method

**Use OAuth Device Flow when:**
- You want the simplest setup experience
- You're using Docker with `--rm` (ephemeral containers)
- You prefer not storing tokens in config files
- Your organization uses OAuth App policies

**Use PAT when:**
- You need offline access after initial setup
- You want to reuse the same token across multiple MCP clients
- You need long-lived credentials for automation
- You're using an environment where interactive authentication isn't possible

### Migration Between Methods

You can switch between authentication methods at any time:

**From PAT to OAuth**: Simply remove the `GITHUB_PERSONAL_ACCESS_TOKEN` environment variable from your configuration and restart the server.

**From OAuth to PAT**: Add `GITHUB_PERSONAL_ACCESS_TOKEN` to your configuration with your token. The server will use PAT authentication and skip OAuth entirely.

## Additional Resources

- [OAuth 2.0 Device Authorization Grant (RFC 8628)](https://datatracker.ietf.org/doc/html/rfc8628)
- [GitHub OAuth Apps Documentation](https://docs.github.com/en/apps/oauth-apps)
- [GitHub Device Flow Documentation](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps#device-flow)
- [Creating a Personal Access Token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
- [GitHub MCP Server Documentation](https://github.com/github/github-mcp-server)

## Need Help?

If you encounter issues not covered in this guide:

1. Check the [GitHub Discussions](https://github.com/github/github-mcp-server/discussions)
2. Search [existing issues](https://github.com/github/github-mcp-server/issues)
3. [Open a new issue](https://github.com/github/github-mcp-server/issues/new) with:
   - Your server configuration (redact sensitive data)
   - Error messages or unexpected behavior
   - Debug logs (if available)
   - GitHub environment (github.com, GHES, GHEC)
