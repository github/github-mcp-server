# 在 Zed 中安装 GitHub MCP Server

[Zed](https://zed.dev) 是一款高性能多人协作代码编辑器，并原生支持 MCP。Zed 通过 `context_servers` 设置键暴露 MCP servers。有关通用设置（前置条件、Docker 安装、安全最佳实践），请参阅[安装指南 README](./README.md)。

## 前置条件

1. 已安装 Zed（建议使用最新版本；Zed v0.224.0+ 推荐用于新版 `agent.tool_permissions` 设置结构）
2. 具备适当 scopes 的 [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new)
3. 本地安装：已安装并运行 [Docker](https://www.docker.com/)

## 安装方式

在 Zed 中安装 GitHub MCP server 有两种方式：

- **方案 A：Zed Extension（最简单）**：Zed extension gallery 中提供了社区维护的 [GitHub MCP extension](https://zed.dev/extensions/mcp-server-github)。可从 Agent Panel 右上角菜单选择 “View Server Extensions” 安装，也可通过 command palette 运行 `zed: extensions` 操作。安装后，Zed 会弹出 modal，要求输入 GitHub Personal Access Token。
- **方案 B：Custom Server（推荐用于官方远程 endpoint）**：手动将配置添加到 `settings.json`，直接使用 GitHub 托管的远程 server 或官方 Docker 镜像。本文其余部分介绍方案 B。

## 远程 Server（推荐）

使用 GitHub 托管在 `https://api.githubcopilot.com/mcp/` 的 server。打开 Zed [settings file](https://zed.dev/docs/configuring-zed.html#settings-files)（Command Palette -> `zed: open settings`），并在 `context_servers` 下添加以下配置。

```json
{
  "context_servers": {
    "github": {
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer YOUR_GITHUB_PAT"
      }
    }
  }
}
```

将 `YOUR_GITHUB_PAT` 替换为你的 [GitHub Personal Access Token](https://github.com/settings/tokens)。如需自定义 toolsets，请向 `headers` 对象添加 `X-MCP-Toolsets` 或 `X-MCP-Readonly` 等 server-side headers，详见 [Server Configuration Guide](../server-configuration.md)。

> [!NOTE]
> 如果省略 `Authorization` header，Zed 会在首次使用时尝试标准 MCP OAuth 流程。GitHub MCP server 当前不会为非 Copilot hosts 公告 OAuth，因此在 `Authorization` header 中提供 Personal Access Token 是受支持的路径。

## 本地 Server（Docker）

本地 GitHub MCP server 通过 Docker 运行，需要已安装并运行 Docker Desktop（或其他 Docker runtime）。

使用 OAuth 登录而不是 token。在 github.com 上，官方镜像已经包含 app credentials，因此无需自行提供；server 会在首次使用时打开浏览器登录，并且只在内存中保留 token。在 Docker 中，需要将固定 callback port 发布到 loopback：

```json
{
  "context_servers": {
    "github": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-p", "127.0.0.1:8085:8085",
        "-e", "GITHUB_OAUTH_CALLBACK_PORT",
        "ghcr.io/github/github-mcp-server"
      ],
      "env": {
        "GITHUB_OAUTH_CALLBACK_PORT": "8085"
      }
    }
  }
}
```

有关 native-binary 流程（无固定端口）、headless/device-code fallback、GitHub Enterprise，以及自带 OAuth 或 GitHub App 的设置，请参阅 **[Local Server OAuth Login](../oauth-login.md)**。

如果改用 Personal Access Token 进行身份验证（其优先级高于 OAuth）：

```json
{
  "context_servers": {
    "github": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "GITHUB_PERSONAL_ACCESS_TOKEN",
        "ghcr.io/github/github-mcp-server"
      ],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "YOUR_GITHUB_PAT"
      }
    }
  }
}
```

> [!IMPORTANT]
> Zed 要求 `command` 是一个**字符串**，并使用单独的 `args` 数组；不能把二者合并成一个数组。这与 OpenCode 和 Claude Desktop 等 hosts 不同。

## 验证安装

1. 打开 Agent Panel，并进入其 Settings 视图（或运行 `agent: open settings`）。
2. 在 context servers 列表中找到 `github`。带有 “Server is active” tooltip 的绿色指示点表示配置可用。其他颜色和 tooltip 消息表示启动或身份验证错误。
3. 尝试一个会调用工具的 prompt，例如 `List my recent GitHub pull requests`。除非 `agent.tool_permissions.default` 设置为 `"allow"`，否则 Zed 会在第一次调用前请求工具批准。

## 工具权限（可选）

Zed v0.224.0+ 通过 `agent.tool_permissions` 控制工具批准。使用 `mcp:<server>:<tool_name>` key 格式，可以批准特定 GitHub MCP 工具，避免每次调用都弹出确认：

```json
{
  "agent": {
    "tool_permissions": {
      "default": "confirm",
      "rules": [
        { "tool": "mcp:github:list_pull_requests", "permission": "allow" },
        { "tool": "mcp:github:list_issues", "permission": "allow" }
      ]
    }
  }
}
```

完整 schema 请参阅 [Zed tool permissions docs](https://zed.dev/docs/ai/tool-permissions.html)。

## 故障排除

- **Server 指示器保持红色 / “Server is not running”**：检查 Agent Panel 的 settings 视图中对应 server 的错误字符串。最常见原因是 `settings.json` 中存在无效 JSON；Zed 会在编辑器中直接显示 JSON 解析错误。
- **`401 Unauthorized`**：确认 PAT 未过期，并包含你要调用的工具所需 scopes。远程 endpoint 会拒绝没有 `Authorization` header 的请求（不允许匿名访问）。
- **prompt 中缺少工具**：确认正在使用的 Agent profile 未禁用该 server。如果你使用 [custom profile](https://zed.dev/docs/ai/agent-panel.html#custom-profiles)，请确保 `enable_all_context_servers` 为 `true`，或已显式列出 `github`。
- **本地 server 的 Docker 错误**：确保 Docker Desktop 正在运行，并且至少拉取过一次 `ghcr.io/github/github-mcp-server` 镜像。可在终端中尝试 `docker pull ghcr.io/github/github-mcp-server`。

## 重要说明

- **配置键**：Zed 使用 `context_servers`（不是 `mcpServers`）。
- **命令结构**：`command` 是字符串，`args` 是单独的数组。
- **OAuth**：省略 `Authorization` 会触发 Zed 的 MCP OAuth 流程，但 GitHub MCP server 当前支持的路径是基于 PAT 的身份验证。
- **External agents**：配置在 `context_servers` 中的 MCP servers 会通过 Agent Client Protocol 转发给 [external agents](https://zed.dev/docs/ai/external-agents.html)。
