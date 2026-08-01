# GitHub MCP Server 安装指南

本目录包含 GitHub MCP Server 在不同 host 应用和 IDE 中的详细安装说明。请选择与你的开发环境匹配的指南。

## 按 Host 应用划分的安装指南

- **[Copilot CLI](install-copilot-cli.md)** - GitHub Copilot CLI 安装指南
- **[其他 IDE 中的 GitHub Copilot](install-other-copilot-ides.md)** - JetBrains、Visual Studio、Eclipse 和 Xcode 中 GitHub Copilot 的安装说明
- **[Antigravity](install-antigravity.md)** - Google Antigravity IDE 安装指南
- **[Claude 应用](install-claude.md)** - Claude Desktop 和 Claude Code CLI 安装指南
- **[Cline](install-cline.md)** - Cline 安装指南
- **[Cursor](install-cursor.md)** - Cursor IDE 安装指南
- **[Google Gemini CLI](install-gemini-cli.md)** - Google Gemini CLI 安装指南
- **[OpenAI Codex](install-codex.md)** - OpenAI Codex 安装指南
- **[OpenCode](install-opencode.md)** - OpenCode 终端 agent 安装指南
- **[Roo Code](install-roo-code.md)** - Roo Code 安装指南
- **[Windsurf](install-windsurf.md)** - Windsurf IDE 安装指南
- **[Xcode（Codex 与 Claude Agent）](install-xcode.md)** - Xcode 内 Codex 和 Claude Agent 的安装指南
- **[Zed](install-zed.md)** - Zed 编辑器安装指南

## 按 Host 应用划分的支持情况

| Host 应用 | 本地 GitHub MCP 支持 | 远程 GitHub MCP 支持 | 前置条件 | 难度 |
|-----------------|---------------|----------------|---------------|------------|
| Copilot CLI | ✅ | ✅ PAT + ❌ 无 OAuth | Docker 或 Go 构建、GitHub PAT | 简单 |
| VS Code 中的 Copilot | ✅ | ✅ 完整支持（OAuth + PAT） | 本地：Docker 或 Go 构建、GitHub PAT<br>远程：VS Code 1.101+ | 简单 |
| Copilot Coding Agent | ✅ | ✅ 完整支持（默认开启，无需身份验证） | 任意 _付费_ Copilot license | 默认开启 |
| Visual Studio 中的 Copilot | ✅ | ✅ 完整支持（OAuth + PAT） | 本地：Docker 或 Go 构建、GitHub PAT<br>远程：Visual Studio 17.14+ | 简单 |
| JetBrains 中的 Copilot | ✅ | ✅ 完整支持（OAuth + PAT） | 本地：Docker 或 Go 构建、GitHub PAT<br>远程：JetBrains Copilot Extension v1.5.53+ | 简单 |
| Claude Code | ✅ | ✅ PAT + ❌ 无 OAuth | GitHub MCP Server binary 或远程 URL、GitHub PAT | 简单 |
| Claude Desktop | ✅ | ✅ PAT + ❌ 无 OAuth | Docker 或 Go 构建、GitHub PAT | 中等 |
| Cline | ✅ | ✅ PAT + ❌ 无 OAuth | Docker 或 Go 构建、GitHub PAT | 简单 |
| Cursor | ✅ | ✅ PAT + ❌ 无 OAuth | Docker 或 Go 构建、GitHub PAT | 简单 |
| Google Gemini CLI | ✅ | ✅ PAT + ❌ 无 OAuth | Docker 或 Go 构建、GitHub PAT | 简单 |
| OpenCode | ✅ | ✅ PAT + ❌ 无 OAuth | Docker 或 Go 构建、GitHub PAT | 简单 |
| Roo Code | ✅ | ✅ PAT + ❌ 无 OAuth | Docker 或 Go 构建、GitHub PAT | 简单 |
| Windsurf | ✅ | ✅ PAT + ❌ 无 OAuth | Docker 或 Go 构建、GitHub PAT | 简单 |
| Zed | ✅ | ✅ PAT + ❌ 无 OAuth | Docker 或 Go 构建、GitHub PAT | 简单 |
| Xcode 中的 Copilot | ✅ | ✅ 完整支持（OAuth + PAT） | 本地：Docker 或 Go 构建、GitHub PAT<br>远程：Copilot for Xcode 0.41.0+ | 简单 |
| Eclipse 中的 Copilot | ✅ | ✅ 完整支持（OAuth + PAT） | 本地：Docker 或 Go 构建、GitHub PAT<br>远程：Eclipse Plug-in for Copilot 0.10.0+ | 简单 |
| Xcode（Codex） | ✅ | ✅ PAT + ❌ 无 OAuth | 本地：Docker（需要完整路径）、GitHub PAT<br>远程：通过 `GITHUB_PAT_TOKEN` env var（`bearer_token_env_var`）提供 GitHub PAT | 简单 |
| Xcode（Claude Agent） | ✅ | ✅ PAT + ❌ 无 OAuth | 本地：Docker（需要完整路径）、GitHub PAT<br>远程：GitHub PAT | 简单 |

