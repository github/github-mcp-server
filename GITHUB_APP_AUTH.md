# GitHub App Authentication for MCP Server

This document describes how to configure and use GitHub App authentication with the local GitHub MCP server (stdio transport).

## Overview

The GitHub MCP server now supports **GitHub App authentication** in addition to Personal Access Tokens (PAT). GitHub App authentication provides:

- ✅ **Enterprise-friendly** authentication with fine-grained permissions
- ✅ **Automatic token refresh** (tokens are refreshed 5 minutes before expiration)
- ✅ **Multi-tenant support** (different installations for different accounts/organizations)
- ✅ **Audit trail** (GitHub Apps provide better visibility into which app is making requests)
- ✅ **Higher rate limits** for GitHub API calls

## Prerequisites

1. A GitHub App with the following permissions:
   - **Repository permissions** (based on your needs)
     - Contents: Read/Write
     - Issues: Read/Write
     - Pull requests: Read/Write
     - Actions: Read
     - Code scanning alerts: Read
     - Dependabot alerts: Read
     - etc.

2. The GitHub App must be **installed** on the account/organization you want to access

3. You need:
   - GitHub App ID
   - GitHub App private key (PEM file)
   - Installation ID for the target account

## Environment Variables

To use GitHub App authentication, set the following environment variables:

```bash
export GITHUB_APP_ID="your-app-id"
export GITHUB_APP_PRIVATE_KEY_PATH="/path/to/private-key.pem"
export GITHUB_APP_INSTALLATION_ID="installation-id"
```

### Finding Your GitHub App Information

#### App ID
1. Go to GitHub Settings → Developer settings → GitHub Apps
2. Click on your app
3. The App ID is shown at the top of the page

#### Private Key
1. In your GitHub App settings, scroll to "Private keys"
2. Click "Generate a private key" if you haven't already
3. Download the `.pem` file and store it securely
4. Set `GITHUB_APP_PRIVATE_KEY_PATH` to the full path of this file

#### Installation ID
You can find the installation ID by:

**Method 1: Via GitHub UI**
1. Go to GitHub Settings → Applications → Installed GitHub Apps
2. Click "Configure" on your app
3. The installation ID is in the URL: `https://github.com/settings/installations/{installation_id}`

**Method 2: Via API**
```bash
# Generate a JWT token (see below for script)
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
     -H "Accept: application/vnd.github+json" \
     https://api.github.com/app/installations
```

**Method 3: Use the discovery script**
Create a file `get-installations.js`:

```javascript
const jwt = require('jsonwebtoken');
const fs = require('fs');

const APP_ID = 'your-app-id';
const PRIVATE_KEY_PATH = '/path/to/private-key.pem';

const privateKey = fs.readFileSync(PRIVATE_KEY_PATH, 'utf8');
const now = Math.floor(Date.now() / 1000);

const payload = {
  iat: now,
  exp: now + (5 * 60),
  iss: APP_ID
};

const token = jwt.sign(payload, privateKey, { algorithm: 'RS256' });

fetch('https://api.github.com/app/installations', {
  headers: {
    'Authorization': `Bearer ${token}`,
    'Accept': 'application/vnd.github+json'
  }
})
.then(r => r.json())
.then(data => {
  console.log('Installations:');
  data.forEach(install => {
    console.log(`- ${install.account.login}: ${install.id}`);
  });
});
```

Run with: `node get-installations.js`

## Configuration Examples

### Claude Code (.claude.json)

```json
{
  "mcpServers": {
    "github-app": {
      "type": "stdio",
      "command": "/path/to/github-mcp-server",
      "args": ["stdio"],
      "env": {
        "GITHUB_APP_ID": "2137233",
        "GITHUB_APP_PRIVATE_KEY_PATH": "/home/user/.github/apps/my-app.pem",
        "GITHUB_APP_INSTALLATION_ID": "90590829"
      }
    }
  }
}
```

### Windows Configuration

For Windows paths, use double backslashes or forward slashes:

```json
{
  "mcpServers": {
    "github-app": {
      "type": "stdio",
      "command": "D:\\Code\\github-mcp-server\\github-mcp-server.exe",
      "args": ["stdio"],
      "env": {
        "GITHUB_APP_ID": "2137233",
        "GITHUB_APP_PRIVATE_KEY_PATH": "C:/Users/user/.config/gh/github-apps/my-app.pem",
        "GITHUB_APP_INSTALLATION_ID": "90590829"
      }
    }
  }
}
```

### Multiple Installations (Multi-tenant)

If your GitHub App is installed on multiple accounts, you can create separate MCP server instances:

```json
{
  "mcpServers": {
    "github-personal": {
      "type": "stdio",
      "command": "/path/to/github-mcp-server",
      "args": ["stdio"],
      "env": {
        "GITHUB_APP_ID": "2137233",
        "GITHUB_APP_PRIVATE_KEY_PATH": "/path/to/private-key.pem",
        "GITHUB_APP_INSTALLATION_ID": "90590829"
      }
    },
    "github-org": {
      "type": "stdio",
      "command": "/path/to/github-mcp-server",
      "args": ["stdio"],
      "env": {
        "GITHUB_APP_ID": "2137233",
        "GITHUB_APP_PRIVATE_KEY_PATH": "/path/to/private-key.pem",
        "GITHUB_APP_INSTALLATION_ID": "90545428"
      }
    }
  }
}
```

## Backward Compatibility

The Personal Access Token (PAT) authentication method is still fully supported. If you set `GITHUB_PERSONAL_ACCESS_TOKEN`, the server will use PAT authentication:

```json
{
  "mcpServers": {
    "github-pat": {
      "type": "stdio",
      "command": "/path/to/github-mcp-server",
      "args": ["stdio"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "ghp_xxxxxxxxxxxxx"
      }
    }
  }
}
```

## How It Works

### Authentication Flow

1. **JWT Generation**: When the server starts, it generates a JSON Web Token (JWT) signed with your GitHub App's private key
2. **Token Exchange**: The JWT is exchanged with GitHub's API for an installation access token
3. **Token Caching**: The installation token (valid for 1 hour) is cached and reused
4. **Automatic Refresh**: A background goroutine refreshes the token 5 minutes before it expires
5. **GitHub API Calls**: All GitHub API requests use the current installation token

### Token Lifecycle

```
Server Start
    ↓
Generate JWT (5-minute expiration)
    ↓
Exchange JWT for Installation Token (1-hour expiration)
    ↓
Use Token for GitHub API Calls
    ↓
[After 55 minutes]
    ↓
Background Refresh Triggered
    ↓
Generate New JWT
    ↓
Exchange for New Installation Token
    ↓
Continue Using Fresh Token
```

### Security Notes

- **Private Key**: Never commit your private key to version control. Store it securely and reference it via environment variable
- **Installation Tokens**: Installation tokens are scoped to a specific installation and have the permissions you granted the app
- **JWT Expiration**: JWTs are short-lived (5 minutes) to minimize security risk if intercepted
- **Token Refresh**: Tokens are proactively refreshed to avoid service interruption

## Troubleshooting

### Error: "authentication error: no authentication credentials provided"

Make sure you've set either `GITHUB_APP_*` variables or `GITHUB_PERSONAL_ACCESS_TOKEN`.

### Error: "'Expiration time' claim ('exp') is too far in the future"

This error should not occur in the current implementation (JWT expiration is set to 5 minutes). If you see this, verify you're using the latest version of the server.

### Error: "failed to load private key"

Check that:
- The path in `GITHUB_APP_PRIVATE_KEY_PATH` is correct
- The file exists and is readable
- The file is a valid PEM-formatted RSA private key

### Error: "failed to get installation token: status 401"

This usually means:
- The App ID is incorrect
- The private key doesn't match the App ID
- The JWT is malformed or expired

### Error: "failed to get installation token: status 404"

The installation ID is incorrect or the app is not installed on the target account.

### Server Logs

When the server starts successfully with GitHub App authentication, you'll see:

```
[github-mcp-server] Using GitHub App authentication
time=2025-10-20T12:09:33.308-07:00 level=INFO msg="starting server" version=version
GitHub MCP Server running on stdio
```

If using PAT, you'll see:

```
[github-mcp-server] Using Personal Access Token authentication
```

## Implementation Details

### Code Structure

- **`internal/auth/githubapp.go`**: Core GitHub App authentication logic
  - `GitHubAppAuthProvider`: Main authentication provider type
  - `generateJWT()`: Creates signed JWTs for GitHub API
  - `refreshToken()`: Exchanges JWT for installation token
  - `tokenRefreshLoop()`: Background refresh goroutine
  - `LoadAuthConfigFromEnv()`: Environment variable configuration loader

- **`cmd/github-mcp-server/main.go`**: Integration point
  - Calls `auth.LoadAuthConfigFromEnv()` to determine auth method
  - Supports both PAT and GitHub App authentication
  - Logs which authentication method is being used

### Dependencies

The implementation uses:
- `github.com/golang-jwt/jwt/v5` - JWT generation and signing
- Go standard library (`crypto/rsa`, `crypto/x509`, `encoding/pem`) - Private key handling

## Further Reading

- [GitHub Apps Documentation](https://docs.github.com/en/apps)
- [Authenticating with GitHub Apps](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/about-authentication-with-a-github-app)
- [GitHub App Installation Tokens](https://docs.github.com/en/rest/apps/apps#create-an-installation-access-token-for-an-app)
- [Model Context Protocol Specification](https://spec.modelcontextprotocol.io/)

## Contributing

If you encounter issues or have suggestions for improving GitHub App authentication, please:

1. Check existing issues at https://github.com/a5af/github-mcp-server/issues
2. Create a new issue with detailed information about your setup and the problem

## License

This implementation follows the same license as the parent project (github/github-mcp-server).
