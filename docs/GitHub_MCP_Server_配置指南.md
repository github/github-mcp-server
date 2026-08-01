# 服务器配置指南

本指南帮助你根据使用场景选择合适的配置，并说明如何应用这些配置。有关可用 toolsets 和 tools 的完整参考，请参阅 [README](../README.md#tool-configuration)。

## 快速参考
目前，我们支持通过以下方式配置 GitHub MCP Server： 

| 配置项 | Remote Server | Local Server |
|---------------|---------------|--------------|
| Toolsets | `X-MCP-Toolsets` header 或 `/x/{toolset}` URL | `--toolsets` flag 或 `GITHUB_TOOLSETS` 环境变量 |
| 单个 Tools | `X-MCP-Tools` header | `--tools` flag 或 `GITHUB_TOOLS` 环境变量 |
| 排除 Tools | `X-MCP-Exclude-Tools` header | `--exclude-tools` flag 或 `GITHUB_EXCLUDE_TOOLS` 环境变量 |
| 只读模式 | `X-MCP-Readonly` header 或 `/readonly` URL | `--read-only` flag 或 `GITHUB_READ_ONLY` 环境变量 |
| Lockdown 模式 | `X-MCP-Lockdown` header | `--lockdown-mode` flag 或 `GITHUB_LOCKDOWN_MODE` 环境变量 |
| Insiders 模式 | `X-MCP-Insiders` header 或 `/insiders` URL | `--insiders` flag 或 `GITHUB_INSIDERS` 环境变量 |
| Feature Flags | `X-MCP-Features` header | `--features` flag |
| Scope 过滤 | 始终启用 | 始终启用 |
| 服务器名称/标题 | 不可用 | `GITHUB_MCP_SERVER_NAME` / `GITHUB_MCP_SERVER_TITLE` 环境变量或 `github-mcp-server-config.json` |

> **默认行为：** 如果未指定任何配置，服务器将使用以下**默认 toolsets**：`context`、`issues`、`pull_requests`、`repos`、`users`。

---

## 配置的工作方式

所有配置选项都可以**组合使用**：你可以根据工作流需要，以任意方式组合 toolsets、单个 tools、排除的 tools、只读模式和 lockdown 模式。

注意：**read-only** 模式是一种严格的安全过滤器，其优先级高于其他所有配置。即使显式请求了写入 tools，该模式也会将其禁用。

注意：**excluded tools** 的优先级高于 toolsets 和单个 tools——列出的 tools 始终会被排除，即使其所属 toolset 已启用，或通过 `--tools` / `X-MCP-Tools` 显式添加也是如此。

---

## 配置示例

以下示例使用 VS Code 配置格式来说明相关概念。如果你使用其他 MCP Host（Cursor、Claude Desktop、JetBrains 等），配置格式可能略有不同。有关特定 Host 的设置方式，请参阅[安装指南](./installation-guides)。

### 启用指定 Tools

**适用场景：** 明确知道自己需要哪些 tools，并希望仅加载实际使用的 tools，以优化上下文占用的用户。 

**示例：**

<table>
<tr><th>Remote Server</th><th>Local Server</th></tr>
<tr valign="top">
<td>

```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/",
  "headers": {
    "X-MCP-Tools": "get_file_contents,get_me,pull_request_read"
  }
}
```

</td>
<td>

```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--tools=get_file_contents,get_me,pull_request_read"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

</td>
</tr>
</table>

---

### 启用指定 Toolsets

**适用场景：** 希望启用多个相关 toolsets 的用户。

<table>
<tr><th>Remote Server</th><th>Local Server</th></tr>
<tr valign="top">
<td>

```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/",
  "headers": {
    "X-MCP-Toolsets": "issues,pull_requests"
  }
}
```

</td>
<td>

```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--toolsets=issues,pull_requests"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

</td>
</tr>
</table>

---

### 同时启用 Toolsets 和 Tools

**适用场景：** 希望在部分功能领域启用较完整能力，同时从其他领域中选择特定 tools 的用户。

