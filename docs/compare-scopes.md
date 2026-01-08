# Compare PAT Scopes Command

The `compare-scopes` command helps you verify that your GitHub Personal Access Token (PAT) has the necessary OAuth scopes for the tools you want to use.

## Overview

This command:
1. Fetches the OAuth scopes granted to your PAT from the GitHub API
2. Determines the scopes required by your enabled tools (based on toolset configuration)
3. Compares the two and reports any missing or extra scopes

## Usage

### Basic Usage

```bash
# Export your token
export GITHUB_PERSONAL_ACCESS_TOKEN=ghp_yourtoken

# Compare scopes with default toolsets
github-mcp-server compare-scopes

# Or use the convenience script
script/compare-scopes
```

### With Specific Toolsets

```bash
# Compare for specific toolsets
github-mcp-server compare-scopes --toolsets=repos,issues,pull_requests

# Compare with all toolsets
github-mcp-server compare-scopes --toolsets=all

# Compare with read-only mode
github-mcp-server compare-scopes --read-only
```

### Token via Flag

```bash
# Provide token via flag (overrides environment variable)
github-mcp-server compare-scopes --token=ghp_yourtoken
```

### JSON Output

```bash
# Get JSON output for programmatic use
github-mcp-server compare-scopes --output=json
```

## Output Formats

### Text Output (Default)

The default text output provides a human-readable comparison:

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

### JSON Output

Use `--output=json` for machine-readable output:

```json
{
  "comparison": {
    "token_scopes": ["repo", "read:org"],
    "required_scopes": ["read:org", "repo"],
    "missing_scopes": [],
    "extra_scopes": [],
    "has_all_required": true
  },
  "enabled_toolsets": ["context", "issues", "pull_requests", "repos", "users"],
  "read_only": false,
  "tools": [...],
  "scopes_by_tool": {...}
}
```

## Scope Hierarchy

The command understands GitHub's OAuth scope hierarchy. For example:
- `repo` grants access to `public_repo` and `security_events`
- `admin:org` grants access to `write:org` and `read:org`
- `write:org` grants access to `read:org`

If your token has a parent scope, the command will recognize that child scopes are satisfied.

## Fine-Grained PATs

Fine-grained Personal Access Tokens (PATs) don't return the `X-OAuth-Scopes` header, so the command will report:

```
Token Scopes:
  (no scopes - might be a fine-grained PAT)
```

Fine-grained PATs use a different permission model and cannot be validated with this command.

## GitHub Enterprise Support

The command supports GitHub Enterprise Server:

```bash
# For GitHub Enterprise Server
github-mcp-server compare-scopes --gh-host=github.enterprise.com
```

## Common Scenarios

### Creating a New Token

When creating a new token, use this command to verify you've selected all necessary scopes:

```bash
# Check what scopes you need
github-mcp-server list-scopes --output=summary

# After creating the token, verify it has everything
export GITHUB_PERSONAL_ACCESS_TOKEN=ghp_newtoken
github-mcp-server compare-scopes
```

### Debugging Permission Issues

If you're getting permission errors when using the server:

```bash
# Check if your token is missing required scopes
github-mcp-server compare-scopes --toolsets=all
```

### Different Toolset Configurations

Compare scopes for different configurations:

```bash
# Minimal configuration
github-mcp-server compare-scopes --toolsets=repos,issues

# Full configuration
github-mcp-server compare-scopes --toolsets=all

# Read-only mode (fewer permissions needed)
github-mcp-server compare-scopes --read-only --toolsets=all
```

## Exit Codes

- `0`: Success (command ran successfully)
- `1`: Error (missing token, API error, or other failure)

Note: The exit code does NOT indicate whether scopes match - check the output to determine if scopes are missing.

## Related Commands

- `list-scopes`: Lists required OAuth scopes for enabled tools (without checking your token)
- `stdio`: Runs the MCP server with your configured toolsets

## Example Workflows

### Workflow 1: Setting Up a New Token

1. List required scopes:
   ```bash
   script/list-scopes --toolsets=repos,issues,pull_requests --output=summary
   ```

2. Create a PAT on GitHub with those scopes

3. Verify the token:
   ```bash
   export GITHUB_PERSONAL_ACCESS_TOKEN=ghp_newtoken
   script/compare-scopes --toolsets=repos,issues,pull_requests
   ```

### Workflow 2: Troubleshooting Access Issues

1. Check current token scopes:
   ```bash
   script/compare-scopes --toolsets=all
   ```

2. If scopes are missing, update your PAT on GitHub

3. Verify the update:
   ```bash
   script/compare-scopes --toolsets=all
   ```

## Notes

- Classic PATs (starting with `ghp_`) are fully supported
- Fine-grained PATs are detected but cannot be validated
- The command uses a HEAD request to minimize bandwidth
- Scope hierarchy is automatically handled
- The `--read-only` flag reduces required permissions
