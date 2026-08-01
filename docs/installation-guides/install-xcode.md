# 在 Xcode 中安装 GitHub MCP Server

Xcode 目前支持两个内置编码代理：**Codex**（由 OpenAI 提供支持）和 **Claude Agent**（由 Anthropic 提供支持）。请遵循各代理的标准安装指南；但有一项重要区别：Xcode 为每个代理使用独立的隔离配置目录，与全局配置分开。

> 放在这些目录中的配置仅在从 Xcode 启动代理时生效。详见 [Apple 文档](https://developer.apple.com/documentation/xcode/setting-up-coding-intelligence#Customize-the-Claude-Agent-and-Codex-environments)。

## 配置目录

| 代理 | 配置目录 |
|-------|------------------------|
| Codex | `~/Library/Developer/Xcode/CodingAssistant/codex/` |
| Claude Agent | `~/Library/Developer/Xcode/CodingAssistant/ClaudeAgentConfig/` |

请将 MCP server 配置放在上方相应的目录中，而不是独立 CLI 使用的默认位置。

## 设置指南

- **[Codex](install-codex.md)**：在 `~/Library/Developer/Xcode/CodingAssistant/codex/` 中配置 `config.toml`
- **[Claude Agent](install-claude.md#xcode-claude-agent)**：在 `~/Library/Developer/Xcode/CodingAssistant/ClaudeAgentConfig/` 中配置 `.claude.json`

## macOS 路径说明

Xcode 使用精简的 `PATH` 运行，通常不包含常见二进制文件位置。若使用本地 STDIO server（例如 Docker 或预构建二进制文件），请在配置中使用命令的**完整路径**。在 Terminal 中运行 `which docker`（或 `which github-mcp-server`）以查找系统上的正确路径。常见位置：

| 安装方式 | 常见路径 |
|---|---|
| Docker (Intel Mac) | `/usr/local/bin/docker` |
| Docker (Apple Silicon) | `/usr/local/bin/docker` |
| Homebrew (Intel Mac) | `/usr/local/bin/` |
| Homebrew (Apple Silicon) | `/opt/homebrew/bin/` |

> **使用 OAuth 登录？** 本地 server 无需 PAT 即可运行：首次使用时会打开浏览器登录，且仅在内存中保存 token。Docker 需要将固定回调端口发布到 loopback（使用 `GITHUB_OAUTH_CALLBACK_PORT=8085` 的 `-p 127.0.0.1:8085:8085 -e GITHUB_OAUTH_CALLBACK_PORT`）；原生二进制文件使用随机 loopback 端口，无需额外配置。请参阅 **[本地 Server OAuth 登录](../oauth-login.md)**。

## 故障排除

| 问题 | 可能原因 | 解决方法 |
|-------|----------------|-----|
| 工具未加载 | 配置位于错误目录 | 确保配置位于上方 Xcode 专用路径，而非 `~/.codex/` 或 `~/.claude.json` |
| 找不到命令（STDIO） | Xcode 的 PATH 未包含二进制文件位置 | 使用完整路径（如 `/usr/local/bin/docker` 或 `/opt/homebrew/bin/docker`）；在 Terminal 中运行 `which docker` 确认 |
| 找不到 Docker | Docker 未运行 | 启动 Docker Desktop 并重启 Xcode |
| 身份验证失败 | PAT 无效或已过期 | 重新生成 PAT 并更新配置 |

## 参考资料

- [Apple Developer 文档：设置编码智能](https://developer.apple.com/documentation/xcode/setting-up-coding-intelligence#Customize-the-Claude-Agent-and-Codex-environments)
- [Codex MCP documentation](https://developers.openai.com/codex/mcp)
- 主项目 README：[高级配置选项](../../README.md)
