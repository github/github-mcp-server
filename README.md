[![Go Report Card](https://goreportcard.com/badge/github.com/github/github-mcp-server)](https://goreportcard.com/report/github.com/github/github-mcp-server)

# GitHub MCP Server

GitHub MCP Server 将 AI 工具直接连接到 GitHub 平台，使 AI agents、助手和聊天机器人能够读取仓库与代码文件、管理 issues 和 PR、分析代码以及自动化工作流，全部通过自然语言交互完成。

### 使用场景

- 仓库管理：浏览和查询代码、搜索文件、分析 commits，并了解您有访问权限的任意仓库中的项目结构。
- Issue 与 PR 自动化：创建、更新和管理 issues 与 pull requests。让 AI 协助分类 bugs、审查代码变更和维护项目看板。
- CI/CD 与工作流洞察：监控 GitHub Actions 工作流运行、分析构建失败、管理 releases，并洞察开发 pipeline。
- 代码分析：检查安全发现、审查 Dependabot alerts、了解代码模式，并获得代码库的全面洞察。
- 团队协作：访问 discussions、管理 notifications、分析团队活动，并简化团队流程。

面向希望将 AI 工具接入 GitHub 上下文和能力的开发者，支持从简单的自然语言查询到复杂的多步骤 agent 工作流。

---

## Remote GitHub MCP Server

[![Install in VS Code](https://img.shields.io/badge/VS_Code-Install_Server-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2F%22%7D) [![Install in VS Code Insiders](https://img.shields.io/badge/VS_Code_Insiders-Install_Server-24bfa5?style=flat-square&logo=visualstudiocode&logoColor=white)](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2F%22%7D&quality=insiders) [![Install in Visual Studio](https://img.shields.io/badge/Visual_Studio-Install_Server-C16FDE?style=flat-square&logo=visualstudio&logoColor=white)](https://aka.ms/vs/mcp-install?%7B%22name%22%3A%22github%22%2C%22gallery%22%3Atrue%2C%22url%22%3A%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2F%22%7D)

远程 GitHub MCP Server 由 GitHub 托管，是最简便的上手方式。如果您的 MCP host 不支持远程 MCP servers，也无需担心，可以改用 [GitHub MCP Server 的本地版本](https://github.com/github/github-mcp-server?tab=readme-ov-file#local-github-mcp-server)。

### 前提条件

1. 支持远程 server 的兼容 MCP host（VS Code 1.101+、Claude Desktop、Cursor、Windsurf 等）
2. 启用所有适用的[策略](https://github.com/github/github-mcp-server/blob/main/docs/policies-and-governance.md)

### 在 VS Code 中安装

如需快速安装，请使用上方任一一键安装按钮。完成流程后，切换 Agent mode（位于 Copilot Chat 文本输入框附近），server 即会启动。请确保使用 [VS Code 1.101](https://code.visualstudio.com/updates/v1_101) 或[更高版本](https://code.visualstudio.com/updates)，以支持远程 MCP 和 OAuth。

或者，若要手动配置 VS Code，请从下方示例中选择相应 JSON 块并添加到 host 配置中：

<table>
<tr><th>使用 OAuth</th><th>使用 GitHub PAT</th></tr>
<tr><th align=left colspan=2>VS Code（版本 1.101 或更高）</th></tr>
<tr valign=top>
<td>

```json
{
  "servers": {
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/"
    }
  }
}
```

</td>
<td>

```json
{
  "servers": {
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer ${input:github_mcp_pat}"
      }
    }
  },
  "inputs": [
    {
      "type": "promptString",
      "id": "github_mcp_pat",
      "description": "GitHub Personal Access Token",
      "password": true
    }
  ]
}
```

</td>
</tr>
</table>

### 在其他 MCP hosts 中安装

- **[Copilot CLI](/docs/installation-guides/install-copilot-cli.md)** - GitHub Copilot CLI 安装指南
- **[GitHub Copilot in other IDEs](/docs/installation-guides/install-other-copilot-ides.md)** - 在 JetBrains、Visual Studio、Eclipse 和 Xcode 中安装 GitHub Copilot
- **[Claude Applications](/docs/installation-guides/install-claude.md)** - Claude Desktop 和 Claude Code CLI 安装指南
- **[Codex](/docs/installation-guides/install-codex.md)** - OpenAI Codex 安装指南
- **[Cursor](/docs/installation-guides/install-cursor.md)** - Cursor IDE 安装指南
- **[OpenCode](/docs/installation-guides/install-opencode.md)** - OpenCode terminal agent 安装指南
- **[Windsurf](/docs/installation-guides/install-windsurf.md)** - Windsurf IDE 安装指南
- **[Zed](/docs/installation-guides/install-zed.md)** - Zed editor 安装指南
- **[Rovo Dev CLI](/docs/installation-guides/install-rovo-dev-cli.md)** - Rovo Dev CLI 安装指南

> **注意：**每个 MCP host application 都需要配置 GitHub App 或 OAuth App，才能通过 OAuth 支持远程访问。支持远程 MCP servers 的任何 host application 都应能通过 PAT authentication 支持远程 GitHub server。配置细节和支持级别因 host 而异，请参阅 host application 的文档了解更多信息。

### 配置

#### Toolset 配置

有关远程 server 配置、toolsets、headers 和高级用法的完整详情，请参阅 [Remote Server Documentation](docs/remote-server.md)。该文件提供了在 VS Code 和其他 MCP hosts 中连接、自定义及安装远程 GitHub MCP Server 的完整说明与示例。

未指定 toolsets 时，将使用[默认 toolsets](#default-toolset)。

#### Insiders Mode

> **抢先试用新功能！**远程 server 提供 insiders 版本，可提前使用新功能和实验性工具。

<table>
<tr><th>使用 URL Path</th><th>使用 Header</th></tr>
<tr valign=top>
<td>

```json
{
  "servers": {
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/insiders"
    }
  }
}
```

</td>
<td>

```json
{
  "servers": {
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "X-MCP-Insiders": "true"
      }
    }
  }
}
```

</td>
</tr>
</table>

更多详情和示例请参阅 [Remote Server Documentation](docs/remote-server.md#insiders-mode)；可用功能完整列表请参阅 [Insiders Features](docs/insiders-features.md)。

#### GitHub Enterprise

##### GitHub Enterprise Cloud with data residency (ghe.com)

GitHub Enterprise Cloud 也可以使用远程 server。

使用 GitHub PAT token 的 `https://octocorp.ghe.com` 示例：

```
{
    ...
    "github-octocorp": {
      "type": "http",
      "url": "https://copilot-api.octocorp.ghe.com/mcp",
      "headers": {
        "Authorization": "Bearer ${input:github_mcp_pat}"
      }
    },
    ...
}
```

> **注意：**在 VS Code 和 GitHub Copilot 中配合 GitHub Enterprise 使用 OAuth 时，还需要配置 VS Code settings 以指向 GitHub Enterprise 实例。请参阅 [Authenticate from VS Code](https://docs.github.com/en/enterprise-cloud@latest/copilot/how-tos/configure-personal-settings/authenticate-to-ghecom)。

##### GitHub Enterprise Server

GitHub Enterprise Server 不支持远程 server 托管。请参阅本地 server 配置中的 [GitHub Enterprise Server and Enterprise Cloud with data residency (ghe.com)](#github-enterprise-server-and-enterprise-cloud-with-data-residency-ghecom)。

---

## Local GitHub MCP Server

[![Install with Docker in VS Code](https://img.shields.io/badge/VS_Code-Install_Server-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%7B%22command%22%3A%22docker%22%2C%22args%22%3A%5B%22run%22%2C%22-i%22%2C%22--rm%22%2C%22-p%22%2C%22127.0.0.1%3A8085%3A8085%22%2C%22-e%22%2C%22GITHUB_OAUTH_CALLBACK_PORT%22%2C%22ghcr.io%2Fgithub%2Fgithub-mcp-server%22%5D%2C%22env%22%3A%7B%22GITHUB_OAUTH_CALLBACK_PORT%22%3A%228085%22%7D%7D) [![Install with Docker in VS Code Insiders](https://img.shields.io/badge/VS_Code_Insiders-Install_Server-24bfa5?style=flat-square&logo=visualstudiocode&logoColor=white)](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%7B%22command%22%3A%22docker%22%2C%22args%22%3A%5B%22run%22%2C%22-i%22%2C%22--rm%22%2C%22-p%22%2C%22127.0.0.1%3A8085%3A8085%22%2C%22-e%22%2C%22GITHUB_OAUTH_CALLBACK_PORT%22%2C%22ghcr.io%2Fgithub%2Fgithub-mcp-server%22%5D%2C%22env%22%3A%7B%22GITHUB_OAUTH_CALLBACK_PORT%22%3A%228085%22%7D%7D&quality=insiders) [![Install with Docker in Visual Studio](https://img.shields.io/badge/Visual_Studio-Install_Server-C16FDE?style=flat-square&logo=visualstudio&logoColor=white)](https://aka.ms/vs/mcp-install?%7B%22name%22%3A%22github%22%2C%22command%22%3A%22docker%22%2C%22args%22%3A%5B%22run%22%2C%22-i%22%2C%22--rm%22%2C%22-p%22%2C%22127.0.0.1%3A8085%3A8085%22%2C%22-e%22%2C%22GITHUB_OAUTH_CALLBACK_PORT%3D8085%22%2C%22ghcr.io%2Fgithub%2Fgithub-mcp-server%22%5D%7D)

### 前提条件

1. 若要在 container 中运行 server，需要安装 [Docker](https://www.docker.com/)。
2. 安装 Docker 后，还需要确保 Docker 正在运行。Docker image 位于 `ghcr.io/github/github-mcp-server`，且为公开 image；若拉取时出错，可能是 token 已过期，需要执行 `docker logout ghcr.io`。
3. **Authentication。**在 github.com 上无需预先创建任何内容：上方一键按钮会在首次使用时通过 OAuth 登录（browser-based flow；token 仅保存在 memory 中）。Docker 按钮会发布固定 callback port（`127.0.0.1:8085`），使 container 的登录 callback 可访问。其工作方式、headless/device-code fallback 以及自带 OAuth 或 GitHub App（GitHub Enterprise Server 和 `ghe.com` 必需）的说明，请参阅 **[Local Server OAuth Login](docs/oauth-login.md)**。

   希望使用 token？仍可改为设置 `GITHUB_PERSONAL_ACCESS_TOKEN`，通过 [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) authentication（它优先于 OAuth）。MCP server 可以使用多项 GitHub APIs，因此请仅启用愿意授予 AI 工具的权限（有关 access tokens 的更多信息，请参阅[文档](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)）。

<details><summary><b>安全处理 PAT</b></summary>

### Environment Variables（推荐）

为使 GitHub PAT 保持安全并能在不同 MCP hosts 中复用：

1. **将 PAT 存储在 environment variables 中**

   ```bash
   export GITHUB_PAT=your_token_here
   ```

   或创建 `.env` 文件：

   ```env
   GITHUB_PAT=your_token_here
   ```

2. **保护 `.env` 文件**

   ```bash
   # Add to .gitignore to prevent accidental commits
   echo ".env" >> .gitignore
   ```

3. **在配置中引用 token**

   ```bash
   # CLI usage
   claude mcp add github -e GITHUB_PERSONAL_ACCESS_TOKEN=$GITHUB_PAT -- docker run -i --rm -e GITHUB_PERSONAL_ACCESS_TOKEN ghcr.io/github/github-mcp-server

   # In config files (where supported)
   "env": {
     "GITHUB_PERSONAL_ACCESS_TOKEN": "$GITHUB_PAT"
   }
   ```

> **注意：**Environment variable 支持情况因 host app 和 IDE 而异。某些 applications（如 Windsurf）要求在 config files 中硬编码 token。

### Token 安全最佳实践

- **最小 scopes**：仅授予必要权限
  - `repo` - 仓库操作
  - `read:packages` - Docker image 访问权限
  - `read:org` - organization team 访问权限
- **独立 tokens**：为不同 projects/environments 使用不同 PATs
- **定期轮换**：定期更新 tokens
- **切勿 commit**：不要将 tokens 纳入 version control
- **文件权限**：限制对包含 tokens 的 config files 的访问

  ```bash
  chmod 600 ~/.your-app/config.json
  ```

</details>

### GitHub Enterprise Server and Enterprise Cloud with data residency (ghe.com)

可使用 flag `--gh-host` 和 environment variable `GITHUB_HOST` 设置 GitHub Enterprise Server 或具有 data residency 的 GitHub Enterprise Cloud 的 hostname。

- 对 GitHub Enterprise Server，请为 hostname 添加 `https://` URI scheme 前缀；否则默认使用 GitHub Enterprise Server 不支持的 `http://`。
- 对具有 data residency 的 GitHub Enterprise Cloud，请使用 `https://YOURSUBDOMAIN.ghe.com` 作为 hostname。

``` json
"github": {
    "command": "docker",
    "args": [
    "run",
    "-i",
    "--rm",
    "-e",
    "GITHUB_PERSONAL_ACCESS_TOKEN",
    "-e",
    "GITHUB_HOST",
    "ghcr.io/github/github-mcp-server"
    ],
    "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}",
        "GITHUB_HOST": "https://<your GHES or ghe.com domain name>"
    }
}
```

## 安装

### 在 VS Code 的 GitHub Copilot 中安装

如需快速安装，请使用上方任一一键安装按钮。完成流程后，切换 Agent mode（位于 Copilot Chat 文本输入框附近），server 即会启动。

有关在 VS Code 中使用 MCP server tools 的更多信息，请参阅 [agent mode documentation](https://code.visualstudio.com/docs/copilot/chat/mcp-servers)。

在其他 IDEs（JetBrains、Visual Studio、Eclipse 等）的 GitHub Copilot 中安装

将下列任一 JSON 块添加到 IDE 的 MCP settings 中。

**使用 OAuth 登录（无需创建或存储 token）。**github.com 的官方 image 已包含 app credentials，因此您无需自行提供：它会在首次使用时执行 browser-based login，并将得到的 token **仅保存在 memory 中**。在 Docker 中，需要将固定 callback port 发布到 loopback，以便 container 的登录 callback 可访问：

```json
{
  "mcp": {
    "servers": {
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
}
```

native-binary flow（无需固定 port）、headless/device-code fallback、GitHub Enterprise Server / `ghe.com` 以及自带 OAuth 或 GitHub App 的说明，请参阅 **[Local Server OAuth Login](docs/oauth-login.md)**。

对于非交互式 stdio deployments，请参阅 **[GitHub App Authentication](docs/github-app-auth.md)**。

**或者使用 Personal Access Token authentication。**改为设置 `GITHUB_PERSONAL_ACCESS_TOKEN`（它优先于 OAuth）：

```json
{
  "mcp": {
    "inputs": [
      {
        "type": "promptString",
        "id": "github_token",
        "description": "GitHub Personal Access Token",
        "password": true
      }
    ],
    "servers": {
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
          "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
        }
      }
    }
  }
}
```

可选地，您可以将类似示例（即不含 mcp key）添加到 workspace 中名为 `.vscode/mcp.json` 的文件。这使您能与接受相同格式的其他 host applications 共享配置。

<details>
<summary><b>不含 MCP key 的 JSON 块示例</b></summary>
<br>

```json
{
  "inputs": [
    {
      "type": "promptString",
      "id": "github_token",
      "description": "GitHub Personal Access Token",
      "password": true
    }
  ],
  "servers": {
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
        "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
      }
    }
  }
}
```

</details>

### 在其他 MCP Hosts 中安装

对于其他 MCP host applications，请参阅安装指南：

- **[Copilot CLI](docs/installation-guides/install-copilot-cli.md)** - GitHub Copilot CLI 安装指南
- **[GitHub Copilot in other IDEs](/docs/installation-guides/install-other-copilot-ides.md)** - 在 JetBrains、Visual Studio、Eclipse 和 Xcode 中安装 GitHub Copilot
- **[Claude Code & Claude Desktop](docs/installation-guides/install-claude.md)** - Claude Code 和 Claude Desktop 安装指南
- **[Cursor](docs/installation-guides/install-cursor.md)** - Cursor IDE 安装指南
- **[Google Gemini CLI](docs/installation-guides/install-gemini-cli.md)** - Google Gemini CLI 安装指南
- **[OpenCode](docs/installation-guides/install-opencode.md)** - OpenCode terminal agent 安装指南
- **[Windsurf](docs/installation-guides/install-windsurf.md)** - Windsurf IDE 安装指南
- **[Zed](docs/installation-guides/install-zed.md)** - Zed editor 安装指南

所有安装选项的完整概览，请参阅 **[Installation Guides Index](docs/installation-guides)**。

> **注意：**支持本地 MCP servers 的任何 host application 都应能访问本地 GitHub MCP server。但是，具体的配置流程、syntax 和 integration 稳定性因 host application 而异。许多 host application 可能遵循与上述示例类似的格式，但并不保证如此。请参阅 host application 的文档以获取正确的 MCP configuration syntax 和设置流程。

### 从 source 构建

如果没有 Docker，可以使用 `go build` 构建 `cmd/github-mcp-server` directory 中的 binary，并使用设置为 token 的 `GITHUB_PERSONAL_ACCESS_TOKEN` environment variable 运行 `github-mcp-server stdio` command。若要指定 build output 位置，请使用 `-o` flag。应将 server 配置为使用构建出的 executable 作为其 `command`。例如：

```JSON
{
  "mcp": {
    "servers": {
      "github": {
        "command": "/path/to/github-mcp-server",
        "args": ["stdio"],
        "env": {
          "GITHUB_PERSONAL_ACCESS_TOKEN": "<YOUR_TOKEN>"
        }
      }
    }
  }
}
```

### CLI utilities

`github-mcp-server` binary 包含一些 CLI subcommands，可用于 debugging 和探索 server。

- `github-mcp-server tool-search "<query>"` 会按名称、description 和 input parameter names 搜索 tools。使用 `--max-results` 返回更多匹配项。
示例（color output 需要 TTY；在 Docker 中运行时使用 `docker run -t` 或 `-it`）：
```bash
docker run -it --rm ghcr.io/github/github-mcp-server tool-search "issue" --max-results 5
github-mcp-server tool-search "issue" --max-results 5
```

## Tool 配置

GitHub MCP Server 支持通过 `--toolsets` flag 启用或禁用特定功能组。这让您可以控制哪些 GitHub API capabilities 可供 AI tools 使用。仅启用所需 toolsets 有助于 LLM 选择 tools 并缩小 context size。

_Toolsets 不仅限于 Tools；适用时也会包含相关的 MCP Resources 和 Prompts。_

未指定 toolsets 时，将使用[默认 toolsets](#default-toolset)。

> **需要示例？**常见方案（如最小配置、read-only mode 以及组合 tools 和 toolsets）请参阅 [Server Configuration Guide](./docs/server-configuration.md)。

#### 指定 Toolsets

若要指定可供 LLM 使用的 toolsets，可通过以下两种方式传入 allow-list：

1. **使用 Command Line Argument：**

   ```bash
   github-mcp-server --toolsets repos,issues,pull_requests,actions,code_security
   ```

2. **使用 Environment Variable：**

   ```bash
   GITHUB_TOOLSETS="repos,issues,pull_requests,actions,code_security" ./github-mcp-server
   ```

如果同时提供，environment variable `GITHUB_TOOLSETS` 的优先级高于 command line argument。

#### 指定单个 Tools

还可以使用 `--tools` flag 配置特定 tools。Tools 可独立使用，也可与 toolsets 组合以进行精细控制。

1. **使用 Command Line Argument：**

   ```bash
   github-mcp-server --tools get_file_contents,issue_read,create_pull_request
   ```

2. **使用 Environment Variable：**

   ```bash
   GITHUB_TOOLS="get_file_contents,issue_read,create_pull_request" ./github-mcp-server
   ```

3. **与 Toolsets 组合**（累加）：

   ```bash
   github-mcp-server --toolsets repos,issues --tools get_gist
   ```

   这会注册 `repos` 和 `issues` toolsets 中的所有 tools，外加 `get_gist`。

**重要说明：**

- Tools 和 toolsets 可一起使用
- Read-only mode 优先：如果设置了 `--read-only`，write tools 会被跳过，即使通过 `--tools` 显式请求也是如此
- Tool names 必须完全匹配（例如使用 `get_file_contents`，而非 `getFileContents`）。无效 tool names 会导致 server 在启动时失败并显示 error message
- Tools 更名时会保留旧名称作为 aliases，以实现 backward compatibility。详情请参阅 [Tool Renaming](docs/tool-renaming.md)。

### 在 Docker 中使用 Toolsets

使用 Docker 时，可将 toolsets 作为 environment variables 传入：

```bash
docker run -i --rm \
  -e GITHUB_PERSONAL_ACCESS_TOKEN=<your-token> \
  -e GITHUB_TOOLSETS="repos,issues,pull_requests,actions,code_security" \
  ghcr.io/github/github-mcp-server
```

### 在 Docker 中使用 Tools

使用 Docker 时，可将特定 tools 作为 environment variables 传入，也可以组合 tools 与 toolsets：

```bash
# Tools only
docker run -i --rm \
  -e GITHUB_PERSONAL_ACCESS_TOKEN=<your-token> \
  -e GITHUB_TOOLS="get_file_contents,issue_read,create_pull_request" \
  ghcr.io/github/github-mcp-server

# Tools combined with toolsets (additive)
docker run -i --rm \
  -e GITHUB_PERSONAL_ACCESS_TOKEN=<your-token> \
  -e GITHUB_TOOLSETS="repos,issues" \
  -e GITHUB_TOOLS="get_gist" \
  ghcr.io/github/github-mcp-server
```

### 特殊 toolsets

#### "all" toolset

可提供特殊 toolset `all`，忽略其他任何配置并启用所有可用 toolsets：

```bash
./github-mcp-server --toolsets all
```

或使用 environment variable：

```bash
GITHUB_TOOLSETS="all" ./github-mcp-server
```

#### "default" toolset

未指定 toolsets 时，默认 toolset `default` 是传给 server 的配置。

默认配置为：

- context
- repos
- issues
- pull_requests
- users

若要保留默认配置并添加额外 toolsets：

```bash
GITHUB_TOOLSETS="default,stargazers" ./github-mcp-server
```

### Insiders Mode

本地 GitHub MCP Server 提供 insiders 版本，可提前使用新功能和实验性工具。

1. **使用 Command Line Argument：**

   ```bash
   ./github-mcp-server --insiders
   ```

2. **使用 Environment Variable：**

   ```bash
   GITHUB_INSIDERS=true ./github-mcp-server
   ```

使用 Docker 时：

```bash
docker run -i --rm \
  -e GITHUB_PERSONAL_ACCESS_TOKEN=<your-token> \
  -e GITHUB_INSIDERS=true \
  ghcr.io/github/github-mcp-server
```

### 可用 Toolsets

可使用以下 tools 集合：

<!-- START AUTOMATED TOOLSETS -->
|     | 工具集                  | 描述                                                          |
| --- | ----------------------- | ------------------------------------------------------------- |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/person-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/person-light.png"><img src="pkg/octicons/icons/person-light.png" width="20" height="20" alt="person"></picture> | `context`               | **强烈推荐**：提供当前用户以及正在操作的 GitHub 上下文的工具 |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/workflow-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/workflow-light.png"><img src="pkg/octicons/icons/workflow-light.png" width="20" height="20" alt="workflow"></picture> | `actions` | GitHub Actions workflows and CI/CD operations |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/code-square-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/code-square-light.png"><img src="pkg/octicons/icons/code-square-light.png" width="20" height="20" alt="code-square"></picture> | `code_quality` | GitHub Code Quality related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/codescan-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/codescan-light.png"><img src="pkg/octicons/icons/codescan-light.png" width="20" height="20" alt="codescan"></picture> | `code_security` | Code security related tools, such as GitHub Code Scanning |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/copilot-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/copilot-light.png"><img src="pkg/octicons/icons/copilot-light.png" width="20" height="20" alt="copilot"></picture> | `copilot` | Copilot related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/copilot-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/copilot-light.png"><img src="pkg/octicons/icons/copilot-light.png" width="20" height="20" alt="copilot"></picture> | `copilot_issue_intents` | Opt-in Copilot issue assignment tools that carry intent metadata (rationale, confidence, suggestion) |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/dependabot-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/dependabot-light.png"><img src="pkg/octicons/icons/dependabot-light.png" width="20" height="20" alt="dependabot"></picture> | `dependabot` | Dependabot tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/comment-discussion-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/comment-discussion-light.png"><img src="pkg/octicons/icons/comment-discussion-light.png" width="20" height="20" alt="comment-discussion"></picture> | `discussions` | GitHub Discussions related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/logo-gist-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/logo-gist-light.png"><img src="pkg/octicons/icons/logo-gist-light.png" width="20" height="20" alt="logo-gist"></picture> | `gists` | GitHub Gist related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/git-branch-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/git-branch-light.png"><img src="pkg/octicons/icons/git-branch-light.png" width="20" height="20" alt="git-branch"></picture> | `git` | GitHub Git API related tools for low-level Git operations |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/issue-opened-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/issue-opened-light.png"><img src="pkg/octicons/icons/issue-opened-light.png" width="20" height="20" alt="issue-opened"></picture> | `issues` | GitHub Issues related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/tag-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/tag-light.png"><img src="pkg/octicons/icons/tag-light.png" width="20" height="20" alt="tag"></picture> | `labels` | GitHub Labels related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/bell-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/bell-light.png"><img src="pkg/octicons/icons/bell-light.png" width="20" height="20" alt="bell"></picture> | `notifications` | GitHub Notifications related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/organization-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/organization-light.png"><img src="pkg/octicons/icons/organization-light.png" width="20" height="20" alt="organization"></picture> | `orgs` | GitHub Organization related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/project-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/project-light.png"><img src="pkg/octicons/icons/project-light.png" width="20" height="20" alt="project"></picture> | `projects` | GitHub Projects related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/git-pull-request-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/git-pull-request-light.png"><img src="pkg/octicons/icons/git-pull-request-light.png" width="20" height="20" alt="git-pull-request"></picture> | `pull_requests` | GitHub Pull Request related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/repo-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/repo-light.png"><img src="pkg/octicons/icons/repo-light.png" width="20" height="20" alt="repo"></picture> | `repos` | GitHub Repository related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/shield-lock-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/shield-lock-light.png"><img src="pkg/octicons/icons/shield-lock-light.png" width="20" height="20" alt="shield-lock"></picture> | `secret_protection` | Secret protection related tools, such as GitHub Secret Scanning |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/shield-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/shield-light.png"><img src="pkg/octicons/icons/shield-light.png" width="20" height="20" alt="shield"></picture> | `security_advisories` | Security advisories related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/star-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/star-light.png"><img src="pkg/octicons/icons/star-light.png" width="20" height="20" alt="star"></picture> | `stargazers` | GitHub Stargazers related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/people-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/people-light.png"><img src="pkg/octicons/icons/people-light.png" width="20" height="20" alt="people"></picture> | `users` | GitHub User related tools |
<!-- END AUTOMATED TOOLSETS -->

### 远程 GitHub MCP Server 中的额外 Toolsets

| Toolset                 | Description                                                   |
| ----------------------- | ------------------------------------------------------------- |
| `copilot` | Copilot 相关 tools（例如 Copilot Coding Agent） |
| `copilot_spaces` | Copilot Spaces 相关 tools |
| `github_support_docs_search` | 搜索 docs 以回答 GitHub product 和 support questions |

## Tools

<!-- START AUTOMATED TOOLS -->
<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/workflow-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/workflow-light.png"><img src="pkg/octicons/icons/workflow-light.png" width="20" height="20" alt="workflow"></picture> Actions</summary>

- **actions_get** - Get details of GitHub Actions resources (workflows, workflow runs, jobs, and artifacts)
  - **所需 OAuth Scopes**：`repo`
  - `method`: The method to execute (string, 必需)
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)
  - `resource_id`: The unique identifier of the resource. This will vary based on the "method" provided, so ensure you provide the correct ID:
    - Provide a workflow ID or workflow file name (e.g. ci.yaml) for 'get_workflow' method.
    - Provide a workflow run ID for 'get_workflow_run', 'get_workflow_run_usage', and 'get_workflow_run_logs_url' methods.
    - Provide an artifact ID for 'download_workflow_run_artifact' method.
    - Provide a job ID for 'get_workflow_job' method.
     (string, 必需)

- **actions_list** - List GitHub Actions workflows in a repository
  - **所需 OAuth Scopes**：`repo`
  - `method`: The action to perform (string, 必需)
  - `owner`: Repository owner (string, 必需)
  - `page`: Page number for pagination (default: 1) (number, 可选)
  - `per_page`: Results per page for pagination (default: 30, max: 100) (number, 可选)
  - `repo`: Repository name (string, 必需)
  - `resource_id`: The unique identifier of the resource. This will vary based on the "method" provided, so ensure you provide the correct ID:
    - Do not provide any resource ID for 'list_workflows' method.
    - Provide a workflow ID or workflow file name (e.g. ci.yaml) for 'list_workflow_runs' method, or omit to list all workflow runs in the repository.
    - Provide a workflow run ID for 'list_workflow_jobs' and 'list_workflow_run_artifacts' methods.
     (string, 可选)
  - `workflow_jobs_filter`: Filters for workflow jobs. **ONLY** used when method is 'list_workflow_jobs' (object, 可选)
  - `workflow_runs_filter`: Filters for workflow runs. **ONLY** used when method is 'list_workflow_runs' (object, 可选)

- **actions_run_trigger** - Trigger GitHub Actions workflow actions
  - **所需 OAuth Scopes**：`repo`
  - `inputs`: Inputs the workflow accepts. Only used for 'run_workflow' method. (object, 可选)
  - `method`: The method to execute (string, 必需)
  - `owner`: Repository owner (string, 必需)
  - `ref`: The git reference for the workflow. The reference can be a branch or tag name. Required for 'run_workflow' method. (string, 可选)
  - `repo`: Repository name (string, 必需)
  - `run_id`: The ID of the workflow run. Required for all methods except 'run_workflow'. (number, 可选)
  - `workflow_id`: The workflow ID (numeric) or workflow file name (e.g., main.yml, ci.yaml). Required for 'run_workflow' method. (string, 可选)

- **get_job_logs** - Get GitHub Actions workflow job logs
  - **所需 OAuth Scopes**：`repo`
  - `failed_only`: When true, gets logs for all failed jobs in the workflow run specified by run_id. Requires run_id to be provided. (boolean, 可选)
  - `job_id`: The unique identifier of the workflow job. Required when getting logs for a single job. (number, 可选)
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)
  - `return_content`: Returns actual log content instead of URLs (boolean, 可选)
  - `run_id`: The unique identifier of the workflow run. Required when failed_only is true to get logs for all failed jobs in the run. (number, 可选)
  - `tail_lines`: Number of lines to return from the end of the log (number, 可选)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/code-square-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/code-square-light.png"><img src="pkg/octicons/icons/code-square-light.png" width="20" height="20" alt="code-square"></picture> Code Quality</summary>

- **get_code_quality_finding** - Get code quality finding
  - **所需 OAuth Scopes**：`repo`
  - `findingNumber`: The number of the finding. (number, 必需)
  - `owner`: The owner of the repository. (string, 必需)
  - `repo`: The name of the repository. (string, 必需)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/codescan-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/codescan-light.png"><img src="pkg/octicons/icons/codescan-light.png" width="20" height="20" alt="codescan"></picture> Code Security</summary>

- **get_code_scanning_alert** - Get code scanning alert
  - **所需 OAuth Scopes**：`security_events`
  - **可接受 OAuth Scopes**：`repo`, `security_events`
  - `alertNumber`: The number of the alert. (number, 必需)
  - `owner`: The owner of the repository. (string, 必需)
  - `repo`: The name of the repository. (string, 必需)

- **list_code_scanning_alerts** - List code scanning alerts
  - **所需 OAuth Scopes**：`security_events`
  - **可接受 OAuth Scopes**：`repo`, `security_events`
  - `owner`: The owner of the repository. (string, 必需)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `ref`: The Git reference for the results you want to list. (string, 可选)
  - `repo`: The name of the repository. (string, 必需)
  - `severity`: Filter code scanning alerts by severity (string, 可选)
  - `state`: Filter code scanning alerts by state. Defaults to open (string, 可选)
  - `tool_name`: The name of the tool used for code scanning. (string, 可选)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/person-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/person-light.png"><img src="pkg/octicons/icons/person-light.png" width="20" height="20" alt="person"></picture> Context</summary>

- **get_me** - Get my user profile
  - 无需参数

- **get_team_members** - Get team members
  - **所需 OAuth Scopes**：`read:org`
  - **可接受 OAuth Scopes**：`admin:org`, `read:org`, `write:org`
  - `org`: Organization login (owner) that contains the team. (string, 必需)
  - `team_slug`: Team slug (string, 必需)

- **get_teams** - Get teams
  - **所需 OAuth Scopes**：`read:org`
  - **可接受 OAuth Scopes**：`admin:org`, `read:org`, `write:org`
  - `user`: Username to get teams for. If not provided, uses the authenticated user. (string, 可选)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/copilot-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/copilot-light.png"><img src="pkg/octicons/icons/copilot-light.png" width="20" height="20" alt="copilot"></picture> Copilot</summary>

- **assign_copilot_to_issue** - Assign Copilot to issue
  - **所需 OAuth Scopes**：`repo`
  - `base_ref`: Git reference (e.g., branch) that the agent will start its work from. If not specified, defaults to the repository's default branch (string, 可选)
  - `custom_instructions`: Optional custom instructions to guide the agent beyond the issue body. Use this to provide additional context, constraints, or guidance that is not captured in the issue description (string, 可选)
  - `issue_number`: Issue number (number, 必需)
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)

- **request_copilot_review** - Request Copilot review
  - **所需 OAuth Scopes**：`repo`
  - `owner`: Repository owner (string, 必需)
  - `pullNumber`: Pull request number (number, 必需)
  - `repo`: Repository name (string, 必需)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/copilot-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/copilot-light.png"><img src="pkg/octicons/icons/copilot-light.png" width="20" height="20" alt="copilot"></picture> Copilot Issue Intents</summary>

- **assign_copilot_to_issue_with_intent** - Assign Copilot to issue with intent
  - **所需 OAuth Scopes**：`repo`
  - `base_ref`: Git reference (e.g., branch) that the agent will start its work from. If not specified, defaults to the repository's default branch. Ignored when is_suggestion is true (string, 可选)
  - `confidence`: How confident you are in this choice. 'HIGH' for clear signal or explicit user request, 'MEDIUM' for reasonable inference with some ambiguity, 'LOW' for best guess with limited signal. (string, 必需)
  - `custom_instructions`: Optional custom instructions to guide the agent beyond the issue body. Ignored when is_suggestion is true (string, 可选)
  - `is_suggestion`: If true, records a pending Copilot assignment intent rather than launching the agent. Approval later supplies the launch context; base_ref and custom_instructions are ignored in this case. (boolean, 必需)
  - `issue_number`: Issue number (number, 必需)
  - `owner`: Repository owner (string, 必需)
  - `rationale`: One concise sentence explaining what specifically about the issue led to choosing Copilot. State the concrete signal (e.g. 'Well-scoped task with clear acceptance criteria'). (string, 必需)
  - `repo`: Repository name (string, 必需)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/dependabot-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/dependabot-light.png"><img src="pkg/octicons/icons/dependabot-light.png" width="20" height="20" alt="dependabot"></picture> Dependabot</summary>

- **get_dependabot_alert** - Get dependabot alert
  - **所需 OAuth Scopes**：`security_events`
  - **可接受 OAuth Scopes**：`repo`, `security_events`
  - `alertNumber`: The number of the alert. (number, 必需)
  - `owner`: The owner of the repository. (string, 必需)
  - `repo`: The name of the repository. (string, 必需)

- **list_dependabot_alerts** - List dependabot alerts
  - **所需 OAuth Scopes**：`security_events`
  - **可接受 OAuth Scopes**：`repo`, `security_events`
  - `after`: Cursor for pagination. Use the cursor from the previous response. (string, 可选)
  - `owner`: The owner of the repository. (string, 必需)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `repo`: The name of the repository. (string, 必需)
  - `severity`: Filter dependabot alerts by severity (string, 可选)
  - `state`: Filter dependabot alerts by state. Defaults to open (string, 可选)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/comment-discussion-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/comment-discussion-light.png"><img src="pkg/octicons/icons/comment-discussion-light.png" width="20" height="20" alt="comment-discussion"></picture> Discussions</summary>

- **discussion_comment_write** - Manage discussion comments
  - **所需 OAuth Scopes**：`repo`
  - `body`: Comment content (required for 'add', 'reply', and 'update' methods) (string, 可选)
  - `commentNodeID`: The Node ID of the discussion comment (required for 'reply', 'update', 'delete', 'mark_answer', and 'unmark_answer' methods). For 'reply', this is the top-level comment to reply to; GitHub Discussions only support one level of nesting. (string, 可选)
  - `discussionNumber`: Discussion number (required for 'add' and 'reply' methods) (number, 可选)
  - `method`: Write operation to perform on a discussion comment.
    Options are:
    - 'add' - adds a new top-level comment to a discussion.
    - 'reply' - replies to a top-level discussion comment (GitHub Discussions only support one level of nesting).
    - 'update' - updates an existing discussion comment.
    - 'delete' - deletes a discussion comment.
    - 'mark_answer' - marks a discussion comment as the answer (Q&A only).
    - 'unmark_answer' - unmarks a discussion comment as the answer (Q&A only).
     (string, 必需)
  - `owner`: Repository owner (required for 'add' and 'reply' methods) (string, 可选)
  - `repo`: Repository name (required for 'add' and 'reply' methods) (string, 可选)

- **get_discussion** - Get discussion
  - **所需 OAuth Scopes**：`repo`
  - `discussionNumber`: Discussion Number (number, 必需)
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)

- **get_discussion_comments** - Get discussion comments
  - **所需 OAuth Scopes**：`repo`
  - `after`: Cursor for pagination. Use the cursor from the previous response. (string, 可选)
  - `discussionNumber`: Discussion Number (number, 必需)
  - `includeReplies`: When true, each top-level comment will include its replies nested within it (up to 100 replies per comment, which is the GitHub API maximum). Defaults to false. (boolean, 可选)
  - `owner`: Repository owner (string, 必需)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `repo`: Repository name (string, 必需)

- **list_discussion_categories** - List discussion categories
  - **所需 OAuth Scopes**：`repo`
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name. If not provided, discussion categories will be queried at the organisation level. (string, 可选)

- **list_discussions** - List discussions
  - **所需 OAuth Scopes**：`repo`
  - `after`: Cursor for pagination. Use the cursor from the previous response. (string, 可选)
  - `category`: Optional filter by discussion category ID. If provided, only discussions with this category are listed. (string, 可选)
  - `direction`: Order direction. (string, 可选)
  - `orderBy`: Order discussions by field. If provided, the 'direction' also needs to be provided. (string, 可选)
  - `owner`: Repository owner (string, 必需)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `repo`: Repository name. If not provided, discussions will be queried at the organisation level. (string, 可选)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/logo-gist-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/logo-gist-light.png"><img src="pkg/octicons/icons/logo-gist-light.png" width="20" height="20" alt="logo-gist"></picture> Gists</summary>

- **create_gist** - Create Gist
  - **所需 OAuth Scopes**：`gist`
  - `content`: Content for simple single-file gist creation (string, 必需)
  - `description`: Description of the gist (string, 可选)
  - `filename`: Filename for simple single-file gist creation (string, 必需)
  - `public`: Whether the gist is public (boolean, 可选)

- **get_gist** - Get Gist Content
  - `gist_id`: The ID of the gist (string, 必需)

- **list_gists** - List Gists
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `since`: Only gists updated after this time (ISO 8601 timestamp) (string, 可选)
  - `username`: GitHub username (omit for authenticated user's gists) (string, 可选)

- **update_gist** - Update Gist
  - **所需 OAuth Scopes**：`gist`
  - `content`: Content for the file (string, 必需)
  - `description`: Updated description of the gist (string, 可选)
  - `filename`: Filename to update or create (string, 必需)
  - `gist_id`: ID of the gist to update (string, 必需)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/git-branch-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/git-branch-light.png"><img src="pkg/octicons/icons/git-branch-light.png" width="20" height="20" alt="git-branch"></picture> Git</summary>

- **get_repository_tree** - Get repository tree
  - **所需 OAuth Scopes**：`repo`
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `path_filter`: Optional path prefix to filter the tree results (e.g., 'src/' to only show files in the src directory) (string, 可选)
  - `recursive`: Setting this parameter to true returns the objects or subtrees referenced by the tree. Default is false (boolean, 可选)
  - `repo`: Repository name (string, 必需)
  - `tree_sha`: The SHA1 value or ref (branch or tag) name of the tree. Defaults to the repository's default branch (string, 可选)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/issue-opened-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/issue-opened-light.png"><img src="pkg/octicons/icons/issue-opened-light.png" width="20" height="20" alt="issue-opened"></picture> Issues</summary>

- **add_issue_comment** - Add comment to issue or pull request
  - **所需 OAuth Scopes**：`repo`
  - `body`: Comment content. Required unless reaction is provided. (string, 可选)
  - `comment_id`: The numeric ID of the issue or pull request comment to react to. Use this for reactions to comments; omit it to react to the issue or pull request itself. Cannot be combined with body. (number, 可选)
  - `issue_number`: Issue or pull request number to comment on or react to. (number, 必需)
  - `owner`: Repository owner (string, 必需)
  - `reaction`: Emoji reaction to add. Required unless body is provided. (string, 可选)
  - `repo`: Repository name (string, 必需)

- **get_label** - Get a specific label from a repository
  - **所需 OAuth Scopes**：`repo`
  - `name`: Label name. (string, 必需)
  - `owner`: Repository owner (username or organization name) (string, 必需)
  - `repo`: Repository name (string, 必需)

- **issue_read** - Get issue details
  - **所需 OAuth Scopes**：`repo`
  - `issue_number`: The number of the issue (number, 必需)
  - `method`: The read operation to perform on a single issue.
    Options are:
    1. get - Get issue details. Also returns best-effort hierarchy flags (`has_parent`, `has_children`); `parent` and `sub_issues_summary` are optional relationship summaries.
    2. get_comments - Get issue comments.
    3. get_sub_issues - Get sub-issues (children) of the issue.
    4. get_parent - Get the parent issue, if this issue is a sub-issue of another.
    5. get_labels - Get labels assigned to the issue.
     (string, 必需)
  - `owner`: The owner of the repository (string, 必需)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `repo`: The name of the repository (string, 必需)

- **issue_write** - Create or update issue/pull request
  - **所需 OAuth Scopes**：`repo`
  - `assignees`: Usernames to assign to this issue (string[], 可选)
  - `body`: Issue body content (string, 可选)
  - `duplicate_of`: Issue number that this issue is a duplicate of. Only used when state_reason is 'duplicate'. (number, 可选)
  - `issue_fields`: Issue field values to set or clear. Each item requires 'field_name' and exactly one of 'value', 'field_option_name', or 'delete: true'. (object[], 可选)
  - `issue_number`: Issue number to update (number, 可选)
  - `labels`: Labels to apply to this issue (string[], 可选)
  - `method`: Write operation to perform on a single issue.
    Options are:
    - 'create' - creates a new issue.
    - 'update' - updates an existing issue.
     (string, 必需)
  - `milestone`: Milestone number (number, 可选)
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)
  - `state`: New state (string, 可选)
  - `state_reason`: Reason for the state change. Ignored unless state is changed. (string, 可选)
  - `title`: Issue title (string, 可选)
  - `type`: Type of this issue. Only use if issue types are enabled for this repository. Use list_issue_types tool to get valid type values for this repository or its owner organization. If the repository doesn't support issue types, omit this parameter. (string, 可选)

- **list_issue_fields** - List issue fields
  - **所需 OAuth Scopes（任一）**：`repo`, `read:org`
  - **可接受 OAuth Scopes**：`admin:org`, `read:org`, `repo`, `write:org`
  - `owner`: The account owner of the repository or organization. The name is not case sensitive. (string, 必需)
  - `repo`: The name of the repository. When provided, returns fields for this specific repository (inherited from its organization). When omitted, returns org-level fields directly. (string, 可选)

- **list_issue_types** - List available issue types
  - **所需 OAuth Scopes（任一）**：`repo`, `read:org`
  - **可接受 OAuth Scopes**：`admin:org`, `read:org`, `repo`, `write:org`
  - `owner`: The account owner of the repository or organization. (string, 必需)
  - `repo`: The name of the repository. When provided, returns issue types for this specific repository. When omitted, returns org-level issue types directly. (string, 可选)

- **list_issues** - List issues
  - **所需 OAuth Scopes**：`repo`
  - `after`: Cursor for pagination. Use the cursor from the previous response. (string, 可选)
  - `direction`: Order direction. If provided, the 'orderBy' also needs to be provided. (string, 可选)
  - `field_filters`: Filter by custom issue field values. Each entry takes a field_name and a value; the server looks up the field and coerces the value to its type (single-select option name, text, number, or YYYY-MM-DD date). (object[], 可选)
  - `fields`: Subset of fields to return for each issue. If omitted, all fields are returned. Use this to reduce response size when you only need specific fields; omitting 'body' and 'field_values' in particular drops the largest per-result data. (string[], 可选)
  - `labels`: Filter by labels (string[], 可选)
  - `orderBy`: Order issues by field. If provided, the 'direction' also needs to be provided. (string, 可选)
  - `owner`: Repository owner (string, 必需)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `repo`: Repository name (string, 必需)
  - `since`: Filter by date (ISO 8601 timestamp) (string, 可选)
  - `state`: Filter by state, by default both open and closed issues are returned when not provided (string, 可选)

- **search_issues** - Search issues
  - **所需 OAuth Scopes**：`repo`
  - `fields`: Subset of fields to return for each issue result. If omitted, all fields are returned. Use this to reduce response size when you only need specific fields; omitting 'body', 'reactions', and 'labels' in particular drops the largest per-result data. (string[], 可选)
  - `order`: Sort order (string, 可选)
  - `owner`: Optional repository owner. If provided with repo, only issues for this repository are listed. (string, 可选)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `query`: Search query using GitHub issues search syntax (string, 必需)
  - `repo`: Optional repository name. If provided with owner, only issues for this repository are listed. (string, 可选)
  - `sort`: Sort field by number of matches of categories, defaults to best match (string, 可选)

- **sub_issue_write** - Change sub-issue
  - **所需 OAuth Scopes**：`repo`
  - `after_id`: The ID of the sub-issue to be prioritized after (either after_id OR before_id should be specified) (number, 可选)
  - `before_id`: The ID of the sub-issue to be prioritized before (either after_id OR before_id should be specified) (number, 可选)
  - `issue_number`: The number of the parent issue (number, 必需)
  - `method`: The action to perform on a single sub-issue
    Options are:
    - 'add' - add a sub-issue to a parent issue in a GitHub repository.
    - 'remove' - remove a sub-issue from a parent issue in a GitHub repository.
    - 'reprioritize' - change the order of sub-issues within a parent issue in a GitHub repository. Use either 'after_id' or 'before_id' to specify the new position.
    Writes issue hierarchy. To move a sub-issue to a new parent, use `add` with `replace_parent=true`; there is no writable parent field.
     (string, 必需)
  - `owner`: Repository owner (string, 必需)
  - `replace_parent`: When true, replaces the sub-issue's current parent issue. Use with 'add' method only. (boolean, 可选)
  - `repo`: Repository name (string, 必需)
  - `sub_issue_id`: The ID of the sub-issue to add. ID is not the same as issue number (number, 必需)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/tag-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/tag-light.png"><img src="pkg/octicons/icons/tag-light.png" width="20" height="20" alt="tag"></picture> Labels</summary>

- **get_label** - Get a specific label from a repository
  - **所需 OAuth Scopes**：`repo`
  - `name`: Label name. (string, 必需)
  - `owner`: Repository owner (username or organization name) (string, 必需)
  - `repo`: Repository name (string, 必需)

- **label_write** - Write operations on repository labels
  - **所需 OAuth Scopes**：`repo`
  - `color`: Label color as 6-character hex code without '#' prefix (e.g., 'f29513'). Required for 'create', optional for 'update'. (string, 可选)
  - `description`: Label description text. Optional for 'create' and 'update'. (string, 可选)
  - `method`: Operation to perform: 'create', 'update', or 'delete' (string, 必需)
  - `name`: Label name - required for all operations (string, 必需)
  - `new_name`: New name for the label (used only with 'update' method to rename) (string, 可选)
  - `owner`: Repository owner (username or organization name) (string, 必需)
  - `repo`: Repository name (string, 必需)

- **list_label** - List labels from a repository
  - **所需 OAuth Scopes**：`repo`
  - `owner`: Repository owner (username or organization name) - required for all operations (string, 必需)
  - `repo`: Repository name - required for all operations (string, 必需)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/bell-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/bell-light.png"><img src="pkg/octicons/icons/bell-light.png" width="20" height="20" alt="bell"></picture> Notifications</summary>

- **dismiss_notification** - Dismiss notification
  - **所需 OAuth Scopes**：`notifications`
  - `state`: The new state of the notification (read/done) (string, 必需)
  - `threadID`: The ID of the notification thread (string, 必需)

- **get_notification_details** - Get notification details
  - **所需 OAuth Scopes**：`notifications`
  - `notificationID`: The ID of the notification (string, 必需)

- **list_notifications** - List notifications
  - **所需 OAuth Scopes**：`notifications`
  - `before`: Only show notifications updated before the given time (ISO 8601 format) (string, 可选)
  - `filter`: Filter notifications to, use default unless specified. Read notifications are ones that have already been acknowledged by the user. Participating notifications are those that the user is directly involved in, such as issues or pull requests they have commented on or created. (string, 可选)
  - `owner`: Optional repository owner. If provided with repo, only notifications for this repository are listed. (string, 可选)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `repo`: Optional repository name. If provided with owner, only notifications for this repository are listed. (string, 可选)
  - `since`: Only show notifications updated after the given time (ISO 8601 format) (string, 可选)

- **manage_notification_subscription** - Manage notification subscription
  - **所需 OAuth Scopes**：`notifications`
  - `action`: Action to perform: ignore, watch, or delete the notification subscription. (string, 必需)
  - `notificationID`: The ID of the notification thread. (string, 必需)

- **manage_repository_notification_subscription** - Manage repository notification subscription
  - **所需 OAuth Scopes**：`notifications`
  - `action`: Action to perform: ignore, watch, or delete the repository notification subscription. (string, 必需)
  - `owner`: The account owner of the repository. (string, 必需)
  - `repo`: The name of the repository. (string, 必需)

- **mark_all_notifications_read** - Mark all notifications as read
  - **所需 OAuth Scopes**：`notifications`
  - `lastReadAt`: Describes the last point that notifications were checked (optional). Default: Now (string, 可选)
  - `owner`: Optional repository owner. If provided with repo, only notifications for this repository are marked as read. (string, 可选)
  - `repo`: Optional repository name. If provided with owner, only notifications for this repository are marked as read. (string, 可选)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/organization-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/organization-light.png"><img src="pkg/octicons/icons/organization-light.png" width="20" height="20" alt="organization"></picture> Organizations</summary>

- **search_orgs** - Search organizations
  - **所需 OAuth Scopes**：`read:org`
  - **可接受 OAuth Scopes**：`admin:org`, `read:org`, `write:org`
  - `order`: Sort order (string, 可选)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `query`: Organization search query. Examples: 'microsoft', 'location:california', 'created:>=2025-01-01'. Search is automatically scoped to type:org. (string, 必需)
  - `sort`: Sort field by category (string, 可选)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/project-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/project-light.png"><img src="pkg/octicons/icons/project-light.png" width="20" height="20" alt="project"></picture> Projects</summary>

- **projects_get** - Get details of GitHub Projects resources
  - **所需 OAuth Scopes**：`read:project`
  - **可接受 OAuth Scopes**：`project`, `read:project`
  - `field_id`: The field's ID. Required for 'get_project_field' method. (number, 可选)
  - `field_names`: Specific list of field names to include in the response when getting a project item (e.g. ["Status", "Priority"]). Resolved server-side to field IDs — pass this instead of 'fields' when you only know the human-readable names. Mutually exclusive with 'fields' — provide one, not both. Only used for 'get_project_item' method. (string[], 可选)
  - `fields`: Specific list of field IDs to include in the response when getting a project item (e.g. ["102589", "985201", "169875"]). If neither 'fields' nor 'field_names' is provided, only the title field is included. Mutually exclusive with 'field_names' — provide one, not both. Only used for 'get_project_item' method. (string[], 可选)
  - `item_id`: The item's ID. Required for 'get_project_item' method. (number, 可选)
  - `method`: The method to execute (string, 必需)
  - `owner`: The owner (user or organization login). The name is not case sensitive. (string, 可选)
  - `owner_type`: Owner type (user or org). If not provided, will be automatically detected. (string, 可选)
  - `project_number`: The project's number. (number, 可选)
  - `status_update_id`: The node ID of the project status update. Required for 'get_project_status_update' method. (string, 可选)

- **projects_list** - List GitHub Projects resources
  - **所需 OAuth Scopes**：`read:project`
  - **可接受 OAuth Scopes**：`project`, `read:project`
  - `after`: Forward pagination cursor from previous pageInfo.nextCursor. (string, 可选)
  - `before`: Backward pagination cursor from previous pageInfo.prevCursor (rare). (string, 可选)
  - `field_names`: Field names to include when listing project items (e.g. ["Status", "Priority"]). Resolved server-side to field IDs — pass this instead of 'fields' when you only know the human-readable names. Names that fail to resolve return a structured error. Mutually exclusive with 'fields' — provide one, not both. Only used for 'list_project_items' method. (string[], 可选)
  - `fields`: Field IDs to include when listing project items (e.g. ["102589", "985201"]). CRITICAL: Always provide to get field values. Without this (and without 'field_names'), only titles returned. Mutually exclusive with 'field_names' — provide one, not both. Only used for 'list_project_items' method. (string[], 可选)
  - `method`: The action to perform (string, 必需)
  - `owner`: The owner (user or organization login). The name is not case sensitive. (string, 必需)
  - `owner_type`: Owner type (user or org). If not provided, will automatically try both. (string, 可选)
  - `per_page`: Results per page (max 50) (number, 可选)
  - `project_number`: The project's number. Required for 'list_project_fields', 'list_project_items', and 'list_project_status_updates' methods. (number, 可选)
  - `query`: Filter/query string. For list_projects: filter by title text and state (e.g. "roadmap is:open"). For list_project_items: advanced filtering using GitHub's project filtering syntax. (string, 可选)

- **projects_write** - Manage GitHub Projects
  - **所需 OAuth Scopes**：`project`
  - `body`: The body of the status update (markdown). Used for 'create_project_status_update' method. (string, 可选)
  - `field_name`: The name of the iteration field (e.g. 'Sprint'). Required for 'create_iteration_field' method. (string, 可选)
  - `issue_number`: The issue number. Required for 'add_project_item' when item_type is 'issue'. Also accepted by 'update_project_item' to resolve the item by issue number (combine with item_owner and item_repo). (number, 可选)
  - `item_id`: The project item ID. Required for 'delete_project_item'. For 'update_project_item', provide either item_id, or (item_owner + item_repo + issue_number) to resolve the item by issue. (number, 可选)
  - `item_owner`: The owner (user or organization) of the repository containing the issue or pull request. Required for 'add_project_item' method. Also accepted by 'update_project_item' when resolving the item by issue number. (string, 可选)
  - `item_repo`: The name of the repository containing the issue or pull request. Required for 'add_project_item' method. Also accepted by 'update_project_item' when resolving the item by issue number. (string, 可选)
  - `item_type`: The item's type, either issue or pull_request. Required for 'add_project_item' method. (string, 可选)
  - `items`: The items to update with the top-level 'updated_field'. Required for 'update_project_items'; prefer it over calling 'update_project_item' in a loop. Each entry must match exactly one reference variant: 'node_id', numeric 'item_id', or 'item_owner' + 'item_repo' + 'issue_number'. Limit: 50 items per call. (object[], 可选)
  - `iteration_duration`: Duration in days for iterations of the field (e.g. 7 for weekly, 14 for bi-weekly). Required for 'create_iteration_field' method. (number, 可选)
  - `iterations`: Custom iterations for 'create_iteration_field' method. Only set this when you need iterations with varying durations, breaks between them, or specific titles. Otherwise omit it: GitHub auto-creates three iterations of 'iteration_duration' days starting on 'start_date', which is the right choice for most cases. (object[], 可选)
  - `method`: The method to execute (string, 必需)
  - `owner`: The project owner (user or organization login). The name is not case sensitive. (string, 必需)
  - `owner_type`: Owner type (user or org). Required for 'create_project' method. If not provided for other methods, will be automatically detected. (string, 可选)
  - `project_number`: The project's number. Required for all methods except 'create_project'. (number, 可选)
  - `pull_request_number`: The pull request number (use when item_type is 'pull_request' for 'add_project_item' method). Provide either issue_number or pull_request_number. (number, 可选)
  - `start_date`: Start date in YYYY-MM-DD format. Used for 'create_project_status_update' and 'create_iteration_field' methods. (string, 可选)
  - `status`: The status of the project. Used for 'create_project_status_update' method. (string, 可选)
  - `target_date`: The target date of the status update in YYYY-MM-DD format. Used for 'create_project_status_update' method. (string, 可选)
  - `title`: The project title. Required for 'create_project' method. (string, 可选)
  - `updated_field`: The field/value to apply, using {"id": 123, "value": ...} or {"name": "Status", "value": ...}; null clears the field. Required for 'update_project_item' and 'update_project_items', where one top-level field/value applies to every item in a batch. For 'update_project_item' SINGLE_SELECT fields, the name form accepts option names; the ID form expects an option ID. (object, 可选)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/git-pull-request-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/git-pull-request-light.png"><img src="pkg/octicons/icons/git-pull-request-light.png" width="20" height="20" alt="git-pull-request"></picture> Pull Requests</summary>

- **add_comment_to_pending_review** - Add review comment to the requester's latest pending pull request review
  - **所需 OAuth Scopes**：`repo`
  - `body`: The text of the review comment (string, 必需)
  - `line`: The line of the blob in the pull request diff that the comment applies to. For multi-line comments, the last line of the range (number, 可选)
  - `owner`: Repository owner (string, 必需)
  - `path`: The relative path to the file that necessitates a comment (string, 必需)
  - `pullNumber`: Pull request number (number, 必需)
  - `repo`: Repository name (string, 必需)
  - `side`: The side of the diff to comment on. LEFT indicates the previous state, RIGHT indicates the new state (string, 可选)
  - `startLine`: For multi-line comments, the first line of the range that the comment applies to (number, 可选)
  - `startSide`: For multi-line comments, the starting side of the diff that the comment applies to. LEFT indicates the previous state, RIGHT indicates the new state (string, 可选)
  - `subjectType`: The level at which the comment is targeted (string, 必需)

- **add_reply_to_pull_request_comment** - Add reply to pull request comment
  - **所需 OAuth Scopes**：`repo`
  - `body`: The text of the reply. Required unless reaction is provided. (string, 可选)
  - `commentId`: The numeric ID of the pull request review comment to reply or react to. Use the number from a #discussion_r... anchor, not the GraphQL thread node ID (PRRT_...). (number, 必需)
  - `owner`: Repository owner (string, 必需)
  - `pullNumber`: Pull request number. Required when body is provided. (number, 可选)
  - `reaction`: Emoji reaction to add. Required unless body is provided. (string, 可选)
  - `repo`: Repository name (string, 必需)

- **create_pull_request** - Open new pull request
  - **所需 OAuth Scopes**：`repo`
  - `base`: Branch to merge into (string, 必需)
  - `body`: PR description (string, 可选)
  - `draft`: Create as draft PR (boolean, 可选)
  - `head`: Branch containing changes (string, 必需)
  - `maintainer_can_modify`: Allow maintainer edits (boolean, 可选)
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)
  - `reviewers`: GitHub usernames or ORG/team-slug team reviewers to request reviews from (string[], 可选)
  - `title`: PR title (string, 必需)

- **list_pull_requests** - List pull requests
  - **所需 OAuth Scopes**：`repo`
  - `base`: Filter by base branch (string, 可选)
  - `direction`: Sort direction (string, 可选)
  - `fields`: Subset of fields to return for each pull request. If omitted, all fields are returned. Use this to reduce response size when you only need specific fields; omitting 'body' in particular drops the largest per-result data. (string[], 可选)
  - `head`: Filter by head user/org and branch (string, 可选)
  - `owner`: Repository owner (string, 必需)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `repo`: Repository name (string, 必需)
  - `sort`: Sort by (string, 可选)
  - `state`: Filter by state (string, 可选)

- **merge_pull_request** - Merge pull request
  - **所需 OAuth Scopes**：`repo`
  - `commit_message`: Extra detail for merge commit (string, 可选)
  - `commit_title`: Title for merge commit (string, 可选)
  - `merge_method`: Merge method (string, 可选)
  - `owner`: Repository owner (string, 必需)
  - `pullNumber`: Pull request number (number, 必需)
  - `repo`: Repository name (string, 必需)

- **pull_request_read** - Get details for a single pull request
  - **所需 OAuth Scopes**：`repo`
  - `after`: Cursor for pagination, used only by the get_review_comments method. Pass the endCursor from the previous page's PageInfo to fetch the next page. (string, 可选)
  - `method`: Action to specify what pull request data needs to be retrieved from GitHub. 
    Possible options: 
     1. get - Get details of a specific pull request.
     2. get_diff - Get the diff of a pull request.
     3. get_status - Get combined commit status of a head commit in a pull request.
     4. get_files - Get the list of files changed in a pull request. Use with pagination parameters to control the number of results returned.
     5. get_commits - Get the list of commits on a pull request. Use with pagination parameters to control the number of results returned.
     6. get_review_comments - Get review threads on a pull request. Each thread contains logically grouped review comments made on the same code location during pull request reviews. Returns threads with metadata (isResolved, isOutdated, isCollapsed) and their associated comments. Use cursor-based pagination (perPage, after) to control results.
     7. get_reviews - Get the reviews on a pull request. When asked for review comments, use get_review_comments method. Use with pagination parameters to control the number of results returned.
     8. get_comments - Get comments on a pull request. Use this if user doesn't specifically want review comments. Use with pagination parameters to control the number of results returned.
     9. get_check_runs - Get check runs for the head commit of a pull request. Check runs are the individual CI/CD jobs and checks that run on the PR.
     (string, 必需)
  - `owner`: Repository owner (string, 必需)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `pullNumber`: Pull request number (number, 必需)
  - `repo`: Repository name (string, 必需)

- **pull_request_review_write** - Write operations (create, submit, delete) on pull request reviews
  - **所需 OAuth Scopes**：`repo`
  - `body`: Review comment text (string, 可选)
  - `commitID`: SHA of commit to review (string, 可选)
  - `event`: Review action to perform. (string, 可选)
  - `method`: The write operation to perform on pull request review. (string, 必需)
  - `owner`: Repository owner (string, 必需)
  - `pullNumber`: Pull request number (number, 必需)
  - `repo`: Repository name (string, 必需)
  - `threadId`: The node ID of the review thread (e.g., PRRT_kwDOxxx). Required for resolve_thread and unresolve_thread methods. Get thread IDs from pull_request_read with method get_review_comments. (string, 可选)

- **search_pull_requests** - Search pull requests
  - **所需 OAuth Scopes**：`repo`
  - `fields`: Subset of fields to return for each pull request result. If omitted, all fields are returned. Use this to reduce response size when you only need specific fields; omitting 'body', 'reactions', and 'labels' in particular drops the largest per-result data. (string[], 可选)
  - `order`: Sort order (string, 可选)
  - `owner`: Optional repository owner. If provided with repo, only pull requests for this repository are listed. (string, 可选)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `query`: Search query using GitHub pull request search syntax (string, 必需)
  - `repo`: Optional repository name. If provided with owner, only pull requests for this repository are listed. (string, 可选)
  - `sort`: Sort field by number of matches of categories, defaults to best match (string, 可选)

- **update_pull_request** - Edit pull request
  - **所需 OAuth Scopes**：`repo`
  - `base`: New base branch name (string, 可选)
  - `body`: New description (string, 可选)
  - `draft`: Mark pull request as draft (true) or ready for review (false) (boolean, 可选)
  - `maintainer_can_modify`: Allow maintainer edits (boolean, 可选)
  - `owner`: Repository owner (string, 必需)
  - `pullNumber`: Pull request number to update (number, 必需)
  - `repo`: Repository name (string, 必需)
  - `reviewers`: GitHub usernames or ORG/team-slug team reviewers to request reviews from (string[], 可选)
  - `state`: New state (string, 可选)
  - `title`: New title (string, 可选)

- **update_pull_request_branch** - Update pull request branch
  - **所需 OAuth Scopes**：`repo`
  - `expectedHeadSha`: The expected SHA of the pull request's HEAD ref (string, 可选)
  - `owner`: Repository owner (string, 必需)
  - `pullNumber`: Pull request number (number, 必需)
  - `repo`: Repository name (string, 必需)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/repo-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/repo-light.png"><img src="pkg/octicons/icons/repo-light.png" width="20" height="20" alt="repo"></picture> Repositories</summary>

- **create_branch** - Create branch
  - **所需 OAuth Scopes**：`repo`
  - `branch`: Name for new branch (string, 必需)
  - `from_branch`: Source branch (defaults to repo default) (string, 可选)
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)

- **create_or_update_file** - Create or update file
  - **所需 OAuth Scopes**：`repo`
  - `branch`: Branch to create/update the file in (string, 必需)
  - `content`: Content of the file, exactly as it should appear once written. Do not base64-encode it; this server does that before calling the REST API. (string, 必需)
  - `message`: Commit message (string, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `path`: Path where to create/update the file (string, 必需)
  - `repo`: Repository name (string, 必需)
  - `sha`: The blob SHA of the file being replaced. Required if the file already exists. (string, 可选)

- **create_repository** - Create repository
  - **所需 OAuth Scopes**：`repo`
  - `autoInit`: Initialize with README (boolean, 可选)
  - `description`: Repository description (string, 可选)
  - `name`: Repository name (string, 必需)
  - `organization`: Organization to create the repository in (omit to create in your personal account) (string, 可选)
  - `private`: Whether the repository should be private. Defaults to true (private) when omitted. (boolean, 可选)

- **delete_file** - Delete file
  - **所需 OAuth Scopes**：`repo`
  - `branch`: Branch to delete the file from (string, 必需)
  - `message`: Commit message (string, 必需)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `path`: Path to the file to delete (string, 必需)
  - `repo`: Repository name (string, 必需)

- **fork_repository** - Fork repository
  - **所需 OAuth Scopes**：`repo`
  - `organization`: Organization to fork to (string, 可选)
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)

- **get_commit** - Get commit details
  - **所需 OAuth Scopes**：`repo`
  - `detail`: Level of detail to include for changed files. "none" omits stats and files entirely. "stats" (default) includes per-file metadata: filename, status, and lines-of-code counts (additions, deletions, changes), with no patch content. "full_patch" additionally includes the unified diff content for each file and can be very large. (string, 可选)
  - `owner`: Repository owner (string, 必需)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `repo`: Repository name (string, 必需)
  - `sha`: Commit SHA, branch name, or tag name (string, 必需)

- **get_file_contents** - Get file or directory contents
  - **所需 OAuth Scopes**：`repo`
  - `fields`: Subset of fields to return for each entry when the path is a directory. If omitted, all fields are returned. Ignored when the path is a single file. Use this to reduce response size when listing directories and you only need specific fields, e.g. just 'name' and 'type'. (string[], 可选)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `path`: Path to file/directory (string, 可选)
  - `ref`: Accepts optional git refs such as `refs/tags/{tag}`, `refs/heads/{branch}` or `refs/pull/{pr_number}/head` (string, 可选)
  - `repo`: Repository name (string, 必需)
  - `sha`: Accepts optional commit SHA. If specified, it will be used instead of ref (string, 可选)

- **get_latest_release** - Get latest release
  - **所需 OAuth Scopes**：`repo`
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)

- **get_release_by_tag** - Get a release by tag name
  - **所需 OAuth Scopes**：`repo`
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)
  - `tag`: Tag name (e.g., 'v1.0.0') (string, 必需)

- **get_tag** - Get tag details
  - **所需 OAuth Scopes**：`repo`
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)
  - `tag`: Tag name (string, 必需)

- **list_branches** - List branches
  - **所需 OAuth Scopes**：`repo`
  - `owner`: Repository owner (string, 必需)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `repo`: Repository name (string, 必需)

- **list_commits** - List commits
  - **所需 OAuth Scopes**：`repo`
  - `author`: Author username or email address to filter commits by (string, 可选)
  - `fields`: Subset of fields to return for each commit. If omitted, all fields are returned. Use this to reduce response size when you only need specific fields, e.g. just 'sha' and 'html_url'. (string[], 可选)
  - `owner`: Repository owner (string, 必需)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `path`: Only commits containing this file path will be returned (string, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `repo`: Repository name (string, 必需)
  - `sha`: Commit SHA, branch or tag name to list commits of. If not provided, uses the default branch of the repository. If a commit SHA is provided, will list commits up to that SHA. (string, 可选)
  - `since`: Only commits after this date will be returned (ISO 8601 format: YYYY-MM-DDTHH:MM:SSZ or YYYY-MM-DD) (string, 可选)
  - `until`: Only commits before this date will be returned (ISO 8601 format: YYYY-MM-DDTHH:MM:SSZ or YYYY-MM-DD) (string, 可选)

- **list_releases** - List releases
  - **所需 OAuth Scopes**：`repo`
  - `fields`: Subset of fields to return for each release. If omitted, all fields are returned. Use this to reduce response size when you only need specific fields; omitting 'body' in particular drops the largest per-release data. (string[], 可选)
  - `owner`: Repository owner (string, 必需)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `repo`: Repository name (string, 必需)

- **list_repository_collaborators** - List repository collaborators
  - **所需 OAuth Scopes**：`repo`
  - `affiliation`: Filter by affiliation. Can be one of: 'outside' (outside collaborators), 'direct' (all with permissions regardless of org membership), 'all' (all collaborators). Default: 'all' (string, 可选)
  - `owner`: Repository owner (string, 必需)
  - `page`: Page number for pagination (default 1, min 1) (number, 可选)
  - `perPage`: Results per page for pagination (default 30, min 1, max 100) (number, 可选)
  - `repo`: Repository name (string, 必需)

- **list_tags** - List tags
  - **所需 OAuth Scopes**：`repo`
  - `owner`: Repository owner (string, 必需)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `repo`: Repository name (string, 必需)

- **push_files** - Push files to repository
  - **所需 OAuth Scopes**：`repo`
  - `branch`: Branch to push to (string, 必需)
  - `files`: Array of file objects to push, each object with path (string) and content (string) (object[], 必需)
  - `message`: Commit message (string, 必需)
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)

- **search_code** - Search code
  - **所需 OAuth Scopes**：`repo`
  - `fields`: Subset of fields to return for each code search result. If omitted, all fields are returned. Use this to reduce response size when you only need specific fields; omitting 'repository' and 'text_matches' in particular drops the largest per-result data. (string[], 可选)
  - `order`: Sort order for results (string, 可选)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `query`: Search query (GitHub code search REST). Implicit AND between terms; supports `OR`, `NOT`, and `"quoted phrase"` for exact match. Qualifiers: `repo:owner/repo`, `org:`, `user:`, `language:`, `path:dir` (prefix match), `filename:exact.ext`, `extension:`, `in:file`, `in:path`, `size:`, `is:archived`, `is:fork`. Max 256 chars. Examples: `WithContext language:go org:github`; `"package main" repo:o/r`; `func extension:go path:cmd repo:o/r`; `NOT TODO language:go repo:o/r`. (string, 必需)
  - `sort`: Sort field ('indexed' only) (string, 可选)

- **search_commits** - Search commits
  - **所需 OAuth Scopes**：`repo`
  - `order`: Sort order (string, 可选)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `query`: Commit search query (GitHub commit search REST). Searches commit messages on the default branch only. Scope the search with `repo:owner/repo`, `org:`, or `user:` (queries without a scope qualifier match across all of GitHub and are usually not what you want). Other qualifiers: `author:`, `committer:`, `author-name:`, `committer-name:`, `author-email:`, `committer-email:`, `author-date:`, `committer-date:` (supports `>`, `<`, `>=`, `<=`, and `YYYY-MM-DD..YYYY-MM-DD` ranges), `merge:true|false`, `hash:`, `tree:`, `parent:`, `is:public`. Examples: `repo:owner/repo fix panic`; `org:github author:defunkt committer-date:>=2024-01-01`; `"refactor cache" repo:o/r`; `hash:abc1234 repo:o/r`. (string, 必需)
  - `sort`: Sort by author or committer date (defaults to best match) (string, 可选)

- **search_repositories** - Search repositories
  - **所需 OAuth Scopes**：`repo`
  - `minimal_output`: Return minimal repository information (default: true). When false, returns full GitHub API repository objects. (boolean, 可选)
  - `order`: Sort order (string, 可选)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `query`: Repository search query. Examples: 'machine learning in:name stars:>1000 language:python', 'topic:react', 'user:facebook'. Supports advanced search syntax for precise filtering. (string, 必需)
  - `sort`: Sort repositories by field, defaults to best match (string, 可选)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/shield-lock-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/shield-lock-light.png"><img src="pkg/octicons/icons/shield-lock-light.png" width="20" height="20" alt="shield-lock"></picture> Secret Protection</summary>

- **get_secret_scanning_alert** - Get secret scanning alert
  - **所需 OAuth Scopes**：`security_events`
  - **可接受 OAuth Scopes**：`repo`, `security_events`
  - `alertNumber`: The number of the alert. (number, 必需)
  - `owner`: The owner of the repository. (string, 必需)
  - `repo`: The name of the repository. (string, 必需)

- **list_secret_scanning_alerts** - List secret scanning alerts
  - **所需 OAuth Scopes**：`security_events`
  - **可接受 OAuth Scopes**：`repo`, `security_events`
  - `owner`: The owner of the repository. (string, 必需)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `repo`: The name of the repository. (string, 必需)
  - `resolution`: Filter by resolution (string, 可选)
  - `secret_type`: A comma-separated list of secret types to return. All default secret patterns are returned. To return generic patterns, pass the token name(s) in the parameter. (string, 可选)
  - `state`: Filter by state (string, 可选)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/shield-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/shield-light.png"><img src="pkg/octicons/icons/shield-light.png" width="20" height="20" alt="shield"></picture> Security Advisories</summary>

- **get_global_security_advisory** - Get a global security advisory
  - **所需 OAuth Scopes**：`security_events`
  - **可接受 OAuth Scopes**：`repo`, `security_events`
  - `ghsaId`: GitHub Security Advisory ID (format: GHSA-xxxx-xxxx-xxxx). (string, 必需)

- **list_global_security_advisories** - List global security advisories
  - **所需 OAuth Scopes**：`security_events`
  - **可接受 OAuth Scopes**：`repo`, `security_events`
  - `affects`: Filter advisories by affected package or version (e.g. "package1,package2@1.0.0"). (string, 可选)
  - `cveId`: Filter by CVE ID. (string, 可选)
  - `cwes`: Filter by Common Weakness Enumeration IDs (e.g. ["79", "284", "22"]). (string[], 可选)
  - `ecosystem`: Filter by package ecosystem. (string, 可选)
  - `ghsaId`: Filter by GitHub Security Advisory ID (format: GHSA-xxxx-xxxx-xxxx). (string, 可选)
  - `isWithdrawn`: Whether to only return withdrawn advisories. (boolean, 可选)
  - `modified`: Filter by publish or update date or date range (ISO 8601 date or range). (string, 可选)
  - `published`: Filter by publish date or date range (ISO 8601 date or range). (string, 可选)
  - `severity`: Filter by severity. (string, 可选)
  - `type`: Advisory type. (string, 可选)
  - `updated`: Filter by update date or date range (ISO 8601 date or range). (string, 可选)

- **list_org_repository_security_advisories** - List org repository security advisories
  - **所需 OAuth Scopes**：`security_events`
  - **可接受 OAuth Scopes**：`repo`, `security_events`
  - `direction`: Sort direction. (string, 可选)
  - `org`: The organization login. (string, 必需)
  - `sort`: Sort field. (string, 可选)
  - `state`: Filter by advisory state. (string, 可选)

- **list_repository_security_advisories** - List repository security advisories
  - **所需 OAuth Scopes**：`security_events`
  - **可接受 OAuth Scopes**：`repo`, `security_events`
  - `direction`: Sort direction. (string, 可选)
  - `owner`: The owner of the repository. (string, 必需)
  - `repo`: The name of the repository. (string, 必需)
  - `sort`: Sort field. (string, 可选)
  - `state`: Filter by advisory state. (string, 可选)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/star-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/star-light.png"><img src="pkg/octicons/icons/star-light.png" width="20" height="20" alt="star"></picture> Stargazers</summary>

- **list_starred_repositories** - List starred repositories
  - **所需 OAuth Scopes**：`repo`
  - `direction`: The direction to sort the results by. (string, 可选)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `sort`: How to sort the results. Can be either 'created' (when the repository was starred) or 'updated' (when the repository was last pushed to). (string, 可选)
  - `username`: Username to list starred repositories for. Defaults to the authenticated user. (string, 可选)

- **star_repository** - Star repository
  - **所需 OAuth Scopes**：`repo`
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)

- **unstar_repository** - Unstar repository
  - **所需 OAuth Scopes**：`repo`
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/people-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/people-light.png"><img src="pkg/octicons/icons/people-light.png" width="20" height="20" alt="people"></picture> Users</summary>

- **search_users** - Search users
  - **所需 OAuth Scopes**：`repo`
  - `order`: Sort order (string, 可选)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `query`: User search query. Examples: 'john smith', 'location:seattle', 'followers:>100'. Search is automatically scoped to type:user. (string, 必需)
  - `sort`: Sort users by number of followers or repositories, or when the person joined GitHub. (string, 可选)

</details>
<!-- END AUTOMATED TOOLS -->

### 远程 GitHub MCP Server 中的额外 Tools

<details>

<summary>Copilot</summary>

- **create_pull_request_with_copilot** - 使用 GitHub Copilot coding agent 执行任务
  - `owner`: 仓库 owner。可以推测 owner，但继续前应与用户确认。（string，必需）
  - `repo`: 仓库名称。可以推测仓库名称，但继续前应与用户确认。（string，必需）
  - `problem_statement`: 要执行任务的详细描述（例如“实现功能 X”“修复 bug Y”等）。（string，必需）
  - `title`: 将创建的 pull request 的标题。（string，必需）
  - `base_ref`: agent 开始工作的 Git reference（例如 branch）。未指定时默认为仓库的默认 branch。（string，可选）

</details>

<details>

<summary>Copilot Spaces</summary>

- **Authentication 说明**
  - classic PAT scope filtering 不会隐藏 fine-grained PATs，因此即使 token 无法使用这些 tools，它们仍可能显示。
  - 对 org-owned spaces，fine-grained PATs 必须安装在拥有该 space 的 organization 上，并包含 `organization_copilot_spaces: read`。
  - 如果 org-owned space 包含 repository-backed resources，token 还必须有权访问每个被引用的仓库，否则该 space 可能被视为未找到。

- **get_copilot_space** - 获取 Copilot Space
  - `owner`: space 的 owner。（string，必需）
  - `name`: space 的名称。（string，必需）

- **list_copilot_spaces** - 列出 Copilot Spaces

</details>

<details>

<summary>GitHub Support Docs 搜索</summary>

- **github_support_docs_search** - 获取与回答 GitHub product 和 support questions 相关的文档。Support topics 包括：GitHub Actions Workflows、Authentication、GitHub Support Inquiries、Pull Request Practices、Repository Maintenance、GitHub Pages、GitHub Packages、GitHub Discussions、Copilot Spaces。
  - `query`: 用户就其需要解答的问题提供的输入。这是最新的原始、未经编辑的用户消息。必须始终保持用户消息原样，绝不可修改。（string，必需）

</details>

## Read-Only Mode

要以 read-only mode 运行 server，可使用 `--read-only` flag。这只会提供 read-only tools，从而阻止对仓库、issues、pull requests 等的任何修改。

```bash
./github-mcp-server --read-only
```

使用 Docker 时，可将 read-only mode 作为 environment variable 传入：

```bash
docker run -i --rm \
  -e GITHUB_PERSONAL_ACCESS_TOKEN=<your-token> \
  -e GITHUB_READ_ONLY=1 \
  ghcr.io/github/github-mcp-server
```

## Lockdown Mode

Lockdown mode 限制 server 从 public repositories 提供的内容。启用后，server 会检查每个项目的 author 是否拥有对仓库的 push access。Private repositories 不受影响，collaborators 保持对其自身内容的完全访问权限。

```bash
./github-mcp-server --lockdown-mode
```

使用 Docker 运行时，请设置相应的 environment variable：

```bash
docker run -i --rm \
  -e GITHUB_PERSONAL_ACCESS_TOKEN=<your-token> \
  -e GITHUB_LOCKDOWN_MODE=1 \
  ghcr.io/github/github-mcp-server
```

Lockdown mode 的行为取决于调用的 tool。

当 author 缺少 push access 时，以下 tools 会返回 error：

- `issue_read:get`
- `pull_request_read:get`

以下 tools 会过滤掉缺少 push access 的 users 的内容：

- `issue_read:get_comments`
- `issue_read:get_sub_issues`
- `pull_request_read:get_comments`
- `pull_request_read:get_review_comments`
- `pull_request_read:get_reviews`

## i18n / 覆盖 Descriptions

可在与 binary 相同的 directory 中创建 `github-mcp-server-config.json` 文件，以覆盖 tools 的 descriptions。

该文件应包含 JSON object，其中 tool names 为 keys，新 descriptions 为 values。例如：

```json
{
  "TOOL_ADD_ISSUE_COMMENT_DESCRIPTION": "an alternative description",
  "TOOL_CREATE_BRANCH_DESCRIPTION": "Create a new branch in a GitHub repository"
}
```

可通过使用 `--export-translations` flag 运行 binary，导出当前 translations。

该 flag 会保留您所做的 translations/overrides，同时添加自上次导出后加入 binary 的所有新 translations。

```sh
./github-mcp-server --export-translations
cat github-mcp-server-config.json
```

也可以使用 ENV vars 覆盖 descriptions。environment variable names 与 JSON file 中的 keys 相同，前缀为 `GITHUB_MCP_`，且全部使用大写。

例如，若要覆盖 `TOOL_ADD_ISSUE_COMMENT_DESCRIPTION` tool，可设置以下 environment variable：

```sh
export GITHUB_MCP_TOOL_ADD_ISSUE_COMMENT_DESCRIPTION="an alternative description"
```

### 覆盖 Server Name 和 Title

同一覆盖机制可用于自定义 initialization response 中 MCP server 的 `name` 和 `title` fields。运行多个 GitHub MCP Server instances（例如一个用于 github.com，另一个用于 GitHub Enterprise Server）时，这可让 agents 区分它们。

| Key | Environment Variable | Default |
|-----|---------------------|---------|
| `SERVER_NAME` | `GITHUB_MCP_SERVER_NAME` | `github-mcp-server` |
| `SERVER_TITLE` | `GITHUB_MCP_SERVER_TITLE` | `GitHub MCP Server` |

例如，若要为 GitHub Enterprise Server 配置 server instance：

```json
{
  "SERVER_NAME": "ghes-mcp-server",
  "SERVER_TITLE": "GHES MCP Server"
}
```

或使用 environment variables：

```sh
export GITHUB_MCP_SERVER_NAME="ghes-mcp-server"
export GITHUB_MCP_SERVER_TITLE="GHES MCP Server"
```

## Library 使用

目前应将此 module 导出的 Go API 视为不稳定，且可能发生 breaking changes。未来可能提供稳定性保证；如有值得支持的使用场景，请提交 issue。

## 贡献

欢迎贡献。在创建 pull request 前，请阅读[贡献指南](CONTRIBUTING.md)，了解 setup、testing、linting 和 documentation generation 说明。

## 支持

使用 GitHub MCP Server 时如需帮助，请参阅[支持指南](SUPPORT.md)。如果发现 bug 或希望请求 feature，请先搜索现有 issues，再创建新的 issue。

## 安全

请不要通过 public issues 报告 security vulnerabilities。请遵循[安全策略](SECURITY.md)中的说明，以负责任的方式报告 vulnerabilities。

## 许可证

本项目采用 MIT open source license。完整条款请参阅 [MIT](./LICENSE)。
