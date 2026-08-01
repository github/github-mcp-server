# GitHub MCP Server - Copilot 说明

## 项目概述

这是 **GitHub MCP Server**，一个将 AI 工具连接到 GitHub 平台的 Model Context Protocol (MCP) server。它使 AI agents 能够通过自然语言管理 repositories、issues、pull requests、workflows 等。

**关键信息：**
- **语言：**Go 1.24+（约 3.8 万行代码）
- **类型：**带有 CLI 接口的 MCP server application
- **主要 package：**github-mcp-server（stdio MCP server，**这是主要关注点**）
- **次要 package：**mcpcurl（testing utility，不要破坏它，但它不是优先事项）
- **框架：**MCP protocol 使用 modelcontextprotocol/go-sdk，GitHub API 使用 google/go-github
- **规模：**约 60MB repository、70 个 Go 文件
- **Library 用途：**此 repository 也被 remote server 作为 library 使用。其他 repositories 可能调用的 functions 即使内部不需要，也应 export（首字母大写）。保留现有 export patterns。

**代码质量标准：**
- **热门开源 Repository**：代码质量和清晰度要求很高
- **可理解性优先**：代码必须让广泛受众易于理解
- **干净的 Commits**：原子化、聚焦的改动，并附有清晰消息
- **结构**：始终保持或改善，绝不降低
- **代码优先于注释**：优先使用自说明代码；仅在必要时添加注释

## 关键构建与验证步骤

### 必需命令（Commit 前运行）

**在使用 report_progress 或完成工作前，务必严格按以下顺序运行这些命令：**

1. **格式化代码：**`script/lint`（运行 `gofmt -s -w .`，然后运行 `golangci-lint`）
2. **运行测试：**`script/test`（运行 `go test -race ./...`）
3. **更新文档：**`script/generate-docs`（如果修改了 MCP tools/toolsets）

**这些命令很快：**Lint 约 1 秒、Tests 约 1 秒（已缓存）、Build 约 1 秒

### 修改 MCP Tools/Endpoints 时

如果更改任何 MCP tool definitions 或 schemas：
1. 使用 `UPDATE_TOOLSNAPS=true go test ./...` 运行测试以更新 toolsnaps
2. Commit `pkg/github/__toolsnaps__/` 中更新后的 `.snap` files
3. 运行 `script/generate-docs` 以更新 README.md
4. Toolsnaps 记录 API surface，并确保改动是有意的

### 常用构建命令

```bash
# 下载 dependencies（很少需要，通常已缓存）
go mod download

# 构建 server binary
go build -v ./cmd/github-mcp-server

# 运行 server
./github-mcp-server stdio

# 运行特定 package tests
go test ./pkg/github -v

# 运行特定 test
go test ./pkg/github -run TestGetMe
```

## 项目结构

### 目录布局

```
.
├── cmd/
│   ├── github-mcp-server/    # Main MCP server entry point (PRIMARY FOCUS)
│   └── mcpcurl/              # MCP testing utility (secondary - don't break it)
├── pkg/                      # Public API packages
│   ├── github/               # GitHub API MCP tools implementation
│   │   └── __toolsnaps__/    # Tool schema snapshots (*.snap files)
│   ├── toolsets/             # Toolset configuration & management
│   ├── errors/               # Error handling utilities
│   ├── sanitize/             # HTML/content sanitization
│   ├── log/                  # Logging utilities
│   ├── raw/                  # Raw data handling
│   ├── buffer/               # Buffer utilities
│   └── translations/         # i18n translation support
├── internal/                 # Internal implementation packages
│   ├── ghmcp/                # GitHub MCP server core logic
│   ├── githubv4mock/         # GraphQL API mocking for tests
│   ├── toolsnaps/            # Toolsnap validation system
│   └── profiler/             # Performance profiling
├── e2e/                      # End-to-end tests (require GitHub PAT)
├── script/                   # Build and maintenance scripts
├── docs/                     # Documentation
├── .github/workflows/        # CI/CD workflows
└── [config files]            # See below
```

### 关键配置文件

- **go.mod / go.sum：**Go module dependencies（Go 1.24.0+）
- **.golangci.yml：**Linter configuration（v2 格式，启用约 15 个 linters）
- **Dockerfile：**Multi-stage build（golang:1.25.8-alpine → distroless）
- **server.json：**registry 的 MCP server metadata
- **.goreleaser.yaml：**Release automation config
- **.gitignore：**排除 bin/、dist/、vendor/、*.DS_Store、github-mcp-server binary

### 重要脚本（script/ 目录）

