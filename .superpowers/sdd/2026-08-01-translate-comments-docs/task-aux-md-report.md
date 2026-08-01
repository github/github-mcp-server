# 一手辅助 Markdown 翻译报告

日期：2026-08-01

## 状态

已完成指定范围内英文自然语言的中文翻译；未创建 commit。

## 修改文件

- `SECURITY.md`
- `cmd/mcpcurl/README.md`
- `e2e/README.md`
- `.github/copilot-instructions.md`
- `.github/pull_request_template.md`
- `.github/agents/go-sdk-tool-migrator.md`
- `.github/ISSUE_TEMPLATE/bug_report.md`
- `.github/ISSUE_TEMPLATE/feature_request.md`
- `.github/ISSUE_TEMPLATE/insiders-feedback.md`

## 保留内容

- 保留 GitHub、MCP、OAuth、PAT、Copilot、VS Code、Docker、CLI、API 等专有名词。
- 保留工具名、字段名、环境变量、命令、URL、链接目标、代码块内容，以及 issue/PR 模板字段结构。
- 未修改 third-party、快照文件或已删除文件。

## 验证

- `git diff --check` 通过。
- 以上九个翻译文件均通过严格 UTF-8 解码检查。

## 疑虑

无。工作区原有的其他未提交改动未被修改。
