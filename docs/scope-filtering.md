# PAT Scope Filtering

The GitHub MCP Server automatically filters available tools based on your classic Personal Access Token's (PAT) OAuth scopes. This ensures you only see tools that your token has permission to use, reducing clutter and preventing errors from attempting operations your token can't perform.

> **Note:** This feature applies to **classic PATs** (tokens starting with `ghp_`). Fine-grained PATs and other token types don't support scope detection.

## How It Works

When the server starts with a classic PAT, it makes a lightweight HTTP HEAD request to the GitHub API to discover your token's scopes from the `X-OAuth-Scopes` header. Tools that require scopes your token doesn't have are automatically hidden.

**Example:** If your token only has `repo` and `gist` scopes, you won't see tools that require `admin:org`, `project`, or `notifications` scopes.

## PAT vs OAuth Authentication

| Authentication | Scope Handling |
|---------------|----------------|
| **Classic PAT** (`ghp_`) | Filters tools at startup based on token scopes—tools requiring unavailable scopes are hidden |
| **OAuth** (remote server only) | Uses OAuth scope challenges—when a tool needs a scope you haven't granted, you're prompted to authorize it |
| **Fine-grained PAT** (`github_pat_`) | No filtering—all tools shown, API enforces permissions |

With OAuth, the remote server can dynamically request additional scopes as needed. With PATs, scopes are fixed at token creation, so the server proactively hides tools you can't use.

## Checking Your Token's Scopes

To see what scopes your token has, you can run:

```bash
curl -sI -H "Authorization: Bearer $GITHUB_PERSONAL_ACCESS_TOKEN" \
  https://api.github.com/user | grep -i x-oauth-scopes
```

Example output:
```
x-oauth-scopes: delete_repo, gist, read:org, repo
```

## Scope Hierarchy

Some scopes implicitly include others:

- `repo` → includes `public_repo`, `security_events`
- `admin:org` → includes `write:org` → includes `read:org`
- `project` → includes `read:project`

This means if your token has `repo`, tools requiring `security_events` will also be available.

Each tool in the [README](../README.md#tools) lists its required and accepted OAuth scopes.

## Graceful Degradation

If the server cannot fetch your token's scopes (e.g., network issues, rate limiting), it logs a warning and continues **without filtering**. This ensures the server remains usable even when scope detection fails.

```
WARN: failed to fetch token scopes, continuing without scope filtering
```

## Classic vs Fine-Grained Personal Access Tokens

**Classic PATs** (`ghp_` prefix) support OAuth scopes and return them in the `X-OAuth-Scopes` header. Scope filtering works fully with these tokens.

**Fine-grained PATs** (`github_pat_` prefix) use a different permission model based on repository access and specific permissions rather than OAuth scopes. They don't return the `X-OAuth-Scopes` header, so scope filtering is skipped. All tools will be available, but the GitHub API will still enforce permissions at the API level—you'll get errors if you try to use tools your token doesn't have permission for.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Missing expected tools | Token lacks required scope | Add the scope to your PAT |
| All tools visible despite limited PAT | Scope detection failed | Check logs for warnings about scope fetching |
| "Insufficient permissions" errors | Tool visible but scope insufficient | This shouldn't happen with scope filtering; report as bug |

## Related Documentation

- [Server Configuration Guide](./server-configuration.md)
- [GitHub PAT Documentation](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
- [OAuth Scopes Reference](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps)
