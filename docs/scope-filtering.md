# OAuth Scope Filtering

The GitHub MCP Server automatically filters available tools based on your Personal Access Token's (PAT) OAuth scopes. This ensures you only see tools that your token has permission to use, reducing clutter and preventing errors from attempting operations your token can't perform.

## How It Works

When the server starts, it makes a lightweight HTTP HEAD request to the GitHub API to discover your token's scopes from the `X-OAuth-Scopes` header. Tools that require scopes your token doesn't have are automatically hidden.

**Example:** If your token only has `repo` and `gist` scopes, you won't see tools that require `admin:org`, `project`, or `notifications` scopes.

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

## Scopes and Tools

The following table shows which OAuth scopes are required for each category of tools:

| Scope | Tools Enabled |
|-------|---------------|
| `repo` | Repository operations, issues, PRs, commits, branches, code search, workflows |
| `public_repo` | Star/unstar public repositories (implicit with `repo`) |
| `read:org` | Read organization info, list teams, team members |
| `write:org` | Organization management (includes `read:org`) |
| `admin:org` | Full organization administration (includes `write:org`, `read:org`) |
| `gist` | Create, update, and manage gists |
| `notifications` | List, manage, and dismiss notifications |
| `read:project` | Read GitHub Projects |
| `project` | Create and manage GitHub Projects (includes `read:project`) |
| `security_events` | Code scanning, Dependabot, secret scanning alerts (implicit with `repo`) |
| `user` | Update user profile |
| `read:user` | Read user profile information |

### Scope Hierarchy

Some scopes implicitly include others:

- `repo` → includes `public_repo`, `security_events`
- `admin:org` → includes `write:org` → includes `read:org`
- `project` → includes `read:project`

This means if your token has `repo`, tools requiring `security_events` will also be available.

## Recommended Token Scopes

For full functionality, we recommend these scopes:

| Use Case | Recommended Scopes |
|----------|-------------------|
| Basic development | `repo`, `read:org` |
| Full development | `repo`, `admin:org`, `gist`, `notifications`, `project` |
| Read-only access | `repo` (with `--read-only` flag) |
| Security analysis | `repo` (includes `security_events`) |

## Graceful Degradation

If the server cannot fetch your token's scopes (e.g., network issues, rate limiting), it logs a warning and continues **without filtering**. This ensures the server remains usable even when scope detection fails.

```
WARN: failed to fetch token scopes, continuing without scope filtering
```

## Fine-Grained Personal Access Tokens

Fine-grained PATs use a different permission model and don't return OAuth scopes in the `X-OAuth-Scopes` header. When using fine-grained PATs, scope filtering will be skipped and all tools will be available. The GitHub API will still enforce permissions at the API level.

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