- **script/lint**：运行 `gofmt` + `golangci-lint`。Commit 前**必须运行**
- **script/test**：运行 `go test -race ./...`（完整 test suite）
- **script/generate-docs**：更新 README.md tool documentation。tool 改动后运行
- **script/licenses**：dependencies 改动时更新 third-party license files
- **script/licenses-check**：验证 license compliance（在 CI 中运行）
- **script/get-me**：get_me tool 的快速 test script
- **script/get-discussions**：discussions 的快速 test
- **script/tag-release**：**绝不使用此命令**，releases 单独管理

## GitHub Workflows (CI/CD)

除非另有说明，所有 workflows 都会在 push/PR 时运行，位于 `.github/workflows/`：

1. **go.yml**：在 ubuntu/windows/macos 上 Build 和 test。运行 `script/test` 并构建 binary
2. **lint.yml**：结合 actions/setup-go stable 运行 golangci-lint-action v2.5 (GitHub Action)
3. **docs-check.yml**：通过运行 generate-docs 并检查 git diff 验证 README.md 是否最新
4. **code-scanning.yml**：针对 Go 和 GitHub Actions 的 CodeQL security analysis
5. **license-check.yml**：运行 `script/licenses-check` 以验证 compliance
6. **docker-publish.yml**：将 container image 发布至 ghcr.io
7. **goreleaser.yml**：创建 releases（仅 main branch）
8. **registry-releaser.yml**：更新 MCP registry

**PR merge 前，以上全部必须通过。**如果 docs-check 失败，请运行 `script/generate-docs` 并 commit 改动。

## 测试指南

### Unit Tests

- 使用 `testify` 进行 assertions（关键检查使用 `require`，非阻塞检查使用 `assert`）
- Tests 位于与 implementation 同级的 `*_test.go` files 中（internal tests，而非 `_test` package）
- 使用 `go-github-mock` (REST) 或 `githubv4mock` (GraphQL) Mock GitHub API
- tools 的 test structure：
  1. 测试 tool snapshot
  2. 验证关键 schema properties（例如 ReadOnly annotation）
  3. Table-driven behavioral tests

### Toolsnaps (Tool Schema Snapshots)

- 每个 MCP tool 在 `pkg/github/__toolsnaps__/*.snap` 中都有 JSON schema snapshot
- 如果当前 schema 与 snapshot 不同，Tests 会失败（显示 diff）
- 有意改动后更新：`UPDATE_TOOLSNAPS=true go test ./...`
- **必须 commit 更新后的 .snap files**，它们记录 API changes
- 缺少 snapshots 会导致 CI failure

### End-to-End Tests

- 位于 `e2e/` 目录，文件为 `e2e_test.go`
- **需要 GitHub PAT token**，通常无法自行运行
- 运行方式：`GITHUB_MCP_SERVER_E2E_TOKEN=<token> go test -v --tags e2e ./e2e`
- Tests 通过 Docker container 与实时 GitHub API 交互
- **变更 MCP tools 时，保持 e2e tests 更新**
- 修改此目录中的 tests 时，**仅使用 e2e test style**
- 调试时：`GITHUB_MCP_SERVER_E2E_DEBUG=true` 在进程内运行（无需 Docker）

## 代码风格与 Linting

### Go 代码要求

- **带 simplify flag (-s) 的 gofmt**：由 `script/lint` 自动运行
- **golangci-lint** 启用以下 linters：
  - bodyclose, gocritic, gosec, makezero, misspell, nakedret, revive
  - errcheck, staticcheck, govet, ineffassign, unused
- 排除项：third_party/、builtin/、examples/、generated code

### Go 命名约定

- **identifiers 中的 Acronyms：**使用 `ID` 而非 `Id`、`API` 而非 `Api`、`URL` 而非 `Url`、`HTTP` 而非 `Http`
- 示例：`userID`、`getAPI`、`parseURL`、`HTTPClient`
- 适用于 variable names、function names、struct fields 等

### 代码模式

- **保持改动最小且聚焦**于要解决的特定问题
- **清晰优先于炫技**：代码必须让广泛受众易于理解
- **Atomic commits**：每个 commit 都应是完整、合乎逻辑的改动
- **保持或改善结构**：绝不降低代码组织质量
- behavioral testing 使用 table-driven tests
- 谨慎添加注释，代码应当自说明
- 遵循标准 Go conventions（Effective Go、Go proverbs）
- Commit 前**彻底测试改动**
- 其他 repos 可能作为 library 使用的 functions 应 export（首字母大写）

## 常见开发流程

### 添加新的 MCP Tool

1. 在 `pkg/github/` 中添加 tool implementation（例如 `foo_tools.go`）
2. 在 `pkg/github/` 或 `pkg/toolsets/` 的适当 toolset 中注册 tool
3. 按照 tool test pattern 编写 unit tests
4. 运行 `UPDATE_TOOLSNAPS=true go test ./...` 创建 snapshot
5. 运行 `script/generate-docs` 更新 README
6. Commit 前运行 `script/lint` 和 `script/test`
7. 如果 e2e tests 相关，则使用现有 test style 更新 `e2e/e2e_test.go`
8. 一同 Commit code、snapshots 和 README changes

