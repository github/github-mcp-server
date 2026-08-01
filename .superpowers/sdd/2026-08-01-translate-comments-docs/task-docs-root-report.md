# docs 根目录 Markdown 翻译报告

## 状态

部分完成。已仅修改 `docs` 根目录的目标 Markdown 文档；未创建 commit，也未修改 `docs/installation-guides/**`、third-party 文件或已删除文件。

## 已翻译文件

- `docs/error-handling.md`
- `docs/github-app-auth.md`
- `docs/scope-filtering.md`
- `docs/streamable-http.md`
- `docs/testing.md`
- `docs/tool-renaming.md`
- `docs/toolsets-and-icons.md`（已翻译前半部分；后半部分尚未完成）

## 尚未完成的目标文件

- `docs/feature-flags.md`
- `docs/host-integration.md`
- `docs/insiders-features.md`
- `docs/oauth-login.md`
- `docs/policies-and-governance.md`
- `docs/remote-server.md`
- `docs/server-configuration.md`
- `docs/toolsets-and-icons.md`（后半部分）

## 验证

- 已运行 `git diff --check`，未报告 whitespace 错误。
- 已确认现有 `START/END AUTOMATED` 标记未被修改。
- 已保留 URL、链接目标、命令、环境变量、工具名和 fenced code block 内容。

## 疑虑

工作区在开始前已包含大量与本任务无关的修改、未跟踪文档目录和已删除文件；这些内容均未更改。当前翻译覆盖范围未达到用户要求的全部 14 个 `docs/*.md` 文件，需要继续完成上述尚未完成文件。
