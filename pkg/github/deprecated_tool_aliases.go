// deprecated_工具_aliases.go
package github

// DeprecatedToolAliases maps old 工具 names to their 新的 canonical names.
// When 工具 are renamed, add 一个entry here to maintain backward compatibility.
// Users referencing old name will receive 新的 工具 with 一个deprecation warning.
//
// Example:
//
//	"获取_议题": "议题_读取",
//	"创建_pr": "pull_请求_创建",
var DeprecatedToolAliases = map[string]string{
	// Add entries as 工具 are renamed
	// Actions 工具 consolidated
	"list_workflows":                 "actions_list",
	"list_workflow_runs":             "actions_list",
	"list_workflow_jobs":             "actions_list",
	"list_workflow_run_artifacts":    "actions_list",
	"get_workflow":                   "actions_get",
	"get_workflow_run":               "actions_get",
	"get_workflow_job":               "actions_get",
	"get_workflow_run_usage":         "actions_get",
	"get_workflow_run_logs":          "actions_get",
	"get_workflow_job_logs":          "actions_get",
	"download_workflow_run_artifact": "actions_get",
	"run_workflow":                   "actions_run_trigger",
	"rerun_workflow_run":             "actions_run_trigger",
	"rerun_failed_jobs":              "actions_run_trigger",
	"cancel_workflow_run":            "actions_run_trigger",
	"delete_workflow_run_logs":       "actions_run_trigger",

	// Projects 工具 consolidated
	"list_projects":       "projects_list",
	"list_project_fields": "projects_list",
	"list_project_items":  "projects_list",
	"get_project":         "projects_get",
	"get_project_field":   "projects_get",
	"get_project_item":    "projects_get",
	"add_project_item":    "projects_write",
	"update_project_item": "projects_write",
	"delete_project_item": "projects_write",
}
