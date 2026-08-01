# 在 Rovo Dev CLI 中安装 GitHub MCP Server

## 前提条件

1. 已安装 Rovo Dev CLI（最新版本）
2. 具有适当作用域的 [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new)

## MCP Server 设置

使用 GitHub 托管的服务器：https://api.githubcopilot.com/mcp/。

### 安装步骤

1. 运行 `acli rovodev mcp`，打开 Rovo Dev CLI 的 MCP 配置
2. 按照下方示例添加配置。
3. 将 `YOUR_GITHUB_PAT` 替换为实际的 [GitHub Personal Access Token](https://github.com/settings/tokens)
4. 保存文件，并通过 `acli rovodev` 重启 Rovo Dev CLI

### 配置示例

```json
{
  "mcpServers": {
    "github": {
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer YOUR_GITHUB_PAT"
      }
    }
  }
}
```
