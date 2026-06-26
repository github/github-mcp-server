# PAT Scope Filtering

The GitHub MCP Server automatically filters available tools based on your classic Personal Access Token's (PAT) OAuth scopes. This ensures you only see tools that your token has permission to use, reducing clutter and preventing errors from attempting operations your token can't perform.

> **Note:** This feature applies to **classic PATs** (tokens starting with `ghp_`). Fine-grained PATs, GitHub App installation tokens, and server-to-server tokens don't support scope detection and show all tools.

> **Important:** Scope filtering is a best-effort UX convenience, **not an authorization boundary**. The GitHub API is always the source of truth and enforces real permissions. The server therefore **fails open**: it only hides a tool when confident your token cannot use it, and shows the tool whenever access is plausible. See [Limitations and Fail-Open Posture](#limitations-and-fail-open-posture).

## How It Works

When the server starts with a classic PAT, it makes a lightweight HTTP HEAD request to the GitHub API to discover your token's scopes from the `X-OAuth-Scopes` header. Tools that require scopes your token doesn't have are automatically hidden.

**Example:** If your token only has `repo` and `gist` scopes, you won't see tools that require `admin:org`, `project`, or `notifications` scopes.

## PAT vs OAuth Authentication

| Authentication | Scope Handling |
|---------------|----------------|
| **Classic PAT** (`ghp_`) | Filters tools at startup based on token scopes—tools requiring unavailable scopes are hidden |
| **OAuth** (remote server only) | Uses OAuth scope challenges—when a tool needs a scope you haven't granted, you're prompted to authorize it |
| **Fine-grained PAT** (`github_pat_`) | No filtering—all tools shown, API enforces permissions |
| **GitHub App** (`ghs_`) | No filtering—all tools shown, permissions based on app installation |
| **Server-to-server** | No filtering—all tools shown, permissions based on app/token configuration |

With OAuth, the remote server can dynamically request additional scopes as needed. With PATs, scopes are fixed at token creation, so the server proactively hides tools you can't use.

## OAuth Scope Challenges (Remote Server)

When using the [remote MCP server](./remote-server.md) with OAuth authentication, the server uses a different approach called **scope challenges**. Instead of hiding tools upfront, all tools are available, and the server requests additional scopes on-demand when you try to use a tool that requires them.

**How it works:**
1. You attempt to use a tool (e.g., creating an issue)
2. If your current OAuth token lacks the required scope, the server returns an OAuth scope challenge
3. Your MCP client prompts you to authorize the additional scope
4. After authorization, the operation completes successfully

This provides a smoother user experience for OAuth users since you only grant permissions as needed, rather than requesting all scopes upfront.

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

## Public Repository Access

Read-only tools that only require `repo` or `public_repo` scopes are **always visible**, even if your token doesn't have these scopes. This is because these tools work on public repositories without authentication.

For example, `get_file_contents` is always available—you can read files from any public repository regardless of your token's scopes. However, write operations like `create_or_update_file` will be hidden if your token lacks `repo` scope.

> **Note:** The GitHub API doesn't return `public_repo` in the `X-OAuth-Scopes` header—it's implicit. The server handles this by not filtering read-only repository tools.

## Graceful Degradation

If the server cannot fetch your token's scopes (e.g., network issues, rate limiting), it logs a warning and continues **without filtering**. This ensures the server remains usable even when scope detection fails.

```
WARN: failed to fetch token scopes, continuing without scope filtering
```

## Limitations and Fail-Open Posture

Scope filtering is a **best-effort UX nicety**, not an authorization boundary. The GitHub API is the source of truth and enforces real permissions regardless of what the server shows. Because of this, the server is designed to **fail open**: it only hides a tool (or, for OAuth, issues a scope challenge) when it is confident the token cannot use it. When access is plausible, it prefers to show the tool and let the API decide. Filtering is also limited to classic PATs (`ghp_`) and is skipped entirely when scopes can't be fetched.

A tool's declared scopes are **all required** (logical AND), and each one may be satisfied directly or by an ancestor scope from the [hierarchy](#scope-hierarchy). However, some ways a tool can legitimately be used cannot be determined from OAuth scopes alone, so the server intentionally does not try to model them:

- **Sibling scopes outside the hierarchy.** The hierarchy only models *ancestor* substitution. For example, code scanning alerts on a **public** repository are readable with `public_repo`, which is a *sibling* of the declared `security_events` (both are children of `repo`), not an ancestor. Token expansion can't bridge siblings. Capturing this faithfully would require representing requirements as a list of OR-groups (groups AND-ed, members within a group OR-ed), e.g. code scanning = `security_events` OR `public_repo` OR `repo`. That model isn't implemented today; instead the server relies on the fail-open posture so these cases aren't wrongly hidden.
- **Organization roles.** Roles such as *security manager* grant access orthogonally to OAuth scopes and are invisible to scope detection. A user may legitimately have access the server cannot see from scopes alone.
- **Public vs. private repositories.** Whether a given scope suffices depends on the target repository's visibility, which isn't known at filter time.

In each of these cases the server errs toward showing the tool; if the token truly lacks access, the API returns the appropriate error.

## Classic vs Fine-Grained Personal Access Tokens

**Classic PATs** (`ghp_` prefix) support OAuth scopes and return them in the `X-OAuth-Scopes` header. Scope filtering works fully with these tokens.

**Fine-grained PATs** (`github_pat_` prefix) use a different permission model based on repository access and specific permissions rather than OAuth scopes. They don't return the `X-OAuth-Scopes` header, so scope filtering is skipped. All tools will be available, but the GitHub API will still enforce permissions at the API level—you'll get errors if you try to use tools your token doesn't have permission for.

## GitHub App and Server-to-Server Tokens

**GitHub App installation tokens** (`ghs_` prefix) and other server-to-server tokens use a permission model based on the app's installation permissions rather than OAuth scopes. These tokens don't return the `X-OAuth-Scopes` header, so scope filtering is skipped. The GitHub API enforces permissions based on the app's configuration.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Missing expected tools | Token lacks required scope | [Edit your PAT's scopes](https://github.com/settings/tokens) in GitHub settings |
| All tools visible despite limited PAT | Scope detection failed | Check logs for warnings about scope fetching |
| "Insufficient permissions" errors | Tool visible but scope insufficient | Expected in some cases (fail-open, public/private ambiguity, org roles, or scope detection skipped). The API enforces the real boundary—grant the needed scope or access |

> **Tip:** You can adjust the scopes of an existing classic PAT at any time via [GitHub's token settings](https://github.com/settings/tokens). After updating scopes, restart the MCP server to pick up the changes.

## Related Documentation

- [Server Configuration Guide](./server-configuration.md)
- [GitHub PAT Documentation](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
- [OAuth Scopes Reference](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps)
