# Translate Comments And Docs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将当前项目的一手英文文档和源码注释翻译成中文，同时保留专有名词、函数名称、字段名、配置键和代码示例。

**Architecture:** 先明确排除第三方许可证、生成快照和用户已有删除项，再按 Markdown 文档与源码注释两类处理。文档保持原有标题层级、链接、代码块和自动生成标记；源码只改注释文本，不改字符串字面量、API schema、测试数据或行为。

**Tech Stack:** Go, TypeScript/TSX, Markdown, PowerShell, gofmt, go test.

## Global Constraints

- 保留 GitHub、MCP、OAuth、PAT、Copilot、VS Code、Docker、CLI、API 等专有名词。
- 保留函数名称、字段名称、工具名称、环境变量、命令、路径、URL 和代码块内容。
- 不恢复或修改当前已有删除项：`CODE_OF_CONDUCT.md`、`CONTRIBUTING.md`、`LICENSE`、`SUPPORT.md`、`third-party-licenses.*.md`。
- 不翻译 `third-party/**`、`pkg/github/__toolsnaps__/**` 和许可证法律文本。
- 修改 Go 注释后运行 `gofmt`；最终运行可用的验证命令并报告基线失败。

---

### Task 1: Translate First-Party Markdown

**Files:**
- Modify: `README.md`
- Modify: `SECURITY.md`
- Modify: `.github/**/*.md`
- Modify: `docs/**/*.md`
- Modify: `cmd/mcpcurl/README.md`
- Modify: `e2e/README.md`

**Interfaces:**
- Consumes: 当前 Markdown 文档结构、链接、代码块和自动生成区域标记。
- Produces: 中文 Markdown 文档，代码块、链接目标、表格列数、HTML 注释标记保持有效。

- [ ] **Step 1: Identify Markdown files**

Run: `rg --files -g "*.md" -g "!third-party/**" -g "!pkg/github/__toolsnaps__/**"`

- [ ] **Step 2: Translate prose**

Translate headings, paragraphs, list item prose and table headers into Chinese. Preserve inline code, fenced code blocks, URLs, env vars, command names, tool names and schema field names.

- [ ] **Step 3: Check generated markers**

Run: `Select-String -Path docs\*.md,README.md -Pattern "START AUTOMATED|END AUTOMATED"`

- [ ] **Step 4: Review diff**

Run: `git diff -- README.md SECURITY.md .github docs cmd/mcpcurl/README.md e2e/README.md`

### Task 2: Translate Source Comments

**Files:**
- Modify: `cmd/**/*.go`
- Modify: `internal/**/*.go`
- Modify: `pkg/**/*.go`
- Modify: `ui/**/*.ts`
- Modify: `ui/**/*.tsx`
- Modify: `ui/**/*.mjs`
- Modify: `internal/oauth/templates/*.html`

**Interfaces:**
- Consumes: Existing comments only: `//`, `/* */`, JSX comments and HTML comments.
- Produces: 中文源码注释，exported Go doc comments still start with exported identifier where required by lint conventions.

- [ ] **Step 1: Extract comment candidates**

Use search tooling to find English comments in Go, TS/TSX, MJS and HTML, excluding `third-party/**` and snapshots.

- [ ] **Step 2: Translate comments only**

Change only natural language comment text. Leave identifiers, string literals, struct tags, test names, schema descriptions and request/response fixtures unchanged.

- [ ] **Step 3: Format Go files**

Run: `gofmt` on modified Go files.

- [ ] **Step 4: Check remaining English comments**

Search modified files for comment lines containing English prose and review any remaining hits.

### Task 3: Verify

**Files:**
- No new production files.

**Interfaces:**
- Consumes: Modified documentation and comments.
- Produces: Verification report with formatter/test results and any pre-existing baseline failures.

- [ ] **Step 1: Run formatting**

Run: `gofmt` for changed Go files.

- [ ] **Step 2: Run tests**

Run: `go test ./...`

- [ ] **Step 3: Report status**

Summarize changed scope, tests, and known pre-existing failures.
