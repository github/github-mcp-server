# 在 Windsurf 中安装 GitHub MCP Server

## 前提条件
1. 已安装 Windsurf IDE（最新版本）
2. 具有适当作用域的 [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new)
3. 用于本地安装：已安装并运行 [Docker](https://www.docker.com/)

## 远程 Server 设置（推荐）

远程 GitHub MCP server 由 GitHub 托管，地址为 `https://api.githubcopilot.com/mcp/`，支持 Streamable HTTP 协议。Windsurf 目前仅支持 PAT 身份验证。

### Streamable HTTP 配置
Windsurf 通过 `serverUrl` 字段支持 Streamable HTTP server：

```json
{
  "mcpServers": {
    "github": {
      "serverUrl": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer YOUR_GITHUB_PAT"
      }
    }
  }
}
```

## 本地 Server 设置

### Docker 安装（必需）
**重要提示**：npm 包 `@modelcontextprotocol/server-github` 自 2025 年 4 月起不再受支持。请改用官方 Docker 镜像 `ghcr.io/github/github-mcp-server`。

使用 OAuth 登录而非 token。github.com 上的官方镜像已包含 app 凭据，因此无需自行提供；server 会在首次使用时打开浏览器登录，并且仅在内存中保留 token。在 Docker 中，请将固定回调端口发布到 loopback：

```json
{
  "mcpServers": {
    "github": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "-p",
        "127.0.0.1:8085:8085",
        "-e",
        "GITHUB_OAUTH_CALLBACK_PORT",
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

若改用 Personal Access Token 进行身份验证（其优先级高于 OAuth）：

```json
{
  "mcpServers": {
    "github": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "-e",
        "GITHUB_PERSONAL_ACCESS_TOKEN",
        "ghcr.io/github/github-mcp-server"
      ],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "YOUR_GITHUB_PAT"
      }
    }
  }
}
```

## 安装步骤

### 通过 Plugin Store
1. 打开 Windsurf 并进入 Cascade
2. 单击 **Plugins** 图标或**锤子图标**（🔨）
3. 搜索 "GitHub MCP Server"
4. 单击 **Install**，并在提示时输入 PAT
5. 单击 **Refresh**（🔄）

### 手动配置
1. 单击 Cascade 中的锤子图标（🔨）
2. 单击 **Configure** 打开 `~/.codeium/windsurf/mcp_config.json`
3. 添加上方选定的配置
4. 保存文件
5. 单击 MCP 工具栏中的 **Refresh**（🔄）

## 配置详情

- **文件路径**：`~/.codeium/windsurf/mcp_config.json`
- **范围**：仅全局配置（不支持按项目配置）
- **格式**：必须为有效 JSON（使用 linter 验证）

## 验证

安装后：
1. 在 MCP 工具栏中查找 "1 available MCP server"
2. 单击锤子图标查看可用 GitHub 工具
3. 使用 "List my GitHub repositories" 测试
4. 检查 server 名称旁是否有绿色圆点

## 故障排除

### 远程 Server 问题
- **身份验证失败**：确认 PAT 具有正确作用域且未过期
- **连接错误**：检查 HTTPS 连接的防火墙/代理设置
- **Streamable HTTP 无法工作**：确保使用正确的 `serverUrl` 字段格式

### 本地 Server 问题
- **Docker 错误**：确保 Docker Desktop 正在运行
- **镜像拉取失败**：尝试 `docker logout ghcr.io` 后重试
- **找不到 Docker**：安装 Docker Desktop 并确保其正在运行

### 常规问题
- **无效 JSON**：使用 [jsonlint.com](https://jsonlint.com) 验证
- **工具未显示**：完全重启 Windsurf
- **检查日志**：`~/.codeium/windsurf/logs/`

## 重要说明

- **官方仓库**：[github/github-mcp-server](https://github.com/github/github-mcp-server)
- **远程 server URL**：`https://api.githubcopilot.com/mcp/`
- **Docker 镜像**：`ghcr.io/github/github-mcp-server`（官方且受支持）
- **npm 包**：`@modelcontextprotocol/server-github`（自 2025 年 4 月起已废弃，无法再使用）
- **Windsurf 限制**：不支持环境变量插值，仅支持全局配置