先启用完整的 toolsets，再从不希望完整启用的其他 toolsets 中添加单个 tools。

<table>
<tr><th>Remote Server</th><th>Local Server</th></tr>
<tr valign="top">
<td>

```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/",
  "headers": {
    "X-MCP-Toolsets": "repos,issues",
    "X-MCP-Tools": "get_gist,pull_request_read"
  }
}
```

</td>
<td>

```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--toolsets=repos,issues",
    "--tools=get_gist,pull_request_read"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

</td>
</tr>
</table>

**结果：** 启用所有 repository 和 issue tools，并额外启用所需的 gist tools。

---

### 排除指定 Tools

**适用场景：** 希望启用范围较广的 toolset，但出于安全、合规或防止非预期行为等原因，需要排除特定 tools 的用户。

无论其他配置如何，列出的 tools 都会被移除——即使其所属 toolset 已启用，或这些 tools 已被单独添加也是如此。

<table>
<tr><th>Remote Server</th><th>Local Server</th></tr>
<tr valign="top">
<td>

```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/",
  "headers": {
    "X-MCP-Toolsets": "pull_requests",
    "X-MCP-Exclude-Tools": "create_pull_request,merge_pull_request"
  }
}
```

</td>
<td>

```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--toolsets=pull_requests",
    "--exclude-tools=create_pull_request,merge_pull_request"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

</td>
</tr>
</table>

**结果：** 启用除 `create_pull_request` 和 `merge_pull_request` 以外的所有 pull request tools——用户只能使用读取和审查 tools。

---

### 只读模式

**适用场景：** 注重安全，希望确保服务器不允许执行修改 issue、pull request、repository 等资源操作的用户。

启用后，该模式会禁用所有非只读 tools，即使这些 tools 已被请求也是如此。

**示例：** 
<table>
<tr><th>Remote Server</th><th>Local Server</th></tr>
<tr valign="top">
<td>

**选项 A：Header**
```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/",
  "headers": {
    "X-MCP-Toolsets": "issues,repos,pull_requests",
    "X-MCP-Readonly": "true"
  }
}
```

**选项 B：URL 路径**
```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/x/all/readonly"
}
```

</td>
<td>


```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--toolsets=issues,repos,pull_requests",
    "--read-only"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

</td>
</tr>
</table>

> 即使 `issues` toolset 中包含 `create_issue`，它也会在只读模式下被排除。

---

### Lockdown 模式

**适用场景：** 对于公共 repository，希望限制没有 push 权限的用户所提供的内容。

Lockdown 模式确保服务器只会展示公共 repository 中由具备该 repository push 权限的用户所提供的内容。私有 repository 不受影响，协作者仍可完整访问其自己的内容。

**示例：**
<table>
<tr><th>Remote Server</th><th>Local Server</th></tr>
<tr valign="top">
<td>

```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/",
  "headers": {
    "X-MCP-Lockdown": "true"
  }
}
```

</td>
<td>

```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--lockdown-mode"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

</td>
</tr>
</table>

---

### Insiders 模式

**适用场景：** 希望在实验性功能和新 tools 正式发布前抢先体验的用户。

