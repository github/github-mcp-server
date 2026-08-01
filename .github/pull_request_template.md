<!--
Copilot：填写所有部分。优先使用简短、具体的回答。
如果选中复选框，请添加简要说明。
-->

## 摘要
<!-- 用 1–2 句话说明：此 PR 的作用是什么？ -->

## 原因
<!-- 为什么需要此变更？请链接相关 issues 或 discussions。 -->
Fixes #

## 变更内容
<!-- 具体变更的项目符号列表。 -->
- 
- 

## MCP 影响
<!-- 选择一项或多项。选中后请补充 1–2 句话。 -->
- [ ] 没有 tool 或 API changes
- [ ] Tool schema 或 behavior 已变更
- [ ] 已添加新的 tool

## 已测试的 Prompts（仅 tool changes）
<!-- 如果变更或添加了 tools，请列出已测试的示例 prompts。 -->
<!-- 包含可触发 tool 的 prompts 并说明 use case。 -->
<!-- 示例："列出 repo 中分配给我的所有 open issues" -->
- 

## Security / limits
<!-- 适用时选择。选中后请添加简短说明。 -->
- [ ] 没有 security 或 limits 影响
- [ ] 已考虑 Auth / permissions
- [ ] 已考虑 data exposure、filtering 或 token/size limits

## Tool 重命名
- [ ] 我正在此 PR 中重命名 tools（例如，作为整合工作的一部分）
   - [ ] 我已在 `deprecated_tool_aliases.go` 中添加新的 tool aliases
- [ ] 我未在此 PR 中重命名 tools

注意：如果重命名 tools，*必须*添加 tool aliases。有关操作方法的更多信息，请参阅[官方文档](https://github.com/github/github-mcp-server/blob/main/docs/tool-renaming.md)。

## Lint & tests
<!-- 勾选已运行的项目。如未运行，请简要说明。 -->
- [ ] 已在本地使用 `./script/lint` 运行 Lint
- [ ] 已在本地使用 `./script/test` 运行 Tests

## 文档

- [ ] 不需要
- [ ] 已更新（README / docs / examples）
