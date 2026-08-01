# 工具集和图标

本文档介绍如何在 GitHub MCP Server 中使用工具集和图标。

## 工具集概述

工具集是相关工具的逻辑分组。每个工具集的 metadata 都在 `pkg/github/tools.go` 中定义：

```go
ToolsetMetadataRepos = inventory.ToolsetMetadata{
    ID:          "repos",
    Description: "GitHub Repository related tools",
    Default:     true,
    Icon:        "repo",
}
```

### 工具集字段

| 字段 | 类型 | 描述 |
|-------|------|-------------|
| `ID` | `ToolsetID` | 用于 URL 和 CLI flag 的唯一标识符（如 `repos`、`issues`） |
| `Description` | `string` | 文档中显示的可读描述 |
| `Default` | `bool` | 是否默认启用该工具集 |
| `Icon` | `string` | 用于 MCP 客户端视觉呈现的 Octicon 名称 |

## 向工具集添加图标

图标可帮助用户在兼容 MCP 的客户端中快速识别工具集。所有图标均使用 [Primer Octicons](https://primer.style/foundations/icons)。

### 第 1 步：选择 Octicon

浏览 [Octicon 图库](https://primer.style/foundations/icons)，选择合适的图标。请使用不带尺寸后缀的基础名称，例如 `repo`，而不是 `repo-16`。

### 第 2 步：将图标添加到所需图标列表

图标定义在 `pkg/octicons/required_icons.txt` 中，这是决定应嵌入哪些图标的唯一事实来源：

```text
# Required icons for the GitHub MCP Server
# Add new icons below (one per line)
repo
issue-opened
git-pull-request
your-new-icon  # Add your icon here
```

### 第 3 步：获取图标文件

运行 fetch-icons script 下载并转换图标：

```bash
# Fetch a specific icon
script/fetch-icons your-new-icon

# Or fetch all required icons
script/fetch-icons
```

该 script 会：

- 从 [Primer Octicons](https://github.com/primer/octicons) 下载 24px SVG
- 转换为浅色 theme 的 PNG（浅色背景使用深色图标）
- 转换为深色 theme 的 PNG（深色背景使用白色图标）
- 将两个 variant 保存到 `pkg/octicons/icons/`

**要求：**该 script 需要 `rsvg-convert`：

- Ubuntu/Debian：`sudo apt-get install librsvg2-bin`
- macOS：`brew install librsvg`

### 第 4 步：更新工具集 metadata

在工具集定义中添加或更新 `Icon` 字段：

```go
// In pkg/github/tools.go
ToolsetMetadataRepos = inventory.ToolsetMetadata{
    ID:          "repos",
    Description: "GitHub Repository related tools",
    Default:     true,
    Icon:        "repo",  // Add this line
}
```

### 第 5 步：重新生成文档

运行文档生成器以更新所有 Markdown 文件：

```bash
go run ./cmd/github-mcp-server generate-docs
```

这会更新以下位置的图标：

- `README.md`：工具集表格和工具章节标题
- `docs/remote-server.md`：远程工具集表格

## 仅远程可用的工具集

部分工具集只在远程 GitHub MCP Server（托管于 `api.githubcopilot.com`）中可用。这些工具集及其图标定义在 `pkg/github/tools.go` 中，但不会注册到本地 server：

```go
// Remote-only toolsets
ToolsetMetadataCopilot = inventory.ToolsetMetadata{
    ID:          "copilot",
    Description: "Copilot related tools",
    Icon:        "copilot",
}
```

`RemoteOnlyToolsets()` 函数会返回这些工具集列表，用于生成文档。

添加新的仅远程可用工具集时：

1. 在 `pkg/github/tools.go` 中添加 metadata 定义
2. 将其添加到 `RemoteOnlyToolsets()` 返回的 slice
3. 重新生成文档

## 工具图标继承

各工具会继承其父工具集的图标。工具注册到工具集时，会自动设置其图标：

```go
// In pkg/inventory/server_tool.go
toolCopy.Icons = tool.Toolset.Icons()
```

这表示只需在工具集上设置一次图标，该工具集中的所有工具都会显示相同图标。

## MCP 中图标的工作方式

MCP protocol 通过 `icons` 字段支持工具图标。我们提供两种格式：

1. **Data URIs** - 嵌入到工具定义中的 Base64 编码 PNG 图片
2. **浅色/深色 variants** - 同时提供两种 theme variants，以便正确显示

`octicons.Icons()` 函数会生成兼容 MCP 的图标对象：

```go
// Returns []mcp.Icon with both light and dark variants
icons := octicons.Icons("repo")
```

## 现有工具集图标

| 工具集 | Octicon 名称 |
|---------|--------------|
| Context | `person` |
| Repositories | `repo` |
| Issues | `issue-opened` |
| Pull Requests | `git-pull-request` |
| Git | `git-branch` |
| Users | `people` |
| Organizations | `organization` |
| Actions | `workflow` |
| Code Quality | `code-square` |
| Code Security | `codescan` |
| Secret Protection | `shield-lock` |
| Dependabot | `dependabot` |
| Discussions | `comment-discussion` |
| Gists | `logo-gist` |
| Security Advisories | `shield` |
| Projects | `project` |
| Labels | `tag` |
| Stargazers | `star` |
| Notifications | `bell` |
| Copilot | `copilot` |
| Support Search | `book` |

## 故障排除

### 文档中未显示图标

1. 确认 `pkg/octicons/icons/` 中存在带 `-light.png` 和 `-dark.png` 后缀的 PNG 文件
2. 运行 `go run ./cmd/github-mcp-server generate-docs` 重新生成文档
3. 检查工具集 metadata 上是否设置了 `Icon` 字段

### MCP 客户端中未显示图标

1. 确认客户端支持 MCP 工具图标
2. 检查 octicons package 是否正确生成 base64 data URIs
3. 确认图标名称匹配 `pkg/octicons/icons/` 中的文件

## CI 验证

CI 中会运行以下测试，以尽早发现图标问题：

### `pkg/octicons.TestEmbeddedIconsExist`

验证 `pkg/octicons/required_icons.txt` 中列出的所有图标都有对应的嵌入 PNG 文件。

### `pkg/github.TestAllToolsetIconsExist`

验证所有工具集 `Icon` 字段都引用了已正确嵌入的图标。

### `pkg/github.TestToolsetMetadataHasIcons`

确保所有工具集都设置了 `Icon` 字段。

如果这些测试失败：

1. 将缺失图标添加到 `pkg/octicons/required_icons.txt`
2. 运行 `script/fetch-icons` 下载图标
3. 提交新的图标文件
