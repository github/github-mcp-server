package catalog

// Tool associations are server-side availability rules, not skill frontmatter
// or permission declarations. Each required group accepts any one alternative.
type toolRequirements struct {
	required [][]string
	optional []string
}

var requirements = map[string]toolRequirements{
	"account-and-team-context": {
		required: [][]string{{"get_me"}},
		optional: []string{"get_teams", "get_team_members", "list_repository_collaborators", "search_users"},
	},
	"codebase-exploration": {
		required: [][]string{{"get_file_contents"}, {"search_code"}},
		optional: []string{"get_file_blame", "get_repository_tree", "list_branches", "list_tags", "list_commits", "get_commit", "search_commits", "get_tag"},
	},
	"copilot-task-delegation": {
		required: [][]string{{"issue_read"}, {"assign_copilot_to_issue", "assign_copilot_to_issue_with_intent"}},
		optional: []string{"pull_request_read", "request_copilot_review"},
	},
	"discussion-participation": {
		required: [][]string{{"get_discussion"}, {"discussion_comment_write"}},
		optional: []string{"list_discussion_categories", "list_discussions", "get_discussion_comments"},
	},
	"issue-authoring": {
		required: [][]string{{"search_issues"}, {"issue_write", "create_issue"}},
		optional: []string{"issue_read", "list_issue_types", "list_issue_fields", "list_label", "sub_issue_write", "add_sub_issue", "remove_sub_issue", "set_issue_fields", "update_issue_assignees", "update_issue_body", "update_issue_labels", "update_issue_milestone", "update_issue_title", "update_issue_type"},
	},
	"issue-backlog-triage": {
		required: [][]string{{"list_issues"}, {"issue_write", "update_issue_state"}},
		optional: []string{"search_issues", "issue_read", "sub_issue_write", "add_issue_comment", "list_label", "list_issue_types", "find_duplicate", "add_sub_issue", "remove_sub_issue", "reprioritize_sub_issue", "set_issue_fields", "update_issue_assignees", "update_issue_body", "update_issue_labels", "update_issue_milestone", "update_issue_title", "update_issue_type"},
	},
	"label-scheme-management": {
		required: [][]string{{"list_label"}, {"label_write"}},
		optional: []string{"get_label", "list_issues"},
	},
	"notification-triage": {
		required: [][]string{{"list_notifications"}, {"get_notification_details"}, {"dismiss_notification"}},
		optional: []string{"mark_all_notifications_read", "manage_notification_subscription", "manage_repository_notification_subscription"},
	},
	"project-planning": {
		required: [][]string{{"projects_list"}, {"projects_get"}, {"projects_write"}},
	},
	"pull-request-authoring": {
		required: [][]string{{"create_pull_request"}},
		optional: []string{"list_branches", "list_commits", "update_pull_request", "pull_request_read", "update_pull_request_branch", "actions_list", "merge_pull_request", "fork_repository", "get_file_contents", "request_pull_request_reviewers", "update_pull_request_body", "update_pull_request_draft_state", "update_pull_request_state", "update_pull_request_title"},
	},
	"pull-request-code-review": {
		required: [][]string{{"pull_request_read"}, {"pull_request_review_write", "create_pull_request_review"}, {"pull_request_review_write", "submit_pending_pull_request_review"}, {"add_comment_to_pending_review", "add_pull_request_review_comment"}},
		optional: []string{"get_file_contents", "get_code_quality_finding", "delete_pending_pull_request_review", "resolve_review_thread", "unresolve_review_thread"},
	},
	"pull-request-review-feedback": {
		required: [][]string{{"pull_request_read"}, {"add_reply_to_pull_request_comment"}},
		optional: []string{"add_issue_comment", "pull_request_review_write", "update_pull_request", "get_file_contents"},
	},
	"release-preparation": {
		required: [][]string{{"list_tags"}, {"list_commits"}},
		optional: []string{"get_latest_release", "get_release_by_tag", "list_releases", "list_pull_requests", "search_pull_requests", "actions_list", "actions_get"},
	},
	"repository-content-changes": {
		required: [][]string{{"get_file_contents"}, {"push_files", "create_or_update_file"}},
		optional: []string{"list_branches", "create_branch", "delete_file", "create_repository", "fork_repository", "delete_repository"},
	},
	"repository-discovery": {
		required: [][]string{{"search_repositories"}},
		optional: []string{"search_orgs", "list_starred_repositories", "star_repository", "unstar_repository", "get_file_contents"},
	},
	"repository-health-review": {
		required: [][]string{{"list_commits"}, {"list_issues"}, {"list_pull_requests"}},
		optional: []string{"list_branches", "list_releases", "list_repository_collaborators", "actions_list", "list_code_scanning_alerts", "list_dependabot_alerts"},
	},
	"security-alert-triage": {
		required: [][]string{{"list_code_scanning_alerts", "list_dependabot_alerts", "list_secret_scanning_alerts", "list_repository_security_advisories", "list_org_repository_security_advisories"}},
		optional: []string{"get_code_scanning_alert", "get_dependabot_alert", "get_secret_scanning_alert", "list_global_security_advisories", "get_global_security_advisory", "get_file_contents", "search_code"},
	},
	"workflow-failure-triage": {
		required: [][]string{{"actions_list"}, {"actions_get"}, {"get_job_logs"}},
		optional: []string{"actions_run_trigger"},
	},
}
