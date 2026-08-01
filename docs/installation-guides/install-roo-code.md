# 在 Roo Code 中安装 GitHub MCP Server

[Roo Code](https://github.com/RooCodeInc/Roo-Code) 是在兼容 VS Code 的编辑器（VS Code、Cursor、Windsurf 等）中运行的 AI 编码助手。有关通用设置信息（前提条件、Docker 安装和安全最佳实践），请参阅[安装指南 README](./README.md)。

## 远程 Server

### 分步设置

1. 单击编辑器侧边栏中的 **Roo Code 图标**，打开 Roo Code 面板
2. 单击 Roo Code 面板顶部导航中的**齿轮图标**（⚙️），再单击左侧的 **"MCP Servers"** 图标。
3. 滚动到底部，单击 **"Edit Global MCP"**（所有项目）或 **"Edit Project MCP"**（仅当前项目）
4. 将下方配置添加到打开的文件（`mcp_settings.json` 或 `.roo/mcp.json`）
5. 将 `YOUR_GITHUB_PAT` 替换为你的 [GitHub Personal Access Token](https://github.com/settings/tokens)
6. 保存文件，server 应自动连接

```json
{
  "mcpServers": {
    "github": {
      "type": "streamable-http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer YOUR_GITHUB_PAT"
      }
    }
  }
}
```

> **重要提示：** `type` 必须为 `"streamable-http"`（带连字符）。使用 `"http"` 或省略该类型会失败。

要自定义 toolsets，请在 `headers` 对象中添加 `X-MCP-Toolsets` 或 `X-MCP-Readonly` 等服务端标头。请参阅 [Server Configuration Guide](../server-configuration.md)。

## 本地 Server（Docker）

使用 OAuth 登录而非 token。github.com 上的官方镜像已包含 app 凭据，因此无需自行提供；server 会在首次使用时打开浏览器登录，并且仅在内存中保留 token。在 Docker 中，请将固定回调端口发布到 loopback：

```json
{
  "mcpServers": {
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

有关原生二进制文件流程（无固定端口）、无头/device-code 回退、GitHub Enterprise，以及自带 OAuth 或 GitHub App，请参阅 **[Local Server OAuth Login](../oauth-login.md)**。

若改用 Personal Access Token 进行身份验证（替换 `YOUR_GITHUB_PAT`；其优先级高于 OAuth）：

```json
{
  "mcpServers": {
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

## 故障排除

- **连接失败**：确保 `type` 是 `streamable-http`，而不是 `http`
- **身份验证失败**：确认 `Authorization` 标头中的 PAT 带有 `Bearer ` 前缀
- **Docker 问题**：确保 Docker Desktop 正在运行
