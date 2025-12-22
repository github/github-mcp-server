// deprecated_tool_aliases.go
package github

// DeprecatedToolAliases maps old tool names to their new canonical names.
// When tools are renamed, add an entry here to maintain backward compatibility.
// Users referencing the old name will receive the new tool with a deprecation warning.
//
// Example:
//
//	"get_issue": "issue_read",
//	"create_pr": "pull_request_create",
var DeprecatedToolAliases = map[string]string{
	// Add entries as tools are renamed
	// Actions tools consolidated
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
}
