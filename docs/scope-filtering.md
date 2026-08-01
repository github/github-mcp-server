# PAT Scope Filtering

GitHub MCP Server 会根据您的 classic Personal Access Token（PAT）的 OAuth scope 自动筛选可用工具。这可确保您只会看到 token 有权使用的工具，减少干扰并避免尝试 token 无法执行的操作时出现错误。

> **注意：**此功能适用于 **classic PAT**（以 `ghp_` 开头的 token）。Fine-grained PAT、GitHub App installation token 和 server-to-server token 不支持 scope 检测，会显示所有工具。

## 工作方式

server 使用 classic PAT 启动时，会向 GitHub API 发出轻量 HTTP HEAD 请求，从 `X-OAuth-Scopes` header 中发现 token 的 scope。需要 token 不具备 scope 的工具会自动隐藏。

**示例：**如果您的 token 仅有 `repo` 和 `gist` scope，将看不到需要 `admin:org`、`project` 或 `notifications` scope 的工具。

## PAT 与 OAuth 身份验证

| 身份验证 | Scope 处理 |
|---------------|----------------|
| **Classic PAT** (`ghp_`) | 在启动时按 token scope 筛选工具，需要不可用 scope 的工具将隐藏 |
| **OAuth**（仅远程 server） | 使用 OAuth scope challenge：当工具需要尚未授予的 scope 时，会提示您授权 |
| **Fine-grained PAT** (`github_pat_`) | 不筛选，显示所有工具，API 强制执行权限 |
| **GitHub App** (`ghs_`) | 不筛选，显示所有工具，权限基于 App installation |
| **Server-to-server** | 不筛选，显示所有工具，权限基于 App/token 配置 |

使用 OAuth 时，远程 server 可按需动态请求额外 scope。PAT 的 scope 在创建 token 时固定，因此 server 会主动隐藏无法使用的工具。

## OAuth Scope Challenge（远程 Server）

通过 OAuth 身份验证使用[远程 MCP server](./remote-server.md)时，server 会采用称为 **scope challenge** 的不同方式。它不会预先隐藏工具，而是提供所有工具；当您尝试使用需要额外 scope 的工具时，server 会按需请求这些 scope。

**工作方式：**
1. 您尝试使用一个工具（例如创建 issue）
2. 如果当前 OAuth token 缺少所需 scope，server 会返回 OAuth scope challenge
3. MCP 客户端提示您授权额外 scope
4. 授权后，操作会成功完成

这为 OAuth 用户提供更顺畅的体验，因为您只在需要时才授予权限，而非预先请求所有 scope。

## 检查 Token 的 Scope

To see what scopes your token has, you can run:

```bash
curl -sI -H "Authorization: Bearer $GITHUB_PERSONAL_ACCESS_TOKEN" \
  https://api.github.com/user | grep -i x-oauth-scopes
```

示例输出：
```
x-oauth-scopes: delete_repo, gist, read:org, repo
```

## Scope 层级

某些 scope 隐式包含其他 scope：

- `repo` → includes `public_repo`, `security_events`
- `admin:org` → includes `write:org` → includes `read:org`
- `project` → includes `read:project`

这意味着，如果 token 包含 `repo`，需要 `security_events` 的工具同样可用。

[README](../README.md#tools) 中的每个工具都列出了其所需和接受的 OAuth scope。

## Public Repository 访问

只需要 `repo` 或 `public_repo` scope 的只读工具**始终可见**，即使 token 没有这些 scope。这是因为这些工具无需身份验证即可在 public repository 上工作。

例如，`get_file_contents` 始终可用，无论 token 的 scope 如何，您都可以读取任何 public repository 中的文件。但如果 token 缺少 `repo` scope，则会隐藏 `create_or_update_file` 等写操作。

> **注意：**GitHub API 不会在 `X-OAuth-Scopes` header 中返回 `public_repo`，它是隐式的。server 通过不筛选只读 repository 工具来处理这一点。

## 优雅降级

如果 server 无法获取 token 的 scope（例如网络问题、限流），会记录 warning 并在**不筛选**的情况下继续运行。这可确保 scope 检测失败时 server 仍可使用。

```
WARN: failed to fetch token scopes, continuing without scope filtering
```

## Classic 与 Fine-grained Personal Access Token

**Classic PAT**（`ghp_` 前缀）支持 OAuth scope，并在 `X-OAuth-Scopes` header 中返回它们。Scope filtering 可完整适用于此类 token。

**Fine-grained PAT**（`github_pat_` 前缀）使用基于 repository 访问权限和特定权限的不同权限模型，而非 OAuth scope。它们不会返回 `X-OAuth-Scopes` header，因此会跳过 scope filtering。所有工具都可用，但 GitHub API 仍会在 API 层强制执行权限；若尝试使用 token 无权使用的工具，将收到错误。

## GitHub App 和 Server-to-Server Token

**GitHub App installation token**（`ghs_` 前缀）和其他 server-to-server token 使用基于 App installation 权限的权限模型，而非 OAuth scope。这些 token 不会返回 `X-OAuth-Scopes` header，因此会跳过 scope filtering。GitHub API 会根据 App 配置强制执行权限。

## 故障排除

| 问题 | 原因 | 解决方案 |
|---------|-------|----------|
| 缺少预期工具 | Token 缺少所需 scope | 在 GitHub settings 中[编辑 PAT 的 scope](https://github.com/settings/tokens) |
| PAT 受限但所有工具可见 | Scope 检测失败 | 检查日志中是否有获取 scope 的 warning |
| “权限不足”错误 | 工具可见但 scope 不足 | 使用 scope filtering 时不应发生；请报告为 bug |

> **提示：**您可随时在 [GitHub 的 token settings](https://github.com/settings/tokens) 中调整现有 classic PAT 的 scope。更新 scope 后，请重启 MCP server 以应用更改。

## 相关文档

- [Server Configuration Guide](./server-configuration.md)
- [GitHub PAT 文档](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
- [OAuth Scope 参考](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps)
