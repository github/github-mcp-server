# 工具重命名指南

如何安全地重命名 MCP 工具而不破坏现有用户配置。

## 概述

工具重命名后，MCP 配置中使用旧工具名称的用户（例如远程 MCP server 的 `X-MCP-Tools` header 或本地 MCP server 的 `--tools` flag）通常会收到错误。
弃用别名系统通过静默将旧工具名称解析为新的规范名称，来保持向后兼容性。

这使我们能够安全地重命名工具，而不会为在 server 配置中硬编码这些工具的用户引入破坏性变更。

## 快速步骤

1. 在代码中**重命名工具**（通常还需要更新工具注册、测试和 toolsnaps 等内容）。
2. 在 [pkg/github/deprecated_tool_aliases.go](../pkg/github/deprecated_tool_aliases.go) 中**添加弃用别名**：
   ```go
   var DeprecatedToolAliases = map[string]string{
       "old_tool_name": "new_tool_name",
   }
   ```
3. **更新文档**（README 等），引用新的规范名称

完成后，server 会静默将旧名称解析为新名称。这同时适用于本地和远程 MCP server。

## 示例

若将 `get_issue` 重命名为 `issue_read`：

```go
var DeprecatedToolAliases = map[string]string{
    "get_issue": "issue_read",
}
```

使用以下配置的用户：
```json
{
  "--tools": "get_issue,get_file_contents"
}
```

将注册 `issue_read` 和 `get_file_contents` 工具，不会出现错误。

## 当前弃用项

<!-- START AUTOMATED ALIASES -->
| 旧名称 | 新名称 |
|----------|----------|
| `add_project_item` | `projects_write` |
| `cancel_workflow_run` | `actions_run_trigger` |
| `delete_project_item` | `projects_write` |
| `delete_workflow_run_logs` | `actions_run_trigger` |
| `download_workflow_run_artifact` | `actions_get` |
| `get_project` | `projects_get` |
| `get_project_field` | `projects_get` |
| `get_project_item` | `projects_get` |
| `get_workflow` | `actions_get` |
| `get_workflow_job` | `actions_get` |
| `get_workflow_job_logs` | `actions_get` |
| `get_workflow_run` | `actions_get` |
| `get_workflow_run_logs` | `actions_get` |
| `get_workflow_run_usage` | `actions_get` |
| `list_project_fields` | `projects_list` |
| `list_project_items` | `projects_list` |
| `list_projects` | `projects_list` |
| `list_workflow_jobs` | `actions_list` |
| `list_workflow_run_artifacts` | `actions_list` |
| `list_workflow_runs` | `actions_list` |
| `list_workflows` | `actions_list` |
| `rerun_failed_jobs` | `actions_run_trigger` |
| `rerun_workflow_run` | `actions_run_trigger` |
| `run_workflow` | `actions_run_trigger` |
| `update_project_item` | `projects_write` |
<!-- END AUTOMATED ALIASES -->