**图例：**
- ✅ = 完全支持
- ❌ = 尚不支持

**注意：**远程 MCP 支持要求 host 应用注册 GitHub App 或 OAuth app 来支持 OAuth 流程，即使该 host app 已支持新的 OAuth 规范。目前只有 VS Code 完整支持远程 GitHub server。

## 安装方法

GitHub MCP Server 可以通过多种方式安装。对大多数用户来说，**Docker 是最常见、最推荐的方式**；也可根据需要选择其他方案：

### Docker（最常见且推荐）

- **优点**：无需本地构建，环境一致，更新简单，跨平台可用
- **缺点**：需要安装并运行 Docker
- **适合**：大多数用户，尤其是已经使用 Docker 或希望设置最简单的用户
- **使用场景**：Claude Desktop、VS Code 中的 Copilot、Cursor、Windsurf 等

### 预构建 Binary（轻量替代方案）

- **优点**：无需 Docker，可通过 stdio 直接执行，设置最少
- **缺点**：需要手动下载和管理更新，binary 按平台区分
- **适合**：最小化环境、偏好不使用 Docker 的用户
- **使用场景**：Claude Code CLI、轻量设置

### 从源码构建（高级用户）

- **优点**：最新功能、完全可定制、无外部运行依赖
- **缺点**：需要 Go 开发环境，设置更复杂
- **前置条件**：[Go 1.24+](https://go.dev/doc/install)
- **构建命令**：`go build -o github-mcp-server cmd/github-mcp-server/main.go`
- **适合**：希望使用最新功能或需要自定义修改的开发者

### 关于 GitHub MCP Server 的重要说明

- **Docker 镜像**：官方 Docker 镜像现在是 `ghcr.io/github/github-mcp-server`
- **npm 包**：`@modelcontextprotocol/server-github` npm 包自 2025 年 4 月起不再受支持
- **远程 Server**：远程 server URL 为 `https://api.githubcopilot.com/mcp/`

## 通用前置条件

所有使用 Personal Access Token（PAT）的安装都需要：

- **GitHub Personal Access Token（PAT）**：[在此创建](https://github.com/settings/personal-access-tokens/new)

可选项（取决于安装方式）：

- **Docker**（用于基于 Docker 的安装）：[下载 Docker](https://www.docker.com/)
- **Go 1.24+**（用于从源码构建）：[安装 Go](https://go.dev/doc/install)

## 安全最佳实践

无论选择哪种安装方式，都请遵循以下安全准则：

1. **安全存储 Token**：不要将 GitHub PAT 提交到版本控制
2. **限制 Token Scope**：只授予 GitHub PAT 必需权限
3. **文件权限**：限制对包含 token 的配置文件的访问
4. **定期轮换**：定期轮换 GitHub Personal Access Tokens
5. **环境变量**：host 支持时优先使用环境变量

## 获取帮助

如果遇到问题：

1. 查看对应安装指南中的故障排除章节
2. 确认 GitHub PAT 具备所需权限
3. 确保 Docker 正在运行（本地安装）
4. 查看 host 应用日志中的错误信息
5. 查阅主 [README.md](../../README.md) 了解更多配置选项

## 配置选项

安装后，你可能还需要了解：

- **Toolsets**：启用或禁用特定 GitHub API 能力
- **Read-Only Mode**：限制为只读操作
- **Lockdown Mode**：隐藏无 push 权限用户创建的公开 issue 详情