### 修复 Bug

1. 编写能复现 Bug 的 failing test
2. 以最小改动修复 Bug
3. 验证 test 通过，现有 tests 仍然通过
4. 运行 `script/lint` 和 `script/test`
5. 如果 tool schema 已变更，更新 toolsnaps（参见上文）

### 更新 Dependencies

1. 更新 `go.mod`（例如使用 `go get -u ./...` 或手动更新）
2. 运行 `go mod tidy`
3. 运行 `script/licenses` 更新 license files
4. 运行 `script/test` 验证没有问题
5. Commit go.mod、go.sum 和 third-party-licenses* files

## 常见错误与解决方案

### CI 中出现“Documentation is out of date”

**修复：**运行 `script/generate-docs` 并 commit README.md changes

### Toolsnap mismatch failures

**修复：**运行 `UPDATE_TOOLSNAPS=true go test ./...` 并 commit 更新后的 .snap files

### Lint failures

**修复：**在本地运行 `script/lint`，它将自动格式化并显示问题。手动修复报告的问题。

### License check failures

**修复：**dependency changes 后运行 `script/licenses` 以重新生成 license files

### 变更 tool 后的 Test failures

**可能原因：**
1. 忘记更新 toolsnaps：使用 `UPDATE_TOOLSNAPS=true` 运行
2. 行为变更破坏了现有 tests：验证意图并修复 tests
3. Schema change 未反映到 test 中：更新 test expectations

## Environment Variables

- **GITHUB_PERSONAL_ACCESS_TOKEN**：server operation 和 e2e tests 所必需
- **GITHUB_HOST**：用于 GitHub Enterprise Server（以 `https://` 开头）
- **GITHUB_TOOLSETS**：以逗号分隔的 toolset list（覆盖 --toolsets flag）
- **GITHUB_READ_ONLY**：设置为 "1" 以启用 read-only mode
- **UPDATE_TOOLSNAPS**：运行 tests 更新 snapshots 时设置为 "true"
- **GITHUB_MCP_SERVER_E2E_TOKEN**：用于 e2e tests 的 Token
- **GITHUB_MCP_SERVER_E2E_DEBUG**：设置为 "true" 以进行进程内 e2e debugging

## 关键文件参考

### Root Directory Files
```
.dockerignore        - Docker build exclusions
.gitignore          - Git exclusions (includes bin/, dist/, vendor/, binaries)
.golangci.yml       - Linter configuration
.goreleaser.yaml    - Release automation
CODE_OF_CONDUCT.md  - Community guidelines
CONTRIBUTING.md     - Contribution guide (fork, clone, test, lint workflow)
Dockerfile          - Multi-stage Go build
LICENSE             - MIT license
README.md           - Main documentation (auto-generated sections)
SECURITY.md         - Security policy
SUPPORT.md          - Support resources
gemini-extension.json - Gemini CLI configuration
go.mod / go.sum     - Go dependencies
server.json         - MCP server registry metadata
```

### 主要入口点

`cmd/github-mcp-server/main.go`：CLI 使用 cobra，config 使用 viper，支持：
- `stdio` command（默认）：MCP stdio transport
- `generate-docs` command：Documentation generation
- Flags：--toolsets、--read-only、--gh-host、--log-file

## 重要提醒

1. **主要关注点：**本地 stdio MCP server（github-mcp-server），应在其上工作和测试
2. **REMOTE SERVER：**进行代码改动时忽略 remote server instructions（除非特别要求）。此 repo 被 remote server 作为 library 使用，因此其他 repos 可能调用的 functions 即使内部不需要，也应保持 export（首字母大写）。
3. **务必**优先信任这些 instructions，只有信息不完整或不正确时才搜索
4. **绝不**使用 `script/tag-release` 或 push tags
5. Commit Go code changes 前**绝不**跳过 `script/lint`
6. 变更 MCP tool schemas 时**务必**更新 toolsnaps
7. 修改 tools 后**务必**运行 `script/generate-docs`
8. 对于特定 test files，使用 `go test ./path -run TestName`，而非完整 suite
9. E2E tests 需要 PAT token，可能无法运行
10. Toolsnaps 是 API documentation，应严肃对待其 changes
11. Build/test/lint 都很快（每项约 1 秒），应频繁运行
12. docs-check 或 license-check 的 CI failures 有简单修复方式（运行脚本）
13. mcpcurl 是次要项目，不要破坏它，但它不是优先事项
