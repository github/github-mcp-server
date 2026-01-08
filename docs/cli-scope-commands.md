# CLI Commands for OAuth Scope Management

The GitHub MCP Server provides two CLI commands to help manage OAuth scopes for your Personal Access Token (PAT).

## list-scopes

Lists the OAuth scopes required by your enabled tools.

### Usage

```bash
# List scopes for default toolsets
github-mcp-server list-scopes

# List scopes for specific toolsets
github-mcp-server list-scopes --toolsets=repos,issues,pull_requests

# List scopes for all toolsets
github-mcp-server list-scopes --toolsets=all

# Show only unique scopes needed (summary format)
github-mcp-server list-scopes --output=summary

# Get JSON output for programmatic use
github-mcp-server list-scopes --output=json

# Use the convenience script
script/list-scopes --toolsets=all --output=summary
```

### Example Output

```
Required OAuth scopes for enabled tools:

  read:org
  repo

Total: 2 unique scope(s)
```

## compare-scopes

Compares your PAT's granted scopes with the scopes required by enabled tools.

### Usage

```bash
# Export your token
export GITHUB_PERSONAL_ACCESS_TOKEN=ghp_yourtoken

# Compare with default toolsets
github-mcp-server compare-scopes

# Compare with specific toolsets
github-mcp-server compare-scopes --toolsets=repos,issues,pull_requests

# Provide token via flag
github-mcp-server compare-scopes --token=ghp_yourtoken

# Get JSON output
github-mcp-server compare-scopes --output=json

# Use the convenience script
script/compare-scopes --toolsets=all
```

### Example Output

When all scopes are present:
```
PAT Scope Comparison
====================

Token Scopes:
  • repo
  • read:org

Required Scopes:
  • read:org
  • repo

Comparison Result:
  ✓ Token has all required scopes!

Configuration: 5 toolset(s), read-only=false
Toolsets: context, issues, pull_requests, repos, users
```

When scopes are missing:
```
PAT Scope Comparison
====================

Token Scopes:
  • read:org

Required Scopes:
  • read:org
  • repo

Comparison Result:
  ✗ Token is missing required scopes

Missing Scopes (need to add):
  ✗ repo

Configuration: 5 toolset(s), read-only=false
Toolsets: context, issues, pull_requests, repos, users
```

## Workflow: Setting Up a New Token

Use these commands together to set up a new PAT:

1. **Determine required scopes**
   ```bash
   script/list-scopes --toolsets=repos,issues,pull_requests --output=summary
   ```

2. **Create PAT on GitHub** with the listed scopes

3. **Verify the token**
   ```bash
   export GITHUB_PERSONAL_ACCESS_TOKEN=ghp_newtoken
   script/compare-scopes --toolsets=repos,issues,pull_requests
   ```

4. **If scopes are missing**, update your PAT on GitHub and verify again

## Features

### Scope Hierarchy Support

Both commands understand GitHub's OAuth scope hierarchy:
- `repo` grants `public_repo` and `security_events`
- `admin:org` grants `write:org` and `read:org`
- `write:org` grants `read:org`
- `project` grants `read:project`
- `write:packages` grants `read:packages`
- `user` grants `read:user` and `user:email`

### GitHub Enterprise Support

Both commands support GitHub Enterprise Server:

```bash
# For GitHub Enterprise Server
github-mcp-server list-scopes --gh-host=github.enterprise.com
github-mcp-server compare-scopes --gh-host=github.enterprise.com
```

### Fine-Grained PATs

Fine-grained Personal Access Tokens use a different permission model and don't return OAuth scopes. The `compare-scopes` command will detect this:

```
Token Scopes:
  (no scopes - might be a fine-grained PAT)
```

Fine-grained PATs cannot be validated with these commands. Use classic PATs (starting with `ghp_`) for OAuth scope validation.

## Additional Options

Both commands support the same configuration flags as the `stdio` command:

- `--toolsets`: Specify which toolsets to include
- `--tools`: Specify individual tools to include
- `--read-only`: Restrict to read-only operations (reduces required scopes)
- `--gh-host`: Use GitHub Enterprise Server

For complete documentation:
- `list-scopes`: See command help with `github-mcp-server list-scopes --help`
- `compare-scopes`: See [docs/compare-scopes.md](docs/compare-scopes.md) for detailed guide