Insiders 模式会解锁实验性功能，例如对 [MCP Apps](#mcp-apps) 的支持。我们创建此模式，是为了能够逐步推出实验性功能并收集反馈。因此，如果你正在使用 Insiders，请随时向我们提供反馈！Insiders 模式中的功能可能会根据用户反馈发生更改、演进或被移除。

<table>
<tr><th>Remote Server</th><th>Local Server</th></tr>
<tr valign="top">
<td>

**选项 A：URL 路径**
```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/insiders"
}
```

**选项 B：Header**
```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/",
  "headers": {
    "X-MCP-Insiders": "true"
  }
}
```

</td>
<td>

```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--insiders"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

</td>
</tr>
</table>

有关 Insiders 模式中所有可用功能的完整列表，请参阅 [Insiders 功能](./insiders-features.md)。

---

### MCP Apps

[MCP Apps](https://modelcontextprotocol.io/docs/extensions/apps) 是 Model Context Protocol 的一项扩展，使服务器能够向最终用户提供交互式用户界面。tools 不再只是返回需要由 LLM 解释和转述的纯文本，而是可以直接在聊天界面中渲染表单、个人资料和仪表盘。

MCP Apps 可通过 [Insiders 模式](#insiders-模式)启用，也可以通过 `remote_mcp_ui_apps` feature flag 单独启用。

如果希望在保留 MCP App 结果视图的同时，让写入 tools 直接执行，
而不是先打开交互式表单，还需要启用
`mcp_apps_disable_form_deferral` feature flag。对于 Remote Server，请在请求 Header 中同时发送这两个
flags：

```http
X-MCP-Features: remote_mcp_ui_apps,mcp_apps_disable_form_deferral
```

对于 Local Server，请将这两个 flags 都传递给 `--features`。

**支持的 tools：**

| Tool | 说明 |
|------|-------------|
| `get_me` | 以富卡片形式展示你的 GitHub 用户资料，包括头像、个人简介和统计信息 |
| `issue_write` | 打开交互式表单，用于创建或更新 issues |
| `create_pull_request` | 提供完整的 PR 创建表单，用于创建 pull request（或 draft pull request） |

**客户端要求：** MCP Apps 要求 Host 支持 [MCP Apps 扩展](https://modelcontextprotocol.io/docs/extensions/apps)。目前已在 VS Code 中完成测试（`chat.mcp.apps.enabled` 设置）。

<table>
<tr><th>Remote Server</th><th>Local Server</th></tr>
<tr valign="top">
<td>

```json
{
  "type": "http",
  "url": "https://api.githubcopilot.com/mcp/",
  "headers": {
    "X-MCP-Features": "remote_mcp_ui_apps"
  }
}
```

</td>
<td>

```json
{
  "type": "stdio",
  "command": "go",
  "args": [
    "run",
    "./cmd/github-mcp-server",
    "stdio",
    "--features=remote_mcp_ui_apps"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
  }
}
```

</td>
</tr>
</table>

---

### Scope 过滤

**自动功能：** 服务器会根据身份认证类型，以不同方式处理 OAuth scopes：

- **Classic PATs**（`ghp_` 前缀）：启动时会根据 token scopes 过滤 tools——你只能看到自己有权使用的 tools
- **OAuth**（Remote Server）：使用 scope challenges——当某个 tool 需要你尚未授权的 scope 时，系统会提示你进行授权
- **其他 tokens**：不进行过滤——展示所有 tools，由 API 强制执行权限控制

此过程会透明完成，无需任何配置。如果 Classic PAT 的 scope 检测失败（例如发生网络问题），服务器会记录一条警告，并在所有 tools 均可用的情况下继续运行。

有关不同 token 类型的过滤工作方式，请参阅 [Scope 过滤](./scope-filtering.md)。

---

## 故障排查

| 问题 | 原因 | 解决方案 |
|---------|-------|----------|
| 服务器启动失败 | `--tools` 或 `X-MCP-Tools` 中存在无效的 tool 名称 | 检查 tool 名称的拼写；使用 [Tools 列表](../README.md#tools)中的准确名称 |
| 写入 tools 无法工作 | 已启用只读模式 | 移除 `--read-only` flag 或 `X-MCP-Readonly` header |
| Tools 缺失 | 未启用对应 toolset | 添加所需的 toolset 或指定 tool |

---

## 实用链接

- [README：Tool 配置](../README.md#tool-configuration)
- [README：可用 Toolsets](../README.md#available-toolsets) — toolsets 完整列表
- [README：Tools](../README.md#tools) — 单个 tools 完整列表
- [Remote Server 文档](./remote-server.md) — Remote Server 专用选项和 headers
- [安装指南](./installation-guides) — 特定 Host 的设置说明
