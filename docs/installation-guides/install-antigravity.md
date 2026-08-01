# 在 Antigravity 中安装 GitHub MCP Server

本指南介绍如何在 Google 的 Antigravity IDE 中设置 GitHub MCP Server。

## 前提条件

- 已安装 Antigravity IDE（最新版本）
- 具有适当作用域的 GitHub Personal Access Token

## 安装方式

### 选项 1：远程 Server（推荐）

使用 GitHub 托管的 server：`https://api.githubcopilot.com/mcp/`。

> [!NOTE]
> 推荐使用此手动配置方式，因为通过 Antigravity MCP Store 的“官方”安装目前存在已知问题（通常会导致 Docker 错误）。此直接远程连接更可靠。

#### 步骤 1：访问 MCP 配置

1. 打开 Antigravity
2. 单击 Agent 面板中的 "..."（Additional Options）菜单
3. 选择 "MCP Servers"
4. 单击 "Manage MCP Servers"
5. 单击 "View raw config"

这会在以下位置打开 `mcp_config.json` 文件：
- **Windows**: `C:\Users\<USERNAME>\.gemini\antigravity\mcp_config.json`
- **macOS/Linux**: `~/.gemini/antigravity/mcp_config.json`

#### 步骤 2：添加配置

将以下内容添加到 `mcp_config.json`：

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

**重要提示**：对于基于 HTTP 的 MCP server，Antigravity 使用 `serverUrl` 而非 `url`。

#### 步骤 3：配置 token

将 `YOUR_GITHUB_PAT` 替换为实际的 GitHub Personal Access Token。

在此创建 token：https://github.com/settings/tokens

推荐作用域：
- `repo`：完全控制私有仓库
- `read:org`：读取组织和团队成员资格
- `read:user`：读取用户个人资料数据

#### 步骤 4：重启 Antigravity

关闭并重新打开 Antigravity，使更改生效。

#### 步骤 5：验证安装

1. 打开 MCP Servers 面板（... 菜单 → MCP Servers）
2. 应能看到带有可用工具列表的 "github"
3. 现在可以在对话中使用 GitHub 工具

> [!NOTE]
> 在某些版本中，MCP Servers 面板中的状态指示器可能不会立即变绿，但配置正确时工具仍可使用。

### 选项 2：本地 Docker Server

若希望使用 Docker 在本地运行 server：

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

**要求**：
- 已安装并运行 Docker Desktop
- Docker 必须在系统 PATH 中

## Troubleshooting

### "Error: serverUrl or command must be specified"

Make sure you're using `serverUrl` (not `url`) for the remote server configuration. Antigravity requires `serverUrl` for HTTP-based MCP servers.

### Server not appearing in MCP list

- Verify JSON syntax in your config file
- Check that your PAT hasn't expired
- Restart Antigravity completely

### Tools not working

- Ensure your PAT has the correct scopes
- Check the MCP Servers panel for error messages
- Verify internet connection for remote server

## Available Tools

Once installed, you'll have access to tools like:
- `create_repository` - Create new GitHub repositories
- `push_files` - Push files to repositories
- `search_repositories` - Search for repositories
- `create_or_update_file` - Manage file content
- `get_file_contents` - Read file content
- And many more...

For a complete list of available tools and features, see the [main README](../../README.md).

## Differences from Other IDEs

- **Configuration key**: Antigravity uses `serverUrl` instead of `url` for HTTP servers
- **Config location**: `.gemini/antigravity/mcp_config.json` instead of `.cursor/mcp.json`
- **Tool limits**: Antigravity recommends keeping total enabled tools under 50 for optimal performance

## Next Steps

- Explore the [Server Configuration Guide](../server-configuration.md) for advanced options
- Check out [toolsets documentation](../../README.md#available-toolsets) to customize available tools
- See the [Remote Server Documentation](../remote-server.md) for more details
