# Tool Renaming Guide

How to safely rename MCP tools without breaking existing user configurations.

## Overview

When tools are renamed, users who have the old tool name in their MCP configuration (for example, in `X-MCP-Tools` headers for the remote MCP server or `--tools` flags for the local MCP server) would normally get errors. 
The deprecation alias system allows us to maintain backward compatibility by silently resolving old tool names to their new canonical names.

This allows us to rename tools safely, without introducing breaking changes for users that have a hard reference to those tools in their server configuration.

## Quick Steps

1. **Rename the tool** in your code (as usual, this will imply a range of changes like updating the tool registration, the tests and the toolsnaps).
2. **Add a deprecation alias** in [pkg/github/deprecated_tool_aliases.go](../pkg/github/deprecated_tool_aliases.go):
   ```go
   var DeprecatedToolAliases = map[string]string{
       "old_tool_name": "new_tool_name",
   }
   ```
3. **Update documentation** (README, etc.) to reference the new canonical name

That's it. The server will silently resolve old names to new ones. This will work across both local and remote MCP servers.

## Example

If renaming `get_issue` to `issue_read`:

```go
var DeprecatedToolAliases = map[string]string{
    "get_issue": "issue_read",
}
```

A user with this configuration:
```json
{
  "--tools": "get_issue,get_file_contents"
}
```

Will get `issue_read` and `get_file_contents` tools registered, with no errors.

## Current Deprecations

<!-- START AUTOMATED ALIASES -->
| Old Name | New Name |
|----------|----------|
| `cancel_workflow_run` | `actions_run_trigger` |
| `delete_workflow_run_logs` | `actions_run_trigger` |
| `download_workflow_run_artifact` | `actions_get` |
| `get_workflow` | `actions_get` |
| `get_workflow_job` | `actions_get` |
| `get_workflow_job_logs` | `actions_get` |
| `get_workflow_run` | `actions_get` |
| `get_workflow_run_logs` | `actions_get` |
| `get_workflow_run_usage` | `actions_get` |
| `list_workflow_jobs` | `actions_list` |
| `list_workflow_run_artifacts` | `actions_list` |
| `list_workflow_runs` | `actions_list` |
| `list_workflows` | `actions_list` |
| `rerun_failed_jobs` | `actions_run_trigger` |
| `rerun_workflow_run` | `actions_run_trigger` |
| `run_workflow` | `actions_run_trigger` |
<!-- END AUTOMATED ALIASES -->
